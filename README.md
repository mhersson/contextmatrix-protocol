# contextmatrix-protocol

The CM↔backend wire contract: webhook DTOs, HMAC signing/verification,
stable error codes, protocol version. Imported by contextmatrix,
contextmatrix-runner, contextmatrix-agent, and (later) contextmatrix-chat.

- **Stdlib only.** No dependencies, ever.
- **Forward-compatible by discipline:** new fields are `omitempty`, decoders
  tolerate unknown fields, `ProtocolVersion` is observability-only.
- **Versioning:** additive change = minor bump; never break the wire shape.
