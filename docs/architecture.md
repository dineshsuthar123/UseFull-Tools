# Architecture

WhatChanged is a single Go executable with five layers:

1. `commandid` normalizes the project root and hashes the ordered command tuple
   into a stable, human-readable command ID.
2. `snapshot` runs isolated system detectors, captures explicit file tracking
   states, and builds bounded repository context. A failed optional detector
   becomes a diagnostic; it never aborts the core scan.
3. `store` writes append-only JSON checkpoints atomically, resolves latest,
   named, or per-command baselines, and upgrades schema 1 in memory.
4. `compare` separates raw facts from suspicion scores. Fixed base scores receive
   transparent boosts from repository languages, build systems, config
   references, and compose services.
5. `cli` runs commands, records only successful and complete core outcomes, and
   renders concise text, verbose facts, or stable JSON.

## Privacy boundary

File contents are hashed and never persisted. Arbitrary environment plaintext
is never stored: every value is hashed, only an explicit safe allowlist can keep
plaintext, and unknown values are hidden. Everything stays under the project
root. No network request is made.

## Command checkpoint routing

```text
normalized project root + executable + ordered arguments
                         |
                         v
               readable slug + SHA-256 prefix
                         |
                         v
              latest successful matching snapshot
```

Named checkpoints remain an orthogonal label. Supplying `--name` to `run` does
not remove the command ID, so command-based lookup still works.

## File-state correctness and reuse

Every discovered file has an entry. Content-tracked files carry a digest;
oversized files carry size, mtime, `tracked: false`, and `reason: size-limit`.
This makes presence independent from hashing and prevents false removals.

When a prior command checkpoint is available, matching path + size + nanosecond
mtime can reuse its digest. The comparison ignores mtime by itself: hashes prove
normal content changes, while metadata-only oversized files report only facts
the scanner actually knows.

## Detector isolation

Files, runtimes, Git, ports, Docker, and project context each expose a
completeness bit and diagnostics. Optional failure degrades only that comparison.
An incomplete file scan is treated differently because it would make the core
baseline unreliable; the CLI preserves the previous known-good checkpoint.

## Relevance model

The score is a transparent triage heuristic:

| Change | Base score |
|---|---:|
| dependency manifest / lockfile | 96 |
| migration file | 94 |
| Git branch | 92 |
| project configuration | 90 |
| repository-matched runtime version | 95 |
| generic runtime version | 88 |
| repository-referenced environment variable | 93 |
| compose/config-referenced port | 97 |
| source file | 86 |
| relevant environment variable | 84 |
| common development port | 82 |
| container state/image | 78 |
| test file | 72 |
| other tracked file / port | 50-55 |

Ties are resolved deterministically by category, subject, and change. Findings
say exactly which repository fact produced a boost so users can disagree with
the heuristic. The renderer calls them likely relevant changes and never claims
root cause.
