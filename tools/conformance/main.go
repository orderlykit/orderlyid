package main

import (
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/orderlykit/orderlyid"
)

type vectorsFile struct {
	Vectors []vector `json:"vectors"`
}

type vector struct {
	Desc        string `json:"desc"`
	Prefix      string `json:"prefix"`
	TimeMs      int64  `json:"time_ms"`
	Flags       uint8  `json:"flags"`
	Tenant      uint16 `json:"tenant"`
	Seq         uint16 `json:"seq"` // 0..4095 (masked by lib)
	Shard       uint16 `json:"shard"`
	RandomHex   string `json:"random_hex"` // big-endian; we mask to 60 bits
	ID          string `json:"id"`         // canonical string (may have checksum)
	ExpectError bool   `json:"expect_error"`
}

var (
	vectorsPath = flag.String("vectors", filepath.Join("spec", "test-vectors.json"), "path to spec/test-vectors.json")
	verbose     = flag.Bool("v", false, "verbose output")
	failFast    = flag.Bool("fail-fast", false, "stop on first failure")
)

func main() {
	flag.Parse()

	data, err := os.ReadFile(*vectorsPath)
	if err != nil {
		die("read %s: %v", *vectorsPath, err)
	}

	var vf vectorsFile
	if err := json.Unmarshal(data, &vf); err != nil {
		die("parse vectors json: %v", err)
	}
	if len(vf.Vectors) == 0 {
		die("no vectors found in %s", *vectorsPath)
	}

	var encOK, encFail, parseOK, parseFail int

	for i, vc := range vf.Vectors {
		prefix := fmt.Sprintf("[%02d] %s", i, vc.Desc)

		// -------- Encode check (only for valid cases) --------
		if !vc.ExpectError {
			withChecksum := strings.Contains(vc.ID, "-")
			got, err := orderlyid.NewFromPartsHex(orderlyid.Components{
				Prefix: vc.Prefix,
				TimeMs: vc.TimeMs,
				Flags:  vc.Flags,
				Tenant: vc.Tenant,
				Seq:    vc.Seq,
				Shard:  vc.Shard,
				// Random60 is set by hex in NewFromPartsHex
			}, vc.RandomHex, withChecksum)
			if err != nil {
				encFail++
				fail("%s encode error: %v", prefix, err)
				if *failFast {
					exitWith(encOK, encFail, parseOK, parseFail)
				}
			} else if got != vc.ID {
				encFail++
				fail("%s encode mismatch:\n  got:  %s\n  want: %s", prefix, got, vc.ID)
				if *failFast {
					exitWith(encOK, encFail, parseOK, parseFail)
				}
			} else {
				encOK++
				if *verbose {
					ok("%s encode ok", prefix)
				}
			}
		}

		// -------- Parse check (all cases) --------
		parsed, err := orderlyid.Parse(vc.ID)
		if vc.ExpectError {
			if err == nil {
				parseFail++
				fail("%s parse expected error, got none; id=%s", prefix, vc.ID)
				if *failFast {
					exitWith(encOK, encFail, parseOK, parseFail)
				}
			} else if *verbose {
				ok("%s parse correctly failed: %v", prefix, err)
				parseOK++
			}
			continue
		}
		if err != nil {
			parseFail++
			fail("%s parse error: %v", prefix, err)
			if *failFast {
				exitWith(encOK, encFail, parseOK, parseFail)
			}
			continue
		}

		// Field checks (for valid vectors)
		if parsed.Prefix != vc.Prefix {
			parseFail++
			fail("%s prefix mismatch: got=%s want=%s", prefix, parsed.Prefix, vc.Prefix)
			continue
		}
		if parsed.TimeMs != vc.TimeMs {
			parseFail++
			fail("%s time_ms mismatch: got=%d want=%d", prefix, parsed.TimeMs, vc.TimeMs)
			continue
		}
		if parsed.Flags != vc.Flags {
			parseFail++
			fail("%s flags mismatch: got=0x%02x want=0x%02x", prefix, parsed.Flags, vc.Flags)
			continue
		}
		if parsed.Tenant != vc.Tenant {
			parseFail++
			fail("%s tenant mismatch: got=%d want=%d", prefix, parsed.Tenant, vc.Tenant)
			continue
		}
		if parsed.Seq != (vc.Seq & 0x0FFF) {
			parseFail++
			fail("%s seq mismatch: got=%d want=%d", prefix, parsed.Seq, vc.Seq&0x0FFF)
			continue
		}
		if parsed.Shard != vc.Shard {
			parseFail++
			fail("%s shard mismatch: got=%d want=%d", prefix, parsed.Shard, vc.Shard)
			continue
		}
		wantRnd, err := hexTo60(vc.RandomHex)
		if err != nil {
			parseFail++
			fail("%s random_hex invalid: %v", prefix, err)
			continue
		}
		if parsed.Random != wantRnd {
			parseFail++
			fail("%s random60 mismatch: got=0x%x want=0x%x", prefix, parsed.Random, wantRnd)
			continue
		}

		parseOK++
		if *verbose {
			ok("%s parse ok", prefix)
		}
	}

	exitWith(encOK, encFail, parseOK, parseFail)
}

func hexTo60(h string) (uint64, error) {
	b, err := hex.DecodeString(h)
	if err != nil {
		return 0, err
	}
	var u uint64
	for _, x := range b {
		u = (u << 8) | uint64(x)
	}
	return u & ((1 << 60) - 1), nil
}

func ok(format string, args ...any)   { fmt.Printf("✓ "+format+"\n", args...) }
func fail(format string, args ...any) { fmt.Printf("✗ "+format+"\n", args...) }
func die(format string, args ...any)  { fmt.Fprintf(os.Stderr, format+"\n", args...); os.Exit(2) }

func exitWith(encOK, encFail, parseOK, parseFail int) {
	fmt.Printf("\nEncode: %d ok, %d fail\nParse:  %d ok, %d fail\n", encOK, encFail, parseOK, parseFail)
	if encFail+parseFail > 0 {
		os.Exit(1)
	}
	os.Exit(0)
}
