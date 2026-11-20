# Configuration

The settings tabs are **General**, **Releases**, **Notifications** and **Calendar**.
Generated file information stays above the tabs; library access guidance appears
below the tabs, immediately before Navidrome’s permission cards.

## Library access

Grant exactly **one user** in Navidrome's **Users Permission** card and select the
libraries to scan in **Library Permission**. Turn off **Allow all users** when the
server has more than one user. The plugin uses the intersection of that user's
libraries and the libraries granted to the plugin.

MusicBrainz lookups need no account, API key or personal email. Requests identify
the plugin by its project URL and respect MusicBrainz's rate limits.

## General

Artist matching and manual refresh are in this tab.

| Setting | Default | Use |
|---|---|---|
| Match artists without IDs | On | Match exact artist names when tags have no MusicBrainz artist ID |
| Artist ID overrides | `{}` | JSON object mapping Navidrome artist IDs to MusicBrainz artist UUIDs |
| Excluded artist IDs | `[]` | JSON array of Navidrome artist IDs to skip |
| Refresh request | Blank | Change this value to start a new check cycle |

For an ambiguous match, use the Navidrome ID shown in the report's attention
section and the UUID from the correct MusicBrainz artist page:

```json
{"navidrome-artist-id":"11111111-2222-4333-8444-555555555555"}
```

MusicBrainz IDs in your music tags or explicit overrides are more reliable than
name matching. Various Artists is skipped. A refresh checks completed artists
again while retaining their last successful results; unfinished pagination resumes.

## Releases

**Days ahead** defaults to 90 (1–730). **Days back** defaults to 30 (0–365).
Dates use UTC. Changing filters updates cached results without downloading the
artist catalogs again.

Select the release formats you want:

| Format | Default |
|---|---|
| Albums | On |
| EPs | On |
| Singles | Off |
| Broadcasts | Off |
| Other or unspecified types | Off |

**Standard releases** includes releases without secondary tags, such as studio
albums and regular singles. It is on by default. Special types are individually
selectable and off by default:

- Live, Compilation, Remix, Soundtrack, Demo
- Mixtape/Street, DJ-mix
- Spokenword, Interview, Audiobook, Audio drama, Field recording

A release must match a selected format. If it has secondary tags, **any** selected
tag includes it. For example, Albums + Live includes live albums; turn Standard
releases off to show only the selected special types. The report's type filter
can further narrow the displayed results without changing exported files.

These categories follow [MusicBrainz release-group types](https://musicbrainz.org/doc/Release_Group/Type).
A release with only a year or month remains visible when it overlaps your window,
but is excluded from dated calendar events.

## Generated files

The information panel above the tabs explains the local reports. They stay in
`<DataFolder>/plugins/release-radar/storage/`; there is no export-directory setting.
Optional integrations publish release information to ntfy or a configured CalDAV
or Google Calendar destination. They do not expose the local report files through
a web endpoint. See [installation](installation.md) for opening or copying files.

## Notifications and Calendar

Optional integrations are disabled by default. Provider-specific credentials
appear only after selecting a calendar provider. See [setup instructions](integrations.md).

## Automatic retries

Network failures and MusicBrainz HTTP 408/429/5xx responses pause all metadata
lookups. Delays start at 30 seconds and double to 32 minutes. Longer `Retry-After`
delays are respected up to 24 hours. Progress and cooldowns survive restarts.
The report shows when the next retry is due; temporary service failures do not
require artist overrides.
