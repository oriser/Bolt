# Configuration
Bolt is configured using environment variables

## Required Configuration
* `SLACK_SIGNIN_SECRET` - signin secret for a Slack app.
* `SLACK_OAUTH_TOKEN` - OAuth token of installed Slack app in a workspace.

## Optional Configuration
* `DONT_JOIN_AFTER` - If defined, Bolt won't join orders after that time. Time is defined in HH:MM format. Default is None (will always join).
* `DONT_JOIN_AFTER_TZ` - Defining the timezone for the hour defined in `DONT_JOIN_AFTER`. For example: `Europe/London`. Default is none (will be the local time where Bolt is running). 
* `ORDER_READY_TIMEOUT` - Timeout for waiting for the Wolt group order to be sent in duration format (ex: 1m/1h). After that duration, Bolt will stop tracking that order. Default is 1h (1 hour).
* `ORDER_DONE_TIMEOUT` - Timeout for waiting for the Wolt group order to be delivered after payment. After that duration, Bolt will stop tracking that order. Default is 3h (3 hours).
* `TIME_TILL_GET_READY_MESSAGE` - Defines how long before the delivery ETA the "get ready" message will be sent. Default is 7m (7 minutes).
* `ORDER_DESTINATION_EMOJI` - The emoji used to represent the order's destination in the progress message. Default is :house:.
* `JOINED_ORDER_EMOJI` - The emoji Bolt adds to the link message once it joined the order. Default is :eyes:.
* `DEBT_REMINDER_INTERVAL` - Time to wait between each reminder of unpaid debt in duration format. Default is 3h (3 hours).
* `DEBT_MAXIMUM_DURATION` - Maximum duration for keep reminding about unpaid debt in duration format. After that time, no more reminders will be sent. Default is 24h (24 hours).
* `WAIT_BETWEEN_STATUS_CHECK` - Duration between polling for Wolt order status in duration format. Default is 20s (20 seconds).
* `ADMIN_SLACK_USER_IDS` - List of Slack user IDs whose considered as Bolt's admins and can add custom users mapping using `/add-user` slash command.
* `SLACK_SERVER_PORT` - Port for listening for Slack events. Default is 8080.
* `SLACK_MAX_CONCURRENT_LINKS` - Maximum concurrent Slack link shared event handling. Wolt group link is holding a concurrent handler until the group will be finished. Default is 100.
* `SLACK_MAX_CONCURRENT_MENTIONS` - Maximum concurrent Slack mention handling. Default is 100.
* `SLACK_MAX_CONCURRENT_REACTIONS` - Maximum concurrent Slack reaction handling.
* `SLACK_STORE_MAX_CACHE_ENTRY_TIME` - Cache timeout of Wolt name to found Slack user in duration format. Default is 144h (6 days).