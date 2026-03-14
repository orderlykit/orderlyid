package orderlyid

import (
	"encoding/hex"
	"fmt"
)

// Components describes the public fields packed into an OrderlyID.
type Components struct {
	// Prefix is the type prefix before the underscore separator.
	Prefix string
	// TimeMs is the absolute UTC timestamp in Unix milliseconds.
	TimeMs int64
	// Flags is the raw flags byte stored in the payload.
	Flags uint8
	// Tenant is the embedded 16-bit tenant identifier.
	Tenant uint16
	// Seq is the 12-bit sequence number packed into the payload.
	Seq uint16
	// Shard is the embedded 16-bit shard identifier.
	Shard uint16
	// Random60 is the low 60 bits of random payload data.
	Random60 uint64
}

// NewFromParts builds an OrderlyID from explicit component values.
//
// NewFromParts may return an error wrapping ErrInvalidPrefix.
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

// NewFromPartsHex builds an OrderlyID from explicit component values and a
// big-endian random value encoded as hex.
//
// NewFromPartsHex may return an error wrapping ErrInvalidPrefix or
// ErrInvalidRandomHex.
func NewFromPartsHex(c Components, randomHex string, withChecksum bool) (string, error) {
	rb, err := hex.DecodeString(randomHex)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidRandomHex, err)
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
		return fmt.Errorf("%w: %q must match [a-z][a-z0-9]{1,30}", ErrInvalidPrefix, p)
	}
	return nil
}
