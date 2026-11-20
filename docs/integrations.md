# Calendars and notifications

Notification and calendar controls share the **Integrations** tab in Navidrome.

## Local calendar export

Local report files remain in plugin storage. When you enable an integration,
selected release information is sent outbound to that destination: ntfy, CalDAV,
or Google Calendar. This does not provide incoming web access to Navidrome files.

The plugin also generates `<DataFolder>/plugins/navidrome-upcoming-releases-plugin/storage/music-releases.ics`.
The HTML embeds the same calendar for local downloading or sharing,
so it works even when you copy only the HTML file. It exports the complete report's
configured date/type window, independently of the browser search/month filters.

On mobile, open the HTML in a full browser rather than a file/email preview. The
page offers **Save or share calendar** when file sharing is supported, and a
direct download otherwise. If neither works, expand **Show calendar text**, copy it into a file
named `music-releases.ics`, and import that file. The separately generated `.ics`
file can also be copied directly. A calendar with zero dated events is valid but
imports nothing; wait for checks to finish and confirm releases fall in the window.

Events are all-day, with stable UIDs and an exclusive next-day end. Year/month-only
dates are omitted rather than assigned a guessed day. The `.ics` file is a snapshot:
importing it does not establish an automatically refreshing subscription. Calendar
apps differ in how repeated imports merge UIDs. Use direct sync below for automatic
updates. The plugin does not serve a subscription URL.

## ntfy alerts — free hosted or your existing server

1. Install an ntfy client or open its web app and subscribe to your chosen topic.
2. In **Integrations**, turn on **Enable ntfy notifications**.
3. Enter the same **Topic** and leave **ntfy server** as `https://ntfy.sh` for the
   free hosted service. Enter an access token if your server requires one.
4. Choose the notifications you want, then Save.

| Choice | Default | When it sends |
|---|---|---|
| Enable ntfy notifications | Off | Master switch; disabling it keeps topic, credentials and history |
| Newly found releases | On | A release first appears in the configured report window |
| Changed release details | On | A known release changes its date, title or other reported details |
| Release-day reminders | Off | Once per release and confirmed date, on that UTC day |
| Notify about existing releases | Off | Include the first scan's backlog when Newly found releases is enabled |

All alerts follow the date window, release-type settings and library exclusions.
They do not follow temporary search filters in an opened HTML report. Discovery
alerts can include a recent release when it first becomes known to the plugin.

Release-day reminders require the plugin to run on that day and the release to
have a complete date. There is no catch-up for missed days or guessed reminder
for a year/month-only date. A changed release date can receive a reminder on its
new date. Reminder acknowledgements survive restarts.

Turning off a notification category acknowledges skipped discoveries or changes,
so turning it back on does not replay them. Turning off the master switch pauses
sending; re-enabling resumes pending items still in the report window. The initial
backlog option applies when a destination is first initialized, not on every restart.

Delivery is retried with backoff and persistent acknowledgements. A crash after
publication but before acknowledgement can still duplicate an alert. Free hosted
service limits apply; no paid ntfy feature is required.

Public topics are not private unless the server restricts access. Choose an
unguessable topic or an existing access-controlled server for personal data.

## Direct calendar sync — optional and disabled by default

Use a **dedicated music releases calendar**. The plugin manages only its own event
IDs. Changed dates update the same event. Events leaving the configured date/type
window are deleted from that target once a source cycle has completed, as are events
whose exact dates become uncertain. Unrelated events are never intentionally modified.
ETags and ownership markers protect updates; a conflict is re-read before another
write. Configuration/network failures do not silently become an empty success.

`calendar_namespace` defaults to `navidrome-upcoming-releases-plugin`; together with the granted Navidrome user
it controls event identities. Give each installation a unique namespace if several
installations share a calendar. Changing namespace, granted user, provider or target does
**not** remove previously exported events from the old target. Disabling sync also
leaves existing events. Remove obsolete calendars/events manually when switching.

### CalDAV

Set `calendar_provider` to `caldav` and fill in:

- `caldav_calendar_url`: the **canonical writable collection URL** of your dedicated
  calendar, including its calendar identifier; not the generic account discovery URL.
- `caldav_username` and `caldav_app_password`: your service's app credentials.

This uses standard CalDAV GET/PUT/DELETE and conditional writes. iCloud Calendar
works with app-specific passwords; other CalDAV providers can use their own app
credentials. Service URLs must use HTTPS. Redirects are refused to avoid forwarding
credentials elsewhere.

The normal package allows the relevant iCloud domains. For another CalDAV hostname
or a self-hosted ntfy service, build with an explicit additional host permission:

```sh
./scripts/build.sh --allow-host calendar.example.org --allow-host ntfy.example.org
```

A service resolving to a private network IP additionally needs an explicit IP/CIDR
permission in Navidrome 0.64.2, for example `--allow-host 192.168.1.20/32`. This does
not bypass TLS certificate checks. No universal host wildcard is added.

### Google Calendar

Google Calendar uses the Calendar API with OAuth. Enable the API in your own Google
Cloud project and stay within its usage quotas.

1. Create a dedicated calendar in your Google Calendar settings and copy its calendar
   ID. An ID is not the calendar's sharing URL.
2. In your Google Cloud project, enable Calendar API, configure OAuth consent, and
   create an **OAuth Desktop app** client. Download its client JSON privately.
3. On your own computer, run:

   ```sh
   python3 scripts/google_authorize.py --client-json client.json --output google-oauth.json
   ```

   It uses a localhost callback, random OAuth state and PKCE. Complete Google's
   consent in the browser. The helper saves client ID, client secret and refresh
   token to a new owner-only file; it does not print tokens. Its scope is calendar
   event access, and it does not create a calendar for you.
4. Copy those fields into `google_client_id`, `google_client_secret`, and
   `google_refresh_token`. Set `google_calendar_id` and `calendar_provider=google`.

The plugin refreshes short-lived access tokens and persists them only in private
`integrations.json`, never the HTML or `.ics`. No browser callback server runs in
the plugin. Personal OAuth apps left in Google's external **Testing** mode may
have short-lived refresh tokens (commonly seven days for these scopes); configure
your own app appropriately and reauthorize if Google revokes the grant. Authorize
using the account that owns the destination calendar.

## Integration settings

| Setting | Default | Meaning |
|---|---|---|
| `calendar_namespace` | `navidrome-upcoming-releases-plugin` | Stable instance identity for generated events |
| `ntfy_server` | `https://ntfy.sh` | Publishing server; custom hosts need manifest permission |
| `ntfy_enabled` | false | Explicit master switch |
| `ntfy_topic` | blank | Destination topic; required for sending |
| `ntfy_notify_new` | true | Newly found releases |
| `ntfy_notify_updates` | true | Changed release details |
| `ntfy_notify_release_day` | false | Remind on a confirmed UTC release date |
| `ntfy_access_token` | blank | Optional bearer token, kept out of generated files |
| `ntfy_notify_initial` | false | Publish the initial backlog instead of baselining it |
| `calendar_provider` | `none` | `none`, `caldav`, or `google` |
| `caldav_calendar_url` | blank | Writable dedicated calendar collection |
| `caldav_username` | blank | CalDAV user |
| `caldav_app_password` | blank | CalDAV app password |
| `google_calendar_id` | blank | Dedicated target calendar ID |
| `google_client_id` | blank | Your OAuth Desktop client ID |
| `google_client_secret` | blank | Its client secret |
| `google_refresh_token` | blank | Token from local consent helper |

Integrations are **off by default** and are configured separately from metadata
lookups. Status errors appear in the generated report without remote bodies,
credentials, or raw credential-bearing transport errors. Each optional integration is validated independently. Invalid calendar settings do not
block valid ntfy alerts, and invalid notification settings do not block calendar sync.
