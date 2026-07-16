package protocol

// SuccessResponse is the body returned for any 2xx webhook response. `OK` is
// always true; `Message` is a short, free-form human-readable label (never
// derived from user input); `MessageID` is only populated by /message acks so
// CM can correlate the retryable request.
type SuccessResponse struct {
	OK        bool   `json:"ok"`
	Message   string `json:"message,omitempty"`
	MessageID string `json:"message_id,omitempty"`
}

// ErrorResponse is the body returned for any non-2xx webhook response (except
// the custom /readyz shape and the SSE /logs stream). `OK` is always false;
// `Code` is a stable enum from codes.go; `Message` is a terse human-readable
// label that never echoes raw err.Error() strings or user-supplied values
// beyond a single field name for validation errors.
type ErrorResponse struct {
	OK      bool   `json:"ok"`
	Code    string `json:"code"`
	Message string `json:"message,omitempty"`
}

// CardKillResult is one entry in a StopAllResponse: whether the individual
// Kill succeeded for that (project, card_id) and a short reason if not.
type CardKillResult struct {
	CardID  string `json:"card_id,omitempty"`
	Project string `json:"project,omitempty"`
	OK      bool   `json:"ok"`
	Error   string `json:"error,omitempty"`
}

// ContainerListItem is one entry in a ListContainersResponse. StartedAt is
// the backend's tracked start time as an RFC3339 timestamp; CM's reconcile
// sweep uses it to age-cap runaway containers without a second round-trip.
// The agent backend builds the list from its in-memory tracker, so Tracked
// is always true and State is always "running"; both fields stay on the
// wire for compatibility.
type ContainerListItem struct {
	ContainerID string `json:"container_id"`
	CardID      string `json:"card_id"`
	SessionID   string `json:"session_id,omitempty"`
	Project     string `json:"project"`
	State       string `json:"state"`
	StartedAt   string `json:"started_at"`
	Tracked     bool   `json:"tracked"`
}

// ListContainersResponse is the body returned by GET /containers. OK is
// always true — the list comes from the backend's in-memory tracker, so
// there is no error path.
type ListContainersResponse struct {
	OK         bool                `json:"ok"`
	Containers []ContainerListItem `json:"containers"`
}

// StopAllResponse is the body returned by POST /stop-all. `OK` is true iff
// every per-card Kill succeeded; on any failure the status code flips to 207
// and `OK` is false so a single field tells the caller whether they need to
// inspect Results.
type StopAllResponse struct {
	OK      bool             `json:"ok"`
	Total   int              `json:"total"`
	Stopped int              `json:"stopped"`
	Failed  int              `json:"failed"`
	Results []CardKillResult `json:"results"`
}

// HealthResponse is the body returned by GET /health. RunningContainers and
// MaxConcurrent give operators a one-glance snapshot of saturation without
// having to call /containers.
type HealthResponse struct {
	OK                bool `json:"ok"`
	RunningContainers int  `json:"running_containers"`
	MaxConcurrent     int  `json:"max_concurrent"`
}

// ImageListItem is one entry in a ListImagesResponse. Tags carries only the
// repo tags that matched the backend's image_list_filters — an image with no
// matching tag is omitted from the response entirely. Digests carries the
// image's RepoDigests verbatim (informational; empty for locally built,
// never-pushed images). Created is the image's creation time in unix seconds
// and Size its size in bytes, both as reported by the Docker daemon.
type ImageListItem struct {
	Tags    []string `json:"tags"`
	Digests []string `json:"digests,omitempty"`
	Created int64    `json:"created,omitempty"`
	Size    int64    `json:"size,omitempty"`
}

// ListImagesResponse is the body returned by GET /images. OK is always true
// on success (a Docker list error surfaces as a 502 ErrorResponse with the
// upstream-failure code, not a partial success here).
type ListImagesResponse struct {
	OK     bool            `json:"ok"`
	Images []ImageListItem `json:"images"`
}
