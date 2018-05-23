package gymdb

import (
	"github.com/schollz/closestmatch"
	"os"
	"log"
	"bufio"
	"encoding/json"
)

type Gym struct {
	Name      string  `json:"gym_name"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type GymDB struct {
	Gyms    map[string]*Gym  // map of gym full name -> gym itself
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
		gymNames = append(gymNames, gym.Name)
	}
	db.Matcher = closestmatch.New(gymNames, []int{2, 3, 4})

	return db
}

func (g *GymDB) GetGym(query string) *Gym {
	closestN := g.Matcher.ClosestN(query, 10)
	closest := closestN[0]
	gym, ok := g.Gyms[closest]
	if ok {
		log.Printf("query \"%s\" matches:", query)
		for _, m := range closestN {
			log.Printf("  %s\n", m)
		}
	}
	return gym
}