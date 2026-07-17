# VibeCodeMap Quality and Semantic-Zoom Draft 0.2

## 1. Product correction

VCM 0.1 answers **what exists, where it lives, and how it interacts**. That is
necessary but insufficient. The primary product question is:

> What condition is this software in, where is review attention justified, and
> what evidence supports that conclusion?

The architecture graph is therefore the coordinate system, not the result. VCM
0.2 adds an evidence-backed condition model and a level-of-detail contract for
2D or 3D renderers.

The model must not claim to measure universal “code quality.” It represents:

- raw measurements;
- analyzer findings;
- architectural and behavioral observations;
- missing or stale evidence;
- transparent review-priority models;
- renderer queries and aggregation rules.

## 2. Recommended implementation decision

Build a renderer-independent evidence DSL, a Go processing core, and a dedicated
TypeScript/Three.js renderer. Use existing analyzers as process adapters instead
of reimplementing every metric. Use an AI agent for semantic mapping,
requirement interpretation, and evidence-linked anomaly explanations—not as the
only measurement engine.

The Go core owns scan orchestration, normalization, validation, aggregation,
revision diffs, local serving, and an eventual MCP surface. The browser owns
rendering and interaction. Go/WASM rendering is deliberately deferred: it would
add a bridge around Three.js without solving the evidence-model problem.

| Direction | Advantages | Problems | Decision |
|---|---|---|---|
| Extend CodeCharta | Existing 3D city, parsers, metric import, proven hotspot metaphor | Primarily file/folder metrics; insufficient intent, flows, side-effect semantics, evidence status, and agent corrections | Use as a benchmark and possible importer, not the canonical model |
| Sonar/CodeScene-style backend plus thin visual | Broad analyzers and mature quality workflows | Product becomes tied to one vendor’s definitions and API; semantic architecture remains weak | Offer adapters later |
| VCM evidence graph plus Three.js | Full control over architecture, evidence, source links, semantic zoom, and AI workflow | More renderer and normalization work | **Recommended core** |

## 3. Non-negotiable separations

### 3.1 Measurements are not judgments

`complexity = 22`, `line_coverage = 0.16`, and `churn = 1044` are
measurements. “This orchestration component deserves review” is a judgment
derived from those measurements and context. They must be separate records.

### 3.2 Unknown is not zero

No coverage record does not mean 0% or 100% coverage. No runtime observation
does not mean a path never executes. Every measurement has a status:

- `observed`
- `unknown`
- `not_applicable`
- `stale`
- `invalid`

### 3.3 The AI cannot manufacture deterministic values

An AI may infer a responsibility, suspected abstraction leak, or unusual
construct. It may not invent a complexity value, coverage percentage, runtime
latency, or call count. AI findings require source evidence, rationale, and an
uncertainty class.

### 3.4 One unexplained health score is forbidden

The UI may provide a review-priority ranking, but it must expose every factor,
normalization method, weight, and missing input. It must be labeled as
prioritization—not correctness, maintainability, or quality.

## 4. Data model

VCM 0.2 is an extension of the VCM 0.1 graph. It references existing artifact,
element, relation, flow, and finding IDs.

```yaml
vcm_quality: "0.2-draft"

model:
  core_model: uzum-photo-checker
  revision: 4b65d40de6830e6cee70ea7930ae05b9a099df44
  generated_at: "2026-07-17T21:00:00+05:00"

metric_catalog: []
measurements: []
analyzer_findings: []
quality_findings: []
priority_models: []
priority_results: []
layout_profiles: []
lenses: []
```

### 4.1 Metric definitions

Metric meaning belongs in a catalog, not in renderer code.

```yaml
metric_catalog:
  - id: complexity.cyclomatic.max
    name: Maximum function cyclomatic complexity
    value_type: number
    unit: paths
    subject_kinds: [artifact, element]
    direction: contextual
    description: Maximum measured cyclomatic complexity among functions in the subject.
    aggregation:
      allowed: [max, p95, distribution]
      default: max
      forbidden: [sum_as_quality]
    caveats:
      - Language analyzers count some constructs differently.
      - High complexity is a review signal, not proof of a defect.
```

`direction` is one of `higher_is_risk`, `lower_is_risk`, `contextual`, or
`neutral`. The renderer must never infer direction from the metric name.

### 4.2 Measurements

```yaml
measurements:
  - id: measurement.api.max-cc
    subject: file.app.views.api
    metric: complexity.cyclomatic.max
    status: observed
    value: 96
    dimensions:
      language: python
      granularity: function
    measured_at: "2026-07-17T21:00:00+05:00"
    source_refs:
      - artifact: file.app.views.api
        symbol: enhance_image
        lines: [1607, 2220]
        role: evidence
    provenance:
      producer: vibecodemap-python-ast
      producer_version: 0.1
      method: deterministic
      configuration: python-cc-v0.1
      scope: app/**/*.py
      limitations:
        - Prototype implementation; compare against Radon before treating thresholds as portable.
```

Required measurement fields:

- stable ID;
- subject ID;
- metric definition ID;
- explicit status;
- value and unit when observed;
- collection time and repository revision;
- producer, producer version, configuration, and scope;
- source or report evidence where possible;
- limitations.

### 4.3 Analyzer findings

Lint, security, type, duplication, and policy findings are events rather than
scalar measurements. Import SARIF where the tool supports it.

```yaml
analyzer_findings:
  - id: analyzer-finding.bandit.b608.123
    subject: file.app.views.api
    rule: bandit.B608
    category: security
    severity: warning
    state: open
    message: Possible SQL expression constructed through string-based input.
    source_refs: []
    provenance:
      producer: bandit
      format: sarif-2.1.0
      run_id: scan.2026-07-17
```

The DSL preserves the originating rule and message. It does not translate all
tools into one pretend-universal severity scale.

### 4.4 AI and composite quality findings

```yaml
quality_findings:
  - id: quality-finding.api-convergent-hotspot
    kind: convergent_hotspot
    subjects: [file.app.views.api, component.api-blueprint]
    summary: A frequently changed API boundary combines unusually large functions, low test reach, and many outgoing dependencies.
    impact: Unrelated feature changes converge on a weakly verified boundary.
    factors:
      - measurement.api.lines
      - measurement.api.max-cc
      - measurement.api.line-coverage
      - measurement.api.churn
      - measurement.api.fan-out
    evidence: []
    generated:
      method: ai_inferred
      confidence: high
      rationale: Every stated factor is deterministic; the interpretation is explicitly AI-generated.
```

“Strange construct” detection uses two tracks:

1. deterministic patterns from linters, type checkers, security scanners,
   dependency rules, AST queries, and duplication detectors;
2. AI anomaly findings that compare code with project conventions and declared
   architecture, with exact source evidence and uncertainty.

### 4.5 Transparent review-priority models

```yaml
priority_models:
  - id: priority.static-review.v0
    label: Static review priority
    meaning: Relative order for human investigation; not a code-quality score.
    scope: artifact:source
    normalization:
      method: within_scan_percentile_midrank
      baseline: current repository scan
    factors:
      - metric: complexity.cyclomatic.max
        weight: 0.26
      - metric: test.uncovered_lines
        weight: 0.24
      - metric: change.lines_changed_18m
        weight: 0.22
      - metric: effects.mutating_sites
        weight: 0.16
      - metric: coupling.fan_in_plus_out
        weight: 0.12
    missing_data: expose_and_exclude
    validation_state: experimental
```

This provisional model exists to test interaction design. Its weights are not
scientifically validated and must not become a quality gate.

Each `priority_results` record references a priority model and subject, stores
the resulting value, and repeats the resolved factor values so the result is
auditable without rerunning the renderer.

## 5. Initial signal catalog

| Signal family | Useful raw signals | Preferred source | Important limit |
|---|---|---|---|
| Size | physical lines, logical lines, function/class/file size | language parser, Radon, cloc/tokei | Large is not automatically bad |
| Control flow | cyclomatic complexity, cognitive complexity, nesting, longest function | Radon/Ruff/Sonar/language analyzer | Definitions vary by language/tool |
| Coupling | fan-in, fan-out, cycles, dependency centrality, cross-boundary edges | language index, import graph, architecture rules | Dynamic dispatch is often unresolved |
| Cohesion | responsibility count, unrelated feature ownership, semantic dispersion | deterministic symbols plus AI interpretation | AI result must remain a finding, not a metric |
| Effects | database/file/network/process/external-state sites and transitive reach | AST/data-flow adapters plus runtime traces | Static detection is incomplete and may misclassify APIs |
| Verification | line/branch coverage, test mapping, mutation score, assertion evidence | coverage.py/LCOV/Istanbul, test runner, mutation tool | Coverage measures execution, not correctness |
| Static findings | lint, type, security, dead code, duplication, architecture rules | Ruff/Pylint/Mypy/Bandit/Semgrep/Sonar/SARIF | Rule severity is tool-specific |
| Evolution | commits, lines changed, authors, ownership concentration, age | Git | Churn can reflect healthy active development |
| Runtime | latency, errors, throughput, queue lag, allocations, resource use | OpenTelemetry, profiles, load tests | Only observed scenarios and retained samples are known |
| Intent | required/missing/forbidden components, flows, evidence, constraints | PRD/architecture DSL plus conformance checks | Declared intent can itself be incomplete |

## 6. Aggregation rules for zooming

Semantic zoom depends on explicit aggregation. A renderer must not simply hide
labels while retaining thousands of nodes.

| Metric type | Aggregate upward as | Never do |
|---|---|---|
| Lines, uncovered lines, churn, finding counts | sum plus distribution | treat the sum as quality |
| Coverage | covered opportunities / total opportunities | arithmetic mean of file percentages |
| Complexity/nesting/function length | max, p95, and distribution | sum complexities and label the result “complexity” |
| Latency | p50/p95/p99 with sample count and scenario | average across unrelated scenarios |
| Errors | rate plus count, exposure, and window | count without traffic denominator |
| Coupling | bundled edge counts/weights between visible parents | draw every hidden child edge |
| Unknown/stale evidence | count and affected size/exposure | coerce to zero or omit silently |

Each aggregate record states `method`, `members`, `known_members`,
`unknown_members`, `window`, and `computed_at`.

## 7. Level-of-detail contract

The hierarchy is data, while these levels are renderer queries. A project may
skip levels that do not exist.

| LOD | Typical subjects | Required visual question | Edge policy |
|---|---|---|---|
| L0 product | systems/deployables/features or top packages | Where are the broad condition concentrations? | only major aggregated corridors |
| L1 architecture | deployables/components/packages | Which boundaries and responsibilities are under pressure? | bundled inter-group edges on demand |
| L2 source | modules/files/classes | Which concrete artifacts deserve review? | selected ego graph; never all edges |
| L3 operation | functions/routes/jobs/handlers | What local complexity, effects, and evidence create the signal? | selected operation path and transitive effects |
| L4 evidence | source ranges/tests/findings/traces | What exact evidence supports or contradicts the claim? | direct source/test/trace links |

The current Three.js experiment compresses this to three camera levels:

- package aggregates;
- files;
- selected file, top functions, and bounded dependency ports.

## 8. Stable 3D grammar

### 8.1 Geometry that remains stable

- X/Z placement: hierarchy and declared architecture; unchanged subjects keep
  their position across lenses and revisions.
- Footprint: physical or logical size using a documented scale.
- Height: one stable structural measure, initially maximum function complexity.
- Selection: explicit outline and label.

Changing lenses must not rearrange the city or remap every dimension. Spatial
memory is more valuable than filling every geometric channel.

### 8.2 Lens-controlled overlays

- Heat color: exactly one selected metric or transparent priority model.
- Roof marker: a discrete property such as mutating effects or missing evidence.
- Surface pattern: unknown, stale, or low-confidence evidence when technically
  and accessibly feasible.
- Animation: observed runtime/change over time only; no decorative motion.
- Links: bundled at macro levels and bounded to the selected ego graph at close
  levels.

Every color encoding is paired with text, shape, or pattern. Every selected
object resolves to an artifact and source range.

### 8.3 Effect contact and visual legends

The ground plane is a useful metaphor only when its meaning is narrow and
testable:

- a file with directly detected mutating effect sites touches the ground and has
  a roof sphere;
- a file with no directly detected mutating sites floats above its package
  plate and is partially translucent;
- floating means **no direct site detected**, not pure code—transitive calls may
  still reach a database, filesystem, process, or remote service;
- transitive effect reach should later use a typed tether/path to the affected
  resource instead of falsely grounding the caller.

The renderer always shows two compact legend layers:

1. the active heat lens, with its metric name and concrete low/high meanings;
2. stable shapes: grounded/floating, direct-effect sphere, unknown-evidence cone,
   outgoing dependency sphere, and incoming dependency diamond.

Hover text names the subject, raw active-lens value, evidence status, and effect
state. Relationship hover names source, target, direction, and relation type.
Changing a lens updates heat and its scale but never changes the stable shape
grammar.

### 8.3 Why 3D is an experiment, not the truth model

3D provides rotation, additional spatial channels, and a strong whole-system
memory. It also causes occlusion, navigation cost, and perspective distortion.
The same DSL must support a 2D treemap, dependency matrix, flow view, and table.
We should compare task completion—not aesthetics—before making 3D the default.

## 9. Agent workflow

1. Read repository scope, PRD, architecture declarations, and previous human
   corrections.
2. Run deterministic language, coverage, Git, static-analysis, and runtime
   adapters that are available for the project.
3. Normalize outputs into metric definitions, measurements, analyzer findings,
   and evidence references.
4. Map measurements from symbols/files into components, features, flows, and
   requirements without losing the original subject.
5. Produce AI findings only for semantic questions that deterministic tools do
   not answer.
6. Compute explicit aggregates and experimental review priorities.
7. Validate references, status, units, provenance, freshness, and aggregation.
8. Render a useful default overview; drill down to exact source on selection.

## 10. Validator invariants

A VCM 0.2 validator must reject or flag:

1. observed measurements without numeric value, unit, producer, revision, or
   collection time;
2. unknown/stale measurements represented as zero;
3. metric IDs with no catalog definition;
4. aggregate values with no aggregation method or member counts;
5. AI-produced deterministic measurements;
6. AI findings without rationale and source/report evidence;
7. priority scores without factors, weights, normalization, and meaning;
8. source subjects that cannot resolve to a VCM 0.1 artifact;
9. file/function visual nodes with no source navigation target;
10. a lens that changes stable layout coordinates without declaring a different
    layout profile;
11. a whole-repository edge view with no bundling, filtering, or LOD policy;
12. stale coverage/runtime data displayed as current.

## 11. Tool-adapter policy

For the Python wedge:

- coverage.py XML/JSON for line and branch evidence;
- Radon or Ruff McCabe output for production complexity values;
- Ruff/Pylint for lint findings;
- Mypy for type findings;
- Bandit and Semgrep for security/policy findings, normalized through SARIF when
  available;
- Git for churn and ownership;
- the current AST extractor only as a transparent prototype/fallback;
- OpenTelemetry or named load-test reports only when runtime lenses begin.

For other languages, add adapters to the same metric catalog rather than asking
one LLM prompt to simulate every analyzer.

Useful current references:

- [CodeCharta 3D code map and extraction/renderer separation](https://codecharta.com/docs/overview/introduction/)
- [CodeCharta metric guidance](https://codecharta.com/docs/analysis/metrics/)
- [Ruff McCabe complexity rule](https://docs.astral.sh/ruff/rules/complex-structure/)
- [Radon metrics](https://radon.readthedocs.io/en/master/)
- [coverage.py branch coverage](https://coverage.readthedocs.io/en/6.5.0/branch.html)
- [SonarQube metric definitions](https://docs.sonarsource.com/sonarqube-server/user-guide/code-metrics/metrics-definition)
- [GitHub SARIF reference](https://docs.github.com/en/code-security/reference/code-scanning/sarif-files)

## 12. Immediate build sequence

1. Freeze the measurement/provenance/aggregation model and the versioned
   Go-to-adapter process contract.
2. Make a TypeScript/Three.js renderer consume validated VCM JSON instead of
   embedded fixture data.
3. Add a Go CLI that runs adapters, validates and aggregates evidence, stores a
   revision snapshot, and serves the compiled renderer.
4. Replace prototype cyclomatic values with a Radon adapter and compare both
   outputs on `uzumtools`.
5. Add coverage, Git, Ruff/Pylint, Mypy, and Bandit adapters with freshness and
   scope reporting.
6. Generate a complete quality extension file linked to existing VCM artifact
   IDs.
7. Drive both the Three.js landscape and a 2D comparison view from that same
   file.
8. Test five tasks: find a hotspot, explain why, find missing evidence, trace an
   effect, and open exact source. Compare time and correctness against a table
   and conventional dependency graph.
