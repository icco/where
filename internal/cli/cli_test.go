package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/icco/where/internal/config"
	"github.com/icco/where/internal/geo"
	"github.com/icco/where/internal/provider"
)

var testNow = time.Date(2026, 3, 8, 7, 0, 0, 0, time.UTC)

func testDeps() dependencies {
	return dependencies{
		google: func(ctx context.Context, _ provider.GoogleSession) ([]provider.Person, error) {
			if _, ok := ctx.Deadline(); !ok {
				return nil, errors.New("missing deadline")
			}
			old := testNow.Add(-2 * time.Hour)
			return []provider.Person{{Name: "Zed", Provider: "google", Coordinates: &provider.Coordinates{Latitude: 51.5074, Longitude: -0.1278}, UpdatedAt: &old}}, nil
		},
		apple: func(context.Context) ([]provider.Person, error) {
			return []provider.Person{{Name: "Alex", Provider: "apple", LocationText: "New York, NY", Status: "Now"}}, nil
		},
		now: func() time.Time { return testNow },
	}
}

func execute(t *testing.T, path string, deps dependencies, args ...string) (string, string, error) {
	t.Helper()
	cmd := newCommand("1.0.0", "abc", deps)
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs(append([]string{"--config", path}, args...))
	err := cmd.ExecuteContext(context.Background())
	return out.String(), errOut.String(), err
}

func TestAuthLifecycle(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	cookie := filepath.Join(dir, "cookies.txt")
	if err := os.WriteFile(cookie, []byte(".google.com\tTRUE\t/\tTRUE\t0\t__Secure-1PSID\tsynthetic-session\n"), 0600); err != nil {
		t.Fatal(err)
	}
	deps := testDeps()
	for _, args := range [][]string{
		{"auth", "status"}, {"auth", "google", "--cookies", cookie, "--account", "1"}, {"auth", "apple"},
	} {
		out, _, err := execute(t, path, deps, args...)
		if err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		if strings.Contains(out, "synthetic-session") {
			t.Fatal("credential leaked")
		}
	}
	c, err := config.Load(path)
	if err != nil || c.Google == nil || c.Google.Account != "1" || !c.Apple {
		t.Fatalf("%+v %v", c, err)
	}
	for _, args := range [][]string{{"auth", "logout", "google"}, {"auth", "logout", "apple"}} {
		if _, _, err := execute(t, path, deps, args...); err != nil {
			t.Fatal(err)
		}
	}
	c, err = config.Load(path)
	if err != nil || c.Google != nil || c.Apple {
		t.Fatalf("%+v %v", c, err)
	}
	for _, args := range [][]string{
		{"auth", "google"}, {"auth", "google", "--cookies", "/does-not-exist"}, {"auth", "logout", "bad"},
		{"auth", "apple", "--timeout", "0s"}, {"auth", "google", "--cookies", cookie, "--timeout", "0s"},
	} {
		if _, _, err := execute(t, path, deps, args...); err == nil {
			t.Errorf("accepted %v", args)
		}
	}
	deps.google = func(context.Context, provider.GoogleSession) ([]provider.Person, error) {
		return nil, errors.New("expired")
	}
	deps.apple = func(context.Context) ([]provider.Person, error) { return nil, errors.New("permission denied") }
	for _, args := range [][]string{{"auth", "google", "--cookies", cookie}, {"auth", "apple"}} {
		if _, _, err := execute(t, path, deps, args...); err == nil {
			t.Fatalf("auth failure swallowed: %v", args)
		}
	}
	c, err = config.Load(path)
	if err != nil || c.Google != nil || c.Apple {
		t.Fatal("failed auth persisted")
	}
	if err := os.WriteFile(cookie, []byte("bad"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := execute(t, path, deps, "auth", "google", "--cookies", cookie); err == nil {
		t.Fatal("bad cookies accepted")
	}
}

func TestList(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := config.Save(path, config.Config{Apple: true, Google: &provider.GoogleSession{}}); err != nil {
		t.Fatal(err)
	}
	out, _, err := execute(t, path, testDeps(), "list", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var result report
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.People) != 2 || result.People[0].Name != "Alex" || result.People[0].LocalTime != "2026-03-08T03:00:00-04:00" || result.People[1].Stale == nil || !*result.People[1].Stale || result.People[0].Stale != nil {
		t.Fatalf("%+v", result)
	}
	for _, forbidden := range []string{"latitude", "longitude", "cookies", "synthetic-session"} {
		if strings.Contains(out, forbidden) {
			t.Fatalf("leaked %s", forbidden)
		}
	}
	out, _, err = execute(t, path, testDeps())
	if err != nil || !strings.Contains(out, "Europe/London") || !strings.Contains(out, "(stale)") {
		t.Fatalf("%s %v", out, err)
	}
	out, _, err = execute(t, path, testDeps(), "--provider", "google", "--json")
	if err != nil || strings.Contains(out, "Alex") {
		t.Fatalf("filter: %s %v", out, err)
	}
	deps := testDeps()
	deps.google = func(context.Context, provider.GoogleSession) ([]provider.Person, error) {
		return nil, errors.New("expired")
	}
	out, stderr, err := execute(t, path, deps, "--json")
	if err == nil || stderr != "" {
		t.Fatalf("partial: %s %s %v", out, stderr, err)
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil || len(result.People) != 1 || result.Errors["google"] != "expired" {
		t.Fatalf("%s %v", out, err)
	}
	_, stderr, err = execute(t, path, deps)
	if err == nil || !strings.Contains(stderr, "google: expired") {
		t.Fatalf("%s %v", stderr, err)
	}
	if err := config.Save(path, config.Config{Apple: true}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := execute(t, path, deps, "--provider", "google"); err == nil {
		t.Fatal("unconfigured provider accepted")
	}
	deps.apple = func(context.Context) ([]provider.Person, error) { return nil, nil }
	out, _, err = execute(t, path, deps)
	if err != nil || !strings.Contains(out, "No shared people") {
		t.Fatalf("%s %v", out, err)
	}
	for _, args := range [][]string{{"--provider", "bad"}, {"--timeout", "0s"}, {"--stale-after", "-1s"}, {"extra"}} {
		if _, _, err := execute(t, path, deps, args...); err == nil {
			t.Errorf("accepted %v", args)
		}
	}
	if err := config.Save(path, config.Config{}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := execute(t, path, deps); err == nil {
		t.Fatal("no configuration accepted")
	}
}

func TestHelpAndErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	for _, args := range [][]string{{"--help"}, {"version"}, {"--version"}, {"completion", "bash"}, {"completion", "zsh"}, {"completion", "fish"}} {
		out, _, err := execute(t, path, testDeps(), args...)
		if err != nil || out == "" {
			t.Fatalf("%v: %q %v", args, out, err)
		}
	}
	if err := os.WriteFile(path, []byte("bad"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"list"}, {"auth", "status"}, {"auth", "apple"}, {"auth", "google", "--cookies", "x"}, {"auth", "logout", "google"}} {
		if _, _, err := execute(t, path, testDeps(), args...); err == nil {
			t.Errorf("bad config accepted: %v", args)
		}
	}
	t.Setenv("XDG_CONFIG_HOME", "relative")
	if _, _, err := execute(t, "", testDeps(), "auth", "status"); err == nil {
		t.Fatal("invalid XDG accepted")
	}
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if _, _, err := execute(t, "", testDeps(), "auth", "status"); err != nil {
		t.Fatal(err)
	}
	// Public production constructor can render help without a session or UI access.
	cmd := New("dev", "test")
	cmd.SetArgs([]string{"--help"})
	cmd.SetOut(io.Discard)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
}

func TestResolveAndRender(t *testing.T) {
	r, err := geo.New()
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []provider.Person{{Name: "Offline"}, {Name: "Home", LocationText: "Home"}, {Name: "Invalid", Coordinates: &provider.Coordinates{Latitude: 91}}} {
		row := resolve(r, p, testNow, time.Hour)
		if row.LocalTime != "" || row.Status == "" && row.ResolutionError == "" {
			t.Fatalf("%+v", row)
		}
	}
	var out bytes.Buffer
	if err := render(&out, report{People: []Row{{Name: "A\x1b\n\tB", Provider: "google", ResolutionError: "unresolved"}}}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "\x1b") || !strings.Contains(out.String(), "AB") {
		t.Fatal(out.String())
	}
	if err := render(failingWriter{}, report{People: []Row{{Name: "A"}}}); err == nil {
		t.Fatal("output error swallowed")
	}
	if err := render(failingWriter{}, report{}); err == nil {
		t.Fatal("empty output error swallowed")
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("broken pipe") }
