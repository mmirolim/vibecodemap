# Uzumtools DSL fixture

This fixture applies VibeCodeMap DSL 0.1 to the runtime portion of a separate
Uzumtools checkout. The checked-in models use the sibling path
`../../../uzumtools/photochecker`; substitute your own checkout when
regenerating them.

It currently contains:

- 72 source-linked file summaries;
- 48 semantic architecture elements;
- 33 typed communication and side-effect relations;
- 4 end-to-end flows;
- 4 scoped architecture-style assessments;
- 2 declared architecture constraints;
- 6 evidence-backed findings;
- 4 renderer view definitions.

The companion quality fixture, `uzumtools.quality.vcm.yaml`, adds:

- 8 explicitly defined quality metrics;
- 23 source-linked measurements with provenance and freshness;
- 5 transparent review-priority results;
- 4 quality lenses with stable geometry and semantic-zoom rules.

The repository-owned project manifest, `uzumtools.project.vcm.yaml`, adds:

- Python, JavaScript, TypeScript, and Go adapter profiles;
- ordered `analyze`, `summarize`, `externalize`, and `ignore` source-scope rules;
- dependency, generated-code, cache, and build-output exclusions;
- declared and editable inferred decompositions with stable district codes;
- explicit expectations and human corrections that survive regeneration;
- explicit input/output boundaries, payload summaries, persistent resources,
  and source-linked security review candidates;
- a road-city profile with four simultaneous building bands, aggregate district
  roads, typed directional lanes, `D1 → D2` endpoint codes, and bounded feeders.

The scope intentionally excludes tests, migrations, root-level diagnostic
scripts, plans, and most documentation. That limitation is recorded in the DSL
itself so the map cannot honestly claim whole-repository completeness.

## Validate

```bash
go run ./cmd/vibecodemap validate \
  examples/uzumtools/uzumtools.vcm.yaml
```

The Go validator checks JSON Schema shape, IDs, graph/flow references,
provenance, runtime claims, and—when the separate source checkout is
available—artifact paths, line counts, source ranges, and artifact coverage of
the declared include/exclude scope.

Validate the quality extension and all references back to the structural model:

```bash
go run ./cmd/vibecodemap validate \
  -core examples/uzumtools/uzumtools.vcm.yaml \
  examples/uzumtools/uzumtools.quality.vcm.yaml
```

Validate the project manifest and its editable policy:

```bash
go run ./cmd/vibecodemap validate \
  examples/uzumtools/uzumtools.project.vcm.yaml
```

Project validation automatically validates the referenced structural and
quality documents as well as cross-reference, ID, district-code,
correction-operation, pattern, and render-profile invariants.

The Go CLI embeds both contracts so agents and humans can inspect the exact
accepted language without locating documentation files:

```bash
go run ./cmd/vibecodemap describe
go run ./cmd/vibecodemap schema
```

## Run the Python evidence adapter

```bash
go run ./cmd/vibecodemap analyze /path/to/uzumtools/photochecker
```

The Go command applies central exclusions and invokes the embedded Python AST
adapter automatically. It writes
`.vibecodemap/generated/evidence.json` containing source-linked files,
symbols, routes, imports, calls, execution syntax, effect candidates, and
transparent static complexity/nesting facts. It does not infer architecture,
runtime behavior, coverage, Git churn, or a quality score.

## What this iteration exposed

1. Files and architecture components must be separate concepts. One component
   spans files, and one large file can implement several components.
2. `async: true/false` is inadequate. Caller behavior, blocking, concurrency,
   delivery, and execution location need separate fields.
3. Side effects work best as typed relations to stateful resources. They can then
   be aggregated from operation to component to deployable without losing source
   evidence.
4. Names are not behavior. `JobService` persists records but does not enqueue or
   execute background work.
5. Architectural styles must be scoped and partial. This application is a
   modular monolith with service and strategy patterns, but its API boundary
   still owns substantial cross-feature workflow.
6. Missing structure is representable. The client call to `/api/analytics` maps
   to an expected endpoint with `implementation: missing`, not to an invented
   server component.

## Current limitations

- The interactive conversation view is an initial curated projection of this
  fixture, not yet a generic renderer that reads arbitrary VCM files.
- Python, JavaScript, and TypeScript now have orchestrated source analyzers;
  templates and CSS still require repository-aware AI investigation.
- No runtime traces were collected, so runtime state is consistently marked
  `not_observed`.
- Source buttons rely on a host adapter. The current conversation view asks
  Codex to open the exact path and line; an IDE extension could open it directly.
- Coverage and Git churn in this committed fixture came from earlier named
  evidence; the current Python adapter does not collect them automatically.
- The current AST complexity calculation is a conservative prototype. A
  production Python adapter should ingest Ruff/Radon output and SARIF rather
  than treating this implementation as a cross-language standard.

## Best next iteration

Build the mapper as an agent skill with a strict evidence contract:

1. run language-specific fact collectors;
2. write or update a VCM YAML model;
3. validate before rendering;
4. preserve stable IDs and human corrections;
5. ingest analyzer evidence without asking the AI to invent numeric metrics;
6. generate Three.js views directly from `views[]`, quality lenses, and the
   semantic graph;
7. diff the previous and current model after each meaningful code change.

The next useful experiment is to make the current Three.js view data-driven,
then map a second, structurally different repository without changing the DSL.
That tests both halves of the idea: whether semantic zoom remains readable and
whether the evidence model generalizes beyond one Python application.
