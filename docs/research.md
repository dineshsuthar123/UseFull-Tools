# Product selection research

Research date: 2026-08-20. Scores are directional product judgments, not survey
results. The weighted rubric emphasizes a small credible MVP, differentiation,
and reliability over theoretical market size.

| Rank | Product | Score / 100 | Decision |
|---:|---|---:|---|
| 1 | WhatChanged | 86 | Build. A successful command provides a precise, automatic baseline that existing point tools lack. |
| 2 | WhyChanged | 77 | Strong gap, but dependency API matching needs a much larger trust surface before it is useful. |
| 3 | Checkpoint / WhereWasI | 69 | Real pain, but native application restoration is platform-specific and a close Mac product now exists. |
| 4 | ExplainDiff / ImageDiff | 67 | Clear pain; screenshot-only explanations are hard to make deterministic and existing suites claim semantic categories. |
| 5 | WorksOnMyMachine | 64 | Easy MVP and real pain, but `envdiff` now covers the proposed core unusually closely. |
| 6 | Semantic Compressor | 60 | Compelling long-term research project, not a small MVP with the required proof standard. |
| 7 | TraceClip | 58 | Useful, but OS provenance is brittle and new products already focus on this exact promise. |
| 8 | PasteSafe | 56 | Important but crowded; low false positives plus destination awareness is a deceptively large cross-platform surface. |
| 9 | Repro | 53 | Validated category with mature, feature-complete incumbents. |
| 10 | CommandUndo | 48 | Memorable UX, but exact new competitors exist and an incomplete interceptor creates dangerous false confidence. |

## Evidence and gaps

- **Checkpoint:** [Cove](https://covemac.app/) captures apps, browser tabs,
  editor projects, and terminal directories on macOS. VS Code's
  [Session Saver](https://marketplace.visualstudio.com/items?itemName=monkey-sheng.session-saver)
  covers editor tabs, layout, and cursor positions. Cross-platform support and
  a concise “what was I thinking?” summary remain gaps, but the MVP requires
  several integrations before delivering its promise.
- **TraceClip:** Recent user-built products such as
  [ClipTrace](https://www.reddit.com/r/ProductivityApps/comments/1tvot3a/i_kept_forgetting_where_copied_text_came_from_so/)
  already store source application, URL, file, and folder context. This also
  validates the pain, but source capture quality depends on browser and IDE
  integrations rather than the clipboard alone.
- **Repro:** [Bird Eats Bug](https://birdeatsbug.com/) records the screen,
  clicks, console, network, and system details, and its
  [network documentation](https://docs.birdeatsbug.com/latest/recording/network.html)
  illustrates the response-capture and data-volume complexity. A small clone
  would not be differentiated.
- **WorksOnMyMachine:** [envdiff](https://github.com/GBerghoff/envdiff) already
  snapshots runtimes, environment variables, network state, and listening
  ports with redaction and structured comparison. Declarative tools such as
  [envcheck](https://www.mintlify.com/explore/dotandev/envcheck) cover the
  adjacent “expected environment” workflow.
- **WhyChanged:** JDK's [jdeprscan](https://docs.oracle.com/en/java/javase/23/core/running-jdeprscan.html)
  detects deprecated API use and research systems such as
  [UPCY](https://sse.cs.tu-dortmund.de/storages/sse-cs/r/Publications/Preprints/icse-2023_upcy.pdf)
  analyze dependency upgrades, but a polished release-note/API-diff
  intersection still looks differentiated. It is the best second project.
- **PasteSafe:** [BeforePaste](https://github.com/beforewire/beforepaste) is a
  local-first tray tool with positive destination identification, while
  [PasteProof](https://github.com/007jedgar/pasteproof) performs local browser
  paste interception. The proposed core is no longer open territory.
- **CommandUndo:** [oops](https://oops-cli.com/) backs up destructive shell
  changes and restores them across common commands. The shell cannot generally
  know arbitrary command effects, so unsupported cases must remain explicit.
- **WhatChanged:** Searches found environment comparison, Git, file watchers,
  and test-output snapshots, but no focused tool whose baseline is
  automatically updated only after an arbitrary developer command succeeds.
  [WebdriverIO Preserve & Rerun](https://webdriver.io/ta/docs/devtools/wdio/preserve-and-rerun/)
  validates the value of preserving a successful/failing test context, but is
  scoped to WebdriverIO command logs rather than local machine changes.
- **ImageDiff:** [CloudQA](https://cloudqa.io/visual-regression-testing-tool/)
  claims layout, text, font, color, and spacing classification. Deterministic
  explanation is more credible when DOM metadata is available; screenshot-only
  OCR and element correspondence would make a small MVP noisy.
- **Semantic Compressor:** IntelliJ already offers broad boolean and control-flow
  simplifications, and its [inspection platform](https://plugins.jetbrains.com/docs/intellij/inspection-options.html)
  is mature. The proposed `PROVEN_SAFE` bar requires parsing, types, effects,
  control flow, exception preservation, and compilation—not a small prototype.

## Selected wedge

**WhatChanged: “Show me what changed since this command last worked.”**

The key is not another general environment dump. `what-changed run -- <command>`
runs the user's real test/build/start command and records a local checkpoint
only when it exits successfully. When the workflow later breaks,
`what-changed diff` ranks content changes across source, dependency manifests,
configuration, environment variables, runtimes, Git, listening ports, and
Docker containers.

## MVP boundaries

Included:

- one dependency-free executable;
- Windows, macOS, and Linux collection paths;
- content hashes rather than source-file contents;
- secret-aware environment storage and display;
- atomic local checkpoints inside `.what-changed/`;
- partial detector failure without scan failure;
- deterministic, explainable relevance scores;
- text and JSON output.

Excluded deliberately:

- filesystem watching or a resident daemon;
- database connections or reading table data;
- automated root-cause claims;
- full process snapshots (too noisy and privacy-sensitive);
- cloud accounts, telemetry, or remote inference;
- silently editing `.gitignore`.

