// Package geo resolves places locally using GeoNames and time-zone boundaries.
package geo

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata" // Keep DST rules available in minimal installations.

	"github.com/icco/where/internal/provider"
	tz "github.com/ugjka/go-tz/v2"
)

//go:generate go run generate.go
//go:embed cities.csv.gz
var cityData []byte

// Place is a resolved nearest city or unambiguous named city.
type Place struct {
	City     string `json:"city"`
	Region   string `json:"region,omitempty"`
	Country  string `json:"country"`
	TimeZone string `json:"time_zone"`
}

type city struct {
	Place
	point   provider.Coordinates
	names   []string
	context []string
}

// Resolver holds the bundled gazetteer.
type Resolver struct{ cities []city }

// New loads the bundled GeoNames cities15000 gazetteer.
func New() (*Resolver, error) { return load(cityData) }

func load(data []byte) (*Resolver, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	r := csv.NewReader(gz)
	r.FieldsPerRecord = 10
	result := &Resolver{}
	for {
		f, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		lat, err := strconv.ParseFloat(f[6], 64)
		if err != nil {
			return nil, err
		}
		lon, err := strconv.ParseFloat(f[7], 64)
		if err != nil {
			return nil, err
		}
		c := city{Place: Place{City: f[0], Region: f[5], Country: f[2], TimeZone: f[8]}, point: provider.Coordinates{Latitude: lat, Longitude: lon}}
		if !c.point.Valid() {
			return nil, errors.New("invalid city coordinates")
		}
		c.names = strings.Split(strings.ToLower(f[0]+","+f[1]+","+f[9]), ",")
		c.context = []string{strings.ToLower(f[2]), strings.ToLower(f[3]), strings.ToLower(f[4]), strings.ToLower(f[5])}
		result.cities = append(result.cities, c)
	}
	if len(result.cities) == 0 {
		return nil, errors.New("empty city database")
	}
	return result, nil
}

// Nearest returns the closest city center by great-circle distance. The time
// zone comes from the observation's coordinates, not the nearest city's zone.
func (r *Resolver) Nearest(point provider.Coordinates) (Place, error) {
	if !point.Valid() {
		return Place{}, errors.New("invalid coordinates")
	}
	best := math.Inf(1)
	var place Place
	for _, c := range r.cities {
		d := distance(point, c.point)
		if d < best {
			best, place = d, c.Place
		}
	}
	place.TimeZone = ""
	zones, err := tz.GetZone(tz.Point{Lat: point.Latitude, Lon: point.Longitude})
	if err != nil || len(zones) != 1 {
		return place, errors.New("coordinate time zone is unavailable or ambiguous")
	}
	place.TimeZone = zones[0]
	return place, nil
}

func distance(a, b provider.Coordinates) float64 {
	const rad = math.Pi / 180
	dlat, dlon := (b.Latitude-a.Latitude)*rad, (b.Longitude-a.Longitude)*rad
	return math.Pow(math.Sin(dlat/2), 2) + math.Cos(a.Latitude*rad)*math.Cos(b.Latitude*rad)*math.Pow(math.Sin(dlon/2), 2)
}

// Named resolves Apple's comma-separated city/region/country labels. It never
// breaks ties by population or by the user's own location.
func (r *Resolver) Named(label string) (Place, error) {
	parts := strings.Split(strings.ToLower(label), ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	var matches []Place
	best := 0
	for _, c := range r.cities {
		if !contains(c.names, parts[0]) {
			continue
		}
		score := 1
		for _, part := range parts[1:] {
			if contains(c.context, part) {
				score++
			}
		}
		// Every supplied qualifier must match, avoiding an unrelated city when
		// an Apple label contains a custom place name or unknown region.
		if score != len(parts) {
			continue
		}
		if score > best {
			matches, best = nil, score
		}
		if score == best {
			matches = append(matches, c.Place)
		}
	}
	if len(matches) != 1 {
		return Place{}, errors.New("city name is unavailable or ambiguous")
	}
	return matches[0], nil
}

func contains(values []string, wanted string) bool {
	if wanted == "" {
		return false
	}
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

// LocalTime renders the current instant with a date and numeric UTC offset.
func LocalTime(now time.Time, zone string) (string, error) {
	if zone == "" {
		return "", errors.New("missing time zone")
	}
	loc, err := time.LoadLocation(zone)
	if err != nil {
		return "", fmt.Errorf("load time zone: %w", err)
	}
	return now.In(loc).Format(time.RFC3339), nil
}
