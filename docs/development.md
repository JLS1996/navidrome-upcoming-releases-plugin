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
```

The preview uses explicitly fictional sample releases. Open `dist/preview.html`
locally and select **Calendar**. No hosted preview or file-serving route is created.

For the full protocol and permission tests:

```sh
NAVIDROME_BIN=/path/to/navidrome python3 scripts/test_integration.py
```

The harness runs real Navidrome and the WASM plugin against local fixture services.
It synthesizes audio, creates disposable users, and checks ntfy, CalDAV, Google
Calendar, restart recovery and library revocation. It never uses real remote
accounts. Fixture overrides are build-tagged and excluded from normal packages.
Runtime data remains under ignored `test-runtime/`.

Browser tests:

```sh
python3 -m pip install playwright
python3 scripts/test_browser.py
```

Use installed Chromium, or install Playwright's browser. The suite renders the
HTML offline and checks list/calendar filters, navigation, uncertain dates and
calendar download. Direct `file://` navigation may be blocked by managed browser
policies; verify local file opening in your deployment.

The configuration editor also has an end-to-end regression test against Navidrome:

```sh
NAVIDROME_BIN=/path/to/navidrome python3 scripts/test_config_form.py
```

It reproduces the old unsupported-format error, then checks that the fixed schema
renders, masks credential inputs and saves/reloads configuration without enabling
any external integration. The native manifest test runs in CI to prevent putting
UI-only password formats back into JSON Schema.

The Google OAuth helper needs real user consent for a final live-account check.
Neither hosted credentials nor personal data should be used in automated tests.

## Public-source and release checks

Only publish reviewed source, synthetic screenshots and the normal plugin package.
Exclude runtime state, logs, personal config and credential files. Run
`scripts/check_public.py` on a clean publish directory before pushing. Source
bundles include only whitelisted project paths; raw runtime reports are excluded.

A public repository CI run builds and tests the package. It does not deploy a
website, enable GitHub Pages, connect calendar accounts, or publish notifications.
