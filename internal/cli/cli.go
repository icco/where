// Package cli implements the where command.
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"
	"unicode"

	"github.com/icco/where/internal/config"
	"github.com/icco/where/internal/geo"
	"github.com/icco/where/internal/provider"
	"github.com/spf13/cobra"
)

type dependencies struct {
	google func(context.Context, provider.GoogleSession) ([]provider.Person, error)
	apple  func(context.Context) ([]provider.Person, error)
	now    func() time.Time
}

type app struct {
	deps       dependencies
	path       string
	json       bool
	provider   string
	timeout    time.Duration
	staleAfter time.Duration
}

// New constructs a command with production providers. Help never loads sessions.
func New(version, commit string) *cobra.Command {
	return newCommand(version, commit, dependencies{
		google: func(ctx context.Context, session provider.GoogleSession) ([]provider.Person, error) {
			return (provider.Google{Session: session}).People(ctx)
		},
		apple: (provider.Apple{}).People,
		now:   time.Now,
	})
}

func newCommand(version, commit string, deps dependencies) *cobra.Command {
	a := &app{deps: deps}
	root := &cobra.Command{
		Use: "where", Short: "Where your people are, and what time it is there",
		Long:    "List shared Google Maps and Apple Find My people with their nearest city and local time.\nGoogle uses imported browser cookies. Apple uses the signed-in macOS Find My app.",
		Version: version + " (" + commit + ")", Args: cobra.NoArgs, SilenceUsage: true, SilenceErrors: true,
		RunE: a.list,
	}
	root.PersistentFlags().StringVar(&a.path, "config", "", "configuration file (default $XDG_CONFIG_HOME/where/config.json or ~/.config/where/config.json)")
	root.PersistentFlags().DurationVar(&a.timeout, "timeout", 60*time.Second, "timeout for each provider")
	addListFlags := func(cmd *cobra.Command) {
		cmd.Flags().BoolVar(&a.json, "json", false, "print structured JSON")
		cmd.Flags().StringVar(&a.provider, "provider", "all", "provider: all, google, or apple")
		cmd.Flags().DurationVar(&a.staleAfter, "stale-after", time.Hour, "mark timestamped locations stale after this duration")
	}
	addListFlags(root)
	list := &cobra.Command{Use: "list", Short: "List people sharing their location", Args: cobra.NoArgs, RunE: a.list}
	addListFlags(list)
	root.AddCommand(list)
	root.AddCommand(&cobra.Command{Use: "version", Short: "Print build version", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		_, err := fmt.Fprintln(cmd.OutOrStdout(), root.Version)
		return err
	}})
	auth := &cobra.Command{Use: "auth", Short: "Configure or remove provider access"}
	var cookiePath, account string
	google := &cobra.Command{Use: "google", Short: "Validate and import Google Maps browser cookies", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		if cookiePath == "" {
			return errors.New("--cookies is required; export google.com cookies in Netscape cookies.txt format")
		}
		c, path, err := a.load()
		if err != nil {
			return err
		}
		f, err := os.Open(cookiePath) // #nosec G304 -- the user explicitly selects the cookie export.
		if err != nil {
			return fmt.Errorf("open cookie export: %w", err)
		}
		defer f.Close()
		cookies, err := provider.ParseCookies(f, a.deps.now())
		if err != nil {
			return err
		}
		session := provider.GoogleSession{Cookies: cookies, Account: account}
		ctx, cancel, err := a.context(cmd.Context())
		if err != nil {
			return err
		}
		defer cancel()
		if _, err := a.deps.google(ctx, session); err != nil {
			return err
		}
		c.Google = &session
		if err := config.Save(path, c); err != nil {
			return err
		}
		_, err = fmt.Fprintln(cmd.OutOrStdout(), "Google session validated and saved.")
		return err
	}}
	google.Flags().StringVar(&cookiePath, "cookies", "", "Netscape cookie export to import")
	google.Flags().StringVar(&account, "account", "0", "Google account index or email (Maps authuser)")
	auth.AddCommand(google)
	auth.AddCommand(&cobra.Command{Use: "apple", Short: "Check and enable the signed-in macOS Find My session", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		c, path, err := a.load()
		if err != nil {
			return err
		}
		ctx, cancel, err := a.context(cmd.Context())
		if err != nil {
			return err
		}
		defer cancel()
		if _, err := a.deps.apple(ctx); err != nil {
			return err
		}
		c.Apple = true
		if err := config.Save(path, c); err != nil {
			return err
		}
		_, err = fmt.Fprintln(cmd.OutOrStdout(), "Apple Find My access verified and enabled.")
		return err
	}})
	auth.AddCommand(&cobra.Command{Use: "status", Short: "Show local configuration status (does not contact providers)", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		c, _, err := a.load()
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(cmd.OutOrStdout(), "Google configured: %t\nApple enabled: %t\n", c.Google != nil, c.Apple)
		return err
	}})
	auth.AddCommand(&cobra.Command{Use: "logout <google|apple>", Short: "Remove local provider access (does not sign out your browser or Mac)", Args: cobra.ExactArgs(1), ValidArgs: []string{"google", "apple"}, RunE: func(cmd *cobra.Command, args []string) error {
		c, path, err := a.load()
		if err != nil {
			return err
		}
		switch args[0] {
		case "google":
			c.Google = nil
		case "apple":
			c.Apple = false
		default:
			return errors.New("provider must be google or apple")
		}
		if err := config.Save(path, c); err != nil {
			return err
		}
		_, err = fmt.Fprintf(cmd.OutOrStdout(), "Removed local %s access.\n", args[0])
		return err
	}})
	root.AddCommand(auth)
	// Cobra supplies bash, zsh, fish, and PowerShell completion commands.
	return root
}

func (a *app) context(parent context.Context) (context.Context, context.CancelFunc, error) {
	if a.timeout <= 0 {
		return nil, nil, errors.New("--timeout must be positive")
	}
	ctx, cancel := context.WithTimeout(parent, a.timeout)
	return ctx, cancel, nil
}

func (a *app) load() (config.Config, string, error) {
	path := a.path
	if path == "" {
		var err error
		path, err = config.DefaultPath()
		if err != nil {
			return config.Config{}, "", err
		}
	}
	c, err := config.Load(path)
	return c, path, err
}

// Row is a coarse location result; raw coordinates and session data are omitted.
type Row struct {
	Name            string     `json:"name"`
	Provider        string     `json:"provider"`
	City            string     `json:"city,omitempty"`
	Region          string     `json:"region,omitempty"`
	Country         string     `json:"country,omitempty"`
	TimeZone        string     `json:"time_zone,omitempty"`
	LocalTime       string     `json:"local_time,omitempty"`
	UpdatedAt       *time.Time `json:"updated_at,omitempty"`
	Stale           *bool      `json:"stale,omitempty"`
	Status          string     `json:"status,omitempty"`
	ResolutionError string     `json:"resolution_error,omitempty"`
}

type report struct {
	GeneratedAt time.Time         `json:"generated_at"`
	People      []Row             `json:"people"`
	Errors      map[string]string `json:"errors,omitempty"`
}

func (a *app) list(cmd *cobra.Command, _ []string) error {
	if a.provider != "all" && a.provider != "google" && a.provider != "apple" {
		return errors.New("--provider must be all, google, or apple")
	}
	if a.timeout <= 0 || a.staleAfter <= 0 {
		return errors.New("--timeout and --stale-after must be positive")
	}
	c, _, err := a.load()
	if err != nil {
		return err
	}
	if c.Google == nil && !c.Apple {
		return errors.New("no providers configured; run where auth google --cookies cookies.txt or where auth apple")
	}
	resolver, err := geo.New()
	if err != nil {
		return fmt.Errorf("load city database: %w", err)
	}
	result := report{GeneratedAt: a.deps.now().UTC(), People: make([]Row, 0), Errors: map[string]string{}}
	type source struct {
		name    string
		enabled bool
		fetch   func(context.Context) ([]provider.Person, error)
	}
	sources := []source{
		{"google", c.Google != nil, func(ctx context.Context) ([]provider.Person, error) { return a.deps.google(ctx, *c.Google) }},
		{"apple", c.Apple, a.deps.apple},
	}
	for _, s := range sources {
		if a.provider != "all" && a.provider != s.name {
			continue
		}
		if !s.enabled {
			if a.provider != "all" {
				result.Errors[s.name] = "provider is not configured"
			}
			continue
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), a.timeout)
		people, fetchErr := s.fetch(ctx)
		cancel()
		if fetchErr != nil {
			result.Errors[s.name] = fetchErr.Error()
			continue
		}
		for _, p := range people {
			result.People = append(result.People, resolve(resolver, p, result.GeneratedAt, a.staleAfter))
		}
	}
	sort.SliceStable(result.People, func(i, j int) bool {
		a, b := result.People[i], result.People[j]
		if strings.EqualFold(a.Name, b.Name) {
			return a.Provider < b.Provider
		}
		return strings.ToLower(a.Name) < strings.ToLower(b.Name)
	})
	if a.json {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		if err := enc.Encode(result); err != nil {
			return err
		}
	} else {
		if err := render(cmd.OutOrStdout(), result); err != nil {
			return err
		}
		for _, name := range []string{"google", "apple"} {
			if message, ok := result.Errors[name]; ok {
				fmt.Fprintf(cmd.ErrOrStderr(), "%s: %s\n", name, safe(message))
			}
		}
	}
	if len(result.Errors) > 0 {
		return errors.New("one or more providers failed; results may be incomplete")
	}
	return nil
}

func resolve(r *geo.Resolver, p provider.Person, now time.Time, staleAfter time.Duration) Row {
	row := Row{Name: p.Name, Provider: p.Provider, UpdatedAt: p.UpdatedAt, Status: p.Status}
	if p.UpdatedAt != nil {
		stale := now.Sub(*p.UpdatedAt) > staleAfter
		row.Stale = &stale
	}
	var place geo.Place
	var err error
	switch {
	case p.Coordinates != nil:
		place, err = r.Nearest(*p.Coordinates)
	case p.LocationText != "":
		place, err = r.Named(p.LocationText)
	default:
		if row.Status == "" {
			row.Status = "location unavailable"
		}
		return row
	}
	row.City, row.Region, row.Country, row.TimeZone = place.City, place.Region, place.Country, place.TimeZone
	if err != nil {
		row.ResolutionError = err.Error()
		return row
	}
	row.LocalTime, err = geo.LocalTime(now, place.TimeZone)
	if err != nil {
		row.ResolutionError = err.Error()
	}
	return row
}

func render(w io.Writer, result report) error {
	if len(result.People) == 0 {
		_, err := fmt.Fprintln(w, "No shared people returned.")
		return err
	}
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "PERSON\tPROVIDER\tCITY\tTIME ZONE\tLOCAL TIME\tUPDATED / STATUS"); err != nil {
		return err
	}
	for _, row := range result.People {
		city := row.City
		if row.Country != "" {
			city += ", " + row.Country
		}
		updated := row.Status
		if row.UpdatedAt != nil {
			updated = row.UpdatedAt.Format(time.RFC3339)
		}
		if row.Stale != nil && *row.Stale {
			updated += " (stale)"
		}
		if row.ResolutionError != "" {
			updated += "; " + row.ResolutionError
		}
		if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", display(row.Name), display(row.Provider), display(city), display(row.TimeZone), display(row.LocalTime), display(strings.TrimPrefix(updated, "; "))); err != nil {
			return err
		}
	}
	return tw.Flush()
}

func safe(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.In(r, unicode.Cf) {
			return -1
		}
		return r
	}, s)
}
func display(s string) string {
	s = safe(s)
	if s == "" {
		return "—"
	}
	return s
}
