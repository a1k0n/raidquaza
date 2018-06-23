package util

import (
	"strconv"
	"strings"
	"errors"
)

func ParseLatLong(latlon []string) (float64, float64, int, error) {
	if len(latlon) < 1 {
		return 0, 0, 0, errors.New("empty string")
	}
	var q []string
	nConsumed := 1
	// yes, this only works in the western hemisphere
	if len(latlon) >= 2 && latlon[1][0] == '-' {
		// handle lat, lon case
		q = latlon[:2]
		if q[0][len(q[0])-1] == ',' {
			q[0] = q[0][:len(q[0])-1] // trim comma
		}
		nConsumed = 2
	} else {
		// handle lat,lon case
		q = strings.Split(latlon[0], ",")
	}
	if len(q) < 2 {
		return 0, 0, 0, errors.New("missing comma")
	}

	lat, err := strconv.ParseFloat(strings.TrimSpace(q[0]), 64)
	if err != nil {
		return 0, 0, 0, err
	}
	lon, err := strconv.ParseFloat(strings.TrimSpace(q[1]), 64)
	if err != nil {
		return 0, 0, 0, err
	}
	return lat, lon, nConsumed, nil
}
