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
| Python | available | prototype, not orchestrated | Python `ast`, established quality and coverage tools |
| Go | available | not implemented | [`go/packages`](https://pkg.go.dev/golang.org/x/tools/go/packages), `go/types`, AST/SSA |
| JavaScript / TypeScript | available | not implemented | TypeScript project service/compiler model plus ecosystem analyzers |
| Flutter / Dart | available | not implemented | [Dart analyzer plugins](https://dart.dev/tools/analyzer-plugins), package and Flutter metadata |
| Android / Kotlin / Java | available | not implemented | Gradle/Kotlin tooling, [Android manifests](https://developer.android.com/guide/topics/manifest/manifest-intro.html), lint/test reports |
| Apple / Swift / Objective-C | available | not implemented | [SourceKit-LSP and Swift source tooling](https://www.swift.org/documentation/source-code/), Xcode project/index metadata |

“Detection available” means the CLI can identify the stack and its module
scope. It does not mean calls, side effects, navigation, or quality have been
analyzed. That distinction is part of the machine-readable adapter descriptor.

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
