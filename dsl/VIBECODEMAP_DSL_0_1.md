# VibeCodeMap DSL 0.1

Status: experimental working draft
Primary fixture: Uzum Photo Checker
Serialization: YAML (JSON is equivalent)

## 1. Decision

VibeCodeMap is a semantic software-model DSL, not a drawing DSL.

An analyzer describes software concepts, evidence, interactions, execution, and
uncertainty. A renderer decides coordinates, shapes, colors, animation, and
level-of-detail. This keeps the model stable when the visualization changes and
prevents an AI mapper from creating a persuasive but inconsistent picture.

The first version deliberately models a useful subset:

- physical source files and exact source ranges;
- logical containment from system to operation;
- deployables, components, data stores, external systems, interfaces, and key
  operations;
- synchronous, asynchronous, event-driven, scheduled, and manual execution;
- database, filesystem, network, browser-storage, process, session, billing,
  and telemetry effects;
- typed interactions and representative end-to-end flows;
- architectural styles, boundary rules, exceptions, and findings;
- implementation, intent, and runtime reality as separate states;
- evidence and provenance for every important claim;
- renderer views as projections over the semantic model.

It does not attempt to encode every statement, infer runtime performance from
source, or prove that an architecture is good.

## 2. Why the model is a graph, not a tree

Software has several overlapping structures:

1. Physical: repository, directory, file, symbol.
2. Logical: system, deployable, layer, component, operation.
3. Runtime: process, thread, request, task, browser event loop, data store.
4. Feature: authentication, analysis, enhancement, billing, localization.
5. Deployment: browser, web process, database, object storage, external API.

A single hierarchy cannot represent all five without lying. VCM therefore uses:

- `artifacts` for the physical file inventory;
- `elements.parent` for one primary logical containment hierarchy;
- `elements.facets` for overlapping classifications;
- `relations` for topology;
- `flows` for ordered behavior;
- `views` for selecting a projection.

## 3. Top-level document

```yaml
vcm: "0.1"
model: {}
scope: {}
artifacts: []
elements: []
relations: []
flows: []
architecture: {}
findings: []
views: []
```

Only `vcm`, `model`, `scope`, `artifacts`, and `elements` are mandatory. Empty
collections are valid while a model is being built.

## 4. Model identity and scope

```yaml
model:
  id: uzum-photo-checker
  name: Uzum Photo Checker
  repository:
    root: /absolute/repository/or/project/root
    revision: 4b65d40
    dirty: true
  generated:
    at: 2026-07-17T00:00:00Z
    by: codex

scope:
  include:
    - app/**/*.py
    - app/**/*.js
  exclude:
    - app/**/__pycache__/**
  notes: Runtime application sources only; tests and migrations are deferred.
```

Scope is part of the truth claim. A model that omits tests must not display
"all tests covered" or "complete repository". A validator should compare the
artifact inventory with the include/exclude globs.

## 5. Stable identifiers

Entity IDs are document-global and use lowercase dotted names:

```text
system.photochecker
component.image-analysis
operation.api.analyze-image
file.app.views.api
relation.api-to-image-analysis
flow.technical-analysis
```

IDs should describe conceptual identity, not a current filename or screen
coordinate. A component can move files without changing its ID. File artifact
IDs may change when a file is renamed because the file itself is the identity.
Flow-step IDs are the one exception: they are local to their containing flow so
short names such as `validate`, `reserve`, and `persist` remain readable.

## 6. Artifacts: physical source inventory

Every in-scope source file gets one artifact record and a non-empty summary.

```yaml
artifacts:
  - id: file.app.views.api
    path: app/views/api.py
    kind: source
    language: python
    summary: HTTP API handlers for analysis, enhancement, downloads, jobs, and billing.
    responsibilities:
      - Validate API requests and serialize responses.
      - Coordinate application services and persistence.
    metrics:
      lines: 3649
    generated:
      method: ai_inferred
      confidence: high
      rationale: Summary is supported by route and import inventory.
```

Allowed artifact kinds in 0.1:

- `source`
- `template`
- `style`
- `configuration`
- `schema`
- `migration`
- `test`
- `documentation`
- `asset`
- `generated`

An artifact summary explains responsibility, not quality. Concentration and
boundary concerns belong in `findings`.

## 7. Source references and navigation

Every internal visual element must resolve to one or more source references.

```yaml
source_refs:
  - artifact: file.app.views.api
    symbol: analyze_with_ai
    lines: [485, 872]
    role: definition
```

`artifact` is mandatory. `symbol`, `lines`, and `role` are optional, although a
key operation should include all three. Line numbers are one-based and
inclusive. Renderers construct an absolute path from `model.repository.root`
and `artifacts[].path`, then use an environment adapter to open it in Codex, an
IDE, or a browser-based source viewer.

Allowed roles:

- `definition`
- `implementation`
- `configuration`
- `contract`
- `evidence`
- `template`
- `test`
- `generated`
- `schema`

External systems and human actors may have no source reference. Expected but
missing elements instead cite the caller or requirement that implies them.

## 8. Elements: semantic architecture

```yaml
elements:
  - id: component.ai-analysis
    kind: component
    name: AI analysis
    parent: layer.application-services
    summary: Builds prompts, calls Gemini, parses responses, and returns usage metadata.
    responsibilities:
      - Select marketplace requirements.
      - Invoke the AI provider.
      - Parse and validate provider output.
    facets:
      runtime: server
      layer: application
      features: [analysis, ai]
      state: stateless
    reality:
      intent: unspecified
      implementation: present
      runtime: not_observed
    execution:
      trigger: request
      style: synchronous
      blocking: yes
      concurrency: request_thread
    source_refs: []
    generated: {}
```

Allowed element kinds in 0.1:

- `system`: the product or bounded system in scope;
- `deployable`: independently deployed executable unit;
- `process`: runtime process or worker;
- `layer`: logical architectural layer;
- `component`: cohesive responsibility-bearing unit;
- `interface`: HTTP API, CLI, event surface, library API, or UI surface;
- `operation`: route handler, command, use case, job, or important procedure;
- `data_store`: durable or shared state;
- `resource`: filesystem area, cache, model, queue, or other accessed resource;
- `external_system`: service outside the modeled ownership boundary;
- `actor`: user or operator;
- `policy`: security, billing, retention, or other cross-cutting rule;
- `expected_component`: concept implied by intent but not implemented.

`parent` defines only logical ownership/containment. It must not be used to copy
the directory tree.

### 8.1 Reality states

```yaml
reality:
  intent: required | optional | forbidden | unspecified
  implementation: present | partial | missing | unknown | not_applicable
  runtime: observed | contradicted | not_observed | unknown | not_applicable
```

These dimensions must remain separate. Source can prove implementation but not
that a path works in production. A PRD can prove intent but not implementation.

### 8.2 Facets

`facets` are extensible labels. Recommended 0.1 keys are:

```yaml
facets:
  runtime: browser | server | worker | database | external | operator
  layer: presentation | interface | application | domain | infrastructure | data
  features: [analysis, billing]
  state: stateless | process_local | shared | durable | unknown
  trust_zone: public | authenticated | admin | internal | third_party
```

Facets are filterable semantics. They are not renderer colors.

## 9. Execution is not one async boolean

"Async" can mean an async language function, non-blocking caller behavior, a
queued job, concurrent request handling, or eventual message delivery. VCM
keeps those claims separate:

```yaml
execution:
  trigger: request | event | schedule | startup | manual | import | callback
  style: synchronous | asynchronous | event_driven | batch | stream | mixed | unknown
  blocking: yes | no | mixed | unknown
  concurrency: inline | request_thread | browser_event_loop | worker | process | external | unknown
  delivery: request_response | fire_and_forget | at_most_once | at_least_once | exactly_once | unknown
  timeout_ms: 30000
  retries:
    mode: none | immediate | backoff | external | unknown
    maximum: 2
```

Examples:

- A browser `fetch` can be `style: asynchronous`, `blocking: no`,
  `concurrency: browser_event_loop`, and `delivery: request_response`.
- A normal Flask handler calling an SDK can be `style: synchronous`,
  `blocking: yes`, and `concurrency: request_thread`.
- A database record named `Job` is not evidence of a worker. A queued job needs
  a queue/worker relation or runtime evidence.

## 10. Relations: communication and effects

```yaml
relations:
  - id: relation.ai-analysis-to-gemini
    from: component.ai-analysis
    to: external.gemini
    kind: calls
    summary: Sends an image and prompt and waits for a generated response.
    protocol: https
    execution:
      trigger: request
      style: synchronous
      blocking: yes
      concurrency: request_thread
      delivery: request_response
    effect:
      domain: network
      operation: invoke
      mutates_state: unknown
      durability: external
    reality:
      intent: required
      implementation: present
      runtime: not_observed
    evidence: []
    generated: {}
```

Allowed relation kinds:

- `contains`
- `invokes`
- `calls`
- `imports`
- `http_request`
- `redirects_to`
- `callback_from`
- `reads`
- `writes`
- `deletes`
- `queries`
- `persists`
- `publishes`
- `subscribes`
- `renders`
- `loads`
- `authenticates_with`
- `authorizes`
- `rate_limits_with`
- `implements`
- `depends_on`

### 10.1 Side effects

Side effects are represented on relations to the affected resource, not as a
free-text badge on a component.

```yaml
effect:
  domain: database | filesystem | network | browser_storage | process | session | telemetry | billing | external_state
  operation: read | write | create | update | delete | invoke | redirect | log | reserve | charge | release
  mutates_state: yes | no | unknown
  durability: transient | process | request | durable | external | unknown
```

This lets a renderer aggregate effects upward. Selecting a component can show
all descendant operations that mutate a database, touch files, or call external
services while preserving exact source links.

## 11. Evidence and provenance

Every important element, relation, architecture claim, or finding carries a
generation record and evidence.

```yaml
generated:
  method: deterministic | ai_inferred | runtime_observed | human_declared | human_confirmed
  confidence: high | medium | low
  rationale: Why this interpretation follows from the evidence.

evidence:
  - artifact: file.app.services.ai-analysis.gemini-service
    symbol: GeminiService.analyze_image
    lines: [32, 119]
    role: implementation
    supports: A synchronous provider SDK call occurs in the request path.
```

Confidence is only a triage hint. It is not probability. `rationale` and exact
evidence are required for AI-inferred or low-confidence claims.

## 12. Flows: ordered behavior

The component graph answers "what can interact?" A flow answers "what happens
for this scenario?"

```yaml
flows:
  - id: flow.ai-analysis
    name: AI-assisted image analysis
    summary: Authenticated request reserves credits, invokes Gemini, records results, and settles billing.
    trigger: operation.api.analyze-with-ai
    execution:
      style: synchronous
      blocking: yes
      concurrency: request_thread
    steps:
      - id: receive
        relation: relation.browser-to-ai-endpoint
        next: [reserve]
      - id: reserve
        relation: relation.ai-endpoint-to-billing
        next: [provider]
      - id: provider
        relation: relation.ai-analysis-to-gemini
        next: [persist]
      - id: persist
        relation: relation.job-service-to-database
```

A step references a relation instead of restating topology. `next` supports
branches and parallel paths. A later version can add conditions, compensation,
latency, and trace samples without changing element identity.

## 13. Architecture assertions

Architecture styles are scoped claims, not global labels:

```yaml
architecture:
  styles:
    - id: style.modular-monolith
      scope: deployable.flask-web
      name: Modular monolith
      fit: primary
      summary: One deployable process with blueprint and service modules.
      exceptions:
        - API handlers retain substantial orchestration and billing logic.
      evidence: []
      generated: {}
  constraints:
    - id: constraint.routes-use-services
      scope: layer.server-interfaces
      statement: Route handlers should delegate business rules to services.
      severity: warning
      enforcement: review
      evidence: []
```

Allowed `fit` values are `primary`, `supporting`, `partial`, `aspirational`, and
`contradicted`.

## 14. Findings

Findings are reviewable claims derived from the model. They do not alter the
topology.

```yaml
findings:
  - id: finding.api-responsibility-concentration
    kind: responsibility_concentration
    severity: warning
    summary: One API source file owns unrelated analysis, download, background-removal, and payment routes.
    subjects: [component.api-blueprint, file.app.views.api]
    impact: Changes to unrelated features converge on one file and boundary.
    evidence: []
    generated: {}
```

Recommended 0.1 finding kinds include:

- `responsibility_concentration`
- `boundary_violation`
- `dangling_interaction`
- `missing_component`
- `hidden_synchronous_work`
- `shared_state_risk`
- `abstraction_bypass`
- `untested_surface`
- `unobserved_runtime_claim`
- `duplication`
- `cycle`

Severity is `info`, `warning`, `high`, or `critical`. Severity must describe
impact, not visual prominence.

## 15. Views are semantic queries

Views say what to project. They do not assign pixels or colors.

```yaml
views:
  - id: view.runtime-topology
    name: Runtime topology
    roots: [system.photochecker]
    include:
      element_kinds: [deployable, component, data_store, external_system]
      relation_kinds: [http_request, invokes, reads, writes, calls]
    group_by: facets.runtime
    depth: 3
    overlays: [execution, effects, findings]
```

Recommended initial views:

- `logical`: system, layers, components, and ownership;
- `runtime`: deployables, execution mode, stores, and external systems;
- `source`: files, symbols, responsibility density, and navigation;
- `flow`: one selected scenario and its effects;
- `conformance`: intent versus implementation and constraints;
- `change`: base/current model diff.

The renderer owns the visual grammar. For example, it may map runtime to lanes,
effects to small markers, missing implementation to a hollow form, and
uncertainty to texture. Those mappings are not stored in VCM.

## 16. Required validator invariants

A conforming 0.1 validator checks at least:

1. All IDs are valid and globally unique.
2. Every reference resolves.
3. Every artifact path is relative, exists under the repository root, and has a
   summary.
4. Every source range is inside the referenced file.
5. Every internal element has a source reference; exceptions are external
   systems, actors, data stores/resources represented by configuration, and
   missing expected elements with caller/requirement evidence.
6. Every AI-inferred or low-confidence claim has rationale and evidence.
7. Every relation has both endpoints and a summary.
8. Side-effect relations target a data store, resource, external system, policy,
   or operation whose state can actually be affected.
9. Every flow step references a relation and every `next` step exists.
10. Artifact inventory matches declared scope, or omissions are reported.
11. A source-only model never claims runtime observation.

## 17. Agent mapping protocol

An AI mapper should produce the model in four passes:

### Pass 1: deterministic inventory

- enumerate scoped files and line counts;
- parse symbols, routes, imports, configuration, and tests where supported;
- collect candidate database, filesystem, network, and process effects;
- record facts without assigning architecture.

### Pass 2: semantic interpretation

- identify deployables, components, responsibilities, resources, and external
  systems;
- map each claim to source evidence;
- distinguish names from behavior (for example, `JobService` does not imply a
  queue);
- mark ambiguity rather than inventing a component.

### Pass 3: scenario tracing

- select a few high-value user or operator flows;
- follow calls and state changes from entry point to terminal effect;
- record sync/async semantics from the caller's perspective and implementation
  mechanism;
- identify dangling calls, hidden blocking, and abstraction bypasses.

### Pass 4: validation and correction

- validate IDs, references, paths, and ranges;
- compare artifact inventory with scope;
- ask a human only about material ambiguities;
- persist human corrections so later runs do not overwrite them.

The agent should not infer quality from file shape alone. A large file is
evidence of concentration, not proof of a bug. A missing direct call may be
framework wiring. Runtime performance requires traces or measurements.

## 18. Deliberately deferred

The following are valuable but should not block the first useful tool:

- full language-independent call graphs;
- generic type and data-flow analysis;
- 3D geometry or audio mappings;
- automatic architecture-quality scores;
- learned embeddings as the canonical representation;
- performance, load, or reliability claims without runtime evidence;
- formal rule expressions for every architectural constraint;
- every function and line as a visual node.

VCM 0.1 should first prove that a repository-aware agent can create a
source-linked map that helps a human answer: what exists, where it lives, what
it calls, what state it changes, whether work is inline or asynchronous, and
where the implementation does not match its apparent structure.
