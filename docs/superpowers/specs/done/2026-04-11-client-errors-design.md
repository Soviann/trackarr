# Spec — Client-Side Error Reporting

## Overview

When the app crashes with an unhandled frontend error, the crash details are automatically reported to the server so they appear in server logs for diagnosis. No user-visible UI change.

## User-Visible Behaviour

- When the app's error boundary catches a render crash, it silently sends the error message and stack trace to the server in the background
- The user continues to see the normal "Something went wrong" error screen — no change there
- The server logs the error at WARN level, making it diagnosable without needing the browser console
- The call is non-blocking: if the server is unreachable, the error screen still appears normally

## Acceptance Criteria

- After a frontend crash, the error and stack trace appear in server logs
- Unauthenticated requests to the endpoint are rejected (401)
- Malformed payloads are handled gracefully (no server crash)
- The endpoint does not affect the user-facing error screen in any way

<details>
<summary>Technical notes</summary>

- `POST /api/client-errors` — auth required (session cookie), rate-limited
- Payload: `{ message: string, stack?: string }` — already sent by `ErrorBoundary.tsx:24`
- Returns 204 on success, no response body
- Server logs: `WARN [client-error] <message>\n<stack>`
- No DB storage — server log is sufficient for a personal app
- Remove the TODO comment from `ErrorBoundary.tsx` once endpoint is live
- No migration needed

</details>
