# Configuration

The settings tabs are **General**, **Releases**, **Integrations** and **Matching**.
A short Info panel stays above the tabs; library access guidance appears
below the tabs, immediately before Navidrome’s permission cards.

## Library access

Grant exactly **one user** in Navidrome's **Users Permission** card and select the
libraries to scan in **Library Permission**. Turn off **Allow all users** when the
server has more than one user. The plugin uses the intersection of that user's
libraries and the libraries granted to the plugin.

MusicBrainz lookups need no account, API key or personal email. Requests identify
the plugin by its project URL and respect MusicBrainz's rate limits.

## Manual refresh

The **Refresh releases** switch sits below Info, above the tabs. Flip it in either
direction and press **Save**. Each saved transition requests one new check; the
position is not an enabled/disabled state. Restarting with the same saved value
does not request another refresh. Existing results stay visible while checks run,
and unfinished pages resume.

## General

The sections appear in this order: **Date window**, **Appearance**,
**Automatic matching**, then **Exclusions**. Both library filters are under
**Exclusions** and are off by default. Each switch appears above its explanation.

**Days ahead** defaults to 90 (1–730). **Days back** defaults to 30 (0–365).
Dates use UTC. Changing filters updates cached results without downloading the
artist catalogs again.

**Dashboard theme** offers Bright, Dark and Black under Appearance. Dark is the
default. It changes only the HTML colors, including the calendar and artwork
backgrounds. Saving regenerates the report from cached data; refresh or recopy
the HTML to see the change.

**Download cover art** is also under Appearance. See [Cover art](#cover-art) for details.

**Match artists without IDs** is on by default. It uses cautious exact-name
matching when an artist has no MusicBrainz ID in its tags. IDs in tags and explicit
overrides are more reliable.

Both exclusion settings are grouped under **Exclusions**:

**Exclude releases already in library** is off by default. When enabled, the
plugin compares report releases with albums in the selected user's granted
libraries. Artist IDs (or exact artist names when IDs are absent) and album titles
must match, ignoring capitalization and repeated spaces. Tagged release types are
checked to distinguish formats and special editions such as live recordings.

Any matching local album record counts, including a partial album; track-by-track
completeness is not checked. This is not a fuzzy match: renamed or differently titled editions can remain
visible. Navidrome's Subsonic album API exposes edition IDs rather than the release
group IDs used by this plugin, so those IDs are not compared directly.

The library album list refreshes every 15 minutes and when you manually refresh.
It is scanned in resumable pages. Integrations pause during a scan so an incomplete
inventory cannot produce ownership-based notifications or calendar deletions.
The setting applies to HTML, `.ics`, notifications, calendar sync and cover fetching.
Scanning supports up to 100,000 library albums; failures retain the previous snapshot
and show a retry status. Disabling it restores cached releases immediately.

**Exclude artists without directly credited music** is off by default. When enabled,
it checks the whole album-artist and track-artist names in your selected libraries. For example, an entry called
“Guest” created from an album credited to “Lead & Guest” is skipped if no album or
track is credited to “Guest” itself. “Lead” stays included when a track is credited
to “Lead”. Name comparisons ignore case and repeated spaces; substrings and split
participants do not count. With multi-artist tags, this intentionally requires a
standalone credit. Artist ID overrides correct identity but do not bypass this filter.

The scan uses Navidrome locally, without MusicBrainz requests. It supports up to
100,000 albums, 2,000,000 tracks and 100,000 distinct credits. It runs in pages,
refreshes every 15 minutes and on manual refresh, and can take time for large
libraries. Reports retain the previous complete snapshot during rescans;
notifications and calendar sync pause until scanning finishes. Excluded artists'
metadata remains cached, so disabling the filter restores their cached results.
This filter does not discover additional artists outside Navidrome's album-artist
index. It is separate from **General → Exclude releases already in library**.

## Matching

Matching is the last tab. It contains tag-ID guidance, override examples and exclusions.

| Setting | Default | Use |
|---|---|---|
| Artist ID overrides | `{}` | JSON object mapping Navidrome artist IDs to MusicBrainz artist UUIDs |
| Excluded artist IDs | `[]` | JSON array of Navidrome artist IDs to skip |

For an ambiguous match, use the Navidrome ID shown in the report's attention
section and the UUID from the correct MusicBrainz artist page:

```json
{"navidrome-artist-id":"11111111-2222-4333-8444-555555555555"}
```

MusicBrainz IDs in your music tags or explicit overrides are more reliable than
name matching. Various Artists is skipped. A refresh checks completed artists
again while retaining their last successful results; unfinished pagination resumes.

## Cover art

Enable **General → Download cover art** to fetch front thumbnails from the
[Cover Art Archive](https://musicbrainz.org/doc/Cover_Art_Archive/API). It is off by
default and requires no account or paid API. The setting adds requests to Cover Art
Archive and Internet Archive, separate from MusicBrainz lookups.

Images are cached privately in `covers.json` and embedded in the HTML. The copied
report needs no image server or browser image requests. Release cards and
selected-day entries show larger covers on the right. Each available cover creates
its own ambient blur and gradient; missing artwork gets a `?` tile while covers
are enabled. The dashboard uses a neutral charcoal-and-blue theme.
Missing covers are skipped, and artwork failures do not block release checks or
calendar sync. Turning the setting off removes images from the generated report;
the private cache is retained for reuse.

The cache holds up to 100 releases and 4 MiB of encoded thumbnails. Each image is
limited to 256 KiB and 512 pixels per side. Only JPEG and PNG are accepted. Missing
images are checked again after a day. Cached images remain until the release
leaves the report window or the cache is reset. Artwork belongs to its respective
rights holders; the plugin's MIT license applies to its code, not downloaded covers.

## Releases

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
releases off to show only the selected special types. The report's type and timing filters list only values present in that report.
They narrow the displayed results without changing exported files.

These categories follow [MusicBrainz release-group types](https://musicbrainz.org/doc/Release_Group/Type).
A release with only a year or month remains visible when it overlaps your window,
but is excluded from dated calendar events.

## Info

The panel above the tabs gives quick instructions for the local report. Calendar
calendar and notification setup is together in Integrations. Local reports stay in
`<DataFolder>/plugins/<plugin-id>/storage/`; there is no export-directory setting.
The ID comes from the installed package filename. See [upgrade guidance](installation.md#upgrading-from-release-radar) for existing installations.
Optional integrations publish release information to ntfy or a configured CalDAV
or Google Calendar destination. They do not expose the local report files through
a web endpoint. See [installation](installation.md) for opening or copying files.

## Integrations

Optional integrations are disabled by default. Provider-specific credentials
appear only after selecting a calendar provider. See [setup instructions](integrations.md).

## Automatic retries

Network failures and MusicBrainz HTTP 408/429/5xx responses pause all metadata
lookups. Delays start at 30 seconds and double to 32 minutes. Longer `Retry-After`
delays are respected up to 24 hours. Progress and cooldowns survive restarts.
The report shows when the next retry is due; temporary service failures do not
require artist overrides.
