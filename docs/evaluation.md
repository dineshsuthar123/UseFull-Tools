# MVP evaluation

## Core friction test

The workflow must meet these acceptance checks:

| Question | Pass condition |
|---|---|
| Does it require configuration before first value? | No. One `run -- <command>` creates the baseline. |
| Can a failed test destroy the known-good reference? | No. The child exit code is preserved and nothing is saved. |
| Does a user need to inspect a raw machine dump? | No. Unchanged data is omitted and findings are ranked. |
| Can the tool expose source contents or obvious credentials? | Source is hashed; credential-like environment values are redacted. |
| Does an optional integration failure break the scan? | No. Detector completeness is stored and unsafe comparisons are skipped. |
| Can output be automated? | Yes. `diff --json`, `mark --json`, and `list --json` are stable JSON. |

## Test strategy

- unit fixtures for file categories, ignore behavior, hashes, secret names, and
  three operating-system port formats;
- comparison tests that assert both score ordering and redaction;
- a partial-scan test that prevents false “file added” findings;
- storage tests that prove history is append-only and latest/named lookup works;
- a subprocess test proving a failing wrapped command returns the same code and
  creates no checkpoint directory;
- renderer tests for concise limiting and disclosure of skipped detectors;
- native end-to-end smoke tests for capture and immediate no-change comparison.

## Benchmark method

`BenchmarkCaptureFiles1000` creates 1,000 small Go files and measures a complete
walk, SHA-256 hash, classification, and map construction. `BenchmarkCompareTenThousandFiles`
compares two 10,000-file snapshots with 100 changed files, including ranking.

Run locally with:

```bash
go test -run '^$' -bench . -benchmem ./internal/snapshot ./internal/compare
```

Measured on 2026-08-20 on Windows/amd64 with an AMD Ryzen 7 8845HS and Go 1.25.6:

| Measurement | Result |
|---|---:|
| Hash and classify 1,000 small files | 56.2 ms/op |
| Compare 10,000 files with 100 changes | 0.59 ms/op |
| Native checkpoint of this repository (29 tracked files plus system detectors) | 348 ms wall time |
| Native no-change diff, including a fresh system scan | 330 ms wall time |

These are local prototype measurements, not cross-platform performance claims.
The external runtime, Git, Docker, and port probes dominate the small-project
wall time; the in-memory diff itself is well below one millisecond at 10,000
files.

## Decision after MVP

The prototype succeeds if a developer can create a known-good baseline without
configuration and the later report surfaces a deliberately changed dependency,
configuration, environment value, or common port above unrelated file noise.

The native smoke scenario passed:

1. `run --name green -- go test ./...` ran the suite and recorded a checkpoint.
2. An immediate `diff --name green` returned no changes.
3. A temporary `go.mod` edit and `REDIS_POOL_SIZE=40` produced dependency score
   96 and environment score 84 as the top two findings.
4. A wrapped command exiting 7 returned exit code 7 and created no checkpoint.

The next validation step should be a small field study: ask 5–10 developers to
wrap their normal test command for one week, then measure (a) checkpoints
created, (b) failure investigations where at least one top-five finding was
useful, (c) time to first useful clue, and (d) false or noisy top-five findings.
Do not add detectors until this evidence identifies a repeated blind spot.
