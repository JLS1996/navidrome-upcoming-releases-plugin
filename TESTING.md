# Validation

Tested with Navidrome 0.64.2, Go 1.25.x and TinyGo 0.40.1.

- Native regression tests and `go vet` cover date precision, duplicate editions,
  interrupted pagination, retry/backoff, HTML/ICS escaping, corrupt state and
  symlink-safe file writes.
- Integration tests run the actual WASM plugin in Navidrome with disposable local
  services. They exercise metadata discovery, ntfy baselining/update alerts,
  CalDAV conditional writes, Google token refresh and event CRUD, restart
  persistence and user-library revocation while the plugin remains enabled.
- A real Navidrome configuration-form test reproduces the old AJV password-format
  failure, verifies masked credential inputs, and saves/reloads settings. A manifest
  regression test enforces the validation/UI-schema separation in CI.
- Browser tests render the dashboard offline and verify shared filters, calendar
  navigation, separate uncertain dates, `.ics` download and responsive layout.
- No real ntfy topics or calendar accounts are used by the automated tests.

Live provider authorization, service-specific CalDAV setup, cross-user filesystem
permissions and actual local-file opening remain deployment-specific checks.
WASI-created files are owner-only; the plugin cannot change their ownership.

See [development instructions](docs/development.md) to reproduce the checks.
