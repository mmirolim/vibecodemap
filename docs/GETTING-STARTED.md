# Get from clone to a browser map

VibeCodeMap's primary outcome is an interactive browser visualization. The
current prototype uses a Go CLI for deterministic inventory, validation,
composition, and rendering, plus an AI agent skill for the semantic work of
understanding a repository and authoring the editable DSL.

## 1. Clone and build VibeCodeMap

Go 1.24 or newer is required. Python is optional unless you run the prototype
Python extractors or standalone validators.

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
Build VibeCodeMap if needed, inspect the repository, write source-linked DSL
under .vibecodemap, and open the generated visualization in my browser.
```

The skill is intentionally responsible for the complete workflow. It should
not stop after inventory or DSL validation when a browser map was requested.

## 3. Understand the work performed

The agent first runs one repository-root inspection:

```bash
./bin/vibecodemap inspect /absolute/path/to/my-app
./bin/vibecodemap inspect -json /absolute/path/to/my-app
```

`inspect` is an analysis plan, not the semantic map. It:

- applies built-in dependency, generated-source, cache, and build exclusions;
- applies Git ignore rules and an optional root `.vcmignore`;
- reports `analyze`, `summarize`, `externalize`, and `ignore` totals;
- detects multiple stack candidates and candidate scopes;
- exposes approved files, evidence, confidence, and adapter support in JSON.

It does not resolve calls/imports, decide system boundaries, infer side
effects, compute quality, write YAML, or render HTML. The agent uses the
inventory to avoid irrelevant source, then examines manifests, entrypoints,
deployables, interfaces, state, effects, relations, tests, requirements, and
trust boundaries itself.

If the scope is wrong, the agent can propose or—when authorized—write a root
`.vcmignore`, rerun `inspect`, and compare the result:

```text
fixtures/**
summarize generated/**
externalize third_party/source/**
!generated/maintained.go
```

## 4. Review the durable model

For an ordinary repository the agent writes:

```text
my-app/.vibecodemap/
  .gitignore           # normally ignores out/ and generated/
  model.vcm.yaml       # elements, source artifacts, relations, flows, findings
  quality.vcm.yaml     # only tool-backed measurements; may be omitted initially
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

## 5. Generate and open the map

The final command is:

```bash
./bin/vibecodemap show \
  /absolute/path/to/my-app/.vibecodemap/project.vcm.yaml
```

`show` automatically:

1. validates YAML syntax, the configured schemas, model cross-references,
   district codes, profile bands, and source-reference IDs;
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
