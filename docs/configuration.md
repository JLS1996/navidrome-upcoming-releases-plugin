# Configuration

| Setting | Default | Meaning |
|---|---|---|
| `username` | required | Navidrome user assigned to this plugin; no password required |
| `contact` | blank | Contact placed in outbound User-Agent, never in report |
| `ahead_days` | 90 | Upcoming window, 1–730 days |
| `past_days` | 30 | Recent window, 0–365 days |
| `include_singles` | false | Include singles alongside albums and EPs |
| `include_secondary` | false | Include secondary types, such as live/compilation/remix |
| `auto_match` | true | Conservative exact-name matching when tags lack an artist ID |
| `artist_overrides` | `{}` | JSON string mapping Navidrome artist IDs to MusicBrainz UUIDs |
| `excluded_artists` | `[]` | JSON string array of Navidrome artist IDs to skip |
| `export_library_id` | 0 | Optional destination library; requires explicit write permission |
| `refresh_token` | blank | Change to any new value to request a new discovery/check cycle |

Changing filters regenerates the page without fetching the catalog again. A
refresh token starts new checks for completed artists while retaining cached
results until a complete replacement is available; unfinished pagination resumes.
Disabling/re-enabling alone resumes saved work rather than forcing a new cycle.

Example overrides (use IDs shown in the report's attention section or Navidrome):

```json
{"navidrome-artist-id":"a74b1b7f-71a5-4011-9441-d0b5e4122711"}
```

Artist name equality cannot prove identity. Tagging with MusicBrainz artist IDs or
using overrides is preferable. Ambiguous/missing matches are flagged; they do not
silently become empty successful results. Various Artists is skipped.
