package protocol

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// Pins the optional fallback usage on the status callback: a present Usage
// marshals with a "usage" key and round-trips; an absent Usage stays
// omitted, so pre-v0.10.0 senders see identical bytes.
func TestStatusCallbackPayloadUsageRoundTrip(t *testing.T) {
	in := StatusCallbackPayload{
		CardID: "CMX-1", Project: "p", WorkerStatus: "failed",
		Usage: &CallbackUsage{Model: "m", PromptTokens: 10, CompletionTokens: 5, CostUSD: 0.02},
	}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"usage"`) {
		t.Errorf("usage field missing from wire: %s", b)
	}

	var out StatusCallbackPayload
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(in.Usage, out.Usage) {
		t.Errorf("round-trip mismatch:\n got %+v\nwant %+v", out.Usage, in.Usage)
	}

	// Absent usage stays omitted (back-compat with pre-v0.10.0 senders).
	b2, err := json.Marshal(StatusCallbackPayload{CardID: "CMX-1", Project: "p", WorkerStatus: "failed"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b2), `"usage"`) {
		t.Errorf("absent usage must be omitted, got %s", b2)
	}
}
