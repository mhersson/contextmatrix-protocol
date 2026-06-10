package protocol

const (
	// ProtocolVersion identifies the wire-contract version. Carried in the
	// VersionHeader for observability and mismatch diagnostics only — never
	// for branching. Additive changes do not bump it; breaks are not allowed.
	ProtocolVersion = "1"

	// VersionHeader is the HTTP header carrying ProtocolVersion.
	VersionHeader = "X-Protocol-Version"
)
