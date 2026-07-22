# VibeCodeMap agent instructions

VibeCodeMap is an experimental evidence model and generic browser-map
prototype. Do not describe it as a dependable code-audit product.

When asked to inspect or map a repository, use the checked-in
`$analyze-with-vibecodemap` skill. Preserve these truth boundaries:

- `vibecodemap inspect` inventories files and detects stack candidates only;
- `vibecodemap analyze` runs implemented analyzers over the same central scope
  and writes evidence, not architecture or DSL;
- `vibecodemap quality` maps supported deterministic evidence into quality DSL;
  it does not run external analyzers or turn missing evidence into zero;
- `detection_only` is not semantic analysis;
- AI architectural claims use `ai_inferred` provenance and source evidence;
- unknown quality, runtime, coverage, and security state remains unknown;
- static imports are topology, not observed runtime communication;
- native semantic adapters are not yet orchestrated for every detected stack;
- adapter runs marked `runtime_unavailable`, `failed`, or `timed_out` supplied
  no retained evidence and require explicit source investigation or rerun;
- the generic composer renders valid VCM DSL, but it does not make an
  AI-authored semantic model automatically true.

For an end-to-end mapping request, completion means that the skill has built
the CLI if needed, inspected the target at its repository root, run implemented
analyzers over the reviewed scope (corrected only when necessary), explored the
approved source through a tracked subsystem plan, authored or updated
source-linked DSL, generated and linked deterministic quality evidence when
available, run `vibecodemap show`, fixed validation errors, visually checked
the map, and opened the generated HTML map. Do not stop at
`inspect`, `analyze`, or a standalone validation report unless the user asked
for that.

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
go run ./cmd/vibecodemap validate examples/uzumtools/uzumtools.vcm.yaml
go run ./cmd/vibecodemap validate \
  -core examples/uzumtools/uzumtools.vcm.yaml \
  examples/uzumtools/uzumtools.quality.vcm.yaml
go run ./cmd/vibecodemap validate \
  examples/uzumtools/uzumtools.project.vcm.yaml
go run ./cmd/vibecodemap render \
  -output /tmp/vibecodemap-smoke.html \
  examples/uzumtools/uzumtools.project.vcm.yaml
```
