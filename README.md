# where

A Go CLI that lists where your people are and what time it is there: nearest
city, IANA time zone, current local time, and location freshness.

`where` reads **Google Maps Location Sharing** and **Apple Find My People**.
Neither offers a public shared-people API. Google access uses an imported browser
session; Apple access uses the signed-in macOS Find My app through Accessibility.
These unofficial interfaces can change. Apple support is experimental and has
not yet been verified against a live signed-in session on macOS 26.

## Install

After the first release is published:

```sh
brew install --cask icco/tap/where
```

Or install from source with Go 1.26+:

```sh
go install github.com/icco/where@latest
```

Releases contain macOS and Linux binaries for ARM64 and AMD64. Google works on
both platforms; Apple requires macOS. The macOS install is **one Go binary**:
the embedded AppleScript runs using the OS-provided `osascript`, with no Swift
toolchain, downloaded helper, or separate Find My CLI.

**zsh has a builtin named `where`.** Use `command where` (as below), an absolute
path, or add `disable where` to your `.zshrc` to invoke this CLI instead.

## Google setup

1. Sign into [Google Maps](https://www.google.com/maps) in your browser and verify
   that **Location sharing** shows your people.
2. Export `google.com` cookies in Netscape `cookies.txt` format using a cookie
   export tool you trust. The export must include `__Secure-1PSID` or
   `__Secure-3PSID`. HttpOnly entries are supported. Keep this file private:
   cookies grant access to the signed-in session.
3. Import and validate the session:

   ```sh
   command where auth google --cookies ~/Downloads/cookies.txt
   ```

For multiple signed-in Google accounts, pass `--account 1` (the account index
used by Maps), or `--account you@example.com`. The default is account `0`.
The command checks the shared-location response before saving the session. An
empty sharing list is valid. Only cookies scoped to `google.com` or
`www.google.com` are imported; expired cookies are discarded.

Repeat the import if the session expires. Signing out of the browser can
invalidate the imported cookies. `where` does not refresh browser sessions or
collect your Google password. You can remove the original export after import.

## Apple setup (macOS, experimental)

1. Sign into your Apple account in **System Settings**, open **Find My**, and
   confirm your shared people appear under **People**.
2. Use English for the Find My app. Enable **Accessibility** for the terminal or
   host app running `where` under **System Settings → Privacy & Security**.
   Allow its **Automation** access to System Events when prompted. You may need
   to fully quit and relaunch the terminal after granting permission.
3. Verify access and enable the provider:

   ```sh
   command where auth apple
   ```

Apple authentication is inherited from the existing macOS session; `where`
does not request or store an Apple password. Reading brings Find My to the
foreground, selects View → People, and scrolls the sidebar. It needs a working,
unlocked desktop session. Screen Recording permission is not used.

The adapter uses named Accessibility labels and overlapping scroll pages. An
unknown layout, failed scrolling, or pages that do not overlap produce an error
rather than a potentially incomplete successful list. A screen without any
recognizable People rows also produces an error (including an empty/signed-out
screen); support for identifying those empty states needs live validation.

Apple exposes place text, not GPS coordinates. `where` resolves unambiguous
city/region/country labels against the bundled gazetteer. Custom labels such as
**Home**, ambiguous city names, truncated labels, and places absent from the
dataset remain unresolved. Apple freshness text is displayed verbatim; no
precise observation timestamp is fabricated from it.

## Usage

```sh
command where
command where list --provider google
command where list --provider apple --json
command where --stale-after 30m --timeout 90s
command where auth status
command where auth logout google
command where auth logout apple
command where version
command where completion zsh
```

Example output (illustrative):

```text
PERSON  PROVIDER  CITY          TIME ZONE         LOCAL TIME                 UPDATED / STATUS
Alex    apple     New York, US  America/New_York  2026-09-13T10:42:00-04:00   Now
Sam     google    London, GB    Europe/London     2026-09-13T15:42:00+01:00   2026-09-13T14:40:00Z
```

Local time is **now at the last reported location**, including its date, UTC
offset, and daylight saving time. It is separate from the location's update
time. Timestamped observations older than `--stale-after` (default one hour)
are marked stale. Unknown timestamps remain unknown. People are sorted by name;
the same name on different providers remains separate.

`--json` emits an object containing `generated_at`, `people`, and optional
provider `errors`. Rows include `name`, `provider`, resolved city/region/country,
`time_zone`, `local_time`, and available freshness information. Unavailable
locations remain in the list; resolution failures have a `resolution_error`.
Raw coordinates and provider IDs are not included.

When one provider fails, results from the other are still printed. The command
returns a nonzero exit status and reports which provider failed. With `--json`,
stdout remains valid JSON and provider failures appear in its `errors` object.
An empty successful list is `"people": []`.

## Local data

Configuration defaults to `$XDG_CONFIG_HOME/where/config.json`, or
`~/.config/where/config.json`. Override it with `--config /path/config.json`.
Session files are atomically saved with mode `0600`; newly created configuration
directories have mode `0700`. Imported Google cookies are stored in that file,
so it should not be synced or committed. `auth status` reports local configuration
only, without printing credentials or checking session validity. `auth logout`
removes local access without signing out your browser or Mac.

City lookup and time-zone resolution are entirely local. No locations are sent
to third-party geocoders, and `where` keeps no location history. Google contacts
Google's Maps endpoint; Find My itself communicates with Apple as usual.

The bundled GeoNames database covers cities with populations over 15,000 or
capitals. Nearest-city lookup measures distance to those city centers, which
can yield a suburb/municipality rather than a larger metropolitan name. Google
time zones come from the coordinates' geographic boundaries; Apple time zones
come from the matched city's center. Border/ambiguous cases can be unresolved.
See [data sources and licenses](THIRD_PARTY_NOTICES.md).

## Development and releases

```sh
go test -race -coverprofile=coverage.out ./...
go tool cover -func=coverage.out
golangci-lint run ./...
go build .
goreleaser check
goreleaser release --snapshot --clean
```

Tests use synthetic provider data. CI enforces 80% overall coverage and compiles
the embedded AppleScript on macOS. Tests do not sign into personal accounts.
Real Google cookies and a permitted Find My desktop session are needed for an
end-to-end smoke test. A passing unit suite does not establish compatibility
with the providers' current live interfaces.

`go generate ./internal/geo` refreshes the public GeoNames data. Review changes
and update provenance in `THIRD_PARTY_NOTICES.md`; ordinary builds require no
data downloads.

Releases run on main pushes, version-tag pushes, or manual workflow dispatch.
The first automatic release is **v1.0.0**, with subsequent versions calculated
from conventional commits. GoReleaser builds archives, completions, checksums,
and publishes `Casks/where.rb` to `icco/homebrew-tap`, following `icco/etu`'s
distribution pattern.

Before the first release, configure the repository Actions secret **GH_PAT**
with Contents write access to `icco/homebrew-tap`. `GITHUB_TOKEN` publishes the
release in this repository; `GH_PAT` only publishes the cask in the tap. The
workflow fails before tagging when the tap secret is missing. The cask uses
the same unsigned-binary quarantine-removal hook as `etu`.
