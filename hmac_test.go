package protocol

import (
	"strings"
	"testing"
	"time"
)

// Golden vectors computed from the pre-extraction implementations
// (cm internal/runner/hmac.go == cmr internal/hmac/hmac.go) on 2026-06-10.
// These prove the extracted signer is byte-identical to what both repos
// shipped. NEVER regenerate these from this package's own output.
var goldenVectors = []struct {
	name, key, method, uri, ts, want string
	body                             []byte
}{
	{
		"post-with-query", "test-key", "POST", "/webhook/trigger?x=1", "1765432100",
		"804755e4c2979a938007b977be571a1193a0b1e5b6e0a9daef5b91374a938013",
		[]byte(`{"card_id":"CM-001"}`),
	},
	{
		"get-nil-body", "test-key", "GET", "/logs?project=alpha", "1765432100",
		"1495df8d3cdcb4ca3be484c538e20028bc0dd1625d618beacc18bab1986cd248",
		nil,
	},
	{
		"other-key", "other-key", "POST", "/api/runner/status", "1765432101",
		"1b376adf65b27b09ca8641f4545c596bdcdcba77cf39b6e79e7287751a8be6e7",
		[]byte(`{}`),
	},
}

func TestSignPayloadWithTimestampGoldenVectors(t *testing.T) {
	for _, v := range goldenVectors {
		t.Run(v.name, func(t *testing.T) {
			got := SignPayloadWithTimestamp(v.key, v.method, v.uri, v.body, v.ts)
			if got != v.want {
				t.Errorf("signature drift!\n got %s\nwant %s", got, v.want)
			}
		})
	}
}

func TestSignRequestHeaders(t *testing.T) {
	sig, ts := SignRequestHeaders("test-key", "POST", "/webhook/trigger?x=1", []byte(`{"card_id":"CM-001"}`))
	if !strings.HasPrefix(sig, "sha256=") {
		t.Fatalf("signature header missing sha256= prefix: %q", sig)
	}
	// The header pair must verify against itself.
	raw := strings.TrimPrefix(sig, "sha256=")
	if !VerifySignatureWithTimestamp("test-key", "POST", "/webhook/trigger?x=1", raw, ts,
		[]byte(`{"card_id":"CM-001"}`), DefaultMaxClockSkew, nil) {
		t.Error("self-signed headers failed verification")
	}
}

func TestSkewConstants(t *testing.T) {
	if DefaultMaxClockSkew != 5*time.Minute {
		t.Errorf("DefaultMaxClockSkew = %v, want 5m", DefaultMaxClockSkew)
	}
	if DefaultMaxFutureSkew != 30*time.Second {
		t.Errorf("DefaultMaxFutureSkew = %v, want 30s", DefaultMaxFutureSkew)
	}
}
