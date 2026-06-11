Nahook is a webhook delivery platform.

## Concepts

- **Endpoint** (`ep_xxx`): a destination URL that receives webhooks. Has a
  status (active/paused) and a type (webhook or slack).
- **Event**: a typed payload fanned out to every endpoint subscribed to its
  `event_type` (e.g. `order.created`).
- **Delivery** (`del_xxx`): one webhook send to one endpoint. Status is one of:
  `pending`, `delivering`, `delivered`, `failed`, `scheduled_retry`,
  `dead_letter`.
- **Attempt**: a single HTTP call recorded against a delivery. A delivery
  with retries has multiple attempts.
- **Environment**: a namespace for endpoints within a workspace
  (`production`, `staging`). Most workspaces have one default environment.

## Tool-selection guide

- Don't know what to call? Start with `whoami` to confirm credentials,
  then `list_endpoints` to ground on what exists.
- Debugging a failure: `get_delivery` with `include_payload=true` tells
  you what the producer sent. `list_attempts` tells you what each
  receiver attempt returned. Together they explain whether the failure
  was producer-side or receiver-side.
- `retry_delivery` only works for `failed` or `dead_letter` status —
  calling on any other status returns 409.
- `trigger_event` fans out by `event_type` to every subscriber.
  `send_to_endpoint` bypasses subscriptions and targets one endpoint.
- `update_endpoint` is a PATCH — fields you don't set stay unchanged.
  Use `is_active=false` to pause, `is_active=true` to resume.

`trigger_event` and `send_to_endpoint` need a separate ingestion key
(`nhk_xxx`); other tools use the CLI login token. The error message
explains where to put the key if it's missing.
