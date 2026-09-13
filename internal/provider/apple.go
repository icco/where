package provider

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

//go:embed findmy.applescript
var appleScript string

// Apple reads the existing macOS Find My session via Accessibility.
type Apple struct{}

// People reads the People sidebar, without collecting any Apple credentials.
func (Apple) People(ctx context.Context) ([]Person, error) {
	if runtime.GOOS != "darwin" {
		return nil, errors.New("Apple requires macOS and a signed-in Find My desktop session")
	}
	cmd := exec.CommandContext(ctx, "/usr/bin/osascript", "-")
	cmd.Stdin = strings.NewReader(appleScript)
	data, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("read Find My: %w", ctx.Err())
		}
		// Script diagnostics can contain UI text, so do not relay raw stderr.
		return nil, errors.New("cannot read Find My People: sign into Find My, use an English app language, and enable Accessibility and Automation for your terminal in System Settings → Privacy & Security")
	}
	return parseApple(data)
}

type appleRow struct {
	Name     string `json:"name"`
	Location string `json:"location"`
	Status   string `json:"status"`
}

func parseApple(data []byte) ([]Person, error) {
	var envelope struct {
		Pages [][]appleRow `json:"pages"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil || len(envelope.Pages) == 0 {
		return nil, errors.New("unrecognized Find My response; Apple's Accessibility layout may have changed")
	}
	rows := envelope.Pages[0]
	for _, page := range envelope.Pages[1:] {
		// Match names in overlapping pages, preserving distinct rows that have
		// the same name. Status/location text can update between reads.
		overlap := 0
		for size := min(len(rows), len(page)); size > 0; size-- {
			matches := true
			for i := range size {
				if rows[len(rows)-size+i].Name != page[i].Name {
					matches = false
					break
				}
			}
			if matches {
				overlap = size
				break
			}
		}
		if overlap == 0 && len(rows) > 0 {
			return nil, errors.New("find My People pages did not overlap; cannot guarantee a complete list")
		}
		rows = append(rows[:len(rows)-overlap], page...)
	}
	people := make([]Person, 0, len(rows))
	for _, row := range rows {
		if strings.TrimSpace(row.Name) == "" {
			return nil, errors.New("find My returned an unnamed person")
		}
		location, freshness, _ := strings.Cut(row.Location, "•")
		p := Person{Name: row.Name, Provider: "apple", LocationText: strings.TrimSpace(location), Status: row.Status}
		if p.Status == "" {
			p.Status = strings.TrimSpace(freshness)
		}
		lower := strings.ToLower(p.LocationText)
		if lower == "" || strings.Contains(lower, "no location") || strings.Contains(lower, "location not") || strings.Contains(lower, "can see your location") {
			p.LocationText = ""
			p.Status = "location unavailable"
		}
		people = append(people, p)
	}
	return people, nil
}
