# Reliability and limitations

- Metadata and integrations share each callback: at most one external request, with a 6-second HTTP timeout,
  keeping work below Navidrome's 30-second plugin-call limit. The two-second
  scheduler tick is idle when no work is due; the persisted interval gates HTTP.
- Progress, offsets, delivery acknowledgements and prior successful results survive reloads and server
  restarts. A replacement snapshot commits only after every page succeeds.
- 429/503 responses pause all artists. Retry-After and exponential backoff are
  respected (Retry-After capped at one day); after five consecutive failures an
  artist waits for the next discovery refresh.
- HTML, calendar and private state updates use temporary files plus rename. Temporary files are created
  exclusively to avoid following pre-existing temporary-file symlinks. These
  updates are individually atomic, not an atomic transaction across all files or external services.
- Incomplete dates retain their precision. A year/month overlapping the window is
  shown as uncertain, never as an invented exact day.
- Local-day comparisons use UTC. A file already open in a browser does not
  automatically refresh or change its snapshot at midnight.
- No releases displayed means "none known in these filters," not "none announced
  anywhere." MusicBrainz coverage depends on contributors.
- Before every callback uses cached data, plugin library grants are intersected with
  the selected user's current library access. Permission changes invalidate the cache. Report metadata contains listening-library information; share the
  HTML only with intended readers.
- Corrupt/unknown state or integration state is not silently discarded. Preserve `state.json`, disable
  the plugin, move the damaged file aside, then re-enable to rebuild. Check
  Navidrome logs for storage, config, permission or export errors.
- Metadata accepts only HTTP 200 JSON; integrations check their protocol-specific status codes. Redirects are disabled. This avoids
  following a misconfigured endpoint into a login page or another provider.
- Cached catalogs are currently kept in one JSON state file. Very large libraries
  may warrant sharded state after measurement; multi-million-track scaling has
  not been claimed or tested.
