# Evidence rules for VibeCodeMap mapping

Load this reference when drafting VCM models or making claims about
architecture, quality, behavior, clustering, or security.

## Authority and provenance

Keep four evidence layers distinguishable:

| Layer | Appropriate content | Required treatment |
|---|---|---|
| deterministic | syntax, manifests, source paths, imports, tool metrics, coverage | name the producer and exact source/revision |
| runtime observation | traces, measured traffic, timing, resource use | state observation window and freshness |
| AI inference | summaries, responsibilities, candidate boundaries/effects, review leads | `ai_inferred`, confidence, rationale, source evidence |
| human declaration | intended architecture, requirements, accepted corrections | `human_declared` or `human_confirmed`, rationale and evidence |

Later layers may correct a claim but must not erase the earlier evidence.
Unknown is a valid state and never means zero, absent, safe, or pure.

## Minimum evidence for important claims

For a source-backed claim, capture as much of this tuple as the model permits:

```text
repository revision + relative path + symbol + line/column + producer + method
```

Use exact evidence for entrypoints, network boundaries, persistence, direct
mutations, security leads, and required-but-missing expectations. A file summary
alone is not enough to prove a runtime relation.

## Behavior rules

- Static imports and type references are topology/coupling evidence.
- Calls are runtime candidates unless dispatch and execution are resolved.
- `async` syntax describes a callable or local scheduling style, not delivery,
  durability, concurrency, or a queue.
- Events, callbacks, RPC, HTTP, file I/O, database access, and external provider
  calls remain separately typed.
- A side-effect-free source site means no direct effect was found at that site;
  it does not prove transitive purity.
- Animated traffic is reserved for runtime observations, never static counts.

## Quality rules

- Use numeric values only from a named extractor, linter, compiler, coverage
  report, test report, or transparent calculation.
- Define direction and meaning for every metric. High lines or high coupling is
  not automatically bad.
- A combined score is a review-priority heuristic, not code quality, defect
  probability, or correctness.
- Keep freshness and revision association. Stale coverage stays visibly stale.
- Distinguish missing tests from unknown test discovery.
- Never translate missing analyzer support into a favorable value.

## Architecture and clustering rules

- Files and components are different concepts; many files may implement one
  component, and a large file may implement several responsibilities.
- Keep declared architecture, inferred static affinity, and observed runtime
  communities as separate decompositions.
- Shared utility hubs retain their real edges but should not pull every feature
  into one cluster merely because they are widely imported.
- A detector scope is a language-analysis candidate, not proof of product,
  ownership, deployment, or city boundaries.
- Cross-stack links must retain family and execution state so build/import
  dependencies do not appear as runtime roads.

## Multi-stack grouping questions

Ask, in order:

1. Is this independently deployed or released?
2. Does it have its own process/runtime and entrypoint?
3. Is it part of the same user-facing product or operational responsibility?
4. Does it own data or only consume another workload's interface?
5. Is it production behavior, a shared library, generated source, or developer
   tooling?
6. Who owns and changes it, and on what lifecycle?

Use the answers to propose systems and deployables. Language is secondary.

## Security rules

- Distinguish attack surface, vulnerability, control gap, configuration,
  sensitive-data flow, and supply-chain concern.
- Keep status, severity, and evidence confidence separate.
- Red/heat means “review here” for candidates, not “a vulnerability exists.”
- Do not include secrets, credentials, production payloads, or personal data in
  evidence summaries.

## Completion checklist

- Scope decisions were reviewed before source interpretation.
- A task plan and, for a large target, a coverage ledger account for admitted
  source by coherent subsystem; omissions are explicit.
- Every adapter support level was stated honestly.
- Important elements link to source or are explicitly external/expected.
- Inputs, outputs, state, and trust transitions are visible.
- Static and runtime relations are not conflated.
- Unsupported measurements remain unknown.
- Deterministic adapter measurements were converted to quality DSL and linked
  from the project, or their absence was stated explicitly.
- AI claims carry `ai_inferred` provenance and confidence.
- Mixed-stack boundaries use product/runtime/deployment evidence, not language.
- `vibecodemap show` validated all authored files and generated the HTML map.
- The map was visually sanity-checked for grouping, source targets, road types,
  label modes, input/output direction, unknown state, and generation warnings.
