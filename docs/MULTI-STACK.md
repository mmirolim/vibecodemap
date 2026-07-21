# Multi-stack and monorepo modeling

## Decision

Programming language selects an analyzer; it does not define a visual city.
A React frontend, Python API, and Go worker that implement one product should
not be separated merely because their syntax differs.

Use this hierarchy:

| Level | Visual meaning | Software meaning |
|---|---|---|
| workspace | complete map | one repository or portfolio being reviewed |
| `system` | city | product, bounded system, or independently understood operational service |
| `deployable` | area/campus | web/mobile app, API, worker, CLI, or library release unit |
| district | neighborhood | subsystem, feature, layer, owner, or editable cluster |
| building | inspectable unit | component, interface, store, file, class, or function at the active zoom |

The hierarchy is semantic, not forced by directories. Use product purpose,
deployment, process/runtime isolation, ownership, data authority, and release
lifecycle as evidence. Language is secondary.

## One inspection, one graph

Run `vibecodemap inspect REPOSITORY_ROOT` once. Every registered detector sees
the same centrally scoped inventory, so one result may include Go, Python,
JS/TS, Flutter, Android, and Apple candidates without rescanning each subtree.

Detector scopes help an agent choose parsers and reading roots. They do not
prove that a directory is a product, service, deployable, or city. The agent
must inspect manifests, entrypoints, build targets, deployment definitions,
cross-stack calls, and ownership evidence before authoring boundaries.

Compose all important stacks into one structural graph so cross-language
calls, events, shared state, provider I/O, and static dependencies remain
visible. Do not create one disconnected DSL per language.

## Default file convention

Use one unified map under the target root:

```text
.vibecodemap/
  .gitignore
  model.vcm.yaml
  quality.vcm.yaml
  project.vcm.yaml
  out/<project-id>.view.json
  out/<project-id>.html
```

The `.gitignore` normally contains `out/` and `generated/`. In this standard
layout, repository roots in the project and structural models are `..`.

`project.vcm.yaml` points to the structural and optional quality models. One
structural model may contain several `system` elements; the Go composer renders
each active system as a separate `C1`, `C2`, … city in one workspace. District
codes become city-qualified (`C1.D1`, `C2.D1`) when needed. Cross-city
relations remain aggregate roads.

For a very large monorepo, retain that unified workspace map and optionally add
focused views:

```text
.vibecodemap/maps/<system-or-workload>/project.vcm.yaml
```

Names identify the intended focus, not a language. Focused manifests may
reference a focused structural projection or a shared model and a narrower
decomposition. They are navigation/performance views; avoid duplicating and
silently diverging architecture claims.

## Workload rules

- One product with a React client, Python API, and Go worker: one city, three
  deployable areas.
- iOS, Android, and Flutter clients plus a backend for one product: one city;
  each released client and backend is a deployable area.
- Independently owned products in one monorepo: one city per product/system.
- Every independently built executable under Go `cmd/*`: model as a separate
  deployable, unless several folders are merely aliases for the same binary.
- Shared SDKs/packages: shared utility area; imports/build dependencies are not
  runtime roads.
- Generators, migrations, CI, and developer tools: support area, not production
  runtime unless evidence says otherwise.
- A monolith with one process but MVC or layered structure: normally one
  deployable with districts for meaningful features/layers.

## What works now

- one root inspection can return multiple detector results;
- the structural DSL supports multiple systems, deployables, and stacks;
- the generic composer creates multiple cities and city-qualified districts;
- the browser can filter cities and preserves cross-city roads;
- the AI skill is instructed to author one unified model and identify each
  independently built/deployed workload.

Current limits:

- detections are candidate scopes, and some Python/JS detections are broad;
- native semantic adapters are not yet orchestrated for every language;
- workload discovery is primarily agent-authored, not a deterministic CLI
  output;
- large-workspace packing and cross-city routing are still prototype quality;
- focused-map projections are a convention, not yet a dedicated CLI command.

These limits do not require running `inspect` per directory. They require
better workload evidence and adapter implementations behind the same shared
inventory and model contracts.
