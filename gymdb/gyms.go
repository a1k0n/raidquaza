package gymdb

import (
	"github.com/schollz/closestmatch"
	"os"
	"log"
	"bufio"
	"encoding/json"
)

type Gym struct {
	Id        string  `json:"id"`
	Name      string  `json:"gym_name"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	ImageUrl  string  `json:"url"`
	Enabled   bool    `json:"enabled"`
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

	var gymNames []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		gym := Gym{}
		err := json.Unmarshal(scanner.Bytes(), &gym)
		if err != nil {
			log.Fatal(err)
		}
		db.Gyms[gym.Name] = &gym
		db.Gyms[gym.Id] = &gym
		gymNames = append(gymNames, gym.Id)
		gymNames = append(gymNames, gym.Name)
	}
	db.Matcher = closestmatch.New(gymNames, []int{2, 3, 4})

	return db
}

func (g *GymDB) GetGyms(query string, threshold float32) []*Gym {
	closestN, scores := g.Matcher.ClosestN(query, 10)
	var closest []*Gym
	log.Printf("query \"%s\" matches:", query)
	for i, m := range closestN {
		if float32(scores[i]) < float32(scores[0]) * threshold {
			break
		}
		closest = append(closest, g.Gyms[m])
		log.Printf("  %s (%d)\n", m, scores[i])
	}
	return closest
}
