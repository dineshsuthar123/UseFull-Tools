# WhatChanged v0.2.0

WhatChanged v0.2.0 turns the local "it worked earlier" debugging loop into a
command-specific workflow.

> Show me what changed since this command last worked.

## Highlights

- Records a checkpoint only after the exact wrapped command succeeds.
- Keeps independent baselines for normalized project root, executable, and
  ordered arguments.
- Ranks file, dependency, runtime, environment, Git, port, and Docker changes
  using transparent repository context.
- Preserves oversized-file presence separately from content tracking.
- Reuses unchanged file hashes and reports reuse statistics.
- Stores no source content or arbitrary environment plaintext.
- Isolates optional detector failures and rejects incomplete core file scans.
- Reads schema 1 snapshots safely while writing schema 2.

## Example

```bash
what-changed run -- go test ./...
what-changed diff -- go test ./...
```

The second command reports a concise, stable list of likely relevant changes
since the first command last passed. Findings are correlations for investigation,
not claims of root cause.

## Validation

- `go test -count=1 ./...`
- `go vet ./...`
- Windows/amd64 native build and CLI smoke workflow
- successful cross-compilation for the Windows, Linux, and macOS amd64/arm64
  release matrix listed in the release assets
- command-ID reuse, independent baselines, unchanged hash reuse, JSON/text
  rendering, failed-command preservation, and secret redaction coverage

Cross-compiled non-Windows binaries were inspected as build artifacts but were
not runtime-tested on their target operating systems for this release candidate.

## Benchmarks

Measured locally on Windows/amd64 with an AMD Ryzen 7 8845HS and Go 1.25.6:

| Measurement | Result |
|---|---:|
| Hash and classify 1,000 small files | 59.4 ms/op |
| Analyze project context across 10,000 paths | 2.03 ms/op |
| Compare 10,000 files with 100 changes | 0.72 ms/op |
| Native checkpoint, five-run median | 301 ms |
| Native no-change diff, five-run median | 294 ms |

These are local prototype measurements, not cross-platform guarantees.

## Known Limitations

- Ranking is heuristic and cannot prove causality.
- Repository context is bounded and does not fully parse every build or config
  language.
- Database contents, application memory, remote feature flags, unexported shell
  variables, and dynamic service health are outside the snapshot.
- Cross-compilation does not establish runtime compatibility on untested hosts.
