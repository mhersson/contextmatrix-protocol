# AGENTS.md - contextmatrix-protocol

The CM↔backend wire contract: webhook DTOs, HMAC signing/verification, stable
error codes. Imported as a Go module by **contextmatrix**,
**contextmatrix-agent**, and **contextmatrix-chat** - a change here ripples to
all three, so treat every edit as a public API change.

Single flat `package protocol` at the module root. `README.md` is the
file-by-file contract index; read it before editing a specific DTO.

## Boundary discipline

- **Stdlib only. No dependencies, ever.** `go.mod` has zero `require` lines; keep
  it that way.
- **DTOs are pure data - no business logic, no value validation.** Which worker
  statuses are valid, which models are allowed, which transitions are legal all
  live on the CM/backend side. This package pins field *shapes*, nothing more.
- **Never break the wire shape.** Additive change only: new fields are
  `omitempty`, decoders tolerate unknown fields. Removing or renaming a field, or
  changing a JSON tag, breaks every consumer.
- **Versioning:** additive change = minor bump. One-time exception: v0.8.0
  removed the retired runner backend's surface with a coordinated lockstep
  release of all consumers; the never-break policy resumes from v0.8.0.
- **Secrets stay out of logs.** `LLMEndpoint.APIKey`, `GitToken`, `MCPAPIKey`,
  `GitCredentialsToken`, `GuestSpec.Token`, and `TaskSkillsSource.Token` are
  secrets. Never log them.

## Coding conventions

- Every DTO carries a doc comment; document each non-obvious field inline with its
  wire semantics and back-compat story (see `chat.go` for the pattern).
- New optional field → `omitempty`. Use a pointer (`*T`) when the consumer must
  distinguish "absent/legacy" from "present but zero".
- Stable error codes live in `codes.go`. Keep the set small - no codes for one-off
  situations. Each constant documents its HTTP status. Clients branch on `Code`,
  never on `Message`.
- Response bodies never echo raw `err.Error()` or user-supplied values. The single
  field name in a validation error is the only exception.
- Do not write doc comments on simple functions - if what it does is
  straightforward, the code itself is the documentation. (DTO and field doc
  comments are the wire contract - they stay.)
- Never use em-dashes; use hyphens (-).

## Key domain rules

- **HMAC signs `method + "\n" + uri + "\n" + ts + "." + body`** (`hmac.go`).
  Binding method and URI stops a valid signature being replayed against another
  endpoint, or against a concurrent same-second request with a different query.
- **Asymmetric clock skew:** up to `DefaultMaxClockSkew` (5 min) in the past,
  `DefaultMaxFutureSkew` (30 s) in the future.
- **Replay defense** is a caller-supplied `ReplayCache` hook; `Verify` rejects a
  duplicate `(timestamp, signature)` pair when a cache is passed.
- **`BestOfN` and `Selection` are agent-backend inputs.**
- **`omitempty` is the back-compat proof:** an absent field marshals to nothing,
  so a pre-multi-user consumer sees byte-identical JSON. The pin tests assert this.

## Verification & commit discipline

Run before every commit:

```bash
go vet ./...    # must be clean (CI enforces)
go test ./...   # must be clean (CI enforces) - wire-pin tests are the safety net
```

The pin tests (`dto_test.go`, `selection_test.go`, `hmac_test.go`) marshal each
DTO and assert the exact JSON bytes. When you change a field tag on purpose,
update the expected bytes and confirm the matching decoder in every consumer repo.
When a pin test fails and you did not intend a wire change, you introduced drift -
fix the code, not the test.

**NEVER** commit without manual approval from the user. No exceptions.

**NEVER** reference a plan phase or task number in commit messages.

**ALWAYS** write conventional commits - `type(scope): concise description`. Keep
them short and focused; use bullet points in the body for the what and why, no
long paragraphs.

```
feat(selection): add best_of_n trigger field and outcome stats
refactor(protocol): remove knowledge-base refresh DTOs
docs(current-state): document what exists now, not how we got here
```
