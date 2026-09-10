package selection

import (
	"math"
	"testing"
)

func eq[T comparable](t *testing.T, want, got T, msg ...string) {
	t.Helper()
	if want != got {
		t.Errorf("%swant %v, got %v", prefix(msg), want, got)
	}
}

func neq[T comparable](t *testing.T, notWant, got T, msg ...string) {
	t.Helper()
	if notWant == got {
		t.Errorf("%sgot %v, which must differ", prefix(msg), got)
	}
}

func near(t *testing.T, want, got, tol float64, msg ...string) {
	t.Helper()
	if math.Abs(want-got) > tol {
		t.Errorf("%swant %g, got %g (tolerance %g)", prefix(msg), want, got, tol)
	}
}

func truthy(t *testing.T, got bool, msg ...string) {
	t.Helper()
	if !got {
		t.Errorf("%swant true", prefix(msg))
	}
}

func falsy(t *testing.T, got bool, msg ...string) {
	t.Helper()
	if got {
		t.Errorf("%swant false", prefix(msg))
	}
}

func fatalIf(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func prefix(msg []string) string {
	if len(msg) == 0 {
		return ""
	}
	return msg[0] + ": "
}
