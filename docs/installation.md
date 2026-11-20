# Installation

1. Copy `release-radar.ndp` into Navidrome's plugins folder and rescan plugins.
2. In the plugin's **Users Permission** card, grant exactly one user. Prefer a
   regular user with access to the music you want in the report.
3. In **Library Permission**, select the intended libraries. Write access is not
   needed.
4. Review the General and Releases settings, then enable the plugin.
5. Open the generated HTML from Navidrome's data directory:

   ```text
   <DataFolder>/plugins/release-radar/storage/upcoming-releases.html
   ```

The first report appears as checks start. Large libraries take longer to finish;
lookups run gradually and resume after restarts. Artist discovery refreshes daily.
Use **General → Refresh request** to request a new cycle.

## Opening the report

The HTML contains its own data, styles, scripts and calendar export. It works
offline; only source links require internet. Refresh an open local file to load
updated results. A copied file is a snapshot: copy it again to get newer data.

The calendar fits all seven weekdays on mobile. Select a day to read the full
release titles below the grid; badges show how many releases are on each day.
Arrow keys move the selected day when using a keyboard.

On mobile, open the HTML in a full browser. Use **Save or share calendar** when
available. See [calendar export](integrations.md#local-calendar-export) for fallback
options and importing `.ics` files.

## Docker and file permissions

Use the host directory bound to Navidrome's data volume. A container's file path
is not the same as a path on your desktop.

Plugin storage is private: its directory is owner-only and report files are
created with mode `0600`. When Navidrome runs under a different OS user, arrange
host-side access or copy only `upcoming-releases.html` and `music-releases.ics`
to a private location you can open. Keep `state.json` and `integrations.json`
private; they contain library metadata and integration state.

The plugin serves no files over HTTP. Keep reports out of public web roots and
reverse-proxy file mappings.

## Updating

Replace the `.ndp`, rescan plugins, and reload the settings page. Copy the newly
generated HTML again if you view it on another device. Upgrades regenerate the
report even when all lookups are already complete.

For an upgrade from 0.2.x:

- Select exactly one granted user; the old username and contact settings are ignored.
- Review the new independent release-type selectors. Albums and EPs remain enabled
  by default; choose the special types you want.
- Reports now stay only in plugin storage. Delete any old library-root copies
  yourself if you no longer want them.

Existing lookup progress and calendar identities are preserved when the granted
user and libraries stay the same. Changing access clears the active catalog.
Previously copied reports remain snapshots. If Navidrome disables the plugin
immediately after all library access is revoked, remove its old report files
manually when necessary.
