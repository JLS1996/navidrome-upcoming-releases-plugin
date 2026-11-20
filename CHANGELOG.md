# Changelog

## 0.8.0

- Rename the displayed plugin and standard package to navidrome-upcoming-releases-plugin.
- Preserve existing calendar identities and document the filename-compatible upgrade.
- Add an explicit ntfy switch and choices for discoveries, updates and release-day reminders.
- Add bright, dark and black HTML themes without changing release or calendar data.
- Break settings help into readable paragraphs and expand integration setup guidance.
- Publish the installable package, reviewed source archive and checksums as a GitHub release.

## 0.7.1

- Order General as Date window, Appearance, Automatic matching and Exclusions.
- Group both library filters under Exclusions.

## 0.7.0

- Optionally skip artists without a standalone album or track credit in granted libraries.
- Keep excluded metadata cached and pause integrations until scoped credit scans finish.
- Move date window, automatic matching and owned-release filtering into General.

## 0.6.0

- Add an optional Releases setting to exclude music already in the granted libraries.
- Match exact artist/title pairs and check release types when tagged.
- Apply ownership filtering consistently to reports, calendars, notifications and artwork.
- Refresh the local album inventory in resumable pages without extra MusicBrainz requests.
- Preserve inventory snapshots on failure and pause integrations until scans complete.

## 0.5.0

- Separate configuration instructions into paragraphs with space before fields and groups.
- Replace the refresh text field with a switch below Info; both saved transitions request a check.
- Add a final Matching tab with artist-ID and exclusion guidance.
- Combine notification and calendar controls in Integrations.
- Use a neutral charcoal-and-blue dashboard theme with cover-derived ambient lighting.
- Enlarge artwork, put Last checked timestamps on a new line, and add missing-cover placeholders.
- Limit type and timing filters to values actually present in the report.
- Keep manual discovery requests independent of MusicBrainz retry cooldowns.

## 0.4.0

- Replace the long Generated files panel with concise Info and in-tab setup guidance.
- Link artist names, release titles and covers directly to their MusicBrainz release group.
- Add optional offline cover thumbnails with a private cache and subtle card backgrounds.
- Show artwork beside release cards and selected-day entries without shrinking the calendar.
- Bound image sizes, cache usage and redirects; keep artwork requests separate from credentials.
- Add artwork, link navigation, mobile layout and live public-thumbnail checks.

## 0.3.1

- Fit all seven calendar columns on mobile without sideways scrolling.
- Regenerate reports immediately after upgrades without refetching completed artists.
- Show compact release counts with full titles below the selected day.
- Add keyboard date navigation and distinguish today from the selected day.
- Move Generated files above the settings tabs and Library access before permissions.
- Clarify local file storage versus optional outbound ntfy and calendar publishing.
- Keep long artist names and release tags from overflowing the report.
- Keep release counts and the report date window accurate after filtering.
- Rebuild downloads from embedded calendar text so browser-saved HTML stays usable.
- Avoid building hidden calendars.
- Limit search to artist names and release titles; group dates once per calendar render.
- Update setup, configuration and validation documentation.

## 0.3.0

- Select albums, EPs, singles, broadcasts and other formats independently.
- Add individual MusicBrainz secondary-type filters, including live, remix and demo.
- Remove username and contact fields; use one granted user and project identification.
- Keep files exclusively in plugin storage and remove music-library write access.
- Move artist matching and manual refresh into General settings.
- Prefer mobile calendar sharing where available, hiding the direct download action.
- Deduplicate shared releases and keep artist credits consistent across outputs.
- Fix stale retry/status messages, pagination validation and remote event recreation.
- Validate persisted retry metadata and clear report data when access cannot be verified.
- Simplify installation, configuration, integration and limitations documentation.

## 0.2.4

- Back off across the whole MusicBrainz queue for transient service/network failures.
- Keep retrying temporary failures automatically, preserving pagination and snapshots.
- Show a service retry notice and recover saved transient artist errors on upgrade.
- Use Blob calendar downloads with optional mobile file sharing and a text fallback.
- Test repeated 429/503 failures, restart recovery, mobile sharing and fallback paths.

## 0.2.3

- Organize settings into Library, Releases, Local files, Notifications and Calendar tabs.
- Add grouped vertical layouts, shorter labels and multiline artist settings.
- Show only the selected calendar provider’s credentials.
- Automatically use the single user granted access; typing a username is optional.
- Stop safely when grants are missing or ambiguous, retaining explicit selection for multiple users.
- Use the resolved user consistently for library permissions and calendar/notification identities.

## 0.2.2

- Fix the configuration form failing on the unsupported JSON Schema `password` format.
- Keep credential fields masked using Navidrome's JSONForms UI schema options.
- Add regression checks for schema compatibility and real configuration-form rendering.

## 0.2.1

- Recheck current user-library access before using cached data or sending updates.
- Keep working integrations active when another integration has invalid settings.
- Verify Google write responses before acknowledging calendar updates.
- Reject corrupt or schema-less state instead of silently resetting progress.
- Preserve notification acknowledgements for equivalent destination URLs.
- Replace the vendored Navidrome SDK with an independent MIT host adapter.
- Make build/test scripts portable and exclude runtime data from public bundles.
- Strip distributed debug metadata and document the outbound-only privacy model.

Includes the offline list/calendar dashboard, iCalendar export, ntfy alerts,
CalDAV sync and Google Calendar sync. MusicBrainz is the only release data source.
