# OrderlyID Specification Index

This folder contains the **OrderlyID** specification and conformance assets.
Use this README as the entry point. The normative technical definition lives in **[`0001-spec.md`](./0001-spec.md)**; this README provides context, guidance, and links.

> Status: Draft v0.1 — stable enough for experimentation.
> Normative spec may evolve. Reference implementation provided in Go.

---

## What is OrderlyID?

A typed, time-sortable, globally unique identifier:

```
<prefix>_<payload>[-<checksum>]
```

- `prefix` — lowercase type label (e.g., `order`, `user`).
- `payload` — Base32 (Crockford) encoding of a **160-bit** binary body.
- `checksum` — optional 4-character integrity check, Bech32-style polymod over `"<prefix>_" + payload`.

```
Example:
order_00myngy59c0003000dfk59mg3e36j3rr-9xgg
prefix: order
payload: 32 chars = 160 bits (time=…, tenant=…, seq=…, shard=…, random=…)
checksum: 9xgg
```

---

## Goals

- **K-sortable** — lexicographic order ≈ creation time.
- **Self-describing** — `type_...` improves developer ergonomics.
- **Distributed-friendly** — safe to mint in any service/region.
- **Operational safety** — optional checksum catches copy/paste errors.

---

## Spec files

- **[0001-spec.md](./0001-spec.md)** — *Normative*: wire format, encoding rules, parsing, validation.
- **[test-vectors.json](./test-vectors.json)** — *Normative*: shared vectors for conformance.

---

## Wire format (summary)

> Full details and MUST/SHOULD language are in `0001-spec.md`.

### Canonical string
```
<prefix>_<payload>[-<checksum>]
```

- Canonical casing: **lowercase**.
- Parsers SHOULD accept uppercase, but emitters MUST output lowercase.

### Binary layout (160 bits)
```
| 48b time | 8b flags | 16b tenant | 12b seq | 16b shard | 60b random |
```

- `time` — Unix ms since 2020-01-01T00:00:00Z (epoch shift trims bits).
- `flags` — bits7..6 = version (00=v1); bit5 = privacy bucket; bits4..0 = reserved.
- `tenant` — 16-bit optional routing/tenant id.
- `seq` — 12-bit monotonic counter per process, per millisecond.
- `shard` — 16-bit optional routing/storage hint.
- `random` — 60-bit CSPRNG entropy.

### Checksum (4 chars)

- Domain: HRP-expanded `"<prefix>_"` + payload symbols.
- Algorithm: **Bech32 polymod**, truncated to 20 bits, encoded as 4 Crockford Base32 chars.
- Verification: if present, parsers MUST validate and reject mismatches.
- False-accept probability: ~1 in 1,048,576.

---

## Generation (reference behavior)

1. `now_ms = system_clock_ms()`. If privacy bucketing is enabled, quantize to bucket size.
2. Maintain a 12-bit sequence per process for same-ms bursts (wrap allowed).
3. Draw 60 bits from a CSPRNG.
4. Pack fields into the 160-bit body (big-endian).
5. Base32-encode to 32 chars.
6. Emit `"<prefix>_<payload>"`; optionally append `"-<checksum>"`.

*Note*: Lexicographic order within the same prefix ≈ `time → flags → tenant → seq → shard → random`.

---

## Interop & storage guidance

- **SQL**: store both `id_text VARCHAR(64) UNIQUE` and `id_bin BINARY(20)`; index `id_bin` for range scans.
- **DynamoDB**: use ID as sort key; to avoid hot partitions, hash into virtual shards.
- **APIs & logs**: prefer the typed public ID; redact tail chars if sensitive.

---

## Versioning policy

- Wire version is in `flags` bits7..6 (`00` = v1).
- Minor doc updates without wire changes → bump document version only (`v0.1 → v0.2`).
- Incompatible layout changes MUST increment wire version; encodings are not reused.

---

## Conformance

An implementation claiming **OrderlyID v1** support MUST:

- Emit canonical lowercase strings.
- Accept mixed-case input.
- Validate checksum if present.
- Reject unknown wire versions.
- Pass all vectors in `spec/test-vectors.json`.
- Preserve lexicographic ordering semantics for IDs of the same prefix.

A Go conformance tool is provided under [`tools/conformance`](../tools/conformance/).

---

## Security & privacy

- IDs expose coarse creation time by design. Use **privacy bucketing** for public-facing surfaces.
- Checksum = integrity only (not authentication).
- Always use a cryptographically secure RNG for the random field.

---

## FAQ

**Q: Do I need the checksum everywhere?**
No. Use it in external/public APIs. Internally it’s optional.

**Q: Can I omit tenant/shard?**
Yes — set them to zero. The layout is fixed.

**Q: What happens if seq wraps?**
IDs remain unique (random field ensures this), but strict monotonicity within the same millisecond is no longer guaranteed past 4096 IDs.

**Q: Is OrderlyID compatible with UUID?**
No, but you can store alongside UUIDs. Both are 128+ bit identifiers.

**Q: How does ordering work across prefixes?**
Only meaningful within the same prefix. For mixed-type feeds, include the prefix in sort key.

---

## Next steps

- Finalize `test-vectors.json` from the Go reference implementation.
- Add additional language bindings once the spec stabilizes.
