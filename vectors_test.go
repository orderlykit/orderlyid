package orderlyid

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type specVectors struct {
	Vectors []vector `json:"vectors"`
}

type vector struct {
	Desc        string `json:"desc"`
	Prefix      string `json:"prefix"`
	TimeMs      int64  `json:"time_ms"`
	Flags       uint8  `json:"flags"`
	Tenant      uint16 `json:"tenant"`
	Seq         uint16 `json:"seq"` // expect 0..4095
	Shard       uint16 `json:"shard"`
	RandomHex   string `json:"random_hex"` // 60 bits (we will mask lower 60)
	ID          string `json:"id"`         // canonical id to compare; may include checksum suffix
	ExpectError bool   `json:"expect_error"`
}

func loadVectors(t *testing.T) specVectors {
	t.Helper()

	// libs/go -> ./spec/test-vectors.json
	path := filepath.Join(".", "spec", "test-vectors.json")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var v specVectors
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatalf("unmarshal vectors: %v", err)
	}
	if len(v.Vectors) == 0 {
		t.Fatalf("no vectors found in %s", path)
	}
	return v
}

// buildID reconstructs an OrderlyID string from vector fields.
// It uses internal helpers (pack, b32encode, checksum4Base) since this test lives in the same package.
func buildID(v vector) (string, error) {
	withChecksum := strings.Contains(v.ID, "-")
	return NewFromPartsHex(Components{
		Prefix: v.Prefix,
		TimeMs: v.TimeMs,
		Flags:  v.Flags,
		Tenant: v.Tenant,
		Seq:    v.Seq,
		Shard:  v.Shard,
	}, v.RandomHex, withChecksum)
}

func isInvalid(v vector) bool {
	// Simple convention: any vector whose description contains "invalid"
	// is expected to fail parsing.
	return strings.Contains(strings.ToLower(v.Desc), "invalid")
}

func TestSpecVectors_EncodeMatches(t *testing.T) {
	vset := loadVectors(t)
	for _, vec := range vset.Vectors {
		if isInvalid(vec) {
			// Skip encode comparison for invalid cases (they purposefully have bad data)
			continue
		}
		got, err := buildID(vec)
		if err != nil {
			t.Fatalf("[%s] buildID: %v", vec.Desc, err)
		}
		if got != vec.ID {
			t.Fatalf("[%s] encode mismatch:\n got: %s\nwant: %s", vec.Desc, got, vec.ID)
		}
	}
}

func TestSpecVectors_ParseMatches(t *testing.T) {
	vset := loadVectors(t)
	for _, vec := range vset.Vectors {
		parsed, err := Parse(vec.ID)
		if vec.ExpectError {
			if err == nil {
				t.Fatalf("[%s] expected parse error, but got none for id=%s", vec.Desc, vec.ID)
			}
			continue
		}
		if err != nil {
			t.Fatalf("[%s] parse: %v", vec.Desc, err)
		}

		if isInvalid(vec) {
			if err == nil {
				t.Fatalf("[%s] expected parse error, but got none for id=%s", vec.Desc, vec.ID)
			}
			continue
		}
		if err != nil {
			t.Fatalf("[%s] parse: %v", vec.Desc, err)
		}
		// Check prefix
		if parsed.Prefix != vec.Prefix {
			t.Fatalf("[%s] prefix mismatch: got=%s want=%s", vec.Desc, parsed.Prefix, vec.Prefix)
		}
		// Check time (allow exact ms equality)
		if parsed.TimeMs != vec.TimeMs {
			t.Fatalf("[%s] time_ms mismatch: got=%d want=%d (%s vs %s)",
				vec.Desc, parsed.TimeMs, vec.TimeMs,
				time.UnixMilli(parsed.TimeMs).UTC().Format(time.RFC3339Nano),
				time.UnixMilli(vec.TimeMs).UTC().Format(time.RFC3339Nano),
			)
		}
		// Flags/tenant/seq/shard
		if parsed.Flags != vec.Flags {
			t.Fatalf("[%s] flags mismatch: got=0x%02x want=0x%02x", vec.Desc, parsed.Flags, vec.Flags)
		}
		if parsed.Tenant != vec.Tenant {
			t.Fatalf("[%s] tenant mismatch: got=%d want=%d", vec.Desc, parsed.Tenant, vec.Tenant)
		}
		if parsed.Seq != (vec.Seq & 0x0FFF) {
			t.Fatalf("[%s] seq mismatch: got=%d want=%d", vec.Desc, parsed.Seq, vec.Seq&0x0FFF)
		}
		if parsed.Shard != vec.Shard {
			t.Fatalf("[%s] shard mismatch: got=%d want=%d", vec.Desc, parsed.Shard, vec.Shard)
		}
		// Random: Parse exposes Random as 60-bit value. Compare masked.
		wantRnd, err := hexTo60(vec.RandomHex)
		if err != nil {
			t.Fatalf("[%s] random decode: %v", vec.Desc, err)
		}
		if parsed.Random != wantRnd {
			t.Fatalf("[%s] random60 mismatch: got=0x%x want=0x%x", vec.Desc, parsed.Random, wantRnd)
		}
	}
}

func hexTo60(h string) (uint64, error) {
	b, err := hex.DecodeString(h)
	if err != nil {
		return 0, err
	}
	if len(b) == 0 {
		return 0, errors.New("empty random_hex")
	}
	var u uint64
	for _, x := range b {
		u = (u << 8) | uint64(x)
	}
	return u & ((1 << 60) - 1), nil
}
