# Reliability and limitations

- MusicBrainz is community-maintained. A missing result means no matching metadata
  is known in the selected window, not that an artist has announced nothing.
- Dates may change. Year/month-only dates appear separately and are excluded from
  dated calendar exports.
- Lookups and integrations share scheduler callbacks, with at most one external
  request per callback and a six-second timeout. Busy services can delay completion.
- Temporary network and service failures retry automatically with persistent
  backoff. Invalid metadata or non-transient lookup errors are reported per artist;
  after five failed attempts, those artists wait for the next check cycle.
- Reports use UTC and are static snapshots. An open file does not refresh itself
  at midnight. Refresh the file, or copy a new version from plugin storage.
- File updates use temporary files and atomic rename. Updating HTML, calendar and
  state is not a single transaction; a crash may leave files from adjacent updates.
- Notifications use persistent acknowledgements. A crash after delivery but before
  saving the acknowledgement can send a duplicate.
- Calendar sync uses stable event IDs, ownership checks and ETags. Use a dedicated
  music calendar. Changing accounts or destinations leaves old events in place.
- Browser support for downloading or sharing local files varies. Use calendar text
  or copy the generated `.ics` directly if your browser cannot save it.
- Catalogs are stored in one JSON file. Very large libraries require more memory
  and take longer to scan. Catalogs above 50,000 release groups per artist are
  rejected with a reported error rather than silently truncated.

For a corrupt or unsupported state file, disable the plugin and preserve the file
before moving it aside to rebuild. Check Navidrome's logs for configuration,
permission and storage errors. Never share credentials or personal state in a
public issue.
