package gymdb

import "testing"

func TestGetStreetAddress(t *testing.T) {
	addr, err := GetStreetAddress(37.655719, -121.895759)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(addr)
}
