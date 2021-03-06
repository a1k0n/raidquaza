package gymdb

import (
	"github.com/manveru/closestmatch"
	"os"
	"log"
	"bufio"
	"encoding/json"
	"strings"
	"fmt"
	"encoding/hex"
	"math/rand"
	"encoding/binary"
	"io"
	"sort"
	"errors"
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

type GymDB struct {
	Gyms     map[string]*Gym // map of gym full name -> gym itself
	Matcher  *closestmatch.ClosestMatch
	Filename string
}

// we need to fix the apostrophes in the names and in the queries so that
// fancy apostrophes don't prevent matches
func canonicalizeName(q string) string {
	q = strings.Replace(q, "'", "’", -1)
	return q
}

func canonicalizeQuery(q string) string {
	return strings.ToLower(canonicalizeName(q))
}

func genId() string {
	var hexBytes [8]byte
	var valueBytes [4]byte
	binary.LittleEndian.PutUint32(valueBytes[:], uint32(rand.Int63()&0xffffffff))
	hex.Encode(hexBytes[:], valueBytes[:])
	return string(hexBytes[:])
}

func (g *Gym) String() string {
	return fmt.Sprintf("[gym %s] (%0.7f,%0.7f) %s | %s",
		g.Id, g.Latitude, g.Longitude, g.Name, g.StreetAddr)
}

func (g *Gym) SearchKey() string {
	return g.Id + " " + strings.ToLower(g.Name) + " " + strings.ToLower(g.StreetAddr)
}

func NewGymDB(gymfile string) *GymDB {
	db := &GymDB{
		Gyms:     make(map[string]*Gym),
		Filename: gymfile,
	}
	f, err := os.Open(gymfile)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	err = db.LoadGyms(f)
	if err != nil {
		log.Fatal(err)
	}
	return db
}

func (g *GymDB) LoadGyms(r io.Reader) error {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		gym := Gym{}
		err := json.Unmarshal(scanner.Bytes(), &gym)
		if err != nil {
			return err
		}
		gym.Name = canonicalizeName(gym.Name)
		gym.StreetAddr = strings.Join(strings.Split(gym.StreetAddr, ",")[:2], ",")
		// searchable index w/ ids, names, and street addresses
		key := gym.SearchKey()
		g.Gyms[key] = &gym
	}
	g.UpdateSearchDB()

	return nil
}

func (g *GymDB) SaveGyms(w io.Writer) error {
	sortedGyms := make([]*Gym, 0, len(g.Gyms))
	for _, gym := range g.Gyms {
		sortedGyms = append(sortedGyms, gym)
	}
	sort.Slice(sortedGyms, func(i, j int) bool {
		if sortedGyms[i].Name == sortedGyms[j].Name {
			return sortedGyms[i].Id < sortedGyms[j].Id
		}
		return sortedGyms[i].Name < sortedGyms[j].Name
	})
	for _, gym := range sortedGyms {
		data, err := json.Marshal(&gym)
		if err != nil {
			return err
		}
		w.Write(data)
		w.Write([]byte("\n"))
	}
	return nil
}

func (g *GymDB) UpdateDiskDB() error {
	tmpName := g.Filename + ".tmp"
	f, err := os.Create(tmpName)
	if err != nil {
		return err
	}
	err = g.SaveGyms(f)
	if err != nil {
		return err
	}
	f.Close()

	os.Remove(g.Filename)
	err = os.Rename(tmpName, g.Filename)
	if err != nil {
		return err
	}
	return nil
}

func (g *GymDB) UpdateSearchDB() {
	gymKeys := make(map[string]interface{}, len(g.Gyms))
	for k, v := range g.Gyms {
		gymKeys[k] = v
	}
	g.Matcher = closestmatch.New(gymKeys, []int{2, 3, 4})
}

func (g *GymDB) GetGyms(query string, threshold float32) ([]*Gym, []float32) {
	matches := g.Matcher.ClosestN(canonicalizeQuery(query), 10)
	var closest []*Gym
	var normScores []float32
	var normSum float32
	log.Printf("query \"%s\" matches:", query)
	for _, m := range matches {
		if float32(m.Score) < float32(matches[0].Score)*threshold {
			break
		}
		closest = append(closest, m.Data.(*Gym))
		normScores = append(normScores, float32(m.Score))
		normSum += float32(m.Score)
		log.Printf("  %s (%d)\n", m.Data.(*Gym).String(), m.Score)
	}
	for i := range normScores {
		normScores[i] /= normSum
	}
	return closest, normScores
}

func (g *GymDB) AddGym(lat, lon float64, name string) (*Gym, error) {
	streetAddr, err := GetStreetAddress(lat, lon)
	if err != nil {
		return nil, err
	}
	gym := &Gym{
		Id:         genId(),
		Latitude:   lat,
		Longitude:  lon,
		StreetAddr: strings.Join(strings.Split(streetAddr, ",")[:2], ","),
		Name:       canonicalizeName(name),
		Enabled:    true,
	}
	key := gym.SearchKey()
	g.Gyms[key] = gym

	g.UpdateSearchDB()
	err = g.UpdateDiskDB()
	if err != nil {
		return gym, err
	}

	return gym, nil
}

func (g *GymDB) RemoveGym(gym *Gym) error {
	key := gym.SearchKey()
	_, ok := g.Gyms[key]
	if !ok {
		return errors.New("can't find gym in DB")
	}
	delete(g.Gyms, key)
	g.UpdateSearchDB()
	err := g.UpdateDiskDB()
	if err != nil {
		return err
	}
	return nil
}

func (g *GymDB) RenameGym(gym *Gym, name string) error {
	oldKey := gym.SearchKey()
	_, ok := g.Gyms[oldKey]
	if !ok {
		return errors.New("can't find gym in DB")
	}
	delete(g.Gyms, oldKey)
	gym.Name = canonicalizeName(name)
	key := gym.SearchKey()
	g.Gyms[key] = gym
	g.UpdateSearchDB()
	err := g.UpdateDiskDB()
	if err != nil {
		return err
	}
	return nil
}

func (g *GymDB) MoveGym(gym *Gym, lat, lon float64) error {
	newStreetAddr, err := GetStreetAddress(lat, lon)
	if err != nil {
		return err
	}

	oldKey := gym.SearchKey()
	_, ok := g.Gyms[oldKey]
	if !ok {
		return errors.New("can't find gym in DB")
	}
	delete(g.Gyms, oldKey)
	gym.Latitude = lat
	gym.Longitude = lon
	gym.StreetAddr = strings.Join(strings.Split(
		newStreetAddr, ",")[:2], ",")

	key := gym.SearchKey()
	g.Gyms[key] = gym
	g.UpdateSearchDB()
	err = g.UpdateDiskDB()
	if err != nil {
		return err
	}
	return nil
}
