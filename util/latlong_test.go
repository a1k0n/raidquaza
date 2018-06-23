package util

import "testing"

func testStr(t *testing.T, str []string) {
	lat, lon, _, err := ParseLatLong(str)
	if err != nil {
		t.Log(err)
		t.Fail()
	}
	if lat != 37.649183 || lon != -121.896766 {
		t.Log("lat/lon mismatch")
		t.Fail()
	}
	t.Log(str, lat, lon)
}

func TestParseLatLong(t *testing.T) {
	testStr(t, []string{"37.649183,", "-121.896766", "abc"})
	testStr(t, []string{"37.649183", "-121.896766", "abc"})
	testStr(t, []string{"37.649183,-121.896766", "foo"})

	_, _, _, err := ParseLatLong([]string{"37.649183,"})
	t.Log(err)
	if err == nil {
		t.Fail()
	}
	_, _, _, err = ParseLatLong([]string{})
	t.Log(err)
	if err == nil {
		t.Fail()
	}
	_, _, _, err = ParseLatLong([]string{"asdf"})
	t.Log(err)
	if err == nil {
		t.Fail()
	}
}
