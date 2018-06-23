package gymdb

import (
	"fmt"
	"net/http"
	"encoding/json"
	"errors"
	"raidquaza/util"
)

func GetStreetAddress(lat, long float64) (string, error) {
	apiKey := util.LoadAuthToken("mapstoken.txt")
	uri := fmt.Sprintf("https://maps.googleapis.com/maps/api/geocode/json?latlng=%f,%f&key=%s",
		lat, long, apiKey)
	resp, err := http.Get(uri)
	if err != nil {
		return "", err
	}
	var data struct {
		Results []struct {
			StreetAddr string `json:"formatted_address"`
		} `json:"results"`
	}
	dec := json.NewDecoder(resp.Body)
	err = dec.Decode(&data)
	if err != nil {
		return "", err
	}
	if len(data.Results) > 0 {
		return data.Results[0].StreetAddr, nil
	}
	return "", errors.New("no address found")
}
