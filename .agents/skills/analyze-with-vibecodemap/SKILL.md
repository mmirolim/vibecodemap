---
name: analyze-with-vibecodemap
description: Build VibeCodeMap, inspect and semantically map a local repository, author source-linked structural/quality/project VCM DSL, validate and compose it, generate view JSON and HTML, and open the interactive browser visualization. Use when testing VibeCodeMap on an app, visualizing architecture/quality/effects, or mapping a mixed-stack monorepo. Keep detector support, AI inference, measured facts, and unknowns explicit.
---

# Analyze and visualize with VibeCodeMap

The normal outcome is an opened browser map, not an inventory report or a pile
of YAML. Use deterministic inspection to bound the work, perform the missing
semantic investigation, write editable evidence-backed DSL, then run the Go
renderer until the map opens successfully.

Only stop earlier when the user explicitly requests report-only, review-only,
scope-only, or DSL-only work.

## Establish paths and mode

Identify:

- `VCM_ROOT`: the checkout containing `cmd/vibecodemap`, `dsl/`, and this skill;
- `VCM_BIN`: normally `VCM_ROOT/bin/vibecodemap`;
- `TARGET_ROOT`: the absolute repository root to map;
- mode: end-to-end visualization by default, or the narrower mode explicitly
  requested by the user.

When no target is named, use the active non-VibeCodeMap repository if clear;
otherwise use the active repository and state the assumption. Never map the
VibeCodeMap fixture while implying that it represents the user's target.

Before authoring, read from `VCM_ROOT`:

- `README.md`, `docs/GETTING-STARTED.md`, and `docs/ADAPTERS.md`;
- `docs/MULTI-STACK.md` for any repository with several workloads or stacks;
- `references/evidence-rules.md` from this skill.

## 1. Build the CLI

Do not assume a fresh clone already contains an executable. From `VCM_ROOT`,
build it before the first command unless a current binary is already verified:

```bash
make build
./bin/vibecodemap help
```

If `make` is unavailable, use:

```bash
mkdir -p bin
go build -o bin/vibecodemap ./cmd/vibecodemap
```

Go 1.24 or newer is required. Python is optional for the main flow; use it only
for prototype Python extractors or additional validator diagnostics.

## 2. Inspect once at the repository root

Run both human and machine-readable inspection at `TARGET_ROOT`, not once per
language, application directory, or `cmd/*` binary:

```bash
/absolute/path/to/vibecodemap/bin/vibecodemap inspect /absolute/path/to/target
/absolute/path/to/vibecodemap/bin/vibecodemap inspect -json /absolute/path/to/target
```

Interpret:

- analyze/summarize/externalize/ignore totals and pruning warnings;
- applied rules and likely false-positive exclusions;
- every detector's stack, scope, confidence, evidence, and support level;
- manifests, build roots, entrypoint candidates, and approved source files.

`inspect` is a bounded reading plan. It does not resolve calls/imports, detect
effects, calculate quality, decide systems, author DSL, generate JSON, or
render HTML. Treat `detection_only` literally.

If scope is noisy, make the smallest useful root `.vcmignore` correction when
the requested mode permits target changes, rerun root inspection, and compare
the totals. Never let an adapter independently recurse through ignored
dependency/build/cache/generated trees.

## 3. Discover systems and deployables

Use manifests, build targets, process/mobile entrypoints, deployment files,
routes, ownership documentation, and source relationships. Do not equate a
language or detector scope with a system.

Use this hierarchy:

```text
workspace/repository map
  system/product city
    deployable/runtime area
      feature, subsystem, layer, or cluster district
        component/interface/store/file/symbol building
```

Default rules:

- one product containing web, mobile, backend, and worker stacks is normally
  one city with several deployable areas;
- independent products or independently understood operational systems become
  separate cities;
- every independently built/released executable—including meaningful Go
  `cmd/*` binaries—is a deployable unless evidence shows it is only an alias;
- shared libraries belong in utility areas;
- generators, migrations, CI, and developer tools are support deployables or
  districts, not production runtime by default;
- static/build dependencies must not look like runtime communication.

Run one root inspection and author one unified graph. For very large
monorepos, keep a unified workspace map and optionally add focused project
manifests under `.vibecodemap/maps/<system-or-workload>/`.

## 4. Perform the semantic investigation

This is the main work that `inspect` does not do. Read approved source and use
available compiler, analyzer, linter, coverage, test, and repository tools.
Capture enough evidence to make the visualization useful:

1. Purpose and shape: products, entrypoints, deployables, processes, layers,
   features, components, important files/symbols, and external systems.
2. Interfaces: HTTP/RPC handlers, CLI commands, jobs, callbacks, UI/mobile
   entrypoints, payload classes, authentication, and outputs.
3. Behavior: sync calls, async events, callbacks, scheduling, delivery style,
   concurrency, and failure/retry boundaries.
4. State and effects: databases, files, caches, queues, process-local state,
   external POST/write operations, mutations, and the exact source sites.
5. Topology: imports, type references, shared libraries, build dependencies,
   runtime communication, and cross-stack relationships—kept separately typed.
6. Condition: source size, complexity, coupling, tests, coverage, lint/static
   findings, duplication or other metrics only when produced by named tools or
   transparent calculations.
7. Intent and gaps: requirements, declared architecture, expected-but-missing
   concepts, conflicts, unknowns, and likely investigation targets.
8. Security context: trust boundaries, sensitive inputs/outputs, permissions,
   attack surfaces, candidate control gaps, severity, status, and confidence.
9. Navigation: repository-relative artifact path plus symbol and line evidence
   for every important source-backed element and relation.

Names alone are not evidence. `async` syntax is not proof of queued delivery;
an import is not runtime communication; no direct side effect found is not
proof of transitive purity; a security lead is not a confirmed vulnerability.

For report-only mode, stop after this investigation and return evidence,
uncertainties, and recommended boundaries without writing target files.

## 5. Author one editable model

For an ordinary or mixed-stack repository use:

```text
TARGET_ROOT/.vibecodemap/
  .gitignore             # normally ignores out/ and generated/
  model.vcm.yaml
  quality.vcm.yaml       # optional until measurements exist
  project.vcm.yaml
  out/                   # generated; do not hand-edit
```

Read the actual contracts before drafting:

- `dsl/VIBECODEMAP_DSL_0_1.md` and its JSON Schema;
- `dsl/VIBECODEMAP_QUALITY_AND_LOD_0_2.md` and its JSON Schema;
- `dsl/VIBECODEMAP_PROJECT_DSL_0_1.md`;
- `<VCM_ROOT>/bin/vibecodemap describe` and
  `<VCM_ROOT>/bin/vibecodemap schema`;
- `examples/uzumtools/` as a complete but fixture-specific example.

Keep the concerns separate:

- structural model: artifacts, semantic elements, typed relations, flows,
  findings, execution, effects, and source evidence;
- quality model: named metric definitions and measured values with tool,
  revision, status, and provenance;
- project model: repository inputs, editable decompositions, expectations,
  corrections, boundaries, security reviews, and render profiles.

Use stable semantic IDs and repository-relative source paths. Mark
interpretations as `ai_inferred` with confidence, rationale, and evidence.
Never invent coverage, complexity, traffic, defect probability, or security
certainty. Unknown is preferable to a fabricated number.

The YAML is semantic, not geometric. Do not hand-author Three.js coordinates.
One structural model may contain several systems; the composer turns them into
several cities and qualifies district codes automatically. In this standard
layout, set the project and structural repository roots to `..` so generated
source-navigation targets resolve against `TARGET_ROOT`.

For the current MVP, represent a missing visible concept as a structural
`expected_component` with `reality.implementation: missing`, then reference it
from the project expectation. The composer applies direct-mutation correction
paths; it emits generation warnings for validated correction paths it cannot
yet project. Do not report such a correction as visually applied until the
warning is resolved.

## 6. Render, repair, and open

Do not ask the user to run a separate validator or JSON generator. Run:

```bash
/absolute/path/to/vibecodemap/bin/vibecodemap show \
  /absolute/path/to/target/.vibecodemap/project.vcm.yaml
```

`show` automatically validates the project, referenced structural model, and
optional quality model; checks cross-references; composes
`vibecodemap.view/0.1`; writes `.view.json` and `.html` under
`.vibecodemap/out/`; and opens the default browser.

If validation or composition fails, use the exact diagnostic to repair the DSL
and rerun `show`. Continue until the HTML is generated and the browser launch
succeeds. Use `render` only when the user requested headless output or the
environment genuinely cannot open a browser; in that case report the absolute
HTML path and the browser limitation.

Inspect the resulting map for obviously wrong grouping, excessive unmapped
elements, misleading runtime/static roads, absent source targets, unexpected
unknown bands, and generation warnings. Correct the DSL and rerender when the
visual result exposes a modeling mistake.

## Completion report

For end-to-end mode return only the important handoff:

- target and mapped systems/deployables;
- generated HTML and editable project-manifest paths;
- confirmation that `show` validated and opened the map (or the exact browser
  limitation if headless);
- important unknowns/limitations and the next useful investigation.

Do not describe detector support as completed semantic support, validation as
semantic truth, or a review lead as a confirmed bug.
