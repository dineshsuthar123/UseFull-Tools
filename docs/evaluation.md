# v0.2 evaluation

## Core friction test

The workflow must meet these acceptance checks:

| Question | Pass condition |
|---|---|
| Does it require configuration before first value? | No. One `run -- <command>` creates that exact command's baseline. |
| Can different commands replace one another's baseline? | No. Root + executable + ordered arguments produce independent command IDs. |
| Can a failed test destroy the known-good reference? | No. The child exit code is preserved and nothing is saved. |
| Does a user need to inspect a raw machine dump? | No. Unchanged data is omitted and findings are ranked. |
| Can the tool expose source contents or arbitrary environment plaintext? | Source is hashed; unknown and credential-like values are hash-only and hidden. |
| Can an oversized file become a false removal? | No. Presence is stored separately from content-tracking state. |
| Does an optional integration failure break the scan? | No. Detector completeness is stored and unsafe comparisons are skipped. |
| Can output be automated? | Yes. `diff --json`, `mark --json`, and `list --json` are stable JSON. |

## Test strategy

- stable command ID tests for root normalization, repeatability, and argument
  ordering;
- subprocess tests for independent per-command baselines and preservation of a
  prior success when the same command later fails;
- oversized-file tests for presence, threshold crossing, size-only changes, and
  real deletion;
- privacy tests for common and unusual credential names, arbitrary unknown
  variables, checkpoint JSON, output redaction, and schema 1 migration;
- Java, Node, compose-port, and referenced-environment relevance fixtures;
- deterministic ordering/JSON tests and concise versus verbose rendering tests;
- missing Git/Docker and partial project-context isolation tests;
- Windows, Linux, and macOS listening-port parser fixtures;
- native multi-command, hash-reuse, privacy, text, verbose, and JSON sessions.

## Benchmark method

`BenchmarkCaptureFiles1000` measures a complete 1,000-file walk, SHA-256 hash,
classification, and map construction. `BenchmarkProjectContextTenThousandFiles`
analyzes 10,000 paths plus package/compose fixtures. `BenchmarkCompareTenThousandFiles`
compares two 10,000-file snapshots with 100 changes, including context-aware
ranking.

Run locally with:

```bash
go test -run '^$' -bench . -benchmem ./internal/snapshot ./internal/compare
```

Measured on 2026-08-20 on Windows/amd64 with an AMD Ryzen 7 8845HS and Go 1.25.6:

| Measurement | Result |
|---|---:|
| Hash and classify 1,000 small files | 59.4 ms/op |
| Analyze project context across 10,000 paths | 2.03 ms/op |
| Compare 10,000 files with 100 changes | 0.72 ms/op |
| Native checkpoint, five-run median | 301 ms wall time |
| Native no-change diff, five-run median | 294 ms wall time |

These are local prototype measurements, not cross-platform performance claims.
The external runtime, Git, Docker, and port probes dominate the small-project
wall time. The new project-context pass adds about 2 ms at 10,000 paths, and the
in-memory context-aware comparison remains below one millisecond.

## Decision after MVP

v0.2 succeeds if independent commands maintain independent known-good states,
privacy and presence facts survive persistence, and repository evidence moves
the most locally relevant clues above generic machine noise.

The native smoke scenario passed:

1. Two executions of `run -- go test ./...` reused the same command ID;
   the second reused all 36 file hashes.
2. `run -- go test ./internal/commandid` created a second independent ID and
   baseline.
3. `diff -- go test ./...` selected only the first command and returned no
   changes immediately after success.
4. A synthetic unusual credential value was absent from every checkpoint JSON
   file.
5. Setting `NODE_ENV=production` after the baseline produced one concise,
   non-causal environment finding.
6. Unit/subprocess fixtures proved a later exit 7 preserves the prior successful
   baseline for that exact command.

The next validation step should be a small field study: ask 5-10 developers to
wrap their normal test command for one week, then measure (a) checkpoints
created, (b) failure investigations where at least one top-five finding was
useful, (c) time to first useful clue, and (d) false or noisy top-five findings.
Do not add detectors until this evidence identifies a repeated blind spot.
