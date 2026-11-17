package security

import (
	"testing"
	"time"
)

func TestComputeShiftBounds_RegularAndMidnight(t *testing.T) {
	shiftDate := time.Date(2026, 11, 12, 0, 0, 0, 0, JakartaLoc)

	// Day shift 07:00 - 15:00 WIB
	startUTC, endUTC, crosses, err := ComputeShiftBounds(shiftDate, "07:00", "15:00", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if crosses {
		t.Fatal("day shift should not cross midnight")
	}
	startJkt := startUTC.In(JakartaLoc)
	endJkt := endUTC.In(JakartaLoc)
	if startJkt.Format("2006-01-02 15:04") != "2026-11-12 07:00" || endJkt.Format("2006-01-02 15:04") != "2026-11-12 15:00" {
		t.Fatalf("unexpected bounds: %s -> %s", startJkt, endJkt)
	}

	// Night shift 22:00 - 06:00 WIB (auto-detects midnight crossing)
	startUTC, endUTC, crosses, err = ComputeShiftBounds(shiftDate, "22:00", "06:00", false)
	if err != nil {
		t.Fatalf("unexpected error for midnight shift: %v", err)
	}
	if !crosses {
		t.Fatal("expected crosses=true when end <= start")
	}
	startJkt = startUTC.In(JakartaLoc)
	endJkt = endUTC.In(JakartaLoc)
	if startJkt.Format("2006-01-02 15:04") != "2026-11-12 22:00" || endJkt.Format("2006-01-02 15:04") != "2026-11-13 06:00" {
		t.Fatalf("unexpected midnight bounds: %s -> %s", startJkt, endJkt)
	}
}

func TestMaskPhone(t *testing.T) {
	raw := "081234567890"
	masked := MaskPhone(&raw)
	if masked == nil || *masked == raw {
		t.Fatalf("expected phone to be masked, got %v", masked)
	}
	if *masked != "081******890" {
		t.Fatalf("unexpected mask %q", *masked)
	}
}

func TestSplitCompositeTokenAndHash(t *testing.T) {
	id, sec, ok := SplitCompositeToken("11111111-2222-3333-4444-555555555555.mysecret")
	if !ok || id != "11111111-2222-3333-4444-555555555555" || sec != "mysecret" {
		t.Fatalf("unexpected split result: ok=%v id=%s sec=%s", ok, id, sec)
	}
	if _, _, ok := SplitCompositeToken("not-a-uuid.secret"); ok {
		t.Fatal("expected invalid UUID token to be rejected")
	}
	if HashToken("abc") == "abc" || len(HashToken("abc")) != 64 {
		t.Fatal("expected SHA-256 64-char hex digest")
	}
}
