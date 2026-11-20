# Changelog

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
