package raid

import (
	"strings"
	"raidquaza/gymdb"
	"errors"
	"time"
	"fmt"
)

const RaidDuration = 45 * time.Minute // time raid lasts after hatching

var (
	ErrNoEnd     = errors.New("no end time specified")
	ErrNoMatches = errors.New("no matches for gym query")
	ErrNonUnique = errors.New("too many gyms match query")
)

func fuzzyTime(query string, beginTime time.Time) (time.Time, error) {
	// interpret query time as the latest time before endTime
	var t time.Time
	var err error
	query = strings.ToUpper(query)
	lastChar := query[len(query)-1]
	if lastChar == 'A' || lastChar == 'P' {
		query = query + "M"
		lastChar = 'M'
	}
	if lastChar == 'M' {
		t, err = time.Parse("3:04PM", query)
		if err != nil {
			t, err = time.Parse("3:04 PM", query)
			if err != nil {
				return time.Time{}, err
			}
		}
	} else {
		t, err = time.Parse("15:04", query)
		if err != nil {
			return time.Time{}, err
		}
	}

	yy, mm, dd := beginTime.Date()
	h, m, _ := t.Clock()
	bh, _, _ := beginTime.Clock()
	if h < bh && h < 12 { // fix PM if not stated
		h += 12
	}
	result := time.Date(yy, mm, dd, h, m, 0, 0, beginTime.Location())
	return result, nil
}

func parseTimeSpec(spec []string, timebase time.Time) (time.Time, error) {
	if len(spec) == 0 {
		return time.Time{}, errors.New("empty time")
	}
	if spec[0] == "at" {
		return fuzzyTime(strings.Join(spec[1:], " "), timebase)
	} else if spec[0] == "in" {
		dur, err := time.ParseDuration(strings.Join(spec[1:], ""))
		if err != nil {
			return time.Time{}, err
		}
		return timebase.Add(dur), nil
	} else {
		// guess which one they mean...
		glom := strings.Join(spec, "")
		if strings.Contains(glom, ":") {
			return fuzzyTime(glom, timebase)
		} else if glom[len(glom)-1] == 'm' {
			dur, err := time.ParseDuration(glom)
			if err != nil {
				return time.Time{}, err
			}
			return timebase.Add(dur), nil
		} else {
			return time.Time{}, fmt.Errorf("couldn't understand time %s", glom)
		}
	}
}

// returns nil, nil on success; if there are multiple matching gyms, returns array of them
func (r *Raid) ParseRaidRequest(req string, gdb *gymdb.GymDB, timebase time.Time) (error, []*gymdb.Gym) {
	// raid request format:
	// <pokemon> (@)? <gym query> (ends|hatches) <time|duration> (starts <time|duration>)?
	reqSplit := strings.Fields(req)

	// search backward to find "starts" or "ends" or "hatches"
	var endsSpec []string
	var startSpec []string
	var gymQuery []string
	pokemon := reqSplit[0:1]
	fieldEnd := len(reqSplit)
	isHatches := false
	// scan backwards to find various markers
	for n := len(reqSplit) - 1; n >= 0; n-- {
		switch reqSplit[n] {
		case "end", "ends":
			endsSpec = reqSplit[n+1 : fieldEnd]
			fieldEnd = n
		case "hatch", "hatches":
			endsSpec = reqSplit[n+1 : fieldEnd]
			fieldEnd = n
			isHatches = true
		case "start", "starts":
			startSpec = reqSplit[n+1 : fieldEnd]
			fieldEnd = n
		case "@":
			gymQuery = reqSplit[n+1 : fieldEnd]
			pokemon = reqSplit[:n]
			fieldEnd = n
		}
	}
	if gymQuery == nil {
		gymQuery = reqSplit[1:fieldEnd]
	}

	if endsSpec == nil {
		return ErrNoEnd, nil
	}

	endTime, err := parseTimeSpec(endsSpec, timebase)
	if err != nil {
		return err, nil
	}
	matches, _ := gdb.GetGyms(strings.Join(gymQuery, " "), 1.0)
	if len(matches) == 0 {
		return ErrNoMatches, nil
	}
	if len(matches) != 1 {
		return ErrNonUnique, matches
	}

	if isHatches {
		r.EndTime = endTime.Add(RaidDuration)
	} else {
		r.EndTime = endTime
	}
	r.Hatched = endTime.Add(-RaidDuration).After(timebase)
	r.Gym = matches[0]
	r.What = strings.Join(pokemon, " ")

	if startSpec != nil {
		startTime, err := parseTimeSpec(startSpec, timebase)
		if err != nil {
			return err, nil
		}
		if len(r.Groups) == 0 {
			r.Groups = append(r.Groups, &Group{
				raid:      r,
				number:    1,
				StartTime: startTime,
				Members:   make(map[string]int),
			})
		} else { // if we're editing, just update the start time
			r.Groups[0].StartTime = startTime
		}
	}

	return nil, nil
}
