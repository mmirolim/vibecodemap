# VibeCodeMap

VibeCodeMap is an experiment in reviewing AI-generated software as an
evidence-backed condition landscape instead of a collection of source lines.
Its structural model answers what exists and how it interacts; its quality
extension answers where review attention is justified and which evidence
supports that conclusion.

## Current checkpoint

- VCM 0.1 structural DSL and JSON Schema;
- VCM 0.2 draft for measurements, findings, provenance, freshness, review
  priority, semantic zoom, and visual lenses;
- VCM 0.3 draft for multi-signal affinity, software clustering, hub roles,
  revision stability, and relationship-aware visual grammar;
- a Go affinity foundation with per-layer normalization, explainable SArF
  Dedication attenuation, connector/hub role metrics, and a replaceable cluster
  engine boundary;
- conservative Python structure and quality extractors;
- validators for the structural and quality models;
- a source-linked Uzumtools fixture;
- [interaction experiments](prototype/README.md) covering a Three.js condition
  landscape, a clustered 3D pipe city, and directed system, component,
  external-provider, and state flows.

The current prototype uses Go for the first renderer-independent affinity core,
Python for Python-specific evidence adapters, and plain JavaScript with Three.js
or SVG for visual experiments. The concrete Leiden/Infomap engines and generic
TypeScript renderer do not exist yet.

## Implementation direction

The proposed production split is:

- **Go core:** CLI, scan orchestration, adapter process protocol, normalization,
  model validation, aggregation, revision diffing, local HTTP service, and MCP;
- **TypeScript + Three.js:** browser rendering, semantic zoom, picking, lenses,
  legends, source navigation, and accessible 2D fallbacks;
- **replaceable adapters:** language-native analyzers emitting VCM JSON patches,
  SARIF, coverage data, or a small versioned event protocol;
- **AI agent/skill:** semantic mapping, requirement interpretation, anomaly
  explanation, and correction workflow—never fabrication of deterministic
  metrics.

See [ADR-001](docs/ADR-001-IMPLEMENTATION-STACK.md) for the decision and tradeoffs.
The clustering and visual-channel design is specified in
[VCM 0.3](dsl/VIBECODEMAP_CLUSTERING_AND_VISUAL_GRAMMAR_0_3.md).

## Validate the current models

```bash
python3 tools/validate_vcm.py examples/uzumtools/uzumtools.vcm.yaml

python3 tools/validate_quality_vcm.py \
  examples/uzumtools/uzumtools.quality.vcm.yaml \
  --core examples/uzumtools/uzumtools.vcm.yaml

go test ./...
```

The example-specific commands and limitations are documented in
[examples/uzumtools/README.md](examples/uzumtools/README.md).

## Near-term implementation

1. Resolve Python imports and candidate calls into source-linked affinity-layer
   edges at file, class, and function granularity.
2. Add a Leiden/CPM engine adapter behind the Go cluster interface and emit
   reproducible cluster-run records.
3. Make the TypeScript renderer consume validated VCM JSON instead of embedded
   fixture data.
4. Add a Go CLI that runs adapters, validates evidence, computes aggregates,
   and serves the renderer locally.
5. Compare declared ownership with inferred clusters across Uzumtools revisions,
   then map a second structurally different repository.
