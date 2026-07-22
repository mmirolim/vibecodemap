# Get from clone to a browser map

VibeCodeMap's primary outcome is an interactive browser visualization. The
current prototype uses a Go CLI for deterministic inventory, validation,
composition, and rendering, plus an AI agent skill for the semantic work of
understanding a repository and authoring the editable DSL.

## 1. Clone and build VibeCodeMap

Go 1.24 or newer is required. Python 3.10 or newer is needed only to analyze
Python source and may be selected with `VIBECODEMAP_PYTHON`. Validation,
composition, and HTML generation run in the Go binary; the generated page
renders with Three.js in the browser.

```bash
git clone https://github.com/mmirolim/vibecodemap.git
cd vibecodemap
make build
./bin/vibecodemap help
```

The executable is `./bin/vibecodemap`. You can first verify the renderer with
the checked-in source-linked fixture:

```bash
./bin/vibecodemap show \
  examples/uzumtools/uzumtools.project.vcm.yaml
```

This validates the three fixture documents, generates view JSON and HTML under
`examples/uzumtools/out/`, and opens the map.

## 2. Ask the checked-in agent skill to map an app

Open Codex from the cloned VibeCodeMap directory so it discovers
`.agents/skills/analyze-with-vibecodemap`, then ask:

```text
Use $analyze-with-vibecodemap to map /absolute/path/to/my-app end to end.
Build VibeCodeMap if needed, plan and track the repository investigation,
write source-linked structural and quality DSL under .vibecodemap, and open
the generated visualization in my browser.
```

The skill is intentionally responsible for the complete workflow. It should
not stop after inventory or DSL validation when a browser map was requested.

## 3. Understand the work performed

The agent first runs one repository-root inspection. Use JSON for the agent or
text for a human; running both performs two scans:

```bash
./bin/vibecodemap inspect -json /absolute/path/to/my-app
# Human-readable alternative:
./bin/vibecodemap inspect /absolute/path/to/my-app
```

`inspect` is a scope and detection report, not the semantic map. It:

- applies built-in dependency, generated-source, cache, and build exclusions;
- applies Git ignore rules and an optional root `.vcmignore`;
- reports `analyze`, `summarize`, `externalize`, and `ignore` totals;
- detects multiple stack candidates and candidate scopes;
- exposes approved files, evidence, confidence, and adapter support in JSON.

It does not resolve calls/imports, decide system boundaries, infer side
effects, compute quality, write YAML, or render HTML.

If the scope is wrong, the agent can propose or—when authorized—write a root
`.vcmignore`, rerun `inspect`, and compare the result:

```text
fixtures/**
summarize generated/**
externalize third_party/source/**
!generated/maintained.go
```

Scope correction is optional. `inspect` already computes a default scope from
built-in rules, Git ignore rules, generated-file markers, size limits, and any
existing `.vcmignore`. Its four actions mean:

- `analyze`: read and parse full owned source;
- `summarize`: retain metadata/volume without detailed symbol extraction;
- `externalize`: retain a dependency/boundary without reading internals;
- `ignore`: omit irrelevant or derived content.

The AI agent reviews the inventory and reasons; the developer may review or
override them. Add `.vcmignore` only if owned source is excluded/summarized, or
dependencies, generated code, caches, fixtures, vendored source, or build
outputs are being analyzed. If those classifications already look right,
proceed without creating the file.

After scope is acceptable—changed or unchanged—run implemented semantic
adapters once over that same root:

```bash
./bin/vibecodemap analyze /absolute/path/to/my-app
./bin/vibecodemap adapters
```

`analyze` writes `.vibecodemap/generated/evidence.json`. The Go process sends
only centrally approved files to each detected analyzer. Python uses its AST;
Go uses the standard Go parser/AST; JS/TS uses a conservative dependency-free
lexical analyzer. They extract imports, symbols, calls, entrypoints,
route/effect candidates, and transparent structural metrics within their stated
limits. Dart, Kotlin/Java, and Swift/Objective-C remain `not_implemented`, so
the agent investigates their approved source directly. Adapter evidence is
input to DSL authoring, not architecture, DSL, or a rendered map.

Analyzer subprocesses are supervised. The Python startup/import probe is
bounded, and every analyzer has a two-minute default deadline configurable
with `-adapter-timeout`. Runs marked `runtime_unavailable`, `failed`, or
`timed_out` produced no retained evidence for that adapter; continue the source
investigation explicitly or rerun with a working runtime. When multiple Python
installations exist, select one without changing repository configuration:

```bash
VIBECODEMAP_PYTHON=/absolute/path/to/python3 \
  ./bin/vibecodemap analyze -adapter-timeout=5m /absolute/path/to/my-app
```

## 4. Review the durable model

For an ordinary repository the agent writes:

```text
my-app/.vibecodemap/
  .gitignore           # normally ignores out/ and generated/
  model.vcm.yaml       # elements, source artifacts, relations, flows, findings
  quality.vcm.yaml     # deterministic measurements and explicit unknowns
  project.vcm.yaml     # scope, districts, expectations, boundaries, view policy
  out/                 # generated and ignored JSON/HTML
```

The YAML is the editable source of truth. Important elements should link to
repository-relative files, symbols, and lines. AI interpretations carry
`ai_inferred` provenance, confidence, rationale, and evidence. Unknown quality
or security state remains unknown rather than becoming a favorable zero.

You can correct the YAML manually or ask the agent to investigate a suspicious
district, relation, mutation, metric, or missing requirement and update it.
In this standard layout, set both model repository roots to `..` so source
navigation resolves from `.vibecodemap/` back to the target repository.

The MVP composer currently applies `effects.*.mutates_state` corrections to the
building grounding/effect band. Unsupported correction paths are not silently
ignored: they appear as generation warnings. Represent a missing requirement
as an `expected_component` structural element (with missing implementation and
evidence) when it must appear as a wireframe building.

## 5. Generate measured quality

Once the structural model exists, convert analyzer evidence to quality DSL:

```bash
./bin/vibecodemap quality \
  -evidence /absolute/path/to/my-app/.vibecodemap/generated/evidence.json \
  -output /absolute/path/to/my-app/.vibecodemap/quality.vcm.yaml \
  /absolute/path/to/my-app/.vibecodemap/model.vcm.yaml
```

Then set `project.inputs.quality_model: quality.vcm.yaml`. The command validates
the structural and generated quality models, matches source paths and symbols,
and preserves unavailable metrics such as coverage as `unknown`. It does not
run tests, coverage, linters, compilers, or external analyzers; import those
named, revision-matched results separately when available.

## 6. Generate and open the map

The final command is:

```bash
./bin/vibecodemap show \
  /absolute/path/to/my-app/.vibecodemap/project.vcm.yaml
```

`show` automatically:

1. validates YAML syntax, schemas, graph/flow/quality invariants, district
   codes, profile bands, source-reference IDs, and available source ranges;
2. composes the structural/project/quality evidence;
3. writes `.vibecodemap/out/<project-id>.view.json`;
4. embeds that model in `.vibecodemap/out/<project-id>.html`;
5. opens the HTML in the default browser.

There is no manual JSON-generation step. Validation is part of rendering. If a
draft is invalid, the agent should fix the reported field or reference and run
`show` again.

For CI or headless use:

```bash
./bin/vibecodemap render \
  /absolute/path/to/my-app/.vibecodemap/project.vcm.yaml
```

`render` generates the same files without opening a browser. Optional flags
must precede the project path, for example:

```bash
./bin/vibecodemap render \
  -output /tmp/my-map.html \
  -json-output /tmp/my-map.view.json \
  /absolute/path/to/project.vcm.yaml
```

The HTML currently loads Three.js from `esm.sh`; network access is needed for
the 3D runtime. If it cannot load, the document displays an embedded data-card
fallback instead of losing the generated map.

## Mixed-stack and monorepo rule

Run `inspect` once at the highest repository root. Do not run it once for Go,
again for Python, and again for a mobile directory. All detectors use the same
scope and may return several stack candidates.

Create one unified model by default:

- each product or independently understood operational system is a `system`
  and renders as a city;
- each independently built/deployed web app, mobile app, server, worker, CLI,
  library release, and each meaningful `cmd/*` binary is a `deployable` area;
- components and relationships can cross languages and deployables;
- shared libraries become utility areas, with static/build links distinct from
runtime roads.

Run `analyze` once at that same root as well. It dispatches implemented
analyzers automatically; there is no per-language install or invocation flow.
The Python analyzer needs Python 3.10 or newer on `PATH` or selected through
`VIBECODEMAP_PYTHON`. Detection-only stacks need no extra runtime because no
analyzer is run for them.

The usual filenames remain `model.vcm.yaml`, `quality.vcm.yaml`, and
`project.vcm.yaml` even for mixed stacks—the model is unified, not one file per
language. For a very large monorepo, keep the unified workspace map and add
optional focused manifests under:

```text
.vibecodemap/maps/<system-or-workload>/project.vcm.yaml
```

Focused maps are performance/navigation views, not conflicting architecture
sources. See [Multi-stack and monorepo modeling](MULTI-STACK.md).

## Honest interpretation

- A detected stack means the repository shape was recognized, not that deep
  semantic analysis exists for it.
- Static imports describe topology; they are not observed communication.
- A red security marker is a review lead with status/severity/confidence, not
  proof of a vulnerability.
- Quality bands show named evidence and unknown state, not a universal score.
- Rendering success proves the model is internally consistent, not that every
  AI inference is true.
