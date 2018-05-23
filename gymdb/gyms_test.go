package gymdb

import (
	"testing"
)

func TestNewGymDB(t *testing.T) {
	g := NewGymDB("gyms.txt")
	t.Log(g.GetGym("fawn hills"))
	t.Log(g.GetGym("frogboy"))
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
