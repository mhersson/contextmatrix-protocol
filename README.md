# contextmatrix-protocol

The CM↔backend wire contract: webhook DTOs, HMAC signing/verification,
stable error codes. Imported by contextmatrix, contextmatrix-agent, and
contextmatrix-chat.

- **Stdlib only.** No dependencies, ever.
- **Forward-compatible by discipline:** new fields are `omitempty`, decoders
  tolerate unknown fields.
- **Versioning:** additive change = minor bump; never break the wire shape.
  One-time exception: v0.8.0 removed the retired runner backend's surface
  (renamed `runner_image`→`worker_image` and `runner_status`→`worker_status`;
  deleted the skill-engaged payload, the chat primer and chat git-token
  fields, three runner-only error codes, and the version constants). All
  consumers moved in lockstep; the never-break policy resumes from v0.8.0.

## Where the contract lives

The wire contract is documented in source, field by field, with doc comments
and wire-pin tests (`dto_test.go`, `selection_test.go`, `hmac_test.go`). This
index points at the file that owns each part:

| File            | Contract                                                                              |
| --------------- | ------------------------------------------------------------------------------------- |
| `task.go`       | CM→backend task lifecycle: trigger, kill, stop-all, message, promote, end-session.    |
| `callback.go`   | Backend→CM callback body: status.                                                     |
| `selection.go`  | Model-selection inputs shipped to the agent backend: candidates, favorites, blacklist, and per-model Best-of-N outcome stats + floor for win-rate-biased selection. |
| `logentry.go`   | One `data:` frame on a backend's `/logs` SSE stream, plus per-turn token usage.       |
| `chat.go`       | Chat-mode container payloads: start, resume, end, and the start response.             |
| `llm.go`        | CM-provisioned inference endpoint config, carried by trigger and chat-start payloads.  |
| `taskskills.go` | Task-skills repo pointer served by CM to the backends: clone URL, ref, and a short-lived clone token. |
| `verify.go`     | Operator-declared verify gate (command, timeout, env allowlist), carried by the trigger payload. |
| `codes.go`      | Stable `ErrorResponse.Code` constants - branch on these, not on `Message`.            |
| `response.go`   | Webhook response bodies: success/error, container list, stop-all, health, image list. |
| `hmac.go`       | HMAC-SHA256 signing/verification, header names, and clock-skew windows.               |
