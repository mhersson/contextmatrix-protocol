package protocol

import (
	"strconv"
	"strings"
	"testing"
	"time"
)

// Golden vectors pin byte-for-byte compatibility with the HMAC verify
// implementations in the consuming repos (contextmatrix and the agent/chat
// backends). They are external reference values, computed with an
// independent HMAC implementation; NEVER regenerate these from this
// package's own output.
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
		"other-key", "other-key", "POST", "/api/agent/status", "1765432101",
		"01a3bdd97b57bcb95604ae79aece5bca1467519b1d95145924e9fffdc696df3b",
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

type fakeCache struct{ seen map[string]bool }

func (f *fakeCache) CheckAndInsert(ts, sig string) bool {
	k := ts + "|" + sig
	if f.seen[k] {
		return true
	}
	if f.seen == nil {
		f.seen = map[string]bool{}
	}
	f.seen[k] = true

	return false
}

func sigFor(key, method, uri string, body []byte, ts string) string {
	return SignPayloadWithTimestamp(key, method, uri, body, ts)
}

func nowTS(offset time.Duration) string {
	return strconv.FormatInt(time.Now().Add(offset).Unix(), 10)
}

func TestVerifyAsymmetricSkewWindow(t *testing.T) {
	cases := []struct {
		name   string
		offset time.Duration
		want   bool
	}{
		{"fresh", 0, true},
		{"4m30s past", -4*time.Minute - 30*time.Second, true},
		{"6m past", -6 * time.Minute, false},
		{"29s future", 29 * time.Second, true},
		{"2m future", 2 * time.Minute, false}, // symmetric impl would accept this
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ts := nowTS(c.offset)
			sig := sigFor("k", "POST", "/x", []byte("b"), ts)
			got := VerifySignatureWithTimestamp("k", "POST", "/x", sig, ts, []byte("b"), DefaultMaxClockSkew, nil)
			if got != c.want {
				t.Errorf("offset %v: got %v, want %v", c.offset, got, c.want)
			}
		})
	}
}

func TestVerifyRejectsTamper(t *testing.T) {
	ts := nowTS(0)
	sig := sigFor("k", "POST", "/x?a=1", []byte("b"), ts)
	if VerifySignatureWithTimestamp("k", "POST", "/x?a=2", sig, ts, []byte("b"), DefaultMaxClockSkew, nil) {
		t.Error("accepted altered query string")
	}
	if VerifySignatureWithTimestamp("k", "GET", "/x?a=1", sig, ts, []byte("b"), DefaultMaxClockSkew, nil) {
		t.Error("accepted altered method")
	}
	if VerifySignatureWithTimestamp("k", "POST", "/x?a=1", sig, ts, []byte("B"), DefaultMaxClockSkew, nil) {
		t.Error("accepted altered body")
	}
	if VerifySignatureWithTimestamp("wrong", "POST", "/x?a=1", sig, ts, []byte("b"), DefaultMaxClockSkew, nil) {
		t.Error("accepted wrong key")
	}
	if VerifySignatureWithTimestamp("k", "POST", "/x?a=1", sig, "not-a-number", []byte("b"), DefaultMaxClockSkew, nil) {
		t.Error("accepted unparseable timestamp")
	}
	for _, extreme := range []string{"0", "-1", "9223372036854775807"} {
		if VerifySignatureWithTimestamp("k", "POST", "/x?a=1", sigFor("k", "POST", "/x?a=1", []byte("b"), extreme), extreme, []byte("b"), DefaultMaxClockSkew, nil) {
			t.Errorf("accepted extreme timestamp %q", extreme)
		}
	}
}

func TestVerifyReplayCacheRejectsDuplicate(t *testing.T) {
	ts := nowTS(0)
	sig := sigFor("k", "POST", "/x", []byte("b"), ts)
	cache := &fakeCache{}
	if !VerifySignatureWithTimestamp("k", "POST", "/x", sig, ts, []byte("b"), DefaultMaxClockSkew, cache) {
		t.Fatal("first verification should pass")
	}
	if VerifySignatureWithTimestamp("k", "POST", "/x", sig, ts, []byte("b"), DefaultMaxClockSkew, cache) {
		t.Error("replay should be rejected")
	}
}

func TestVerifyFailedSignatureDoesNotConsumeReplayCache(t *testing.T) {
	ts := nowTS(0)
	sig := sigFor("k", "POST", "/x", []byte("b"), ts)
	cache := &fakeCache{}
	// Forged request: valid (ts, sig) pair but tampered body. Must fail
	// verification WITHOUT inserting the pair into the replay cache.
	if VerifySignatureWithTimestamp("k", "POST", "/x", sig, ts, []byte("forged"), DefaultMaxClockSkew, cache) {
		t.Fatal("tampered body should fail verification")
	}
	// The legitimate request with the same (ts, sig) must still pass -
	// proving the failed attempt did not pre-consume the cache entry.
	if !VerifySignatureWithTimestamp("k", "POST", "/x", sig, ts, []byte("b"), DefaultMaxClockSkew, cache) {
		t.Error("legitimate request rejected: failed verification pre-consumed the replay cache entry")
	}
}
