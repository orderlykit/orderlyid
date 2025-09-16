package orderlyid

import (
	"encoding/hex"
	"fmt"
)

type Components struct {
	Prefix   string
	TimeMs   int64
	Flags    uint8
	Tenant   uint16
	Seq      uint16
	Shard    uint16
	Random60 uint64
}

func NewFromParts(c Components, withChecksum bool) (string, error) {
	if err := validatePrefix(c.Prefix); err != nil {
		return "", err
	}
	// Convert absolute time to ms since 2020-01-01 UTC (epoch2020).
	var msSince2020 uint64
	if c.TimeMs >= epoch2020 {
		msSince2020 = uint64(c.TimeMs - epoch2020)
	} else {
		msSince2020 = 0
	}

	seq12 := c.Seq & 0x0FFF
	rand60 := c.Random60 & ((1 << 60) - 1)

	body := pack(msSince2020, c.Flags, c.Tenant, seq12, c.Shard, rand60)
	payload := b32encode(body[:])
	base := c.Prefix + "_" + payload

	if withChecksum {
		return base + "-" + checksum4Base(base), nil
	}
	return base, nil
}

// NewFromPartsHex is a convenience that accepts random as big-endian hex string.
func NewFromPartsHex(c Components, randomHex string, withChecksum bool) (string, error) {
	rb, err := hex.DecodeString(randomHex)
	if err != nil {
		return "", fmt.Errorf("random_hex: %w", err)
	}
	var u uint64
	for _, b := range rb {
		u = (u << 8) | uint64(b)
	}
	c.Random60 = u & ((1 << 60) - 1)
	return NewFromParts(c, withChecksum)
}

// validatePrefix mirrors your existing prefix regex check.
func validatePrefix(p string) error {
	if !prefixRe.MatchString(p) {
		return fmt.Errorf("invalid prefix %q", p)
	}
	return nil
}
