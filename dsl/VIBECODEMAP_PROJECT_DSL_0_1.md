# VibeCodeMap Project Manifest 0.1

Status: experimental working draft

Serialization: YAML; JSON is equivalent
Schema: `vibecodemap-project-0.1.schema.json`

## 1. Decision

The repository owns a small committed `*.project.vcm.yaml` manifest. Scanner
outputs remain generated evidence. The committed manifest records intended
analysis policy and controls composition-facing declarations for:

- which source is analyzed, summarized, externalized, or ignored;
- enabled language adapters and their declared capabilities;
- generated-code markers and resource budgets;
- declared district membership and short navigation codes;
- requirements and expected-but-missing concepts;
- reviewable corrections to analyzer or AI claims;
- explicit input/output boundaries, transports, payload classes, and trust transitions;
- security review leads with separate severity, confidence, and confirmation state;
- semantic visual profiles such as multi-aspect building bands and road
  aggregation.

The scanner must never rewrite this file. A build composes generated facts with
the project manifest into an effective VCM model. This avoids the central failure
mode of editable generated output: the next scan silently erasing human
corrections.

The manifest is not a coordinate file. It names semantic units and mappings;
the renderer still owns exact positions, geometry dimensions, theme colors, and
camera behavior.

## 2. Composition and authority

The effective model is composed in this order:

1. deterministic parser, resolver, metric, coverage, and repository facts;
2. runtime observations, when present, as a separate evidence layer;
3. AI-inferred summaries, roles, candidate effects, and candidate architecture;
4. committed declarations, expectations, and corrections from this manifest.

Later layers do not erase evidence from earlier layers. A correction replaces a
claim value in the effective projection but retains the superseded claim and its
provenance for inspection. `human_confirmed` has more authority than
`ai_inferred`, but a source revision can make a human correction stale; freshness
must therefore still be checked.

## 3. Top-level document

```yaml
vcm_project: "0.1"
project: {}
analysis: {}
decompositions: []
expectations: []
corrections: []
boundaries: []
security_reviews: []
render_profiles: []
```

`project`, `analysis`, `decompositions`, and `render_profiles` are mandatory.
Expectations, corrections, boundaries, and security reviews may start empty.

## 4. Inputs remain separate

```yaml
project:
  id: uzumtools
  name: Uzumtools
  repository:
    root: .
    revision_policy: working_tree
  inputs:
    structural_model: .vcm/generated/structure.vcm.yaml
    quality_model: .vcm/generated/quality.vcm.yaml
    runtime_models: []
    requirements:
      - docs/PRD.md
```

The project file is an instruction and correction layer, not a copy of every
generated source artifact. Generated models may be committed for reproducible
review or ignored and regenerated locally; the project manifest works either
way.

## 5. Analysis scope is a four-state decision

Binary include/exclude is insufficient. Every discovered path resolves to one
of four actions:

| Action | Parse symbols | Keep detailed metrics | Visual treatment |
|---|---:|---:|---|
| `analyze` | yes | yes | normal source-backed hierarchy |
| `summarize` | no, or only public boundary | aggregate only | one collapsed generated/large-source unit |
| `externalize` | no | dependency metadata only | external package/platform unit |
| `ignore` | no | no | absent, but exclusion decision remains in scan report |

Installed dependency trees such as `node_modules`, Python virtual environments,
and Go `vendor` are ignored by default. Their packages are reconstructed from
manifests and lockfiles as external dependencies, so ignoring their installed
source does not mean pretending those dependencies do not exist.

Generated code is normally `summarize`, not `ignore`: its volume and external
coupling can matter, but thousands of generated symbols should not dominate the
city or consume AI review budget.

Rules use slash-separated globs with `**`. Higher priority wins; a later rule
wins a priority tie. Every decision records the matching rule and reason.

```yaml
analysis:
  scope:
    default_action: analyze
    read_gitignore: true
    read_gitattributes: false
    rules:
      - id: dependencies.node-modules
        patterns: ["**/node_modules/**"]
        action: ignore
        classification: installed_dependency
        priority: 50
        reason: Reconstruct package dependencies from manifests.

      - id: project.include-generated-client
        patterns: ["web/client.generated.ts"]
        action: analyze
        classification: source
        priority: 100
        reason: This checked-in file is manually maintained despite its name.
```

Current executable limitation: `inspect` and `analyze` do not yet consume a
project manifest. Their operational scope comes from built-in rules, Git ignore
rules, and the root `.vcmignore`. Mirror any required scope override in
`.vcmignore`; `analysis.scope` remains validated, durable intent for future
orchestration. `read_gitattributes` is reserved and currently has no scanner
effect.

The initial Go policy implementation also recognizes common Python, JS/TS, Go,
protobuf, Swift/Xcode, Android/Gradle, Dart/Flutter, build-output, cache, and
generated-header conventions.

For repositories that do not yet have a project manifest, the Go inventory also
reads an optional root `.vcmignore`. It uses the same slash-separated glob
grammar and records every decision in the inventory report:

```text
# A bare pattern means ignore.
fixtures/**
summarize snapshots/**
externalize third_party/source/**
!fixtures/owned_source.go
```

The explicit action form is preferred when retaining aggregate volume matters.
As with Git ignore files, opting a child back in cannot recover files beneath a
directory that has already been pruned; opt the parent into `summarize` or
`analyze` and narrow the child rules instead.

## 6. Generated-code detection

Path rules are not enough because checked-in generated files frequently have
ordinary names. The scanner reads only a bounded prefix and applies marker
regular expressions. Go's standard convention is a header matching
`// Code generated ... DO NOT EDIT.`. A future scanner may also consume
`.gitattributes` `linguist-generated` markings; the current scanner does not.

```yaml
analysis:
  generated_detection:
    default_action: summarize
    max_header_bytes: 8192
    markers:
      - id: generated.go-header
        regex: '(?m)^// Code generated .* DO NOT EDIT\.$'
        action: summarize
        classification: generated
        priority: 90
        reason: Standard Go generated-code header.
```

The scan report records summarized byte/file counts and the rule for every
pruned ignored/externalized directory. Descendant counts and bytes beneath a
pruned directory are intentionally unknown unless a separate aggregate-only
walk is requested; the report must say `pruned` rather than inventing zero
source. A large ignored tree therefore remains visible as an auditable boundary
without paying to enumerate or parse all of its contents.

## 7. Initial language portfolio

The shared model does not imply a universal shallow parser. Each language
adapter emits the same fact protocol but uses the strongest practical native
analysis:

| Language | First structural adapter | Quality sources | Initial limits |
|---|---|---|---|
| Python | `ast` plus import/effect resolver | Ruff/McCabe, coverage.py, tests | dynamic dispatch and monkey-patching remain uncertain |
| JavaScript | Tree-sitter for structure, resolver for modules | ESLint complexity, coverage reports | dynamic property access and bundler aliases need configuration |
| TypeScript | TypeScript compiler API or language service; Tree-sitter fallback | ESLint complexity, coverage reports | project references and path aliases must be loaded |
| Go | `go/packages`, `go/types`, AST and optional SSA | Go coverage, vet/static analyzers, gocyclo-compatible complexity | reflection and interface dispatch may remain candidate edges |
| Swift / Objective-C | SourceKit-LSP, SwiftSyntax/compiler indexes, Xcode project and plist metadata | SwiftLint-compatible metrics, XCTest coverage | macros, mixed-language bridging, and runtime dispatch need explicit evidence states |
| Kotlin / Java / Android | Kotlin/Java compiler models, Gradle metadata, Android manifests and lint outputs | Detekt/Android Lint, JaCoCo, test reports | variants, generated sources, lifecycle, permissions, and navigation are stack-specific |
| Dart / Flutter | Dart analyzer, package metadata, Flutter platform manifests | analyzer diagnostics, coverage and test reports | widget composition, navigation, plugins, and platform channels require Flutter semantics |

The current executable implements prototype source analyzers for Python, Go,
and JavaScript/TypeScript, plus conservative detection for the mobile rows.
These are not full compiler-grade analyzers. `detection_only`, `prototype`,
and `available` remain distinct support states visible to agents and humans.

An adapter declares capabilities rather than claiming universal completeness:

```yaml
languages:
  - id: go
    enabled: true
    adapter: go-ast-v0
    parser: go-parser-ast
    include: ["**/*.go"]
    capabilities: [artifacts, symbols, imports, calls, types, effects, complexity, tests, entrypoints]
    metric_sources: [go-ast-v0]
```

Unsupported capabilities remain `unknown`; they never become zero.

## 8. Districts and editable clustering

Each decomposition is independently named and sourced. Declared architecture,
inferred static affinity, and observed runtime communities must not be merged.

```yaml
decompositions:
  - id: declared.runtime-boundaries
    name: Declared runtime boundaries
    kind: declared
    districts:
      - id: district.interfaces
        code: D2
        name: HTTP and operator boundary
        role: owned
        members:
          element_ids: [interface.api, interface.operator-cli]
          path_globs: ["app/views/**", "app/cli/**"]
    provenance:
      method: human_declared
      confidence: high
      rationale: Repository-owned architectural review baseline.
```

Short codes are stable navigation identifiers for road endpoints. Long names
remain available on demand. Codes are unique only within one decomposition, so
the inferred layout may use a different code set without corrupting declared
identity.

An inferred cluster run can be exported into the manifest for editing, but it
must retain `ai_inferred` or deterministic algorithm provenance until a human
confirms it. The current Uzumtools affinity sketch is curated because import and
call resolution plus a concrete Leiden engine are not yet implemented; it is a
visual grammar fixture, not computed evidence.

## 9. Corrections survive regeneration

Corrections target a stable entity or selector and apply typed path operations:

```yaml
corrections:
  - id: correction.storage-direct-effects
    target:
      entity: component.storage-adapter
    operations:
      - action: set
        path: effects.filesystem.mutates_state
        value: true
    reason: The adapter writes and deletes local files; the first analyzer pass missed indirect calls.
    provenance:
      method: human_confirmed
      confidence: high
      rationale: Confirmed against the local storage implementation.
      evidence:
        - path: app/services/storage/local.py
          symbol: save
          supports: Direct filesystem mutation.
```

Each correction is an inspectable assertion, not arbitrary executable code. A
later agent can verify it, update its evidence, or remove it. When the target no
longer resolves, validation reports a stale correction instead of silently
dropping it.

## 10. Expectations define “missing”

Static analysis can find unusual structure; it cannot prove a requirement is
missing without an expected state. Expectations provide that state:

```yaml
expectations:
  - id: expectation.analytics-endpoint
    kind: interface
    name: Analytics endpoint
    importance: required
    summary: The landing client requires a server endpoint for analytics events.
    match:
      element_ids: [operation.api.analytics]
    provenance:
      method: human_confirmed
      confidence: high
      rationale: Client call exists but no matching scoped route was found.
      evidence:
        - path: app/static/landing.js
          symbol: trackAnalytics
```

The renderer may show an unmatched required expectation as a wireframe or
missing lot. It must not invent its location without a declared parent or
district selector.

## 11. Inputs, outputs, and persistence are first-class

A component name such as `API` or `CLI` does not explain what crosses its
boundary. Boundary declarations attach direction, transport, payload classes,
trust transition, authentication state, provenance, and an optional mapped
relation to a stable semantic subject:

```yaml
boundaries:
  - id: boundary.api-http-input
    subject: component.api-blueprint
    direction: input
    transport: HTTP
    payloads: [multipart image, JSON request, path and query parameters]
    trust_from: mixed external callers
    trust_to: Flask request boundary
    authentication: mixed
    summary: Upload and API payloads enter here.
    provenance:
      method: human_declared
      confidence: high
      rationale: Declared from mapped routes and client calls.
```

`input` and `output` are relative to the named subject. `bidirectional` is only
used when splitting the interface would lose meaning. Payloads are semantic
classes, never copied secrets or production data. Persistent stores remain
typed structural elements; the renderer may label them explicitly as database,
file/object storage, cache, or counters rather than relying on shape alone.

## 12. Security review is evidence, not red decoration

Security reviews deliberately separate four dimensions:

- category: attack surface, vulnerability, control gap, sensitive-data flow,
  supply chain, configuration, or other;
- status: review candidate, confirmed, accepted, false positive, or fixed;
- severity: potential impact if the claim is true;
- provenance confidence: how strongly the evidence supports the claim.

```yaml
security_reviews:
  - id: security.api-mixed-trust-surface
    subjects: [component.api-blueprint, file.app.views.api]
    category: attack_surface
    status: review_candidate
    severity: high
    summary: Several mixed-trust route families converge on one boundary.
    provenance:
      method: ai_inferred
      confidence: high
      rationale: Route families and trust zones are directly mapped; exploitability is not claimed.
```

The default marker grammar uses size and heat for severity, opacity or pattern
for confidence, and a visible candidate label/outline until a finding is
confirmed. A red candidate means “audit here,” not “a vulnerability exists.”
Security analyzers should emit source-linked findings in a generated model;
human declarations and corrections in this manifest can confirm, suppress, or
amend them without erasing the original evidence.

## 13. Multi-aspect building surfaces

The default road-city profile uses neutral body geometry for semantic kind and
ordered horizontal bands for simultaneous review aspects. The same sequential
low-to-high attention scale is reused for every band; vertical order identifies
the metric. This avoids requiring a unique color for every metric and makes a
uniformly hot building appear consistently hot from any horizontal view.

Recommended initial bands, bottom to top:

1. missing line coverage;
2. maximum operation complexity;
3. cross-boundary coupling/participation;
4. direct mutating-effect density.

Unknown is a pattern (`wireframe`, `hatch`, or an explicit gap), never the same
appearance as a low value. Bands are hints for investigation, not defect
verdicts. Selecting a building exposes the raw values, normalization, source,
and provenance.

```yaml
encodings:
  body: semantic_kind
  unknown_pattern: wireframe
  surface_evidence: texture
  bands:
    - id: band.coverage-gap
      label: Coverage gap
      metric: quality.coverage.line_gap
      order: 1
      direction: higher_attention
      normalization: ratio_0_1
      unknown: wireframe
```

## 14. Roads, lanes, ports, and feeders

Long pipes between arbitrary buildings do not scale. The road-city profile
aggregates all inter-district relations into one road per district pair and
routes it orthogonally around district footprints:

- road width: total named relation strength/count;
- lane identity: relation family;
- lane surface pattern: synchronous, asynchronous, callback, or other execution;
- endpoint sign: the complete district pair (`D2 ↔ D3`), with a local gate
  label such as `→D3`;
- local feeder: thin bounded line from a building to its district road port;
- selected detail: a maximum number of strongest routes plus an aggregated
  remainder count.

The road is a bundle, not a fake runtime bus. Its detail inspector lists the
underlying relations and evidence. Runtime animation is still forbidden until
traces provide direction and rate.

Intra-district relations stay hidden at city LOD and appear only for the
selected building or a bounded district view. This preserves the useful
perception of a crowded area without turning the complete graph into spaghetti.

## 15. Semantic zoom contract

| Level | Visible structure | Main question |
|---|---|---|
| L0 city | coded districts, road network, aggregate condition | Which areas are large, isolated, crowded, or connected? |
| L1 district | component buildings, broad bands, inputs/outputs, persistence, bounded feeders | Which components deserve investigation and where does information or state cross boundaries? |
| L2 neighborhood | selected building, strongest routes, payload detail, security evidence | What directly interacts with this component, by what semantics, and what should be audited? |
| L3 interior (future) | files, classes/functions, local effects and findings | Which source locations explain the observed shape? |

Building identity and district code remain stable across levels. L2 may use an
exploded local layout for legibility, but it must show the original district and
road ports so the user does not lose spatial context.

## 16. Research basis and limits

- CodeCity established the package-as-district/class-as-building metaphor and
  emphasized locality and orientation, but its original metric mapping was
  designed for object-oriented systems. VCM generalizes the hierarchy and keeps
  metric mappings explicit: https://www.inf.usi.ch/faculty/lanza/PUBS/P/Wett2008a.pdf
- DynaCity's controlled experiments support aggregation rather than raw edge
  volume: representative traces plus coordinated building/edge encodings helped
  participants on the tested comprehension tasks. This supports bounded roads
  and feeders, not a claim that 3D is universally superior:
  https://doi.org/10.1016/j.infsof.2022.106989
- Hierarchical edge bundling shows why relations should follow containment and
  aggregate at parent level rather than use independent long curves:
  https://doi.org/10.1109/TVCG.2006.147
- Tree-sitter provides robust multi-language concrete syntax parsing and
  official bindings, but parsing alone does not resolve imports, calls, types,
  or effects: https://tree-sitter.github.io/tree-sitter/
- Go's `go/packages` exposes package files, imports, syntax, and type information
  and is stronger than a generic parser for Go semantics:
  https://pkg.go.dev/golang.org/x/tools/go/packages
- ESLint and Ruff expose language-specific cyclomatic-complexity evidence rather
  than requiring one hand-written metric implementation:
  https://eslint.org/docs/latest/rules/complexity and
  https://docs.astral.sh/ruff/rules/complex-structure/
- `cloc` demonstrates practical exclusion-list support for large source trees:
  https://github.com/AlDanial/cloc
- GitHub's `linguist-generated` attribute and Go's generated-code header provide
  useful project and language conventions, but neither is complete enough to be
  the only detector:
  https://docs.github.com/en/repositories/working-with-files/managing-files/customizing-how-changed-files-appear-on-github
  and https://go.dev/blog/generate

These sources support the architecture, not the claim that a 3D road city is
automatically effective. The product still requires task-based user testing:
locating an architectural mismatch, identifying a high-investigation area,
tracing an effect, and explaining why a component was clustered.
