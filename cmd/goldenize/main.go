package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	orderlyid "github.com/kpiljoong/orderlyid"
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
	Seq         uint16 `json:"seq"`
	Shard       uint16 `json:"shard"`
	RandomHex   string `json:"random_hex"`
	ID          string `json:"id"`
	ExpectError bool   `json:"expect_error"`
}

const epoch2020 = int64(1577836800000)

func main() {
	path := filepath.Join("spec", "test-vectors.json")
	b, err := os.ReadFile(path)
	must(err)

	var v specVectors
	must(json.Unmarshal(b, &v))

	changed := false
	for i := range v.Vectors {
		vec := &v.Vectors[i]

		if vec.ExpectError {
			continue
		}

		withChecksum := strings.Contains(vec.ID, "-")
		got, err := orderlyid.NewFromPartsHex(
			orderlyid.Components{
				Prefix: vec.Prefix,
				TimeMs: vec.TimeMs,
				Flags:  vec.Flags,
				Tenant: vec.Tenant,
				Seq:    vec.Seq,
				Shard:  vec.Shard,
			},
			vec.RandomHex,
			withChecksum,
		)
		must(err)
		if vec.ID != got {
			fmt.Printf("update [%s]\n  old: %s\n  new: %s\n", vec.Desc, vec.ID, got)
			vec.ID = got
			changed = true
		}
	}

	if !changed {
		fmt.Println("no changes")
		return
	}
	out, err := json.MarshalIndent(v, "", "  ")
	must(err)
	must(os.WriteFile(path, out, 0o644))
	fmt.Printf("wrote %s\n", path)
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
