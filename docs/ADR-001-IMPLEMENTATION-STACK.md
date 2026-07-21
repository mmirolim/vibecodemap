# ADR-001: Go core and document generator with a Three.js browser runtime

- Status: accepted; initial implementation exists
- Date: 2026-07-18

## Context

VibeCodeMap must scan large repositories, coordinate language-specific tools,
normalize evidence, validate and aggregate a graph, compare revisions, serve a
local UI, and expose operations to AI agents. It must also provide a highly
interactive browser renderer with WebGL picking, camera control, semantic zoom,
tooltips, legends, and source navigation.

“Go renders the map” has two materially different meanings. Go can generate the
complete deliverable—validate the DSL, compute the view model, and write a
standalone HTML document—while Three.js executes the WebGL scene in the browser.
That is the intended architecture. It does not require a Node.js server or Go
WASM. Using Go itself as the browser WebGL runtime would require a Go/WASM bridge
and is a separate, currently unjustified design.

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
- generation of a standalone HTML document from validated project data;
- optional local HTTP/API serving and, later, an MCP surface;
- embedding the versioned renderer assets in release binaries.

Go is appropriate here because it produces a portable single binary, has good
filesystem and process primitives, supports concurrent independent analyzers,
and can generate or serve the resulting application without a separate server
runtime.

### Three.js browser runtime

The generated document's browser code owns:

- WebGL rendering through Three.js;
- stable layout and camera behavior;
- semantic zoom and bounded relationship views;
- quality lenses and explicit legends;
- pointer and keyboard interaction;
- source navigation and agent follow-up actions;
- accessible 2D/table alternatives using the same model.

The renderer receives a validated renderer-neutral JSON view model emitted by
Go. It does not compute analyzer metrics or silently reinterpret metric
direction, freshness, or missing values. TypeScript is useful for maintaining a
larger renderer, but it is a build-time implementation choice rather than a
runtime requirement; the early prototype may remain plain JavaScript.

The implemented CLI boundary is:

```text
vibecodemap describe [project]       # print the project-manifest grammar
vibecodemap schema [kind]            # print project, structural, or quality schema
vibecodemap inspect repository       # inventory and stack candidates
vibecodemap analyze repository       # implemented adapter evidence
vibecodemap quality structure.yaml   # evidence-backed quality DSL
vibecodemap validate document.yaml   # syntax, schema, and semantic references
vibecodemap render -output map.html project.yaml
vibecodemap show project.yaml        # render and open the browser
```

`render` validates first, composes the current structural, quality, boundary,
and security records into one view model, then injects that model into the
embedded browser template. `show` adds browser launch. Requirements/runtime
evidence remain areas for deeper composition. The renderer must not contain a
second set of architectural inference rules hidden in the HTML template.

### Analyzer adapters

Adapters may run in-process in Go or as versioned subprocesses written in the
most appropriate language. Users invoke them centrally through `analyze`; they
do not install or run each adapter separately. Adapters emit normalized inputs
such as:

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

### Go as the in-browser WebGL runtime

Go/WASM can drive WebGL, but it adds a bridge to browser APIs and the Three.js
ecosystem without improving the evidence model. It also increases binary size,
debugging complexity, and UI iteration cost. This rejection does not reject a
Go CLI that generates HTML; that generator is part of the accepted design.
Reconsider WASM only if profiling shows an actual browser-side computation
bottleneck that cannot be moved to a worker or the Go core.

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
- The prototype is one executable that prints the DSL, validates models, and
  emits a generated map document. Its HTML/data/template are embedded; the
  current Three.js module is CDN-loaded until offline asset packaging lands.
- Python remains a development dependency only for Python adapters that need it.
- SQLite is deferred until scan history/query needs justify it; revision JSON
  snapshots are sufficient for the next experiment.
