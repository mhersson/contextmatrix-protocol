// Package protocol holds the CM↔backend wire contract: webhook DTOs,
// HMAC signing/verification, stable error codes, and the protocol version.
// Stdlib only — this module must never grow a dependency.
package protocol

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"time"
)

const (
	// DefaultMaxClockSkew is the maximum allowed age for webhook timestamps.
	// Payloads older than this are rejected to prevent replay attacks.
	DefaultMaxClockSkew = 5 * time.Minute

	// DefaultMaxFutureSkew is the maximum allowed future drift for webhook
	// timestamps. Tighter than the past window so a compromised signature
	// cannot be pre-issued for long before it is replayed.
	DefaultMaxFutureSkew = 30 * time.Second

	// SignatureHeader carries the HMAC-SHA256 signature ("sha256=<hex>").
	SignatureHeader = "X-Signature-256"

	// TimestampHeader carries the Unix timestamp used in HMAC computation.
	TimestampHeader = "X-Webhook-Timestamp"
)

// ReplayCache is the caller-supplied replay-detection hook.
// CheckAndInsert returns true when the (timestamp, signature) pair was
// already seen (duplicate — reject) and records it otherwise.
// Implementations: CM's runner.SignatureCache; cmr keeps its webhook-layer
// ReplayCache outside Verify and passes nil here.
type ReplayCache interface {
	CheckAndInsert(timestamp, signature string) bool
}

// SignPayloadWithTimestamp computes an HMAC-SHA256 signature bound to the
// HTTP method, request URI, timestamp, and body. The signed content is:
//
//	method + "\n" + uri + "\n" + timestamp + "." + body
//
// uri is the request-target form (path + "?" + raw query, or just path when
// no query is present) — the same value `r.URL.RequestURI()` returns on the
// receiving side.
//
// Including method and URI prevents a valid signature for one endpoint from
// being replayed against another endpoint with an identical body. Binding
// the query string also prevents two concurrent requests to the same path
// (e.g. GET /logs?project=A vs GET /logs?project=B) from producing
// identical signatures and colliding in the receiver's replay cache when
// issued in the same Unix second.
func SignPayloadWithTimestamp(key, method, uri string, body []byte, ts string) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(method))
	mac.Write([]byte("\n"))
	mac.Write([]byte(uri))
	mac.Write([]byte("\n"))
	mac.Write([]byte(ts))
	mac.Write([]byte("."))
	mac.Write(body)

	return hex.EncodeToString(mac.Sum(nil))
}

// SignRequestHeaders computes the X-Signature-256 and X-Webhook-Timestamp
// header values for an outbound request. Use a nil body for GET requests.
func SignRequestHeaders(key, method, uri string, body []byte) (sigHeader, tsHeader string) {
	ts := strconv.FormatInt(time.Now().Unix(), 10)

	return "sha256=" + SignPayloadWithTimestamp(key, method, uri, body, ts), ts
}

// VerifySignatureWithTimestamp checks the signature over
// method/uri/timestamp/body and rejects timestamps outside the asymmetric
// skew window: up to maxSkew in the past, DefaultMaxFutureSkew in the future.
// signature is the raw hex value (without the "sha256=" prefix).
// If cache is non-nil, duplicate (timestamp, signature) pairs are rejected.
func VerifySignatureWithTimestamp(key, method, uri, signature, timestamp string, body []byte, maxSkew time.Duration, cache ReplayCache) bool {
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return false
	}

	age := time.Since(time.Unix(ts, 0))
	if age < -DefaultMaxFutureSkew || age > maxSkew {
		return false
	}

	expected := SignPayloadWithTimestamp(key, method, uri, body, timestamp)
	if !hmac.Equal([]byte(expected), []byte(signature)) {
		return false
	}

	if cache != nil && cache.CheckAndInsert(timestamp, signature) {
		return false
	}

	return true
}
