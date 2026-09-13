package geo

import (
	"bytes"
	"compress/gzip"
	"strings"
	"testing"
	"time"

	"github.com/icco/where/internal/provider"
)

func TestGeography(t *testing.T) {
	r, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if len(r.cities) < 25000 {
		t.Fatalf("only %d cities", len(r.cities))
	}
	for _, test := range []struct {
		point      provider.Coordinates
		city, zone string
	}{
		{provider.Coordinates{Latitude: 51.5074, Longitude: -0.1278}, "London", "Europe/London"},
		{provider.Coordinates{Latitude: 35.6762, Longitude: 139.6503}, "Eifuku", "Asia/Tokyo"},
		{provider.Coordinates{Latitude: 27.7172, Longitude: 85.324}, "Kathmandu", "Asia/Kathmandu"},
		{provider.Coordinates{Latitude: -33.8688, Longitude: 151.2093}, "Sydney", "Australia/Sydney"},
	} {
		place, err := r.Nearest(test.point)
		if err != nil || place.City != test.city || place.TimeZone != test.zone {
			t.Errorf("%+v: %+v %v", test.point, place, err)
		}
	}
	if _, err := r.Nearest(provider.Coordinates{Latitude: 91}); err == nil {
		t.Fatal("invalid coordinates accepted")
	}
	for _, test := range []struct{ label, zone string }{
		{"London, England", "Europe/London"}, {"New York, NY", "America/New_York"}, {"Tokyo, Japan", "Asia/Tokyo"},
	} {
		place, err := r.Named(test.label)
		if err != nil || place.TimeZone != test.zone {
			t.Errorf("%s: %+v %v", test.label, place, err)
		}
	}
	for _, label := range []string{"Springfield", "Home", "London, unknown region", "", "No location found"} {
		if p, err := r.Named(label); err == nil {
			t.Errorf("ambiguous %q resolved to %+v", label, p)
		}
	}
}

func TestLocalTime(t *testing.T) {
	for _, test := range []struct{ instant, zone, want string }{
		{"2026-03-08T06:59:00Z", "America/New_York", "2026-03-08T01:59:00-05:00"},
		{"2026-03-08T07:00:00Z", "America/New_York", "2026-03-08T03:00:00-04:00"},
		{"2026-01-01T23:00:00Z", "Asia/Kathmandu", "2026-01-02T04:45:00+05:45"},
		{"2026-01-01T23:00:00Z", "Pacific/Kiritimati", "2026-01-02T13:00:00+14:00"},
	} {
		now, err := time.Parse(time.RFC3339, test.instant)
		if err != nil {
			t.Fatal(err)
		}
		got, err := LocalTime(now, test.zone)
		if err != nil || got != test.want {
			t.Errorf("%s %v; want %s", got, err, test.want)
		}
	}
	for _, zone := range []string{"", "invalid/zone"} {
		if _, err := LocalTime(time.Now(), zone); err == nil {
			t.Fatal("invalid zone accepted")
		}
	}
}

func TestLoadInvalid(t *testing.T) {
	if _, err := load([]byte("bad")); err == nil {
		t.Fatal("bad gzip accepted")
	}
	for _, input := range []string{"", "bad\n", "City,City,US,United States,CA,California,bad,0,UTC,names\n", "City,City,US,United States,CA,California,0,bad,UTC,names\n", "City,City,US,United States,CA,California,91,0,UTC,names\n"} {
		var b bytes.Buffer
		w := gzip.NewWriter(&b)
		if _, err := strings.NewReader(input).WriteTo(w); err != nil {
			t.Fatal(err)
		}
		if err := w.Close(); err != nil {
			t.Fatal(err)
		}
		if _, err := load(b.Bytes()); err == nil {
			t.Errorf("accepted %q", input)
		}
	}
}
