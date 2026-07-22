# VibeCodeMap

> **Experimental / work in progress.** VibeCodeMap is a research prototype,
> not a dependable code-audit product. Its maps guide investigation; they do
> not prove that software is correct, secure, or complete.

VibeCodeMap turns source-linked descriptions of software into a navigable 3D
condition map. The goal is to review architecture, interactions, side effects,
quality signals, security leads, and missing expectations at system scale—not
only as thousands of generated source lines.

## OpenAI Build Week: built with Codex and GPT-5.6

VibeCodeMap was developed from initial ideation through hackathon submission in
Codex with **GPT-5.6 in Sol Max mode** as the primary engineering environment.
The human builder originated the problem, the software-city metaphor, the
visual grammar, and the product constraints, then continuously reviewed the
output and drove corrections from rendered screenshots. Codex and GPT-5.6
turned that direction into the implementation and repeatedly audited it.

They were used to:

- research adjacent approaches and turn the concept into an evidence-first Go,
  DSL, agent-skill, and Three.js architecture;
- design the structural, quality, clustering, project, and renderer contracts;
- implement repository scoping, Go/Python/JS/TS analyzers, validation,
  evidence-to-quality conversion, composition, typed roads, and the browser
  renderer;
- investigate real repositories, author source-linked models, and dogfood
  VibeCodeMap on its own codebase;
- review screenshots and code, diagnose visual noise and semantic ambiguity,
  add tests, run validators, and correct documentation through submission.

Several important choices came directly from that human/agent review loop:
unknown evidence must remain unknown; static imports must not be presented as
observed runtime communication; AI architectural claims require explicit
provenance; and clutter introduced by the renderer must never masquerade as a
problem in the mapped repository.

GPT-5.6 is a development dependency of this Build Week workflow, not a runtime
API dependency of the generated map. The CLI and hosted visualization do not
require OpenAI API credentials.

## See the prototype

**[Open the public product site and live self-map](https://vibecodemap.alanthedirector.chatgpt.site/)** — no installation or account required.
The hosted artifact maps VibeCodeMap itself and can be explored directly in a
modern desktop browser.

Go 1.24 or newer is required. Python 3.10 or newer is needed only when
`analyze` detects Python source and runs the Python AST adapter; validation and
rendering are Go.

```bash
git clone https://github.com/mmirolim/vibecodemap.git
cd vibecodemap
make build

./bin/vibecodemap show \
  examples/uzumtools/uzumtools.project.vcm.yaml
```

`show` performs the complete deterministic presentation pipeline:

1. validates the project, structural, and optional quality DSL;
2. joins and normalizes the models into renderer-ready JSON;
3. writes `<project-dir>/out/<project-id>.view.json` and `.html`;
4. opens the HTML map in the default browser.

Use `render` instead of `show` in CI or when the browser should not open. The
prototype HTML currently loads Three.js from `esm.sh`; the embedded data
fallback remains readable if that runtime cannot load.

For mapping your own repository, follow
[Getting started](docs/GETTING-STARTED.md).

### Supported platforms

- **Hosted demo:** a modern desktop browser with WebGL and network access to
  the current Three.js CDN dependency; no Go or Python installation is needed.
- **CLI:** developed and tested on macOS with Go 1.24+. The Go implementation is
  intended to compile on Linux and Windows, but this prototype does not yet
  publish cross-platform release binaries or claim a complete OS test matrix.
- **Python analysis:** Python 3.10+ on `PATH` is needed only when `analyze` runs
  the Python AST adapter. Set `VIBECODEMAP_PYTHON=/absolute/path/to/python3`
  when several interpreters are installed and the first one on `PATH` is not
  usable. Go and JS/TS analyzers run from the Go process.
- **Mapped source:** prototype analyzers cover Go, Python, and JS/TS. Dart,
  Kotlin/Java, and Swift/Objective-C are detected but remain agent-investigated,
  detection-only stacks.

## Current end-to-end flow

```text
clone VibeCodeMap and build the Go CLI
        ↓
run one root inspect over the target repository
        ↓
review the computed scope; add target/.vcmignore only if something is misclassified
        ↓
vibecodemap analyze runs implemented adapters over that exact scope
        ↓
AI agent follows a subsystem plan, investigates approved source, and writes structural/project DSL
        ↓
vibecodemap quality maps deterministic evidence into source-linked quality DSL
        ↓
vibecodemap show project.vcm.yaml
        ↓
automatic validation → view JSON → HTML → browser
        ↓
review the map, source links, warnings, and editable DSL; iterate
```

The checked-in `$analyze-with-vibecodemap` agent skill drives this whole flow.
Today AI is the primary architectural mapper between repository evidence and
the DSL. The skill plans the potentially long investigation and tracks source
coverage by subsystem. `inspect` bounds that work; `analyze` adds deterministic
facts where a real adapter exists; `quality` links supported measurements to
structural artifacts and elements. None of these commands invents architecture.

Example prompt from the VibeCodeMap checkout:

```text
Use $analyze-with-vibecodemap to map /absolute/path/to/my-app end to end.
Build the CLI if needed, inspect the repository once at its root, write
source-linked structural and quality DSL under .vibecodemap, track the
investigation by subsystem, and open the generated 3D map.
```

## What works today

- a Go CLI for repository inventory, stack detection, analyzer orchestration,
  evidence-to-quality conversion, DSL validation, composition, standalone HTML
  generation, and browser launch;
- editable structural, quality, clustering/visual, and project DSL with JSON
  Schemas;
- generated renderer-neutral `vibecodemap.view/0.1` JSON;
- a generic Three.js browser renderer with multiple cities, districts,
  buildings, wide condition bands, typed aggregate roads, source navigation,
  inputs/outputs, security-review markers, search, and fluid camera controls;
- built-in exclusions for installed dependencies, generated sources, caches,
  build artifacts, and mobile/web tool output;
- root-owned `.vcmignore` corrections with `analyze`, `summarize`,
  `externalize`, and `ignore` actions;
- multi-stack detection for Python, Go, JavaScript/TypeScript,
  Flutter/Dart, Android/Kotlin/Java, and Apple/Swift/Objective-C;
- explainable affinity and hub metrics plus deterministic district-road/lane
  aggregation;
- Python AST, Go AST, and JS/TS lexical analyzers for source-linked imports,
  symbols, routes, calls, effect candidates, and transparent structural signals;
- a repository-owned agent skill for source exploration and DSL authoring.

Important limits:

- stack detection is not deep semantic analysis;
- `inspect` does not write the DSL;
- Go and JS/TS have prototype source analyzers; Dart, Kotlin/Java, and
  Swift/Objective-C remain detection-only;
- detection-only stacks can still be visualized after the agent or a human
  investigates their approved source and authors valid DSL, but `analyze`
  contributes no language-specific semantic evidence for those stacks;
- analyzer evidence is static and does not establish runtime behavior or
  architecture by itself; JS/TS analysis is lexical rather than compiler-grade;
- `quality` converts measurements already present in analyzer evidence; it does
  not run tests, coverage, linters, compilers, or third-party audit tools;
- the current AI-authored DSL must be reviewed and corrected;
- the MVP composer applies direct-mutation correction paths; other validated
  correction paths are surfaced as generation warnings until implemented;
- the renderer consumes the selected decomposition and condition bands, while
  several declared road, label, boundary, and security style switches are
  still fixed prototype behavior;
- a missing expectation needs a matching structural `expected_component` to
  appear as a wireframe building in the current renderer;
- quality/security markings are evidence and review leads, not proof of bugs;
- layout and aggregation are an MVP, not a finished visualization system.

## What `inspect` does

```bash
./bin/vibecodemap inspect /absolute/path/to/app
./bin/vibecodemap inspect -json /absolute/path/to/app \
  > /tmp/vibecodemap-inspect.json
```

`inspect` walks the repository once, applies built-in rules, Git ignore rules,
an optional root `.vcmignore`, generated-file markers, and size budgets. It
returns the approved inventory plus stack candidates, candidate scopes,
evidence, confidence, and declared adapter support.

It does not resolve imports/calls, infer systems, detect effects, calculate
quality, or author DSL. The AI agent uses its output to avoid reading generated
or installed source and to decide which manifests, entrypoints, deployables,
and source relationships require investigation.

Scope is the repository file set and reading depth used by analyzers and the AI
mapper. Every entry is classified as `analyze` (full source), `summarize`
(metadata/volume only), `externalize` (dependency boundary), or `ignore`.
Reviewing this result is recommended; changing it is optional. Proceed when
owned source/manifests are analyzed and dependencies, generated code, caches,
and build output are not. The AI agent performs this review; a developer can
inspect or override it. `inspect` never creates `.vcmignore` automatically.

## What `analyze` does

```bash
./bin/vibecodemap analyze /absolute/path/to/app
./bin/vibecodemap adapters
```

`analyze` performs the same centrally scoped scan, detects all stacks, and
automatically invokes every implemented analyzer. Its default output is
`/absolute/path/to/app/.vibecodemap/generated/evidence.json`. Python AST, Go
AST, and JS/TS source analyzers are implemented. Mobile detections are recorded
as `not_implemented` instead of being silently treated as analyzed. The
evidence file assists the AI/human DSL author and is not itself DSL or a view
model. Each analyzer is bounded independently (`-adapter-timeout=2m` by
default). A missing runtime, analyzer failure, or timeout is recorded as
`runtime_unavailable`, `failed`, or `timed_out`; healthy adapters still write
their evidence and partial output from the unsuccessful adapter is discarded.
Do not treat one of these statuses as successful analysis of that language.

The Python runtime is health-checked before use. If its startup or required
standard-library imports stall, `analyze` reports `runtime_unavailable` after
the probe deadline instead of waiting forever. Select another interpreter when
available:

```bash
VIBECODEMAP_PYTHON=/opt/homebrew/bin/python3 \
  ./bin/vibecodemap analyze /absolute/path/to/app
```

## What `quality` does

After `model.vcm.yaml` exists, generate a quality document from analyzer
evidence and link it from `project.inputs.quality_model`:

```bash
./bin/vibecodemap quality \
  -evidence /absolute/path/to/app/.vibecodemap/generated/evidence.json \
  -output /absolute/path/to/app/.vibecodemap/quality.vcm.yaml \
  /absolute/path/to/app/.vibecodemap/model.vcm.yaml
```

The command matches evidence to structural artifacts by repository-relative
path and to elements by source symbol/range. Go and Python AST evidence can
therefore populate complexity, nesting, size, and direct-effect bands. JS/TS
lexical decision counts remain distinct from compiler-grade cyclomatic
complexity. Coverage remains explicitly unknown until a real,
revision-matched coverage result is imported.

Example `.vcmignore`:

```text
# A bare pattern means ignore.
fixtures/**

# Keep aggregate volume without indexing symbols.
summarize generated/**

# Model installed source as an external dependency.
externalize third_party/source/**

# Opt a false positive back into analysis.
!generated/maintained.go
```

## Mixed-stack and monorepo convention

Run `inspect` once at the repository root. Do not run it once per language.
The semantic model—not the detector list—defines the visual hierarchy:

| Map level | Software meaning |
|---|---|
| workspace | one unified project manifest and generated map |
| city (`system`) | product or independently understood operational system |
| area (`deployable`) | web/mobile app, service, worker, library release, or each independently built `cmd/*` binary |
| district | feature, subsystem, layer, ownership group, or editable cluster |
| building | component, interface, store, file, class, or function at the active detail level |

A React client, Python server, and Go worker for one product normally share one
city. Independent products become separate cities. Shared libraries remain
shared utility areas, and static/build dependencies stay visually distinct
from runtime communication. One structural model and one project manifest can
render all cities together. Very large monorepos may add optional focused maps
under `.vibecodemap/maps/`, but the unified workspace map is the default.

See [Multi-stack and monorepo modeling](docs/MULTI-STACK.md).

## Architecture

| Part | Technology | Responsibility |
|---|---|---|
| Core and document generator | Go | inventory, validation, evidence composition, aggregation, view-model JSON, HTML generation, and browser launch |
| Language adapters | in-process Go or versioned subprocesses | compiler/analyzer facts; an adapter may be written in any language if it obeys the central scope/evidence contracts |
| Browser presentation | Three.js and plain JavaScript | 3D layout, picking, semantic filtering, legends, source navigation, and fallback presentation |
| Semantic mapping | checked-in AI agent skill | workload discovery, source-linked architecture/effect inference, DSL authoring, and iterative correction |

Go generates the HTML and Three.js renders it; a Node server or Go/WASM browser
runtime is not required. See [ADR-001](docs/ADR-001-IMPLEMENTATION-STACK.md).

The committed YAML is durable, editable semantic intent. Generated view-model
JSON is a renderer-ready compiled artifact, not a second DSL. See
[Project DSL and renderer view model](docs/VIEW-MODEL.md).

## Visual interpretation

Static imports and type references are topology, not observed communication.
They help explain coupling and clustering but are hidden from the default
**Behavior** road view. Calls/events, state access, and provider I/O remain
separately typed lanes. Width represents aggregate mapped evidence, not
measured traffic unless runtime evidence explicitly says so.

## Validate and test

```bash
make check

./bin/vibecodemap schema structural
./bin/vibecodemap schema quality

./bin/vibecodemap validate examples/uzumtools/uzumtools.vcm.yaml
./bin/vibecodemap validate \
  -core examples/uzumtools/uzumtools.vcm.yaml \
  examples/uzumtools/uzumtools.quality.vcm.yaml
./bin/vibecodemap validate examples/uzumtools/uzumtools.project.vcm.yaml
```

## Near-term work

1. Upgrade Go and TypeScript/JavaScript analysis to compiler/project-aware
   adapters, then add mobile-stack analyzers.
2. Add workload discovery and optional deterministic DSL generation.
3. Add replaceable Leiden/CPM clustering and revision-stability reports.
4. Improve large-workspace layout, offline renderer packaging, and accessible
   2D/table views.
5. Test against real Go, Python, web, and mobile repositories.

## License

Licensed under the [Apache License 2.0](LICENSE).
