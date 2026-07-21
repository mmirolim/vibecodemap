# VibeCodeMap agent instructions

VibeCodeMap is an experimental evidence model and generic browser-map
prototype. Do not describe it as a dependable code-audit product.

When asked to inspect or map a repository, use the checked-in
`$analyze-with-vibecodemap` skill. Preserve these truth boundaries:

- `vibecodemap inspect` inventories files and detects stack candidates only;
- `detection_only` is not semantic analysis;
- AI architectural claims use `ai_inferred` provenance and source evidence;
- unknown quality, runtime, coverage, and security state remains unknown;
- static imports are topology, not observed runtime communication;
- native semantic adapters are not yet orchestrated for every detected stack;
- the generic composer renders valid VCM DSL, but it does not make an
  AI-authored semantic model automatically true.

For an end-to-end mapping request, completion means that the skill has built
the CLI if needed, inspected the target once at its repository root, explored
the approved source, authored or updated source-linked DSL, run `vibecodemap
show`, fixed validation errors, and opened the generated HTML map. Do not stop
at `inspect` or a standalone validation report unless the user asked for that.

Use one unified project manifest for a mixed-stack repository by default.
Products/systems become cities; independently built/deployed apps, workers,
services, CLIs (including meaningful `cmd/*` binaries), and release units
become deployables. Languages select adapters, not cities.

Before changing contracts, read the relevant file under `dsl/` and keep the
JSON Schema, examples, validators, and documentation aligned.

Run these checks after applicable changes:

```bash
go test ./...
go vet ./...
python3 tools/validate_vcm.py examples/uzumtools/uzumtools.vcm.yaml
python3 tools/validate_quality_vcm.py \
  examples/uzumtools/uzumtools.quality.vcm.yaml \
  --core examples/uzumtools/uzumtools.vcm.yaml
go run ./cmd/vibecodemap validate \
  examples/uzumtools/uzumtools.project.vcm.yaml
go run ./cmd/vibecodemap render \
  -output /tmp/vibecodemap-smoke.html \
  examples/uzumtools/uzumtools.project.vcm.yaml
```
