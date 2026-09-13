package provider

import (
	"context"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"strings"
	"testing"
	"time"
)

const cookieFixture = "# Netscape HTTP Cookie File\n#HttpOnly_.google.com\tTRUE\t/\tTRUE\t0\t__Secure-1PSID\ttest-session\n.other.com\tTRUE\t/\tTRUE\t0\tother\tignored\n"

func TestCookies(t *testing.T) {
	now := time.Unix(1000, 0)
	cookies, err := ParseCookies(strings.NewReader(cookieFixture), now)
	if err != nil || len(cookies) != 1 || cookies[0].Value != "test-session" {
		t.Fatalf("cookies = %v, %v", cookies, err)
	}
	for _, input := range []string{
		"", "bad", strings.ReplaceAll(cookieFixture, "\t0\t", "\t900\t"),
		strings.ReplaceAll(cookieFixture, "\t0\t", "\tx\t"),
		strings.ReplaceAll(cookieFixture, "TRUE", "maybe"),
		strings.ReplaceAll(cookieFixture, "test-session", "invalid;value"),
		strings.ReplaceAll(cookieFixture, "\t/\t", "\trelative\t"),
	} {
		if _, err := ParseCookies(strings.NewReader(input), now); err == nil {
			t.Errorf("accepted invalid export %q", input)
		}
	}
}

func googleFixture(t *testing.T) string {
	t.Helper()
	root := []any{[]any{
		[]any{nil, []any{nil, []any{nil, -0.12, 51.5}, 1767225600000}, nil, nil, nil, nil, []any{"id-a", nil, "Alex", "Al"}},
		[]any{nil, nil, nil, nil, nil, nil, []any{"id-b", nil, "Offline", ""}},
	}, nil, nil, nil, nil, nil, "authenticated"}
	b, err := json.Marshal(root)
	if err != nil {
		t.Fatal(err)
	}
	return ")]}'\n" + string(b)
}

func TestParseGoogle(t *testing.T) {
	people, err := parseGoogle([]byte(googleFixture(t)))
	if err != nil {
		t.Fatal(err)
	}
	if len(people) != 2 || people[0].Name != "Alex" || people[0].Coordinates.Latitude != 51.5 || people[0].Coordinates.Longitude != -0.12 || people[0].UpdatedAt.Year() != 2026 || people[1].Coordinates != nil {
		t.Fatalf("unexpected people: %+v", people)
	}
	for _, body := range []string{"<html>login</html>", "[]", "[null,null,null,null,null,null,\"GgA=\"]", "[{},null,null,null,null,null,null]", "[[[]],null,null,null,null,null,null]"} {
		if _, err := parseGoogle([]byte(body)); err == nil {
			t.Errorf("accepted %s", body)
		}
	}
	people, err = parseGoogle([]byte("[null,null,null,null,null,null,null]"))
	if err != nil || people == nil || len(people) != 0 {
		t.Fatalf("empty = %v %v", people, err)
	}
	// Nickname and ID fallbacks, and zero coordinates are valid.
	for _, info := range []string{`["id",null,null,"Nick"]`, `["id",null,null,null]`} {
		people, err = parseGoogle([]byte(`[[[null,[null,[null,0,0]],null,null,null,null,` + info + `]],null,null,null,null,null,null]`))
		if err != nil || people[0].Name == "" || people[0].Coordinates == nil {
			t.Fatalf("fallback: %+v %v", people, err)
		}
	}
}

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestGoogleRequest(t *testing.T) {
	cookies, err := ParseCookies(strings.NewReader(cookieFixture), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	for _, status := range []int{200, 302, 401, 403, 429, 500} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			calls := 0
			g := Google{Session: GoogleSession{Cookies: cookies, Account: "1"}, Client: &http.Client{Transport: roundTrip(func(r *http.Request) (*http.Response, error) {
				calls++
				if r.URL.Query().Get("authuser") != "1" || r.URL.Host != "www.google.com" || r.Header.Get("Cookie") != "__Secure-1PSID=test-session" {
					t.Errorf("invalid request: %v", r)
				}
				return &http.Response{StatusCode: status, Header: http.Header{"Location": {"https://example.com"}}, Body: io.NopCloser(strings.NewReader(googleFixture(t)))}, nil
			})}}
			_, err := g.People(context.Background())
			if (err == nil) != (status == 200) || calls != 1 {
				t.Fatalf("status %d, err %v, calls %d", status, err, calls)
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := (Google{}).People(ctx); err == nil {
		t.Fatal("cancellation ignored")
	}
}

func TestCoordinates(t *testing.T) {
	for _, c := range []Coordinates{{0, 0}, {-90, -180}, {90, 180}} {
		if !c.Valid() {
			t.Errorf("invalid: %v", c)
		}
	}
	for _, c := range []Coordinates{{91, 0}, {0, 181}, {math.NaN(), 0}, {0, math.Inf(1)}} {
		if c.Valid() {
			t.Errorf("valid: %v", c)
		}
	}
}

func FuzzGoogle(f *testing.F) {
	f.Add([]byte("[null,null,null,null,null,null,null]"))
	f.Add([]byte(")]}'\n[[[]],null,null,null,null,null,null]"))
	f.Fuzz(func(_ *testing.T, data []byte) { _, _ = parseGoogle(data) })
}
