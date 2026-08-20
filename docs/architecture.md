# Architecture

WhatChanged is a single Go executable with four layers:

1. `snapshot` runs isolated detectors and returns a versioned snapshot. A failed
   optional detector becomes a diagnostic; it never aborts the scan.
2. `store` writes JSON checkpoints atomically to `.what-changed/` and resolves
   named or latest checkpoints.
3. `compare` turns raw differences into findings with a fixed score and an
   explicit reason. It never claims causality.
4. `cli` runs commands, records only successful outcomes, and renders text or
   stable JSON.

## Privacy boundary

File contents are hashed and never persisted. Environment values are hashed;
names that look credential-related are stored without plaintext. Everything
stays under the project root. No network request is made.

## Relevance model

The score is a transparent triage heuristic:

| Change | Base score |
|---|---:|
| dependency manifest / lockfile | 96 |
| migration file | 94 |
| Git branch | 92 |
| project configuration | 90 |
| runtime version | 88 |
| source file | 86 |
| relevant environment variable | 84 |
| common development port | 82 |
| container state/image | 78 |
| test file | 72 |
| other tracked file / port | 50-55 |

Ties are resolved deterministically by category and summary. Findings say why
they received their score so users can disagree with the heuristic.

