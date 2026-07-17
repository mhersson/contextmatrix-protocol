package protocol

// TaskSkillsSource is the pointer a backend fetches from
// GET /api/{agent|chat}/task-skills-source to clone the task-skills repo.
// CM stays the single source of truth for where task-skills live.
type TaskSkillsSource struct {
	// GitRemoteURL is the clone URL of the task-skills repo. Always emitted;
	// empty means the instance has no task-skills configured.
	GitRemoteURL string `json:"git_remote_url"`
	// Ref is the commit or ref to check out. Always emitted; empty means HEAD.
	Ref string `json:"ref"`
	// Token is a CM-provisioned short-lived token for cloning GitRemoteURL -
	// the only clone credential. Minted best-effort: a mint failure omits it.
	Token string `json:"token,omitempty"`
	// TokenExpiresAt is the RFC3339 expiry of Token. Informational for the
	// one-shot clone; consumers do not parse it.
	TokenExpiresAt string `json:"token_expires_at,omitempty"`
}
