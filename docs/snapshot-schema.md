# Snapshot schema

Current schema: **2** (WhatChanged v0.2).

Snapshots are append-only JSON files in `.what-changed/`. A successful command
creates a new file; it never overwrites an older known-good snapshot.

## Schema 2 changes

- `commandId` identifies the normalized project-root/executable/argument tuple.
- Every file is represented in `files`, including files over the hash limit.
- `files[*].tracked` explicitly distinguishes a content hash from metadata-only
  presence.
- `files[*].reason` is `size-limit` for an oversized file.
- `files[*].mtimeUnixNano` enables guarded reuse of a prior content hash.
- `environment[*].sensitivity` is `safe`, `unknown`, or `secret-name`.
- `environment[*].value` is emitted only for the explicit safe allowlist.
- `projectContext` stores sorted lightweight repository facts used for scoring.
- `stats.fileHashesReused` reports hashes reused during the scan.
- `complete.projectContext` records whether all bounded context inputs parsed.

Example oversized file:

```json
{
  "sha256": "",
  "size": 7340032,
  "mtimeUnixNano": 1787244000000000000,
  "kind": "other",
  "tracked": false,
  "reason": "size-limit"
}
```

Example hidden environment value:

```json
{
  "sha256": "e3b0c44298fc...",
  "sensitivity": "unknown"
}
```

## Schema 1 loading

Schema 1 remains readable. During in-memory migration:

1. A file with a SHA-256 digest becomes `tracked: true`.
2. Non-allowlisted environment plaintext is cleared before any result is built.
3. `sensitivity` is assigned from the allowlist and secret-name heuristics.
4. A command ID is derived from the stored root and trigger command when both
   are available.
5. Project context is marked incomplete because schema 1 did not contain it.

Legacy files are not modified on disk. Unsupported future schema versions fail
closed instead of being interpreted incorrectly.

