// Package provider reads the people sharing their locations with an account.
package provider

import (
	"math"
	"time"
)

// Person is a provider observation. Coordinates are nil when unavailable.
// LocationText is used by Apple, which exposes place names rather than GPS.
type Person struct {
	ID           string
	Name         string
	Provider     string
	Coordinates  *Coordinates
	LocationText string
	UpdatedAt    *time.Time
	Status       string
}

// Coordinates is a WGS84 position, including valid zero coordinates.
type Coordinates struct{ Latitude, Longitude float64 }

// Valid reports whether the position is finite and within WGS84 bounds.
func (c Coordinates) Valid() bool {
	return !math.IsNaN(c.Latitude) && !math.IsNaN(c.Longitude) &&
		!math.IsInf(c.Latitude, 0) && !math.IsInf(c.Longitude, 0) &&
		c.Latitude >= -90 && c.Latitude <= 90 && c.Longitude >= -180 && c.Longitude <= 180
}
