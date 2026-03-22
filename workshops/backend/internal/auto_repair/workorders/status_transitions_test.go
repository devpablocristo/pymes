package workorders

import "testing"

func TestNormalizeWorkOrderStatusLegacy(t *testing.T) {
	t.Parallel()
	if g := normalizeWorkOrderStatus("diagnosis"); g != "diagnosing" {
		t.Fatalf("diagnosis -> %q", g)
	}
	if g := normalizeWorkOrderStatus("ready"); g != "ready_for_pickup" {
		t.Fatalf("ready -> %q", g)
	}
	if g := normalizeWorkOrderStatus(""); g != "received" {
		t.Fatalf("empty -> %q", g)
	}
}
