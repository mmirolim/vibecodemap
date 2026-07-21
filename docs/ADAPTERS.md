# Repository inventory and adapter boundary

VibeCodeMap does not attempt to understand every language with one parser. The
Go core owns repository discovery, exclusions, budgets, and adapter
orchestration. A stack adapter receives a deterministic list of approved files
and emits renderer-neutral, source-linked evidence.

```text
repository
    │
    ├─ built-in scope rules
    ├─ Git ignore rules
    ├─ .vcmignore corrections
    └─ generated-header and size budgets
            │
            ▼
    inspectable inventory
            │
            ├─ stack detectors
            └─ language-native analyzers
                    │
                    ▼
         versioned evidence events
```

This boundary matters for both correctness and cost. An adapter must not recurse
through the repository independently: that would let Python, Go, Swift, Kotlin,
and Dart disagree about whether `node_modules`, `DerivedData`, `.gradle`,
`.dart_tool`, generated RPC code, or build output belongs to the application.

## Current support

| Adapter | Detection | Semantic analysis | Intended native foundation |
|---|---|---|---|
| Python | available | implemented prototype, Go-orchestrated | embedded Python `ast` adapter (requires Python 3.10+) |
| Go | available | implemented AST prototype | standard Go parser/AST; future [`go/packages`](https://pkg.go.dev/golang.org/x/tools/go/packages), `go/types`, and SSA |
| JavaScript / TypeScript | available | implemented lexical prototype | dependency-free source analysis; future TypeScript compiler/project model |
| Flutter / Dart | available | not implemented | [Dart analyzer plugins](https://dart.dev/tools/analyzer-plugins), package and Flutter metadata |
| Android / Kotlin / Java | available | not implemented | Gradle/Kotlin tooling, [Android manifests](https://developer.android.com/guide/topics/manifest/manifest-intro.html), lint/test reports |
| Apple / Swift / Objective-C | available | not implemented | [SourceKit-LSP and Swift source tooling](https://www.swift.org/documentation/source-code/), Xcode project/index metadata |

“Detection available” means the CLI can identify the stack and its module
scope. It does not mean calls, side effects, navigation, or quality have been
analyzed. That distinction is part of the machine-readable adapter descriptor.

Compiler-grade Go analysis and deep mobile semantic analysis remain future
work. Appropriate foundations include Go `go/packages`, Swift source tooling,
the Kotlin/Gradle tooling APIs, Dart analyzer plugins, and Android manifests,
as linked above. Detector registration deliberately does not claim unbuilt
analyzers already exist.

Check the executable truth directly:

```bash
vibecodemap adapters
vibecodemap adapters -json
```

The status separates `detection`, `semantic_analysis`, and
`runtime_available`. `vibecodemap analyze REPOSITORY` scans once, dispatches
implemented analyzers automatically, and writes
`REPOSITORY/.vibecodemap/generated/evidence.json`. It records unsupported
detections as `not_implemented`.

An adapter does not have to be written in the language it analyzes. It must
obey the request/event protocol and state its precision limits. Python is an
embedded Python subprocess because its standard-library AST is useful; Go and
JS/TS analysis run in-process in Go. The JS/TS prototype is lexical rather than
compiler-grade, so aliases, types, JSX structure, and dynamic dispatch remain
unresolved. All detectors are Go code, and there are no separate adapter
packages for users to install.

## Mixed-stack repositories

Run `inspect` once at the highest repository root. All detectors run over that
same centrally scoped inventory and can return several detections together;
there is no per-language or per-directory inspection phase. Detection scopes
are parser/reading candidates, however, not confirmed products or deployables.
Some current detectors identify nested manifest roots while Python and JS/TS
may conservatively fall back to repository scope.

Systems should be grouped by product, deployment, runtime, ownership, and data
boundaries—not by language. The current DSL and renderer can express multiple
systems as cities and deployables as area-level elements. Workload discovery
is still primarily performed by the AI skill or a human; detector output does
not automatically author those boundaries. See
[Multi-stack and monorepo modeling](MULTI-STACK.md).

After reviewing scope—and correcting it only when necessary—run `analyze` once
at the same root. Its evidence is a deterministic aid where analyzers exist.
For detection-only stacks, the AI skill reads approved source and tools
directly and labels its architectural conclusions `ai_inferred`.

## Stable contracts

`vibecodemap.adapter-request/0.1` contains:

- repository root;
- adapter identity;
- requested capabilities;
- only `analyze` and `summarize` files from the central inventory;
- action and classification for each file.

`vibecodemap.evidence-event/0.1` contains:

- stable evidence ID and semantic kind;
- subject and producer;
- confidence;
- an optional path, symbol, line, and column source location;
- a versioned JSON payload.

The payload may evolve by adapter and evidence kind. Identity, confidence,
provenance, and source navigation do not.

`vibecodemap.evidence-bundle/0.1` is the JSON file written by `analyze`. It
contains detections, one explicit run status per detected adapter, and the
validated evidence events. It is an intermediate report for an AI or human DSL
author—not structural DSL, quality DSL, or renderer view JSON. After a
structural model exists, `vibecodemap quality STRUCTURAL.vcm.yaml` can convert
supported source-linked measurements in this bundle into validated quality
DSL. That bridge does not infer architecture or execute external quality tools;
unsupported metrics and missing coverage remain explicit unknowns.

## Mobile-specific semantics

Mobile support cannot be reduced to parsing Swift, Kotlin, or Dart syntax. A
useful adapter should expose these optional capabilities:

- `entrypoints`: application, scene, activity, service, isolate, and background
  entrypoints;
- `ui_composition`: SwiftUI/UIKit, Compose/XML, or Flutter widget composition;
- `navigation`: routes, destinations, deep links, and presentation transitions;
- `lifecycle`: foreground/background, activity/scene/widget lifecycle, and
  state restoration;
- `permissions`: declared permissions, entitlements, and sensitive platform
  APIs;
- `platform_boundaries`: Flutter method channels, JNI/FFI, extensions, widgets,
  platform services, and other cross-runtime boundaries.

These produce ordinary VCM elements and relations plus mobile evidence. The
core graph, quality model, clustering, roads, source links, and renderer do not
change for each language. A genuinely new concept extends the versioned model;
language-specific syntax stays inside its adapter.

## Adding an adapter

1. Register a conservative detector and descriptor.
2. Consume the central `AnalyzeRequest`; never perform an unscoped repository
   walk.
3. Use the language's compiler, analyzer, project model, or established tools
   where available.
4. Emit evidence with exact source locations and honest confidence.
5. Keep unresolved dynamic behavior as a candidate or unknown state.
6. Add a small real repository fixture and conformance tests.

An AI agent may interpret ambiguous ownership, requirements, or design intent,
but deterministic imports, syntax, coverage, complexity, manifests, and source
locations should come from tools. Agent conclusions remain a separate,
editable provenance layer.
