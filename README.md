# VibeCodeMap

> **Experimental / work in progress.** VibeCodeMap is a research prototype,
> not a dependable code-audit product. Its maps guide investigation; they do
> not prove that software is correct, secure, or complete.

VibeCodeMap turns source-linked descriptions of software into a navigable 3D
condition map. The goal is to review architecture, interactions, side effects,
quality signals, security leads, and missing expectations at system scale—not
only as thousands of generated source lines.

## See the prototype

Go 1.24 or newer is required. Python is optional and is currently used only by
prototype Python extractors and validators.

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

## Current end-to-end flow

```text
clone VibeCodeMap and build the Go CLI
        ↓
run one root inspect over the target repository
        ↓
review/correct scope with target/.vcmignore
        ↓
AI agent explores approved source and writes target/.vibecodemap/*.yaml
        ↓
vibecodemap show project.vcm.yaml
        ↓
automatic validation → view JSON → HTML → browser
        ↓
review the map, source links, warnings, and editable DSL; iterate
```

The checked-in `$analyze-with-vibecodemap` agent skill drives this whole flow.
Today AI is the primary semantic mapper between repository files and the DSL;
`inspect` is its bounded, explainable reading plan, not an architecture
generator.

Example prompt from the VibeCodeMap checkout:

```text
Use $analyze-with-vibecodemap to map /absolute/path/to/my-app end to end.
Build the CLI if needed, inspect the repository once at its root, write
source-linked DSL under .vibecodemap, and open the generated 3D map.
```

## What works today

- a Go CLI for repository inventory, stack detection, DSL validation,
  composition, standalone HTML generation, and browser launch;
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
- concurrent stack detection for Python, Go, JavaScript/TypeScript,
  Flutter/Dart, Android/Kotlin/Java, and Apple/Swift/Objective-C;
- explainable affinity and hub metrics plus deterministic district-road/lane
  aggregation;
- conservative Python AST and quality extraction prototypes;
- a repository-owned agent skill for source exploration and DSL authoring.

Important limits:

- stack detection is not deep semantic analysis;
- `inspect` does not write the DSL;
- native Go, JS/TS, Swift, Kotlin, and Dart semantic adapters are not yet
  orchestrated;
- the current AI-authored DSL must be reviewed and corrected;
- the MVP composer applies direct-mutation correction paths; other validated
  correction paths are surfaced as generation warnings until implemented;
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
| Language adapters | language-native tools or subprocesses | compiler/analyzer facts, calls, effects, quality, coverage, tests, and stack semantics |
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

python3 tools/validate_vcm.py examples/uzumtools/uzumtools.vcm.yaml
python3 tools/validate_quality_vcm.py \
  examples/uzumtools/uzumtools.quality.vcm.yaml \
  --core examples/uzumtools/uzumtools.vcm.yaml
python3 tools/validate_project_vcm.py \
  examples/uzumtools/uzumtools.project.vcm.yaml
```

## Near-term work

1. Orchestrate native semantic adapters, beginning with Go and Python.
2. Add workload discovery and optional deterministic DSL generation.
3. Add replaceable Leiden/CPM clustering and revision-stability reports.
4. Improve large-workspace layout, offline renderer packaging, and accessible
   2D/table views.
5. Test against real Go, Python, web, and mobile repositories.

## License

Licensed under the [Apache License 2.0](LICENSE).
