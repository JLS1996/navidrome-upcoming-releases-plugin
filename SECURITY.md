# Privacy and security

## No incoming file access

The plugin does not create an HTTP listener, serve a website, expose download routes,
or provide a calendar subscription endpoint. Its dashboard and calendar downloads
work from local files, with all dashboard data embedded. Do not put generated files
in a public web directory or map them through a reverse proxy.

The plugin makes outbound requests for MusicBrainz metadata and for integrations
that an administrator explicitly configures. Optional artwork sends public release
group IDs to Cover Art Archive / Internet Archive and embeds cached thumbnails;
opening the report does not fetch images from those services. Calendar GET requests retrieve events
from the configured calendar provider; they do not make Navidrome files web-accessible.
No additional plugin route or unauthenticated Navidrome view is installed.

## Access and credentials

- Each callback checks the granted user's current library access before using
  cached artist data. Revoked access invalidates the corresponding cache.
- Reports contain library information. Protect them like the underlying library.
  Previously copied reports are snapshots and need manual removal.
- Notification/calendar integrations are off by default. External destinations must
  use HTTPS; redirects are not followed with credentials.
- A public ntfy topic is not private. A bearer token alone does not prove that a
  topic has restricted read access. Use a destination whose access controls you
  have verified when notifications contain private information.
- Use a dedicated music calendar. Only plugin-owned event IDs are managed;
  ownership markers and ETags protect updates to existing events.
- Tokens and delivery acknowledgements stay in private plugin storage and are never
  embedded in HTML or `.ics` output. Do not publish Navidrome config, runtime
  databases, integration state or OAuth credential files.
- The artwork cache (`covers.json`) is private working data. Enabling covers embeds
  its images in HTML, so copied reports include them. Thumbnail downloads use only
  approved HTTPS artwork hosts, without notification or calendar credentials.
- Custom hosts require explicit manifest permissions. Never add a global `*`
  allowlist to work around a network error.

The optional Google authorization helper runs on the user's own computer. Its
short-lived localhost listener handles only the OAuth callback; it serves no files
and is not part of the Navidrome plugin runtime.

## Reporting a problem

Never put credentials, tokens or personal library exports in a public issue. For
sensitive security reports, use GitHub private vulnerability reporting if available,
or contact the maintainer privately before sharing details. Tests are not a
substitute for an independent security review.
