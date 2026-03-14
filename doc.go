// Package orderlyid generates and parses typed, lexicographically sortable IDs.
//
// An OrderlyID has the canonical form "prefix_payload" or
// "prefix_payload-checksum". The prefix is a lowercase typed identifier such as
// "order" or "user" and must match [a-z][a-z0-9]{1,30}. The payload is a
// 32-character Crockford Base32 encoding of a packed 160-bit big-endian body.
//
// The binary body contains:
//
//   - timestamp: 48-bit milliseconds since 2020-01-01T00:00:00Z, which makes IDs
//     approximately ordered by creation time
//   - flags: an 8-bit field where bits 7..6 carry the wire version, bit 5 marks
//     privacy bucketing, and bits 4..0 are reserved
//   - tenant: a 16-bit tenant identifier for multi-tenant systems
//   - sequence: a 12-bit counter for bursts within the same millisecond
//   - shard: a 16-bit routing hint, either provided directly or derived from
//     bytes
//   - random: 60 bits of cryptographic randomness
//
// Canonical output from this package is lowercase. Parsing is
// case-insensitive for payload and checksum characters, but prefixes must still
// satisfy the lowercase prefix rule. A trailing 4-character checksum is
// optional and can be used to detect copy/paste and transcription errors.
package orderlyid
