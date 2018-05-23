package main

import (
	"testing"
	"time"
	"encoding/json"
)

func TestFuzzyTime(t *testing.T) {

	refTime, _ := time.Parse(time.RFC3339, "2018-05-22T20:24:48-07:00")
	refTime2, _ := time.Parse(time.RFC3339, "2018-05-22T02:24:48-07:00")

	t.Log(fuzzyTime("4:00 pm", refTime))
	t.Log(fuzzyTime("4:00pm", refTime))
	t.Log(fuzzyTime("4:30", refTime))
	t.Log(fuzzyTime("3:30", refTime))

	t.Log(fuzzyTime("4:00 pm", refTime2))
	t.Log(fuzzyTime("4:00pm", refTime2))
	t.Log(fuzzyTime("4:30", refTime2))
	t.Log(fuzzyTime("3:30", refTime2))
}

type Blah struct {
	Foo string `json:"foo"`
	Bar string
	Baz string `json:"-"`
	maz string
}

func TestJSON(t *testing.T) {
	b := Blah{"foo", "bar", "baz", "maz"}
	j, err := json.Marshal(&b)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(string(j))
}
