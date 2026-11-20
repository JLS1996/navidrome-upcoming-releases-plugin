# Validation

Tested with Navidrome 0.64.2, Go 1.25.x and TinyGo 0.40.1.

- Native tests cover artwork validation/cache/redirects, release-type selectors, dates, shared releases, pagination,
  retry/backoff, escaping, state validation and safe file writes.
- Integration tests run the actual WASM plugin in Navidrome with disposable local
  services. They exercise grants-only identity, storage-only output, 429/503
  recovery, ntfy alerts, CalDAV writes, Google event CRUD, idle upgrade refresh,
  PNG/JPEG decoding, artwork toggles, restarts and revocation.
- The Navidrome configuration-form test checks categorized settings, spacing,
  release selectors, information outside the tabs, masked credentials and saving/reloading. CI also checks the
  manifest's schema and UI controls.
- Offline browser tests check list/calendar filters, Live selection, uncertain
  dates, direct downloads, simulated mobile sharing and its text fallback. The
  calendar fits seven columns from 320 px upward and supports selected-day details
  and keyboard navigation. Embedded artwork is checked at six viewport widths;
  artist navigation is intercepted locally to verify the exact destination. Saved
  HTML is reopened to verify downloads without the original Blob URL.
- An opt-in live Northlane check verifies public MusicBrainz metadata through the
  lookup, filter, HTML and calendar-export code; a separate opt-in check validates
  live artwork redirects and thumbnails. Normal tests require no external
  service or personal account.

Browser file handling, real-device sharing, calendar account authorization and
host filesystem permissions require checks in the user's deployment. Automated
sharing tests simulate the browser API; they do not certify every mobile browser.

See [development instructions](docs/development.md) to reproduce the checks.
