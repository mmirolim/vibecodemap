---
name: analyze-with-vibecodemap
description: Plan and track a repository-wide VibeCodeMap investigation, build the CLI, centrally inspect source, run implemented semantic adapters, investigate unsupported stacks, author source-linked structural/quality/project VCM DSL, validate and compose it, generate view JSON and HTML, and open the interactive browser visualization. Use when testing VibeCodeMap on an app, visualizing architecture/quality/effects, or mapping a mixed-stack monorepo. Keep detection, adapter evidence, AI inference, measured facts, reviewed scope, and unknowns explicit.
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

## Plan and track the investigation

Repository mapping is usually long-running AI work. Before scanning source,
create a plan or task list and keep only one stage in progress. Include:

1. inventory and scope review;
2. system/deployable discovery;
3. one investigation batch per meaningful subsystem or deployable;
4. cross-boundary relations, flows, inputs, outputs, state, and effects;
5. quality and security evidence;
6. DSL validation/composition; and
7. visual review and correction.

The inventory accounts for the repository; it does not require a human or AI
to narrate every line. Every file admitted for `analyze` must nevertheless be
covered by adapter evidence, included in a semantically reviewed batch, or
explicitly deferred by narrowing scope. For more than roughly 50 approved
source files, several deployables, or several stacks, maintain a compact
coverage ledger in working notes or
`TARGET_ROOT/.vibecodemap/generated/review-ledger.md`. Record path/pattern,
workload or subsystem, adapter status, semantic-review status, and unresolved
questions. Group tasks by coherent subsystem rather than creating one task per
file, and checkpoint valid DSL after each completed batch so progress survives
long agent sessions.

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

Go 1.24 or newer is required. Python 3.10 or newer is required only when the
target has Python files and the Go-orchestrated Python AST analyzer is run.
Validation, composition, HTML generation, and every stack detector are Go;
Three.js renders the generated map in the browser.

## 2. Inspect once at the repository root

Run one machine-readable inspection at `TARGET_ROOT`, not once per language,
application directory, or `cmd/*` binary:

```bash
/absolute/path/to/vibecodemap/bin/vibecodemap inspect -json \
  /absolute/path/to/target > /tmp/vibecodemap-inspect.json
```

Use text mode instead for an interactive human review. Do not run both by
default because each command rescans the repository.

Interpret:

- analyze/summarize/externalize/ignore totals and pruning warnings;
- applied rules and likely false-positive exclusions;
- every detector's stack, scope, confidence, evidence, and support level;
- manifests, build roots, entrypoint candidates, and approved source files.

`inspect` computes a bounded reading plan. It does not resolve calls/imports, detect
effects, calculate quality, decide systems, author DSL, generate JSON, or
render HTML. Treat `detection_only` literally.

Review scope, but correct it only when needed. The four actions mean:

- `analyze`: full source input to adapters and AI investigation;
- `summarize`: metadata/volume without detailed symbols;
- `externalize`: dependency/boundary without internal source;
- `ignore`: omit irrelevant or derived content.

Proceed unchanged when owned source and manifests are analyzed while installed
dependencies, generated code, caches, vendored source, and build output are
not. Otherwise write the smallest useful root `.vcmignore`, rerun inspection,
and compare totals and affected entries. This correction is optional;
`inspect` never creates `.vcmignore`. Never let an adapter independently
recurse through excluded trees.

## 3. Run implemented semantic adapters

After scope is acceptable—changed or unchanged—run one root analysis. Do not
invoke scripts manually or run once per stack:

```bash
/absolute/path/to/vibecodemap/bin/vibecodemap analyze /absolute/path/to/target
/absolute/path/to/vibecodemap/bin/vibecodemap adapters
```

Read `TARGET_ROOT/.vibecodemap/generated/evidence.json`. `analyze` repeats the
central scan with the same rules, dispatches each implemented detected
analyzer, and records every detection-only stack as `not_implemented`.

Review every run status before using the bundle. `runtime_unavailable`,
`failed`, and `timed_out` mean that analyzer supplied no retained evidence;
healthy adapters still complete. For Python, use
`VIBECODEMAP_PYTHON=/absolute/path/to/python3` to select another Python 3.10+
runtime. Increase `-adapter-timeout` only when repository size plausibly
explains the timeout. Otherwise record the limitation and investigate that
language's approved source directly; never silently retry forever or present
the missing adapter evidence as complete.

Current executable truth:

- Python: a prototype semantic analyzer is implemented. Go sends its embedded
  Python AST subprocess only approved `.py` files. It returns source-linked
  imports, symbols, routes, calls, effect candidates, and transparent static
  complexity/nesting facts. Python 3.10 or newer must be on `PATH` or selected
  through `VIBECODEMAP_PYTHON`.
- Go: an in-process parser/AST prototype returns packages, imports, types,
  functions, calls, concurrency, route/effect candidates, and complexity.
  Package loading, types, SSA, and dynamic dispatch remain unresolved.
- JavaScript/TypeScript: in-process lexical prototypes return imports,
  symbol/call/route/effect candidates, entrypoints, and structural counts.
  They are not compiler ASTs and do not resolve aliases, types, JSX, or scope.
- Flutter/Dart, Android/Kotlin/Java, and Apple/Swift/Objective-C: detection
  exists; semantic analyzers do not.

Adapter events are deterministic evidence inputs, not VCM DSL, architecture,
runtime observations, or proof of quality. For stacks without an analyzer,
investigate approved source directly. If Python's runtime is unavailable,
state that limitation and perform the same source investigation; do not claim
the adapter ran.

## 4. Discover systems and deployables

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

## 5. Perform the semantic investigation

This is the main work that detection and adapter facts do not decide. Start
with the generated evidence bundle, then read approved source and use available
compiler, analyzer, linter, coverage, test, and repository tools.
Work through the planned subsystem batches, update the coverage ledger after
each batch, and revisit shared hubs when their callers reveal additional
responsibilities. Do not declare repository coverage complete while admitted
source remains unreviewed or silently omitted.
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

## 6. Author one editable model

For an ordinary or mixed-stack repository use:

```text
  TARGET_ROOT/.vibecodemap/
  .gitignore             # normally ignores out/ and generated/
  model.vcm.yaml
  quality.vcm.yaml       # required when deterministic measurements exist
  project.vcm.yaml
  out/                   # generated; do not hand-edit
```

Read the actual contracts before drafting:

- `dsl/VIBECODEMAP_DSL_0_1.md` and its JSON Schema;
- `dsl/VIBECODEMAP_QUALITY_AND_LOD_0_2.md` and its JSON Schema;
- `dsl/VIBECODEMAP_PROJECT_DSL_0_1.md`;
- `<VCM_ROOT>/bin/vibecodemap describe project` plus
  `<VCM_ROOT>/bin/vibecodemap schema project`, `schema structural`, and
  `schema quality` when exact enum/schema details are needed;
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

## 7. Generate and supplement measured quality

After the structural model exists, convert adapter evidence into a validated
quality model:

```bash
/absolute/path/to/vibecodemap/bin/vibecodemap quality \
  -evidence /absolute/path/to/target/.vibecodemap/generated/evidence.json \
  -output /absolute/path/to/target/.vibecodemap/quality.vcm.yaml \
  /absolute/path/to/target/.vibecodemap/model.vcm.yaml
```

Add `quality_model: quality.vcm.yaml` under `project.inputs` when the command
produces measurements. This bridge maps exact source paths and symbols to
structural artifact/element IDs. Current Go and Python adapters provide
transparent cyclomatic-complexity, nesting, size, and direct-effect facts.
JavaScript/TypeScript lexical decision counts remain explicitly distinct from
compiler-grade cyclomatic complexity. Coverage stays unknown unless an actual
coverage report supplies it.

`quality` does not run tests, linters, compilers, or third-party analyzers.
Supplement it when useful with configured, reproducible outputs such as Go
coverage or gocyclo-compatible metrics, Ruff/Radon complexity, ESLint
complexity, compiler diagnostics, test reports, or SARIF. Record producer,
configuration, revision, timestamp, and scope. AI may explain these facts but
must not invent numeric measurements.

## 8. Render, repair, and open

Do not ask the user to run a separate validator or JSON generator. Run:

```bash
/absolute/path/to/vibecodemap/bin/vibecodemap show \
  /absolute/path/to/target/.vibecodemap/project.vcm.yaml
```

`show` automatically validates the project, referenced structural model, and
optional quality model; checks graph, flow, quality, provenance, and available
source-range invariants; composes
`vibecodemap.view/0.1`; writes `.view.json` and `.html` under
`.vibecodemap/out/`; and opens the default browser.

If validation or composition fails, use the exact diagnostic to repair the DSL
and rerun `show`. Continue until the HTML is generated and the browser launch
succeeds. Use `render` only when the user requested headless output or the
environment genuinely cannot open a browser; in that case report the absolute
HTML path and the browser limitation.

Inspect the resulting map for obviously wrong grouping, excessive unmapped
elements, misleading runtime/static roads, absent source targets, unexpected
unknown bands, and generation warnings. Exercise the label modes—including
`Hidden`—and confirm input arrows point into components while output arrows
point outward. Correct the DSL or renderer and rerender when the visual result
exposes a modeling or presentation mistake.

## Completion report

For end-to-end mode return only the important handoff:

- target and mapped systems/deployables;
- generated HTML and editable project-manifest paths;
- confirmation that `show` validated and opened the map (or the exact browser
  limitation if headless);
- investigation-ledger coverage and whether deterministic quality evidence is
  linked into the project;
- important unknowns/limitations and the next useful investigation.

Do not describe detector support as completed semantic support, validation as
semantic truth, or a review lead as a confirmed bug.
