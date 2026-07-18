# VibeCodeMap Clustering and Visual Grammar Draft 0.3

## 1. Decision

Proximity must mean measured relationship, not directory adjacency and not an
AI-generated aesthetic layout. VibeCodeMap therefore needs a versioned affinity
model between the semantic graph and the renderer.

The recommended first implementation is:

1. construct a typed, multi-layer software graph;
2. attenuate dependencies to omnipresent utilities for clustering without
   deleting the underlying relations;
3. run SArF-style Dedication weighting plus Leiden/CPM for the default static
   affinity decomposition;
4. calculate node roles so shared utilities and connector hubs remain visible
   without being forced into a feature cluster;
5. preserve declared architecture, inferred affinity, and observed runtime flow
   as separate comparable decompositions;
6. lay out stable cluster regions first, then place members inside them.

The clustering result is evidence for review. It is not automatically the
correct architecture and must never rewrite declared ownership.

## 2. Four structures that must remain separate

| Structure | Source | Question answered |
|---|---|---|
| Physical containment | directories, files, symbols | Where is the code stored? |
| Declared architecture | PRD, ADR, manifest, human correction | Where is the code intended to belong? |
| Inferred affinity | calls, imports, data, flows, tests, change | Which elements behave as a cohesive group? |
| Observed runtime flow | traces, messages, call frequency, latency | Which elements actually cooperate in a scenario? |

Agreement increases confidence. Disagreement is often the useful result: an
implementation can be physically inside one package, declared in another
subsystem, statically coupled to a third, and observed on several runtime paths.

## 3. Typed multi-layer affinity graph

Clustering operates independently at a declared granularity: component, file,
class, or operation/function. It never mixes all levels into one flat graph.
Child-level evidence may be aggregated to parents with an explicit method.

| Signal layer | Direction | Initial clustering use | Important caveat |
|---|---|---|---|
| direct call/invocation | directed | strong | dynamic dispatch may be unresolved |
| imported symbol actually referenced | directed | medium | import alone does not prove runtime use |
| inheritance/interface implementation | directed | medium | framework contracts create broad coupling |
| read/write of shared data or resource | directed | medium | shared state can indicate a smell rather than cohesion |
| same declared flow/feature | undirected projection | strong when human-declared | flows can overlap |
| runtime transition or message | directed, weighted | strong in runtime profile | only observed scenarios are covered |
| tests covering the same subjects | bipartite projection | medium | broad integration tests connect everything |
| co-change | undirected, weighted | weak | active maintenance is not architectural cohesion |
| ownership/team overlap | undirected | weak/contextual | organization is not design |
| semantic similarity | undirected | weak AI evidence | names and comments can be stale or generic |

Each layer is normalized separately. A profile then combines only the layers it
declares. The renderer must expose the contributing layers for any selected
affinity; one unexplained similarity score is forbidden.

## 4. Utility attenuation without deleting relations

Shared libraries, logging, configuration, database wrappers, error helpers, and
framework primitives create many dependencies but usually should not pull all
callers into one cluster.

For a directed dependency from `A` to `B`, the initial static profile uses the
SArF Dedication score:

```text
dedication(A -> B) = 1 / fan_in(B)
```

A dependency on a target used by one module is highly dedicated; a dependency
on a target used by fifty modules contributes little to feature clustering.
Member-level evidence should later use the SArF multi-level variant before
lifting results to classes or files.

This attenuation applies only to **affinity calculation**. The original import,
call, read, or write relation remains visible at full semantic fidelity in
topology and flow views.

Common-neighbor and co-use projections must use an equivalent rarity weighting
so ubiquitous dependencies do not create false similarity. Dense framework and
standard-library dependencies may be aggregated into dependency families, but
never silently discarded.

## 5. Recommended algorithm portfolio

No one partition answers every software question.

| Rank | Algorithm/profile | Best use | Advantages | Risks and limits |
|---|---|---|---|---|
| 1 | **SArF Dedication + Leiden with CPM** | default static component/file/class affinity | handles weighted graph communities; downweights omnipresent utilities; Leiden avoids disconnected communities; resolution is controllable | hard partition; weights and resolution require calibration; direction may need a directed or symmetrized projection |
| 2 | **Infomap** | runtime and directed call/message-flow lens | preserves direction and frequency; finds modules that retain information flow | trace coverage biases the result; low-frequency but critical paths can disappear |
| 3 | **constraint-aware hierarchical agglomeration** | semantic zoom and comparison with declared architecture | creates an inspectable hierarchy; can respect must-link/cannot-link and ownership constraints; deterministic with fixed tie rules | linkage choice changes results; can force weak elements into a hierarchy; not suitable as the sole quality objective |

Use Bunch/Modularization Quality as a software-specific benchmark, not the
default engine. It is valuable for comparison with established high-cohesion,
low-coupling recovery, but search cost and objective sensitivity make it a poor
single production truth.

The first experiment should run several Leiden resolutions and retain the
partition only when its connectedness, cluster-size distribution, and revision
stability are acceptable. An unstable element is marked uncertain rather than
placed confidently.

## 6. Hub and utility roles

After clustering, calculate at least:

- fan-in and fan-out by relation family;
- weighted degree;
- within-cluster degree z-score;
- participation coefficient across clusters;
- betweenness or an approximation for larger graphs;
- number of distinct features, flows, and runtime boundaries reached.

Classify roles separately from cluster membership:

| Role | Structural signature | Visual treatment |
|---|---|---|
| private helper | low degree, nearly all links inside one cluster | small member inside cluster |
| cluster hub | high within-cluster degree, low external participation | larger member near cluster center |
| connector | links several clusters without extreme global fan-in | bridge form on a cluster boundary |
| shared utility | broad fan-in and high participation | neutral utility form in a shared belt, not used as a cluster centroid |
| external platform | third-party or platform dependency used broadly | aggregated outside the ownership boundary |
| orphan | weak or no measured affinity | unassigned region with explicit unknown status |

A shared utility can still have a declared owner. Role and ownership are
different dimensions.

## 7. Stability across revisions

Spatial memory is part of correctness. Every cluster run records algorithm and
library versions, input revision, normalized layer weights, resolution, random
seed, constraints, and previous-run reference.

The pipeline should:

1. reuse stable subject IDs;
2. warm-start from the previous partition when the algorithm supports it;
3. align new cluster identities to old clusters by maximum membership overlap;
4. anchor surviving cluster centroids and penalize unnecessary movement;
5. measure partition stability across seeds, nearby resolutions, and adjacent
   revisions;
6. show changed membership as a diff rather than silently moving everything.

Layout coordinates are renderer output. Cluster membership and its evidence are
model records.

## 8. Draft model records

```yaml
affinity_models:
  - id: affinity.static-feature.v0
    subject_granularity: file
    meaning: Static feature-oriented affinity for architectural review.
    layers:
      - signal: direct_call
        normalization: within_layer_p95
        weight: 1.0
      - signal: referenced_import
        normalization: within_layer_p95
        weight: 0.6
      - signal: shared_flow
        normalization: binary
        weight: 0.9
      - signal: co_change
        normalization: within_window_jaccard
        weight: 0.2
    hub_attenuation:
      method: sarf_dedication
      exponent: 1.0
    constraints:
      declared_parent: soft
    validation_state: experimental

cluster_runs:
  - id: cluster-run.static.2026-07-18
    model: affinity.static-feature.v0
    algorithm:
      name: leiden
      objective: cpm
      implementation: pending
      version: pending
      resolution: 0.08
      seed: 17
    repository_revision: pending
    previous_run: null
    status: experimental

clusters:
  - id: inferred-cluster.analysis
    run: cluster-run.static.2026-07-18
    label:
      value: analysis
      method: ai_inferred
      confidence: medium
      evidence: []

cluster_memberships:
  - cluster: inferred-cluster.analysis
    subject: file.app.services.image-service
    strength: 0.82
    stability: 0.91
    role: cluster_hub
    factors: []
```

Weights above are experiment parameters, not validated universal defaults.
Membership `strength` is algorithm/profile-specific and must not be presented as
probability.

## 9. Stable visual channel contract

One channel answers one question. The initial grammar is:

| Visual channel | Meaning |
|---|---|
| enclosure and proximity | primary cluster at the current LOD |
| node shape | semantic kind: system, subsystem, component, interface, class, function, data store, external system, utility |
| node fill/heat | exactly one selected quality or evidence lens |
| footprint/volume | documented code or member size |
| height | one stable structural metric such as maximum operation complexity |
| surface pattern | unknown, stale, inferred, or contradictory evidence |
| edge color | relation family: call/control, import/type, data/state, event/message, external/provider |
| edge dash pattern | execution: synchronous, asynchronous, callback, scheduled/manual |
| edge width | normalized count, affinity, or runtime volume named by the active relation lens |
| edge opacity | evidence confidence/freshness, never importance |
| arrowhead | direction |
| moving particles | observed runtime flow only; no decorative animation |

Cluster identity should not consume node heat color because quality lenses need
that channel. Cluster hulls, labels, spacing, and stable position communicate
membership. Shared utilities use a distinct neutral form and placement, not a
special color alone.

At macro LOD, child edges are bundled into directed corridors labeled with
relation-family counts. At detail LOD, selecting one node shows a bounded ego
graph. The renderer never draws every file/function edge simultaneously.

### 9.1 Physical 3D city grammar

The building metaphor is a navigation and compression grammar, not a claim that
software is literally spatial. It is useful only when every physical property
has one documented meaning and every LOD preserves the same identity.

| 3D element | Meaning |
|---|---|
| district plate | one declared subsystem or one inferred cluster in the active decomposition |
| aggregate tower | a macro summary of the district, never an additional source entity |
| building | source-backed component or file; classes and functions appear only after drill-down |
| building form | semantic role such as interface, ordinary component, utility, state, external platform, or missing expected element |
| footprint | documented size measure for the active granularity |
| height | documented complexity measure, held stable while changing quality lenses |
| ground contact | directly detected mutating side effect in that subject |
| floating offset | no directly detected mutating site; not a proof of transitive purity |
| roof contact | a typed mutating effect such as state, filesystem, network, or process interaction |
| physical pipe | a typed relation or bundled relation corridor |
| pipe sheath color | relation family |
| pipe segmentation | asynchronous, callback, or otherwise non-continuous execution semantics |
| pipe width | named and normalized strength such as relation count or runtime volume |
| collars and arrowhead | endpoint legibility and direction |

The semantic-zoom contract is:

1. **L0 — city:** district plates, aggregate towers, shared-utility/external
   zones, and a small number of bundled cross-district corridors;
2. **L1 — district:** source-backed buildings plus macro corridors, with detail
   labels suppressed unless selected;
3. **L2 — neighborhood:** the selected building and a bounded, typed ego graph,
   with exact source and evidence available on demand;
4. **L3 — interior (future):** class and operation rooms, local control/data
   flow, findings, and evidence—never loaded into the city view at once.

Pipe geometry must not become decorative. Static relations may animate only
during a finite layout transition. Moving particles require observed traces;
their direction, rate, and speed must be derived from named runtime data. Until
then, synchronous pipes are continuous and asynchronous/callback pipes are
segmented but stationary.

An affinity layout may move buildings toward measured collaborators while
keeping shared utilities in a neutral belt and external platforms outside the
ownership boundary. A manually arranged affinity sketch must be labeled
illustrative and must not be presented as a computed cluster run.

## 10. Required interactions

Selecting a cluster or node must answer:

- Why is this element here?
- Which measured signals pulled it toward this cluster?
- What declared owner and physical location does it have?
- Is it a cluster member, connector, utility, external platform, or orphan?
- Which calls, imports, state effects, events, and runtime observations connect
  it to the selection?
- What exact source and analyzer evidence supports each relation?

The user can compare declared, inferred-static, and observed-runtime
decompositions, but the UI must not merge them into one ambiguous layout.

## 11. Evaluation gates

Before treating clustering as useful, evaluate it on at least two structurally
different repositories and several revisions.

Measure:

- connectedness of every cluster;
- internal versus cut edge weight by relation family;
- non-extreme cluster-size distribution;
- stability across seeds, nearby resolutions, and revisions;
- agreement and disagreement with a human decomposition using MoJoFM or an
  equivalent only when such a decomposition exists;
- percentage of elements classified as connectors, shared utilities, or
  unstable rather than forced into ordinary clusters;
- human task time and correctness for finding a feature, tracing an effect,
  identifying an out-of-place component, and explaining a shared utility.

Reject a profile that produces one giant utility-centered cluster, many
singletons without explanation, disconnected communities, or large layout
movement after a minor code change.

## 12. Implementation sequence

1. Extend the Python fact adapter to resolve internal imported symbols and
   candidate calls at file/class/function granularity.
2. Emit typed affinity-layer edges with source evidence; keep the raw topology
   unchanged.
3. Implement SArF Dedication and hub-role calculations in the Go core.
4. Add a replaceable cluster-engine interface and a Leiden/CPM adapter.
5. Run multi-resolution experiments on Uzumtools and compare with its curated
   VCM components and flows.
6. Add Infomap only after real runtime traces exist.
7. Add stable cluster hulls, shared-utility placement, relation-family colors,
   and bounded drill-down to the renderer.
8. Map a second repository and test revision stability before fixing default
   weights or thresholds.

Current implementation checkpoint: the Go core now contains typed affinity
layers, per-layer normalization, explainable SArF Dedication attenuation,
participation/within-cluster role metrics, and a replaceable cluster-engine
interface. Import/call resolution and a concrete Leiden engine remain pending.

## References

- [SArF: Feature-Gathering Dependency-Based Software Clustering Using Dedication and Modularity](https://arxiv.org/abs/1306.2096)
- [Leiden: From Louvain to Leiden—guaranteeing well-connected communities](https://www.nature.com/articles/s41598-019-41695-z)
- [Infomap: Maps of random walks on complex networks reveal community structure](https://doi.org/10.1073/pnas.0706851105)
- [Functional cartography and connector-hub roles](https://www.nature.com/articles/nature03288)
- [Bunch automatic software modularization](https://doi.org/10.1109/TSE.2006.31)
