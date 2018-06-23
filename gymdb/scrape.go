package gymdb

import (
	"time"
	"fmt"
	"net/http"
	"encoding/json"
	"log"
)

// scrape data from gymhuntr. i'm sure they'll be really pleased.

func ScrapeGymhuntr(lat, long float64) ([]*Gym, error) {
	now := time.Now().Unix()
	scale := 284371927. // assuming this is hardcoded, but it comes in as a response header
	thing := (lat+long)*scale + float64(now)
	url := fmt.Sprintf("https://api.gymhuntr.com/api/gyms?latitude=%.7f&longitude=%.7f&hashCheck=57b34b3eca72eed3178b785dcca4289g4&monster=83jhs&timeUntil=%.5f&time=%d",
		lat, long, thing, now)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(resp.Body)
	var data struct {
		Gyms []string `json:"gyms"`
	}
	err = dec.Decode(&data)
	if err != nil {
		return nil, err
	}
	var result []*Gym
	for _, gymdata := range data.Gyms {
		g := &Gym{}
		err = json.Unmarshal([]byte(gymdata), g)
		if err != nil {
			return result, err
		}
		g.Id = g.Id[:8]
		// these are backwards in gymhuntr; fix
		g.Longitude, g.Latitude = g.Latitude, g.Longitude
		log.Printf("gymhunter scrape(%0.7f %0.7f): %s\n", lat, long, g.String())
		result = append(result, g)
	}
	return result, nil
}
