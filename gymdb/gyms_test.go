package gymdb

import (
	"testing"
	"bytes"
)

func TestNewGymDB(t *testing.T) {
	g := NewGymDB("gyms.txt")
	gyms, scores := g.GetGyms("valley trails", 0.9)
	t.Log(gyms)
	t.Log(scores)
	/*
	t.Log(g.GetGyms("frogboy", 0.9)[0])
	t.Log(g.GetGyms("denker", 0.9)[0])
	t.Log(g.GetGyms("val vista denker", 0.9)[0])
	t.Log(g.GetGyms("fairlands park", 0.9)[0])
	t.Log(g.GetGyms("sprint pls", 0.9)[0])
	t.Log(g.GetGyms("sprint dublin", 0.9)[0])
	*/
}

func TestScanGym(t *testing.T) {
	g := NewGymDB("gyms.txt")
	minlat, maxlat := 180.0, -180.0
	minlong, maxlong := 180.0, -180.0
	for _, gym := range g.Gyms {
		if gym.Latitude < minlat {
			minlat = gym.Latitude
		}
		if gym.Longitude < minlong {
			minlong = gym.Longitude
		}
		if gym.Latitude > maxlat {
			maxlat = gym.Latitude
		}
		if gym.Longitude > maxlong {
			maxlong = gym.Longitude
		}
	}
	t.Logf("(%f,%f) -> (%f,%f) %fx%f", minlat, minlong, maxlat, maxlong, maxlat-minlat, maxlong-minlong)
	t.Logf("center (%f,%f)", (minlat+maxlat)/2, (minlong+maxlong)/2)
}

func TestEmoji(t *testing.T) {
	x := "1⃣"
	b := []byte(x)
	t.Log([]byte(x))
	b[0] = 'A'
	t.Log(string(b))
	t.Log(string(b[1:]))
}

func TestGymDB_SaveGyms(t *testing.T) {
	g := NewGymDB("gyms.txt")
	t.Log(len(g.Gyms))
	buf := bytes.NewBuffer(make([]byte, 0))
	g.SaveGyms(buf)
	roundTrip1 := buf.String()

	g = &GymDB{Gyms: make(map[string]*Gym)}
	g.LoadGyms(buf)
	t.Log(len(g.Gyms))

	buf = bytes.NewBuffer(make([]byte, 0))
	g.SaveGyms(buf)
	roundTrip2 := buf.String()
	if roundTrip1 != roundTrip2 {
		t.Fail()
	}
}