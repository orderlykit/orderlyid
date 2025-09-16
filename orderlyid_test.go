package orderlyid

import (
	"strings"
	"testing"
	"time"
)

func TestNewAndParse(t *testing.T) {
	id := New("order", WithTenant(12), WithShard(34))
	if !strings.HasPrefix(id, "order_") {
		t.Fatalf("prefix missing: %s", id)
	}
	p, err := Parse(id)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if p.Prefix != "order" {
		t.Fatalf("prefix mismatch")
	}
	if p.Tenant != 12 || p.Shard != 34 {
		t.Fatalf("tenant/shard mismatch")
	}
	if time.Since(time.UnixMilli(p.TimeMs)) > time.Minute {
		t.Fatalf("time looks off: %d", p.TimeMs)
	}
}

func TestLexicographicOrderFollowsTime(t *testing.T) {
	id1 := New("user")
	time.Sleep(2 * time.Millisecond)
	id2 := New("user")
	if !(id1 < id2) {
		t.Fatalf("expected id1 < id2: %s vs %s", id1, id2)
	}
}

func TestRejectBadPrefix(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatalf("expected panic on bad prefix")
		}
	}()
	_ = New("Bad!")
}

func TestChecksumRoundTrip(t *testing.T) {
	id := New("order", WithChecksum(true))
	if !strings.Contains(id, "-") || len(id) != len("order_")+32+1+4 {
		t.Fatalf("checksum not appended: %s", id)
	}
	if _, err := Parse(id); err != nil {
		t.Fatalf("parse with checksum failed: %v", err)
	}
	// Tamper last char
	bad := id[:len(id)-1] + "0"
	if _, err := Parse(bad); err == nil {
		t.Fatalf("expected checksum mismatch")
	}
}
