# Conformance Tool

The **conformance tool** validates an implementation of OrderlyID against the
official test vectors in [`spec/test-vectors.json`](../../spec/test-vectors.json).

All language libraries claiming **OrderlyID v1** support must pass this tool.

## Usage

### Go
From the repo root:

```sh
go run ./tools/conformance --impl=go
```

### Other languages
Run your library’s encode/decode functions and pipe the results into this tool
using the JSON schema defined in `spec/test-vectors.json`.

Example (pseudo):

```sh
cat spec/test-vectors.json | my-lang-impl verify | go run ./tools/conformance
```

## What it checks
- Encoding of binary layout → canonical string
- Parsing of canonical string → binary layout
- Checksum validation
- Lexicographic ordering semantics
- Rejection of invalid cases

## Notes
- CI will run this tool for the Go reference implementation.
- Third-party implementations should vendor or fetch `spec/test-vectors.json`
  to stay in sync with spec updates.
