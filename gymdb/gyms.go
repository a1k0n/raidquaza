package gymdb

import (
	"github.com/schollz/closestmatch"
	"os"
	"log"
	"bufio"
	"encoding/json"
	"strings"
	"fmt"
)

type Gym struct {
	Id         string  `json:"gym_id"`
	Name       string  `json:"gym_name"`
	Latitude   float64 `json:"latitude"`
	Longitude  float64 `json:"longitude"`
	ImageUrl   string  `json:"url"`
	StreetAddr string  `json:"street_addr"`
	Enabled    bool    `json:"enabled"`
}

// we need to fix the apostrophes in the names and in the queries so that
// fancy apostrophes don't prevent matches
func canonicalizeName(q string) string {
	q = strings.Replace(q, "'", "’", -1)
	return q
}

func (g *Gym) String() string {
	return fmt.Sprintf("[gym %s] (%0.7f,%0.7f) %s | %s",
		g.Id, g.Latitude, g.Longitude, g.Name, g.StreetAddr)
}

type GymDB struct {
	Gyms    map[string]*Gym // map of gym full name -> gym itself
	Matcher *closestmatch.ClosestMatch
}

func NewGymDB(gymfile string) *GymDB {
	db := &GymDB{
		Gyms: make(map[string]*Gym),
	}
	f, err := os.Open(gymfile)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	var gymKeys []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		gym := Gym{}
		err := json.Unmarshal(scanner.Bytes(), &gym)
		if err != nil {
			log.Fatal(err)
		}
		gym.Name = canonicalizeName(gym.Name)
		gym.StreetAddr = strings.Join(strings.Split(gym.StreetAddr, ",")[:2], ",")
		// searchable index w/ ids, names, and street addresses
		key := gym.Id + " " + gym.Name + " " + gym.StreetAddr
		db.Gyms[key] = &gym
		gymKeys = append(gymKeys, key)
	}
	db.Matcher = closestmatch.New(gymKeys, []int{2, 3, 4})

	return db
}

func (g *GymDB) GetGyms(query string, threshold float32) ([]*Gym, []float32) {
	closestN, scores := g.Matcher.ClosestN(canonicalizeName(query), 10)
	var closest []*Gym
	var normScores []float32
	var normSum float32
	log.Printf("query \"%s\" matches:", query)
	for i, m := range closestN {
		if float32(scores[i]) < float32(scores[0])*threshold {
			break
		}
		closest = append(closest, g.Gyms[m])
		normScores = append(normScores, float32(scores[i]))
		normSum += float32(scores[i])
		log.Printf("  %s (%d)\n", m, scores[i])
	}
	for i := range normScores {
		normScores[i] /= normSum
	}
	return closest, normScores
}
