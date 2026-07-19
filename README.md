# VibeCodeMap

> **Experimental / work in progress.** VibeCodeMap is a research prototype,
> not yet a dependable code-audit product. Its maps can guide investigation;
> they do not prove that software is correct, secure, or complete.

VibeCodeMap explores how humans can review AI-generated software at system
scale: as a navigable, evidence-backed condition landscape rather than only as
thousands of source lines. The intended result combines architecture,
interaction, code condition, missing expectations, provenance, and direct
source navigation in one zoomable 3D map.

## What works today

- versioned structural, quality, clustering/visual, and project DSL drafts with
  JSON Schemas;
- a Go CLI that prints and validates the project grammar, inventories a
  repository, explains scope decisions, and detects repository stacks;
- built-in exclusions for installed dependencies, generated sources, caches,
  build artifacts, Go/Python/JS, and Apple/Android/Flutter conventions;
- root-owned `.vcmignore` corrections with `analyze`, `summarize`,
  `externalize`, and `ignore` actions;
- a renderer-neutral adapter request/evidence boundary and conservative stack
  detectors for Python, Go, JavaScript/TypeScript, Flutter/Dart,
  Android/Kotlin/Java, and Apple/Swift/Objective-C;
- explainable multi-signal affinity and hub metrics plus deterministic
  district-road/lane aggregation;
- conservative Python AST and quality extraction prototypes;
- source-linked Uzumtools fixtures and Three.js interaction experiments,
  including simultaneous condition bands, behavior roads, inputs/outputs,
  persistence, security-review markers, semantic zoom, and source focus.

The generic data-driven renderer, `render` command, analyzer subprocess
orchestration, native semantic adapters beyond the Python prototypes, and a
concrete Leiden/Infomap clustering engine do **not** exist yet. Current HTML
scenes contain curated fixture data and test visual semantics rather than
forming an end-to-end product.

## Try the current CLI

Go 1.24 or newer is required by the current module.

```bash
go run ./cmd/vibecodemap inspect .
go run ./cmd/vibecodemap inspect -json . > /tmp/vibecodemap-inventory.json
go run ./cmd/vibecodemap adapters

go run ./cmd/vibecodemap describe
go run ./cmd/vibecodemap schema
go run ./cmd/vibecodemap validate \
  examples/uzumtools/uzumtools.project.vcm.yaml
```

The inventory applies built-in rules, exact Git ignore semantics when Git is
available, an optional root `.vcmignore`, bounded generated-file header checks,
and a configurable large-file budget. Use `inspect -entries` to see every
non-analyzed entry and the rule responsible.

An optional `.vcmignore` is committed with the repository:

```text
# A bare pattern means ignore.
fixtures/**

# Preserve aggregate volume without indexing symbols.
summarize generated/**

# Model installed source as an external dependency.
externalize third_party/source/**

# A later, higher-priority correction can opt a false positive back in.
!generated/maintained.go
```

Patterns are root-relative slash-separated globs with `**`. Directory pruning
means a child cannot be recovered after its parent was ignored; use a narrower
parent rule when an exception is required.

## Architecture

| Part | Technology | Responsibility |
|---|---|---|
| Core and document generator | Go | inventory, validation, adapter orchestration, evidence normalization, aggregation, revision comparison, and eventually standalone HTML generation |
| Language adapters | language-native tools or subprocesses | compiler/analyzer facts, calls, effects, quality, coverage, tests, and stack-specific semantics |
| Browser presentation | Three.js, currently plain JavaScript prototypes | layout, WebGL rendering, semantic zoom, picking, legends, source navigation, and accessible alternatives |
| Semantic agent workflow | AI agent/skill, planned | requirements, ambiguous design intent, candidate architecture, explanations, and reviewable corrections |

Go generating HTML and Three.js rendering it are complementary: no Node server
or Go/WASM browser runtime is required. See
[ADR-001](docs/ADR-001-IMPLEMENTATION-STACK.md).

The committed project DSL is durable, editable intent and policy. A generic
view-model JSON is its derived, renderer-ready projection—not a competing DSL.
The distinction is described in [Project DSL and renderer view model](docs/VIEW-MODEL.md).

## Stack adapters

Current support states are deliberately explicit:

| Stack | Detection | Semantic evidence |
|---|---|---|
| Python | available | prototype scripts, not yet orchestrated |
| Go | available | planned through `go/packages`, `go/types`, AST/SSA |
| JavaScript / TypeScript | available | planned through project/compiler tooling |
| Flutter / Dart | available | planned through the Dart analyzer and Flutter metadata |
| Android / Kotlin / Java | available | planned through Gradle/Kotlin, manifests, lint and test outputs |
| Apple / Swift / Objective-C | available | planned through SourceKit-LSP/compiler and Xcode metadata |

Mobile adapters extend the shared graph with entrypoints, UI composition,
navigation, lifecycle, permissions, and platform boundaries. They do not fork
the renderer or core model for each language. See
[Repository inventory and adapter boundary](docs/ADAPTERS.md).

## Visual interpretation

Static imports and type references are architectural topology, not runtime
communication. They are retained for coupling and clustering but hidden from
the system city's default **Behavior** road view. Runtime calls/events, state
access, and external I/O remain separately typed lanes. Width represents named
aggregate evidence strength, not measured traffic unless runtime observations
explicitly say so.

The latest interaction experiments are documented in
[prototype/README.md](prototype/README.md). The DSLs are in [dsl/](dsl), and the
source-linked fixture is in [examples/uzumtools/](examples/uzumtools).

## Validate and test

```bash
python3 tools/validate_vcm.py examples/uzumtools/uzumtools.vcm.yaml
python3 tools/validate_quality_vcm.py \
  examples/uzumtools/uzumtools.quality.vcm.yaml \
  --core examples/uzumtools/uzumtools.vcm.yaml
python3 tools/validate_project_vcm.py \
  examples/uzumtools/uzumtools.project.vcm.yaml

go test ./...
go vet ./...
```

## Near-term work

1. Feed centrally scoped inventory files into the Python and Go analyzers and
   normalize their evidence events.
2. Compose project DSL plus generated facts into a versioned generic view model.
3. Make the Three.js runtime consume that model and generate a standalone HTML
   map from the Go CLI.
4. Add one native semantic adapter at a time, beginning with Go and Python,
   using real repositories as conformance fixtures.
5. Add a replaceable Leiden/CPM cluster engine and revision-stability reports.

## License

Licensed under the [Apache License 2.0](LICENSE). It is a permissive OSS license
with an explicit patent grant, which is useful for an extensible developer-tool
ecosystem.
