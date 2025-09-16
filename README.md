# OrderlyID

**OrderlyID** is a typed, time-sortable, globally unique identifier format with optional checksums and built-in fields for tenancy and sharding.
It is designed for distributed systems, developer ergonomics, and safer use in public APIs.

> Status: **Draft v0.1** (spec + reference library: Go)

---

## TL;DR

OrderlyID = `<type>_<payload>[-<checksum>]`

- **Typed**: human-readable `type` prefix (`order_...`, `user_...`)
- **K-sortable**: lexicographic order ≈ creation time (UUIDv7-style)
- **Globally unique**: safe to mint in any service/region
- **DX-friendly**: great in logs, APIs, and distributed systems
- **Optional extras**: checksum to catch copy/paste errors; fields for tenant/shard/sequence

If you currently use `AUTO_INCREMENT` or UUIDv4, OrderlyID improves **safety, scale, and developer ergonomics** with minimal fuss.

---

## Why not just INT IDs or UUIDv4?

**vs AUTO_INCREMENT**
- Needs DB round-trips / central sequences
- Predictable & leaks volume (bad for public APIs)
- Harder to shard or generate offline

**vs UUIDv4**
- Not time-sortable (needs extra timestamp/index)
- No type context in logs
- Chunky hex; weaker DX

**OrderlyID**
- App-side generation, K-sortable, self-describing (`order_...`)
- Works across services/regions/edge; fits SQL & NoSQL (DynamoDB) alike
- Optional checksum for fat-finger protection

---

## Developer use cases

- **Greppable logs**: `shipment_...` vs random hex → faster triage
- **Cursor pagination**: `WHERE id < :cursor ORDER BY id DESC LIMIT N`
- **Idempotency keys**: stable across HTTP, queues, DB writes
- **Cross-service correlation**: put the same typed ID in headers/metrics
- **Offline generation**: mint IDs on mobile/edge; sync later
- **Multi-tenant routing** (optional field): cheap partitioning/archival
- **Global feeds**: shard by `hash(id)` and merge per-shard tails
- **Fixtures & tests**: readable seed data (`user_*`, `order_*`)
- **Operational safety**: checksum catches copy/paste/transcription errors

---

## Format

### Canonical string
```
<prefix>_<payload>[-<checksum>]
```
- `prefix`: `[a-z][a-z0-9]{1,30}` (entity type; lowercase)
- `_`: required separator
- `payload`: Base32 (Crockford) of a **160-bit** body (exactly 32 chars)
- `checksum`: optional 4 chars (Bech32-style polymod over `<prefix>_<payload>`)

### Binary layout (160 bits total)
```
| 48b time | 8b flags | 16b tenant | 12b seq | 16b shard | 60b random |
```
- **time (48b)**: Unix ms since 2020-01-01 UTC (epoch shift trims bits)
- **flags (8b)**: version + privacy marker
- **tenant (16b)**: org/region id (0 if unused)
- **seq (12b)**: per-node monotonic counter for same-ms bursts
- **shard (16b)**: storage/routing hint
- **random (60b)**: CSPRNG entropy

**Properties**
- Lexicographic order ≈ `time → flags → tenant → seq → shard → random`
- Canonical output is lowercase; parsers MAY accept uppercase
- URL/file safe; stable length; checksum optional

---

## Quick start

### Go
```go
import "github.com/kpiljoong/orderlyid"

id := orderlyid.New("order",
  orderlyid.WithTenant(12),
  orderlyid.WithShardFromBytes([]byte("customer")),
  orderlyid.WithChecksum(true),
)
// => "order_00myngy59c0003000dfk59mg3e36j3rr-9xgg"
```

### CLI
```sh
go run ./cmd/orderlyid -prefix order -tenant 12 -n 3
go run ./cmd/orderlyid -parse order_00myngy59c0003000dfk59mg3e36j3rr-9xgg
```

---

## DynamoDB & SQL patterns

**DynamoDB timeline**
`PK = ITEM#{itemId}`, `SK = EVT#{orderlyid}` → `Query` gives "latest first"

**Global lookup by public ID**
GSI: `GSI1PK = PUBLICID#{orderlyid}`, `GSI1SK = CONST`

**Postgres / MySQL**
- Store both `id_text VARCHAR(64) UNIQUE` and `id_bin BINARY(20)`
- Index `id_bin` for range scans
- Use `id_text` for APIs/logs

---

## Migration tips
- Dual-write a `public_id` column (keep your existing PK)
- Accept both old and new IDs at read edges
- Prefer new IDs in logs/metrics immediately
- Backfill in batches; flip API docs once ready

## Spec
- `spec/0001-spec.md` — normative spec
- `spec/test-vectors.json` — conformance vectors
- `spec/README.md` — overview and guidance

---

## Prior Art & Acknowledgements
OrderlyID builds on a long line of identifier formats and open-source work:
- UUID (RFC 4122) — the original 128-bit unique identifier
- ULID — introduced time-sortable IDs with a Base32 encoding
- UUIDv7 (IETF draft) — formalizes a time-ordered variant
- TypeID — combined typed prefixes with UUIDv7-style bodies

**OrderlyID is inspired by TypeID** but is not a fork. It shares the motivation of human-friendly, typed identifiers while extending the design with:
- A 160-bit body (vs 128-bit) with tenant/shard/seq fields
- Optional 4-char checksum for safer use in public/admin surfaces
- A privacy flag for time bucketing in user-facing contexts

We thank the authors of UUID, ULID, UUIDv7, and TypeID for the foundational ideas.

---

## Contributing
- See `spec/0001-spec.md` and `spec/test-vectors.json`
- All implementations must pass the conformance tool before merge

---

## License
Apache License 2.0 © 2025 Piljoong Kim
