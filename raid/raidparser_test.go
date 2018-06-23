package raid

import (
	"testing"
	"raidquaza/gymdb"
	"time"
)

func TestFuzzyTime(t *testing.T) {

	refTime, _ := time.Parse(time.RFC3339, "2018-05-22T20:24:48-07:00")
	refTime2, _ := time.Parse(time.RFC3339, "2018-05-22T02:24:48-07:00")

	t.Log(fuzzyTime("4:00 pm", refTime))
	t.Log(fuzzyTime("4:00 p", refTime))
	t.Log(fuzzyTime("4:00pm", refTime))
	t.Log(fuzzyTime("4:00p", refTime))
	t.Log(fuzzyTime("4:30", refTime))
	t.Log(fuzzyTime("3:30", refTime))

	t.Log(fuzzyTime("4:00 pm", refTime2))
	t.Log(fuzzyTime("4:00pm", refTime2))
	t.Log(fuzzyTime("4:30", refTime2))
	t.Log(fuzzyTime("3:30", refTime2))
}

func TestRaid_ParseRaidRequest(t *testing.T) {
	gdb := gymdb.NewGymDB("../gymdb/gyms.txt")
	t0, err := time.Parse(time.RFC3339, "2018-05-28T15:27:30-07:00")
	if err != nil {
		t.Fatal(err)
	}
	r := &Raid{}

	err, matches := r.ParseRaidRequest("ho-oh denker ends 3:45 starts 3:30", gdb, t0)
	if err != nil {
		t.Log(matches)
		t.Fatal(err)
	}
	t.Log(r.String())
	t.Log(r.Groups[0].String())

	r = &Raid{}
	err, matches = r.ParseRaidRequest("ho-oh denker starts 3:30 ends 3:45", gdb, t0)
	if err != nil {
		t.Log(matches)
		t.Fatal(err)
	}
	t.Log(r.String())
	t.Log(r.Groups[0].String())

	r = &Raid{}
	err, matches = r.ParseRaidRequest("ho-oh denker ends 3:45", gdb, t0)
	if err != nil {
		t.Log(matches)
		t.Fatal(err)
	}
	t.Log(r.String())
	t.Log(len(r.Groups))

	r = &Raid{}
	err, matches = r.ParseRaidRequest("stupid thing @ denker ends 3:45", gdb, t0)
	if err != nil {
		t.Log(matches)
		t.Fatal(err)
	}
	t.Log(r.String())

	r = &Raid{}
	err, matches = r.ParseRaidRequest("stupid thing @ denker hatches 2:50", gdb, t0)
	if err != nil {
		t.Log(matches)
		t.Fatal(err)
	}
	t.Log(r.String())
}
