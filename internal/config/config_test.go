package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/icco/where/internal/provider"
)

func TestStorage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "where", "config.json")
	c, err := Load(path)
	if err != nil || c.Apple || c.Google != nil {
		t.Fatalf("%+v %v", c, err)
	}
	cookies, err := provider.ParseCookies(strings.NewReader(".google.com\tTRUE\t/\tTRUE\t0\t__Secure-1PSID\tsynthetic\n"), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	c = Config{Apple: true, Google: &provider.GoogleSession{Account: "1", Cookies: cookies}}
	if err := Save(path, c); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil || !got.Apple || got.Google.Account != "1" || got.Google.Cookies[0].Value != "synthetic" {
		t.Fatalf("%+v %v", got, err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("permissions: %v %v", info, err)
	}
	if err := Save(path, Config{}); err != nil {
		t.Fatal(err)
	}
	got, err = Load(path)
	if err != nil || got.Google != nil {
		t.Fatalf("logout: %+v %v", got, err)
	}
	if err := os.WriteFile(path, []byte("invalid"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("invalid config accepted")
	}
	if err := Save(filepath.Join(path, "file"), Config{}); err == nil {
		t.Fatal("invalid directory accepted")
	}
	if _, err := Load(filepath.Dir(path)); err == nil {
		t.Fatal("directory accepted")
	}
}

func TestDefaultPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	path, err := DefaultPath()
	if err != nil || path != filepath.Join(home, ".config", "where", "config.json") {
		t.Fatalf("%s %v", path, err)
	}
	t.Setenv("XDG_CONFIG_HOME", home)
	path, err = DefaultPath()
	if err != nil || path != filepath.Join(home, "where", "config.json") {
		t.Fatalf("%s %v", path, err)
	}
	t.Setenv("XDG_CONFIG_HOME", "relative")
	if _, err := DefaultPath(); err == nil {
		t.Fatal("relative XDG accepted")
	}
}
