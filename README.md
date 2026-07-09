# contextmatrix-protocol

The CM↔backend wire contract: webhook DTOs, HMAC signing/verification,
stable error codes, protocol version. Imported by contextmatrix,
contextmatrix-runner, and contextmatrix-agent.

- **Stdlib only.** No dependencies, ever.
- **Forward-compatible by discipline:** new fields are `omitempty`, decoders
  tolerate unknown fields, `ProtocolVersion` is observability-only.
- **Versioning:** additive change = minor bump; never break the wire shape.

## Where the contract lives

The wire contract is documented in source, field by field, with doc comments
and wire-pin tests (`dto_test.go`, `selection_test.go`, `hmac_test.go`). This
index points at the file that owns each part:

| File            | Contract                                                                              |
| --------------- | ------------------------------------------------------------------------------------- |
| `task.go`       | CM→backend task lifecycle: trigger, kill, stop-all, message, promote, end-session.    |
| `callback.go`   | Backend→CM callback bodies: status, skill-engaged.                                    |
| `selection.go`  | Model-selection inputs shipped to the agent backend: candidates, favorites, blacklist, and per-model Best-of-N outcome stats + floor for win-rate-biased selection. |
| `logentry.go`   | One `data:` frame on a backend's `/logs` SSE stream, plus per-turn token usage.       |
| `chat.go`       | Chat-mode container payloads: start, resume, end, and the start response.             |
| `llm.go`        | CM-provisioned inference endpoint config, carried by trigger and chat-start payloads.  |
| `verify.go`     | Operator-declared verify gate (command, timeout, env allowlist), carried by the trigger payload. |
| `codes.go`      | Stable `ErrorResponse.Code` constants — branch on these, not on `Message`.            |
| `response.go`   | Webhook response bodies: success/error, container list, stop-all, health.             |
| `hmac.go`       | HMAC-SHA256 signing/verification, header names, and clock-skew windows.               |
| `version.go`    | `ProtocolVersion` (the wire-header version, observability-only) and its header name.  |
