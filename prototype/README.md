# Interaction prototypes

`uzumtools-evidence-landscape.fragment.html` is the current interaction and
visual-grammar experiment. It embeds a reduced Uzumtools scan so camera behavior,
semantic zoom, heat lenses, legends, side-effect contact, bounded dependency
views, and source navigation can be tested before a generic renderer exists.

`uzumtools-system-interactions.fragment.html` is the complementary topology
experiment. It answers a different question: how actors, browser code, HTTP
operations, application services, external providers, and state cooperate. Its
default view aggregates deployables; the detailed view exposes the mapped
components and relations; flow focus shows an ordered scenario; component focus
shows only that component's incoming and outgoing relations.

`software-clustering-grammar.fragment.html` compares declared ownership with an
illustrative inferred-affinity layout and isolates relation families. It tests
cluster enclosures, stable node forms, shared-utility placement, edge colors,
execution patterns, direction, strength, and selection detail. Its inferred
partition is a grammar demonstration, not a computed Uzumtools cluster result.

`uzumtools-pipe-city.fragment.html` combines those experiments as a Three.js
software city. District plates and aggregate towers preserve the architectural
overview; source-backed buildings expose code condition; typed physical pipes
show communication between districts or a selected building's bounded
neighborhood. Its feature-affinity layout remains an explicitly illustrative
sketch until a real cluster run is supplied by the Go core.

All four are intentionally HTML fragments for the Codex visualization host
rather than standalone applications. The landscape and pipe city use
version-pinned Three.js and OrbitControls; the topology and clustering grammar
use inline SVG. The next renderer iteration should replace embedded fixtures
with validated VCM JSON supplied by the Go core while preserving the tested
visual semantics.

Important interpretation rules:

- color belongs to the currently selected lens and always has a visible scale;
- grounded file plus roof sphere means a directly detected mutating effect;
- translucent floating file means no direct mutating site was detected, not
  that the file is pure through all dependencies;
- outgoing static dependencies terminate in spheres;
- incoming static dependencies terminate in diamonds;
- whole-map edges are never drawn at file level; detail uses a bounded ego view.

Interaction-topology rules:

- arrow direction is caller/event source to callee/receiver;
- solid lines are synchronous blocking calls, dashed lines are asynchronous
  client events, dotted lines are external callbacks, and heavier pink lines
  are state or storage access;
- external systems use hexagonal forms and state/resources use rounded forms;
- the macro view uses aggregated corridors instead of drawing every relation;
- selecting a flow hides unrelated edges and numbers the mapped steps;
- selecting a component hides unrelated edges and keeps exact source navigation;
- a record named `Job` does not imply a queue or worker; the current fixture
  explicitly shows inline persistence because that is what the evidence supports.

Clustering-grammar rules:

- proximity and cluster enclosures show the active decomposition;
- declared ownership and inferred affinity remain separate comparable layouts;
- shape identifies semantic kind, including interface, utility, state, and
  external system; class and function forms are reserved for closer LODs;
- edge color identifies relation family, dash pattern identifies execution,
  width identifies named strength, opacity identifies evidence state, and the
  arrowhead identifies direction;
- broad utilities remain in a shared belt and their clustering contribution is
  attenuated without deleting their real call/import relations;
- node heat color remains available for one explicit quality lens rather than
  being consumed by cluster identity.

Three-dimensional pipe-city rules:

- a district plate is a cluster or declared subsystem at macro LOD; a building
  is a source-backed component or file at closer LOD;
- building form identifies semantic role, footprint represents documented size,
  height represents documented complexity, and the selected heat lens represents
  one code-condition metric;
- directly detected mutating effects ground a building and add roof contacts;
  buildings without a directly detected mutating site float slightly, which does
  not claim transitive purity;
- a pipe sheath color identifies relation family, segmentation identifies
  asynchronous or callback execution, width identifies named aggregate strength,
  and collars plus arrowheads preserve endpoints and direction;
- macro LOD bundles child relations into a small number of district corridors;
  detail LOD exposes only the selected building's bounded ego pipes;
- feature-affinity and declared-boundary layouts are separate states, never a
  blended claim; shared utilities and external platforms retain distinct zones;
- animated particles are reserved for measured runtime traces and are absent
  from the current static-evidence prototype.
