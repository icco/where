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

func parseApple(data []byte) ([]Person, error) {
	var rows []struct {
		Name     string `json:"name"`
		Location string `json:"location"`
		Status   string `json:"status"`
	}
	if err := json.Unmarshal(data, &rows); err != nil || rows == nil {
		return nil, errors.New("unrecognized Find My response; Apple's Accessibility layout may have changed")
	}
	people := make([]Person, 0, len(rows))
	for _, row := range rows {
		if strings.TrimSpace(row.Name) == "" {
			return nil, errors.New("Find My returned an unnamed person")
		}
		p := Person{Name: row.Name, Provider: "apple", LocationText: row.Location, Status: row.Status}
		lower := strings.ToLower(row.Location)
		if lower == "" || strings.Contains(lower, "no location") || strings.Contains(lower, "location not") || strings.Contains(lower, "can see your location") {
			p.LocationText = ""
			p.Status = "location unavailable"
		}
		people = append(people, p)
	}
	return people, nil
}
