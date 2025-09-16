package orderlyid

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

type options struct {
	tenant        uint16
	shard         uint16
	withChecksum  bool
	bucketSeconds int
}

type Option func(*options)

func WithTenant(t uint16) Option {
	return func(o *options) {
		o.tenant = t
	}
}

func WithShard(s uint16) Option {
	return func(o *options) {
		o.shard = s
	}
}

func WithShardFromBytes(b []byte) Option {
	return func(o *options) {
		var h uint32
		for _, by := range b {
			h = (h * 16777619) ^ uint32(by) // FNV-ish
		}
		o.shard = uint16(h & 0xFFFF)
	}
}

func WithChecksum(v bool) Option {
	return func(o *options) {
		o.withChecksum = v
	}
}

func WithBucketSeconds(sec int) Option {
	return func(o *options) {
		o.bucketSeconds = sec
	}
}

var (
	alpha          = []byte("0123456789abcdefghjkmnpqrstvwxyz") // crockford, lowercase
	alphaRev       [256]byte
	alphaValidMask [256]bool
	prefixRe       = regexp.MustCompile(`^[a-z][a-z0-9]{1,30}$`)
)

func init() {
	for i := range alphaRev {
		alphaRev[i] = 0xFF
	}
	for i, b := range alpha {
		alphaRev[b] = byte(i)
		alphaValidMask[b] = true
		// also accept uppercase
		if b >= 'a' && b <= 'z' {
			alphaRev[byte(strings.ToUpper(string([]byte{b}))[0])] = byte(i)
			alphaValidMask[byte(strings.ToUpper(string([]byte{b}))[0])] = true
		}
	}

	// map ambiguous glyphs to their intended values (Crockford style)
	alphaRev['I'], alphaRev['i'] = alphaRev['1'], alphaRev['1']
	alphaRev['L'], alphaRev['l'] = alphaRev['1'], alphaRev['1']
	alphaRev['O'], alphaRev['o'] = alphaRev['0'], alphaRev['0']
	alphaRev['U'], alphaRev['u'] = alphaRev['v'], alphaRev['v'] // approximate; not canonical, accept only
	alphaValidMask['I'], alphaValidMask['i'] = true, true
	alphaValidMask['L'], alphaValidMask['l'] = true, true
	alphaValidMask['O'], alphaValidMask['o'] = true, true
	alphaValidMask['U'], alphaValidMask['u'] = true, true
}

const (
	versionBits          = 0 // v1
	privacyBitMask       = 1 << 5
	epoch2020      int64 = 1577836800000 // 2020-01-01T00:00:00Z in ms
)

var (
	mu     sync.Mutex
	lastMs int64
	seq12  uint16 // 12-bit
)

// New generates a new OrderlyID string like "order_0r8h...".
func New(prefix string, opts ...Option) string {
	if !prefixRe.MatchString(prefix) {
		panic("invalid prefix")
	}
	var o options
	for _, fn := range opts {
		fn(&o)
	}

	now := time.Now().UTC().UnixMilli()
	if o.bucketSeconds > 0 {
		bs := int64(o.bucketSeconds) * 1000
		now = (now / bs) * bs
	}
	ms := now - epoch2020

	mu.Lock()
	if ms == lastMs {
		seq12 = (seq12 + 1) & 0x0FFF
	} else {
		lastMs = ms
		seq12 = 0
	}
	localSeq := seq12
	mu.Unlock()

	// flags
	var flags byte = 0
	if o.bucketSeconds > 0 {
		flags |= privacyBitMask
	}
	// version in bits 7..6 already 0
	// random 60 bits
	rnd := make([]byte, 8)
	if _, err := rand.Read(rnd); err != nil {
		panic(err)
	}

	// mask top 4 bits to keep 60-bit space when viewed as uint64
	rnd[0] &= 0x0F
	random60 := binary.BigEndian.Uint64(rnd) // upper 4 bits are zero

	body := pack(uint64(ms), flags, o.tenant, localSeq, o.shard, random60)
	payload := b32encode(body[:])

	id := prefix + "_" + payload

	base := prefix + "_" + payload
	if o.withChecksum {
		cs := checksum4Base(base)
		return base + "-" + cs
	}
	return id
}

// Parse decodes an OrderlyID and returns its components.
type Parsed struct {
	Prefix string
	TimeMs int64 // epoch ms (UTC)
	Flags  byte
	Tenant uint16
	Seq    uint16 // 12-bit
	Shard  uint16
	Random uint64 // 60-bit
}

func Parse(s string) (*Parsed, error) {
	s = strings.TrimSpace(s)
	base := s
	if i := strings.LastIndexByte(s, '-'); i >= 0 {
		base = s[:i]
		csGiven := s[i+1:]
		if len(csGiven) != 4 {
			return nil, errors.New("checksum must be 4 chars")
		}
		expected := checksum4Base(base)
		if !strings.EqualFold(csGiven, expected) {
			return nil, errors.New("checksum mismatch")
		}
	}
	i := strings.IndexByte(base, '_')
	if i <= 0 {
		return nil, errors.New("missing prefix separator")
	}
	prefix := base[:i]
	if !prefixRe.MatchString(prefix) {
		return nil, errors.New("invalid prefix")
	}
	payload := base[i+1:]
	if len(payload) != 32 {
		return nil, errors.New("payload must be 32 chars")
	}
	for j := 0; j < 32; j++ {
		if alphaRev[payload[j]] == 0xFF {
			return nil, fmt.Errorf("invalid base32 at pos %d", j)
		}
	}
	buf, err := b32decode(payload)
	if err != nil {
		return nil, err
	}
	ms, flags, tenant, seq, shard, random60 := unpack(buf)
	return &Parsed{
		Prefix: prefix,
		TimeMs: int64(ms) + epoch2020,
		Flags:  flags,
		Tenant: tenant,
		Seq:    seq,
		Shard:  shard,
		Random: random60,
	}, nil
}

// Packing layout (big-endian)
// | 48b time | 8b flags | 16b tenant | 12b seq | 16b shard | 60b random |
func pack(ms uint64, flags byte, tenant uint16, seq12 uint16, shard uint16, random60 uint64) (out [20]byte) {
	// time 48b
	out[0] = byte(ms >> 40)
	out[1] = byte(ms >> 32)
	out[2] = byte(ms >> 24)
	out[3] = byte(ms >> 16)
	out[4] = byte(ms >> 8)
	out[5] = byte(ms)
	// flags
	out[6] = flags
	// tenant 16b
	out[7] = byte(tenant >> 8)
	out[8] = byte(tenant)
	// seq 12b
	out[9] = byte(seq12 >> 4)
	out[10] = byte((seq12 & 0x0F) << 4)
	// shard 16b spanned
	out[10] |= byte((shard >> 12) & 0x0F)
	out[11] = byte((shard >> 4) & 0xFF)
	out[12] = byte((shard & 0x0F) << 4)
	// random 60b
	out[12] |= byte((random60 >> 56) & 0x0F)
	out[13] = byte((random60 >> 48) & 0xFF)
	out[14] = byte((random60 >> 40) & 0xFF)
	out[15] = byte((random60 >> 32) & 0xFF)
	out[16] = byte((random60 >> 24) & 0xFF)
	out[17] = byte((random60 >> 16) & 0xFF)
	out[18] = byte((random60 >> 8) & 0xFF)
	out[19] = byte(random60 & 0xFF)
	return
}

func unpack(in []byte) (ms uint64, flags byte, tenant uint16, seq12 uint16, shard uint16, random60 uint64) {
	if len(in) != 20 {
		panic("unpack expects 20 bytes")
	}
	ms = (uint64(in[0]) << 40) | (uint64(in[1]) << 32) | (uint64(in[2]) << 24) | (uint64(in[3]) << 16) | (uint64(in[4]) << 8) | uint64(in[5])
	flags = in[6]
	tenant = (uint16(in[7]) << 8) | uint16(in[8])
	seq12 = (uint16(in[9]) << 4) | (uint16(in[10]&0xF0) >> 4)
	shard = (uint16(in[10]&0x0F) << 12) | (uint16(in[11]) << 4) | uint16((in[12]&0xF0)>>4)
	random60 = (uint64(in[12]&0x0F) << 56) | (uint64(in[13]) << 48) | (uint64(in[14]) << 40) | (uint64(in[15]) << 32) | (uint64(in[16]) << 24) | (uint64(in[17]) << 16) | (uint64(in[18]) << 8) | uint64(in[19])
	return
}

func b32encode(src []byte) string {
	if len(src) != 20 {
		panic("b32encode expects 20 bytes")
	}
	var out [32]byte
	var acc uint32
	var bits uint
	var j int
	for _, b := range src {
		acc = (acc << 8) | uint32(b)
		bits += 8
		for bits >= 5 {
			bits -= 5
			idx := byte((acc >> bits) & 31)
			out[j] = alpha[idx]
			j++
		}
	}
	if bits != 0 { // for 160 bits, this should be zero
		idx := byte((acc << (5 - bits)) & 31)
		out[j] = alpha[idx]
		j++
	}
	return string(out[:])
}

func b32decode(s string) ([]byte, error) {
	if len(s) != 32 {
		return nil, errors.New("base32 length must be 32")
	}
	out := make([]byte, 20)
	var acc uint32
	var bits uint
	var j int
	for i := 0; i < len(s); i++ {
		v := alphaRev[s[i]]
		if v == 0xFF {
			return nil, fmt.Errorf("invalid base32 at %d", i)
		}
		acc = (acc << 5) | uint32(v)
		bits += 5
		if bits >= 8 {
			bits -= 8
			out[j] = byte((acc >> bits) & 0xFF)
			j++
		}
	}
	if j != 20 || bits != 0 {
		return nil, errors.New("invalid base32 payload")
	}
	return out, nil
}

// Checksum (Bech32-style polymod, 4 chars = 20 bits)
func checksum4Base(base string) string {
	// base is "prefix_payload"; use hrp = "prefix_" (lowercased), data = payload indices
	idx := strings.IndexByte(base, '_')
	if idx <= 0 {
		panic("checksum4Base: bad base")
	}
	hrp := strings.ToLower(base[:idx+1]) // include '_'
	payload := strings.ToLower(base[idx+1:])
	values := hrpExpand(hrp)
	values = append(values, payloadTo5bits(payload)...)
	// append 4 zero groups for 20-bit checksum
	values = append(values, 0, 0, 0, 0)
	pm := bech32Polymod(values) ^ 1
	// top 20 bits as 4 Base32 chars
	var out [4]byte
	for i := 0; i < 4; i++ {
		idx := byte((pm >> uint(5*(3-i))) & 31)
		out[i] = alpha[idx]
	}
	return string(out[:])
}

func payloadTo5bits(payload string) []byte {
	out := make([]byte, len(payload))
	for i := 0; i < len(payload); i++ {
		v := alphaRev[payload[i]]
		if v == 0xFF {
			panic("invalid payload for checksum")
		}
		out[i] = v
	}
	return out
}

func hrpExpand(s string) []byte {
	vals := make([]byte, 0, len(s)*2+1)
	for i := 0; i < len(s); i++ {
		vals = append(vals, byte(s[i])>>5)
	}
	vals = append(vals, 0)
	for i := 0; i < len(s); i++ {
		vals = append(vals, byte(s[i])&31)
	}
	return vals
}

func bech32Polymod(values []byte) uint32 {
	chk := uint32(1)
	for _, v := range values {
		b := chk >> 25
		chk = ((chk & 0x1ffffff) << 5) ^ uint32(v)
		if (b & 0x01) != 0 {
			chk ^= 0x3b6a57b2
		}
		if (b & 0x02) != 0 {
			chk ^= 0x26508e6d
		}
		if (b & 0x04) != 0 {
			chk ^= 0x1ea119fa
		}
		if (b & 0x08) != 0 {
			chk ^= 0x3d4233dd
		}
		if (b & 0x10) != 0 {
			chk ^= 0x2a1462b3
		}
	}
	return chk
}
