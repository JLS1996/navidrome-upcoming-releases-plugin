# Installation

1. Copy `dist/release-radar.ndp` into Navidrome's plugins folder.
2. Rescan plugins in the Navidrome admin UI.
3. Grant access to **one user** and the intended libraries. The plugin selects
   that user automatically; leave **username** blank. Prefer a dedicated regular user whose library access matches the report.
4. Set **contact** to your email or project URL for MusicBrainz request identification.
5. Enable the plugin. The first report appears immediately; checks progress in the
   background, at most one HTTP request per callback. Library discovery refreshes
   approximately every 24 hours, and incomplete checks survive refresh/restarts.
6. Open this file from the computer or share where Navidrome stores its data:

   ```text
   <DataFolder>/plugins/release-radar/storage/upcoming-releases.html
   ```

Bookmark the file once opened, then refresh it to see updated results. It contains
its own CSS, JavaScript, release data, and downloadable calendar. The **Calendar**
button opens a month grid with previous/next navigation and shared artist/type/time
filters. Releases without exact days appear separately instead of receiving an
invented calendar date. No CDN, remote scripts or web requests are needed.
Source links open MusicBrainz only when clicked.

**A path inside Docker is not a host path.** Mount the relevant data directory to
access the generated file on the host. The plugin does not serve HTTP, add a
Navidrome sidebar page, or promise a clickable `file://` link from Navidrome.

## Optional export beside your music

Set **export_library_id** to an assigned library's numeric ID. Explicitly grant
library write access in Navidrome (UI or `navidrome plugin edit release-radar
--write-access`). The plugin then also writes the fixed filename
`upcoming-releases.html` and `music-releases.ics` into that library's root. It never writes state or integration tokens into the
music folder or changes audio/tag files. Default export ID `0` disables this.

**Important permission limitation:** testing found that Navidrome/TinyGo's WASI
filesystem creates files with mode `0600`, even when `0644` is requested. Plugin
storage directories are also `0700`. WASI does not provide a working chmod.
Therefore, the local workflow works directly when your browser and Navidrome run
as the same OS user. A service account or Docker UID different from your desktop
user requires an explicit host-side file-sharing/permission arrangement. Moving
the report to a music folder alone does not solve ownership. Export is not a
cross-user permissions workaround.

Copies already exported or saved are static documents. Disabling the plugin,
changing its scope, or revoking a library does not remove old copies. Delete them
manually if no longer wanted. A running plugin clears cached results when it detects a new user/library scope.
However, Navidrome disables it immediately when library access is fully revoked,
before it can clear existing private reports. Those snapshots also require manual
removal. It cannot delete files inside a revoked mount.

## Architecture

The plugin opens no network listener and exposes no file download route. Open generated
files locally or push updates to destinations you configure. Do not place report files
in a public web root or add a reverse-proxy/static-file alias for them.

## Updating from 0.2.1

If the settings page reports `unknown format "password"`, replace the old
`release-radar.ndp` with version 0.2.2 or newer, rescan plugins in Navidrome, and
reload the plugin settings page. Existing configuration keys are unchanged; no
manual credential-file edits are needed.
