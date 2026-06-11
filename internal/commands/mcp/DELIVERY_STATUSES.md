# Delivery Status Reference

Each delivery has a `status` field with one of six values. The status
describes where the delivery is in its lifecycle and what (if anything)
the user should do about it.

| Status | Meaning | `retry_delivery` works? |
|---|---|---|
| `pending` | Queued for delivery, no attempt made yet. Usually transient. | No (transient state — wait). |
| `delivering` | An attempt is in flight right now. Usually transient. | No (transient state — wait). |
| `delivered` | A receiver returned 2xx and accepted the webhook. Terminal success. | No (already delivered). |
| `failed` | The most recent attempt failed (non-2xx, timeout, network error) and the delivery is awaiting a retry decision. | **Yes** — re-enqueues a new attempt. |
| `scheduled_retry` | Backing off before the next automatic retry. The retry will happen on schedule; manual retry only useful if the user wants to bypass the backoff. | Yes (bypasses the scheduled backoff). |
| `dead_letter` | All automatic retries exhausted. The delivery will not be retried automatically. Terminal failure unless manually retried. | **Yes** — manually re-enqueues. |

## Lifecycle

```
pending → delivering → delivered           (happy path)
                    → failed → scheduled_retry → delivering → …
                                                            → dead_letter
```

A delivery sits in `pending` only briefly before a worker picks it up.
`delivering` means the HTTP request is open right now. After the
response comes back, the delivery transitions to either `delivered`
(2xx) or `failed` (everything else). A `failed` delivery is immediately
re-scheduled, entering `scheduled_retry` until the backoff window
elapses, at which point it goes back to `delivering`. After enough
retries the delivery lands in `dead_letter` and stays there until
something — a user, an automation, or `retry_delivery` — moves it.

## Common patterns

- **"Why is this delivery stuck?"** → check status. `pending` for more
  than a minute is anomalous and worth surfacing. `scheduled_retry` for
  hours is expected during backoff for a sustained outage.
  `dead_letter` means automatic retries gave up.
- **"How do I retry a failed delivery?"** → `retry_delivery` works for
  `failed` and `dead_letter`. It returns 409 for `pending`,
  `delivering`, `delivered`, or `scheduled_retry`.
- **"Did the receiver actually get the webhook?"** → status `delivered`
  means yes, the receiver returned 2xx. For any other status, use
  `list_attempts` to see the exact HTTP responses recorded against the
  delivery.
- **"Should I retry a `scheduled_retry`?"** → usually no — the system
  will retry on its own. Bypassing the backoff is only useful when the
  user knows the receiver just came back up and they want immediate
  delivery instead of waiting.
