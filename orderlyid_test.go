package orderlyid

import (
	"errors"
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
	} else if !errors.Is(err, ErrInvalidChecksum) {
		t.Fatalf("expected ErrInvalidChecksum, got %v", err)
	}
}

func TestParseErrorsSupportErrorsIs(t *testing.T) {
	tests := []struct {
		name string
		id   string
		want error
	}{
		{name: "format", id: "order", want: ErrInvalidFormat},
		{name: "prefix", id: "Order_00000000000000000000000000000000", want: ErrInvalidPrefix},
		{name: "payload length", id: "order_123", want: ErrInvalidPayloadLength},
		{name: "base32", id: "order_0000000000000000000000000000000!", want: ErrInvalidBase32},
		{name: "checksum length", id: "order_00000000000000000000000000000000-123", want: ErrInvalidChecksum},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(tt.id)
			if err == nil {
				t.Fatalf("expected error")
			}
			if !errors.Is(err, tt.want) {
				t.Fatalf("expected %v, got %v", tt.want, err)
			}
		})
	}
}

func TestNewFromPartsErrorsSupportErrorsIs(t *testing.T) {
	if _, err := NewFromParts(Components{Prefix: "Bad!"}, false); err == nil {
		t.Fatalf("expected invalid prefix error")
	} else if !errors.Is(err, ErrInvalidPrefix) {
		t.Fatalf("expected ErrInvalidPrefix, got %v", err)
	}

	if _, err := NewFromPartsHex(Components{Prefix: "order"}, "zz", false); err == nil {
		t.Fatalf("expected invalid random hex error")
	} else if !errors.Is(err, ErrInvalidRandomHex) {
		t.Fatalf("expected ErrInvalidRandomHex, got %v", err)
	}
}
