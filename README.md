# contextmatrix-protocol

The CM↔backend wire contract: webhook DTOs, HMAC signing/verification,
stable error codes. Imported by contextmatrix,
contextmatrix-agent, contextmatrix-chat, contextmatrix-architect, and
contextmatrix-backendkit.

- **Stdlib only.** No dependencies, ever.
- **Two packages with logic.** `selection/` is the deterministic reading of
  the selection wire types: given candidates, favorites, blacklist and a
  per-role tier ladder, which model a role and tier resolve to. Both CM and
  the agent import it so a preview and a real pick cannot disagree.
  `changelog/` is the reading of a build's merge history: the `git log`
  arguments every component's build runs, and the parser that turns their
  output into the change list `/health` reports. Both hold pure functions
  only - no logging, no I/O, no run-time state.
- **Forward-compatible by discipline:** new fields are `omitempty`, decoders
  tolerate unknown fields.
- **Versioning:** additive change = minor bump. A field may be retired only
  after every importer (contextmatrix, contextmatrix-agent, contextmatrix-chat,
  contextmatrix-architect, contextmatrix-backendkit) has dropped it; the
  retiring commit is marked `!` and ships as a minor bump.

## Where the contract lives

The wire contract is documented in source, field by field, with doc comments
and wire-pin tests (`dto_test.go`, `architect_test.go`, `blueprint_test.go`,
`taskskills_test.go`, `selection_test.go`, `hmac_test.go`). This
index points at the file that owns each part:

| File            | Contract                                                                              |
| --------------- | ------------------------------------------------------------------------------------- |
| `task.go`       | CM→backend task lifecycle: trigger, kill, stop-all, message (and the structured answers a question-round reply carries, or the action, revision and settings of a mockup reply), promote, end-session. |
| `callback.go`   | Backend→CM callback body: status.                                                     |
| `selection.go`  | Model-selection inputs shipped to the agent backend: candidates (prices, priors, creator), favorites, blacklist, per-role tier bars, and the best-value price headroom. |
| `selection/`    | The selector: tier ladders per role, descent, price band, best value, favorites, panel seats, reachability. Pure functions over the types in `selection.go`. |
| `logentry.go`   | One `data:` frame on a backend's `/logs` SSE stream, plus per-call token usage and gateway cost, the reply-button prompt a text frame may carry (display text and its technical detail on a plain-language blueprint, kind hint, recommended action, the label of the text box a feedback action opens, and the tabbed question round a prompt may carry: questions with options, pros, cons and sources, an assumptions tab whose entries may carry their technical detail, and the stored answers, decisions and chat), and the blueprint transcript marker (stage divider, sheet reference or review); a round and a marker name the slice of a per-slice stage. |
| `chat.go`       | Chat-mode container payloads: start, resume, end, and the start response.             |
| `architect.go`  | Blueprint session start fields CM adds to a chat start for the architect backend: target projects, autonomy mode and its constants, cost cap, the resolved model price, the model's context window, whether the blueprint runs in plain language, and the research setting (provider and key; absent when research is off). |
| `blueprint.go`  | Architect blueprint pane payloads, relayed by CM: the view (pipeline steps, run in flight, open prompt, cost, artifact index with reviews (summary, round cap, whether the verdict sent the artifact back, and findings ledger with the reviewer's notes), the blueprint's slices, whether it runs in plain language, and why an ended pipeline stopped, in plain words with the technical reason apart on a plain-language blueprint), one stored artifact version, and the live stream's frames and drafts; a per-slice step, run, artifact, frame and draft names its slice. |
| `llm.go`        | CM-provisioned inference endpoint config, carried by trigger and chat-start payloads.  |
| `taskskills.go` | Task-skills repo pointer served by CM to the backends: clone URL, ref, and a short-lived clone token. |
| `verify.go`     | Operator-declared verify gate (command, timeout, env allowlist), carried by the trigger payload. |
| `codes.go`      | Stable `ErrorResponse.Code` constants - branch on these, not on `Message`.            |
| `response.go`   | Webhook response bodies: success/error, container list, stop-all, health (the build's version, commit, build time and recent changes, and the default worker image's revision), image list. |
| `changelog/`    | The build's change list: the git arguments every build runs (`GitLogArgs`), the 20-entry limit, and `Parse`, which keeps merged PRs newest first with merge dates in UTC and drops Dependabot and the ci, docs, chore, build, test, refactor and style types. |
| `hmac.go`       | HMAC-SHA256 signing/verification, header names, and clock-skew windows.               |
