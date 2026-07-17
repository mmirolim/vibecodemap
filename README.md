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
- conservative Python structure and quality extractors;
- validators for the structural and quality models;
- a source-linked Uzumtools fixture;
- a [Three.js interaction experiment](prototype/README.md) covering package,
  file, and function detail.

The current prototype uses Python for the Python-specific evidence adapters and
plain JavaScript with Three.js for the visual experiment. That is experimental
code, not the intended production boundary.

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

## Validate the current models

```bash
python3 tools/validate_vcm.py examples/uzumtools/uzumtools.vcm.yaml

python3 tools/validate_quality_vcm.py \
  examples/uzumtools/uzumtools.quality.vcm.yaml \
  --core examples/uzumtools/uzumtools.vcm.yaml
```

The example-specific commands and limitations are documented in
[examples/uzumtools/README.md](examples/uzumtools/README.md).

## Near-term implementation

1. Define the versioned Go-to-adapter process contract.
2. Make the TypeScript renderer consume validated VCM JSON instead of embedded
   fixture data.
3. Add a Go CLI that runs adapters, validates evidence, computes aggregates,
   and serves the renderer locally.
4. Compare the 3D landscape with a 2D treemap/matrix on concrete review tasks.
5. Map a second repository without changing the DSL.
