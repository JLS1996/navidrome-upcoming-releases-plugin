# Development

## Requirements

- Go 1.25.x
- TinyGo 0.40.1
- Python 3
- Navidrome 0.64.2 and ffmpeg for full integration tests
- Playwright and Chromium for browser tests

The plugin uses an independently written MIT adapter for the Navidrome JSON host
ABI and the BSD-3-Clause Extism Go PDK. It does not vendor or link the Navidrome Go
SDK. Third-party notices are included in the package.

## Build

```sh
go mod download
./scripts/build.sh
```

The result is `dist/release-radar.ndp`. Debug metadata is stripped from distributed
builds. No credentials or runtime files are needed for a normal build.

For an additional configured service, grant only its explicit hostname:

```sh
./scripts/build.sh --allow-host calendar.example.org
```

Keep host permissions narrow. The build script rejects credential-bearing URLs and
universal wildcards. See [integration setup](integrations.md) for private-IP grants.

`scripts/env.sh` optionally accepts `RADAR_TOOLCHAIN_DIR` and `RADAR_CACHE_ROOT` for
local compiler/cache locations; normal installations can use their existing PATH.

## Testing

```sh
go test -race -cover ./...
go vet ./...
go run ./cmd/preview
go run ./cmd/preview -covers -out dist/preview-covers.html
```

The preview uses explicitly fictional sample releases. Open `dist/preview.html`
locally and select **Calendar**. No hosted preview or file-serving route is created.

For the full protocol and permission tests:

```sh
NAVIDROME_BIN=/path/to/navidrome python3 scripts/test_integration.py
```

The harness runs real Navidrome and the WASM plugin against local fixture services.
It synthesizes audio, creates disposable users, and checks ntfy, CalDAV, Google
Calendar, direct album/track credit filtering, restart recovery and library revocation. It never uses real remote
accounts. Fixture overrides are build-tagged and excluded from normal packages.
Runtime data remains under ignored `test-runtime/`.

Browser tests:

```sh
python3 -m pip install playwright
python3 scripts/test_browser.py
```

Use installed Chromium, or install Playwright's browser. The suite renders the
HTML offline and checks list/calendar filters, navigation, uncertain dates and
calendar download, mobile file sharing and its text fallback. Calendar checks
cover all seven columns at widths from 320 to 1280 pixels, selected-day details,
larger covers, placeholders, available-only filters, long titles, multiple releases,
keyboard navigation across month boundaries, and
calendar downloads after saving and reopening the HTML. Artwork tests use original
synthetic images and verify embedded covers, subtle backgrounds, and artist-link
navigation without sending requests to public MusicBrainz pages. Sharing tests
simulate the browser API; they do not replace a real-device Safari check. Direct `file://` navigation may be blocked by managed browser
policies; verify local file opening in your deployment.

The configuration editor also has an end-to-end regression test against Navidrome:

```sh
NAVIDROME_BIN=/path/to/navidrome python3 scripts/test_config_form.py
```

It reproduces the old unsupported-format error, then checks that the fixed schema
renders, checks the always-visible file/access information and field spacing at desktop and narrow
viewport sizes, masks credential inputs and saves/reloads release-type choices and grant-based user
selection without enabling any external integration. The native manifest test runs in CI to prevent putting
UI-only password formats back into JSON Schema.

To verify the public Northlane lookup through the real filtering and export code:

```sh
RADAR_LIVE_MUSICBRAINZ=1 go test ./internal/radar -run TestLiveNorthlaneLookup -count=1 -v
RADAR_LIVE_ARTWORK=1 go test ./internal/radar -run TestLiveNorthlaneCoverArt -count=1 -v
```

This optional network test checks MusicBrainz's Anemoia metadata against a fixed
November 2026 date window. The artwork check follows approved redirects and validates a public thumbnail.
Both are excluded from ordinary tests and CI. Never use
personal accounts or credentials in automated integration tests.

## Public-source and release checks

Only publish reviewed source, synthetic screenshots and the normal plugin package.
Exclude runtime state, logs, personal config and credential files. Run
`scripts/check_public.py` on a clean publish directory before pushing. Source
bundles include only whitelisted project paths; raw runtime reports are excluded.

CI runs native tests, validates public files, builds the WASM plugin and uploads the
plugin package as a build artifact.
