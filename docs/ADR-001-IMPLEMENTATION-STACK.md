# ADR-001: Go core with a TypeScript/Three.js renderer

- Status: accepted for the next prototype
- Date: 2026-07-18

## Context

VibeCodeMap must scan large repositories, coordinate language-specific tools,
normalize evidence, validate and aggregate a graph, compare revisions, serve a
local UI, and expose operations to AI agents. It must also provide a highly
interactive browser renderer with WebGL picking, camera control, semantic zoom,
tooltips, legends, and source navigation.

One implementation language does not solve both concerns equally well. Using
Go for browser rendering would either abandon Three.js or introduce a Go/WASM
bridge around a JavaScript rendering ecosystem. Neither reduces the hard part:
designing a stable evidence and interaction contract.

## Decision

Use a split architecture.

### Go core

The Go process owns:

- the `vcm` CLI and optional long-running local service;
- repository inventory and scan orchestration;
- a versioned subprocess protocol for analyzer adapters;
- normalization into the canonical VCM evidence graph;
- schema and semantic validation;
- aggregation for semantic zoom;
- revision snapshots and diffs;
- local HTTP/API serving and, later, an MCP surface;
- embedding the compiled web application in release binaries.

Go is appropriate here because it produces a portable single binary, has good
filesystem and process primitives, supports concurrent independent analyzers,
and can serve the resulting local application without a separate runtime.

### TypeScript and Three.js renderer

The browser application owns:

- WebGL rendering through Three.js;
- stable layout and camera behavior;
- semantic zoom and bounded relationship views;
- quality lenses and explicit legends;
- pointer and keyboard interaction;
- source navigation and agent follow-up actions;
- accessible 2D/table alternatives using the same model.

The renderer receives validated JSON. It does not compute analyzer metrics or
silently reinterpret metric direction, freshness, or missing values.

### Analyzer adapters

Adapters are separate executables or scripts. They may be written in the most
appropriate language and emit one of these normalized inputs:

1. a versioned VCM evidence event stream;
2. SARIF for discrete analyzer findings;
3. a documented coverage or profiler format;
4. a VCM JSON patch with producer and scope metadata.

The existing Python AST adapters remain useful for the Python wedge. They do
not need to be rewritten in Go before the data contract stabilizes. Production
complexity values should come from established analyzers such as Ruff or Radon.

### AI integration

An agent skill invokes the Go core, reads validated evidence, maps source facts
to architecture and requirements, and writes evidence-linked semantic findings.
The Go process can later expose the same operations through MCP. Numeric facts
remain analyzer-produced; AI interpretations remain visibly distinct findings.

## Rejected alternatives

### Go for both core and rendering

Go/WASM can render WebGL, but it adds a bridge to browser APIs and the Three.js
ecosystem without improving the evidence model. It also increases binary size,
debugging complexity, and UI iteration cost. Reconsider only if profiling shows
an actual browser-side computation bottleneck that cannot be moved to a worker
or the Go core.

### TypeScript/Node for everything

This would simplify language count, but a Node runtime is less attractive for a
portable local repository tool and analyzer supervisor. It remains a viable
fallback if Go materially slows early iteration.

### Rewrite every analyzer in Go

This would recreate mature language tooling and still fail to cover dynamic or
stack-specific behavior. VibeCodeMap should normalize evidence, not pretend one
parser can replace every compiler, linter, test runner, and profiler.

## Consequences

- The JSON/process boundary must be versioned and tested early.
- Go and TypeScript share generated schema types or conformance fixtures rather
  than hand-maintained duplicate models.
- The first release can still be one executable by embedding compiled web
  assets in Go.
- Python remains a development dependency only for Python adapters that need it.
- SQLite is deferred until scan history/query needs justify it; revision JSON
  snapshots are sufficient for the next experiment.
