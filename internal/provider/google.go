package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const googleEndpoint = "https://www.google.com/maps/rpc/locationsharing/read"

// GoogleSession holds only the cookies scoped to Google's Maps host.
type GoogleSession struct {
	Cookies []*http.Cookie `json:"cookies"`
	Account string         `json:"account"`
}

// ParseCookies imports Netscape cookies.txt exports, including HttpOnly entries.
// Non-Google cookies are discarded; values never appear in errors.
func ParseCookies(r io.Reader, now time.Time) ([]*http.Cookie, error) {
	s := bufio.NewScanner(io.LimitReader(r, 2<<20))
	s.Buffer(make([]byte, 4096), 128<<10)
	var cookies []*http.Cookie
	auth := false
	for line := 1; s.Scan(); line++ {
		text := strings.TrimSpace(s.Text())
		text = strings.TrimPrefix(text, "#HttpOnly_")
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		fields := strings.Split(text, "\t")
		if len(fields) != 7 {
			return nil, fmt.Errorf("cookie line %d: expected seven tab-separated fields", line)
		}
		domain := strings.ToLower(strings.TrimPrefix(fields[0], "."))
		if domain != "google.com" && domain != "www.google.com" {
			continue
		}
		seconds, err := strconv.ParseInt(fields[4], 10, 64)
		if err != nil || seconds < 0 {
			return nil, fmt.Errorf("cookie line %d: invalid expiry", line)
		}
		if seconds > 0 && !time.Unix(seconds, 0).After(now) {
			continue
		}
		if fields[1] != "TRUE" && fields[1] != "FALSE" || fields[3] != "TRUE" && fields[3] != "FALSE" {
			return nil, fmt.Errorf("cookie line %d: invalid boolean", line)
		}
		c := &http.Cookie{Name: fields[5], Value: fields[6], Path: fields[2], Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode}
		// Host-only cookies must retain their host when reloaded into a jar.
		c.Domain = fields[0]
		if seconds > 0 {
			c.Expires = time.Unix(seconds, 0)
		}
		if c.Valid() != nil || !strings.HasPrefix(c.Path, "/") {
			return nil, fmt.Errorf("cookie line %d: invalid cookie", line)
		}
		cookies = append(cookies, c)
		if (c.Name == "__Secure-1PSID" || c.Name == "__Secure-3PSID") && c.Value != "" {
			auth = true
		}
	}
	if err := s.Err(); err != nil {
		return nil, fmt.Errorf("read cookie export: %w", err)
	}
	if !auth {
		return nil, errors.New("no unexpired Google session cookie; export google.com cookies after signing into Google Maps")
	}
	return cookies, nil
}

// Google reads the unofficial Maps location-sharing endpoint.
type Google struct {
	Session GoogleSession
	// Client may customize the transport for tests; requests never follow redirects.
	Client *http.Client
}

// People retrieves shared people, excluding the authenticated account's location.
func (g Google) People(ctx context.Context) ([]Person, error) {
	u, err := url.Parse(googleEndpoint)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("authuser", g.Session.Account)
	q.Set("hl", "en")
	q.Set("gl", "us")
	q.Set("pb", "!1m7!8m6!1m3!1i14!2i8413!3i5385!2i6!3x4095!2m3!1e0!2sm!3i407105169!3m7!2sen!5e1105!12m4!1e68!2m2!1sset!2sRoadmap!4e1!5m4!1e4!8m2!1e0!1e1!6m9!1e12!2i2!26m1!4b1!30m1!1f1.3953487873077393!39b1!44e1!50e0!23i4111425")
	u.RawQuery = q.Encode()
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	for _, c := range g.Session.Cookies {
		if c == nil {
			continue
		}
		domain := strings.TrimPrefix(c.Domain, ".")
		if domain != "google.com" && domain != "www.google.com" {
			continue
		}
		jar.SetCookies(&url.URL{Scheme: "https", Host: domain, Path: "/"}, []*http.Cookie{c})
	}
	client := http.Client{Timeout: 30 * time.Second}
	if g.Client != nil {
		client = *g.Client
	}
	client.Jar = jar
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; where; +https://github.com/icco/where)")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request Google shared locations: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 401 || resp.StatusCode == 403 || resp.StatusCode >= 300 && resp.StatusCode < 400 {
		return nil, errors.New("Google session expired or sign-in required; run where auth google with fresh cookies")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Google returned HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, (8<<20)+1))
	if err != nil {
		return nil, fmt.Errorf("read Google response: %w", err)
	}
	if len(data) > 8<<20 {
		return nil, errors.New("Google response exceeds 8 MiB")
	}
	return parseGoogle(data)
}

func at(value any, indexes ...int) any {
	for _, index := range indexes {
		list, ok := value.([]any)
		if !ok || index >= len(list) {
			return nil
		}
		value = list[index]
	}
	return value
}

func textAt(value any, indexes ...int) string {
	s, _ := at(value, indexes...).(string)
	return s
}

func parseGoogle(data []byte) ([]Person, error) {
	data = bytes.TrimSpace(data)
	data = bytes.TrimPrefix(data, []byte(")]}'"))
	var root []any
	if json.Unmarshal(data, &root) != nil || len(root) < 7 {
		return nil, errors.New("unrecognized Google response; session may have expired or Maps changed its format")
	}
	if textAt(root, 6) == "GgA=" {
		return nil, errors.New("Google session is not authenticated; import fresh cookies with where auth google")
	}
	people := make([]Person, 0)
	if root[0] == nil {
		return people, nil
	}
	entries, ok := root[0].([]any)
	if !ok {
		return nil, errors.New("unrecognized Google people list")
	}
	for i, entry := range entries {
		p := Person{Provider: "google", ID: textAt(entry, 6, 0), Name: textAt(entry, 6, 2)}
		if p.Name == "" {
			p.Name = textAt(entry, 6, 3)
		}
		if p.Name == "" {
			p.Name = p.ID
		}
		if p.Name == "" {
			return nil, fmt.Errorf("unrecognized Google person at index %d", i)
		}
		lat, latOK := at(entry, 1, 1, 2).(float64)
		lon, lonOK := at(entry, 1, 1, 1).(float64)
		c := Coordinates{lat, lon}
		if latOK && lonOK && c.Valid() {
			p.Coordinates = &c
		} else {
			p.Status = "location unavailable"
		}
		if millis, ok := at(entry, 1, 2).(float64); ok && millis > 0 && millis < 1e14 {
			t := time.UnixMilli(int64(millis)).UTC()
			p.UpdatedAt = &t
		}
		people = append(people, p)
	}
	return people, nil
}
