const githubUrl = "https://github.com/mmirolim/vibecodemap";

const stages = [
  ["inspect", "Bound the repository", "Inventory owned source, detect stacks, and keep generated or installed code out of the investigation."],
  ["analyze", "Collect evidence", "Run implemented Go, Python, and JS/TS analyzers over one reviewed scope."],
  ["model", "Describe the system", "An AI agent investigates the source and authors editable, source-linked architectural DSL."],
  ["show", "Review the condition map", "Validate, compose, and open a navigable Three.js map with explicit evidence and unknowns."],
];

const grammar = [
  ["Buildings", "Components, interfaces, resources, actors, and expected-but-missing parts."],
  ["Condition bands", "Complexity, coupling, mutating effects, coverage, and other named evidence. Unknown stays unknown."],
  ["Typed roads", "Calls, events, state access, and static topology remain visually distinct instead of becoming one misleading graph."],
];

export default function Home() {
  return (
    <main>
      <header className="masthead">
        <a className="brand" href="#top" aria-label="VibeCodeMap home">
          <span className="brand-mark" aria-hidden="true"><i /><i /><i /></span>
          <span>VibeCodeMap</span>
        </a>
        <nav aria-label="Primary navigation">
          <a href="#demo">Live map</a>
          <a href="#method">Method</a>
          <a href="#build-story">Build story</a>
          <a className="nav-source" href={githubUrl}>Source ↗</a>
        </nav>
      </header>

      <section className="hero" id="top">
        <div className="hero-copy">
          <p className="eyebrow">OpenAI Build Week · Developer tools</p>
          <h1>See the codebase<br />before it disappears<br />into <em>code.</em></h1>
          <p className="lede">
            VibeCodeMap turns repository evidence and editable, source-linked
            models into a navigable 3D condition map—so humans can inspect the
            shape, interactions, side effects, and uncertainty of AI-built software.
          </p>
          <div className="hero-actions">
            <a className="button primary" href="#demo">Explore the live map</a>
            <a className="button secondary" href={githubUrl}>View the repository</a>
          </div>
          <p className="truth-boundary"><span /> Experimental investigation aid. Not proof that code is correct, secure, or complete.</p>
        </div>

        <div className="hero-instrument" aria-label="VibeCodeMap workflow">
          <div className="instrument-head"><span>Repository signal</span><span>VCM / 0.1</span></div>
          <div className="signal-grid">
            <span className="signal-building b1" /><span className="signal-building b2" />
            <span className="signal-building b3" /><span className="signal-building b4" />
            <span className="signal-road r1" /><span className="signal-road r2" />
            <span className="signal-road r3" />
            <span className="district-label d1">D1 · CLI</span>
            <span className="district-label d2">D3 · ADAPTERS</span>
            <span className="district-label d3">D4 · VIEWER</span>
          </div>
          <div className="instrument-foot">
            <span><b>42</b> buildings</span><span><b>7</b> roads</span><span><b>6</b> districts</span>
          </div>
        </div>
      </section>

      <section className="demo-section" id="demo">
        <div className="section-heading">
          <div>
            <p className="eyebrow">Live artifact · VibeCodeMap maps itself</p>
            <h2>Investigate the prototype from inside the prototype.</h2>
          </div>
          <a className="open-map" href="/demo.html">Open full-screen map ↗</a>
        </div>
        <div className="demo-frame">
          <iframe
            src="/demo.html"
            title="Interactive VibeCodeMap self-map"
            allowFullScreen
          />
          <div className="demo-note">
            <span>Drag to orbit · scroll to zoom · WASD to move · click a building</span>
            <span>Generated from the public VibeCodeMap repository</span>
          </div>
        </div>
      </section>

      <section className="grammar-section">
        <div className="section-kicker">Visual grammar</div>
        <div className="grammar-grid">
          {grammar.map(([name, detail], index) => (
            <article key={name}>
              <span className={`grammar-glyph glyph-${index + 1}`} aria-hidden="true" />
              <h3>{name}</h3>
              <p>{detail}</p>
            </article>
          ))}
        </div>
      </section>

      <section className="method-section" id="method">
        <div className="method-intro">
          <p className="eyebrow">From repository to reviewable form</p>
          <h2>Deterministic evidence where possible. Explicit inference where necessary.</h2>
          <p>
            The CLI does not pretend source parsing equals architecture. It gathers
            bounded evidence; the agent authors a reviewable model; the renderer
            preserves provenance, confidence, and missing information.
          </p>
        </div>
        <ol className="stage-list">
          {stages.map(([command, title, detail], index) => (
            <li key={command}>
              <span className="stage-number">0{index + 1}</span>
              <code>vibecodemap {command}</code>
              <h3>{title}</h3>
              <p>{detail}</p>
            </li>
          ))}
        </ol>
      </section>

      <section className="support-section">
        <div>
          <p className="eyebrow">Current analyzer coverage</p>
          <h2>Useful now, honest about depth.</h2>
        </div>
        <div className="support-table" role="table" aria-label="Supported analyzers">
          <div className="support-row header" role="row"><span>Stack</span><span>Current support</span><span>Method</span></div>
          <div className="support-row" role="row"><strong>Go</strong><span className="status implemented">Analyzed</span><span>Go parser / AST prototype</span></div>
          <div className="support-row" role="row"><strong>Python</strong><span className="status implemented">Analyzed</span><span>Python AST subprocess</span></div>
          <div className="support-row" role="row"><strong>JavaScript / TypeScript</strong><span className="status partial">Analyzed*</span><span>Conservative lexical prototype</span></div>
          <div className="support-row" role="row"><strong>Dart · Kotlin/Java · Swift/Obj-C</strong><span className="status detected">Detected</span><span>Agent investigation; no native analyzer yet</span></div>
        </div>
        <p className="support-note">* JS/TS evidence is not compiler- or type-aware. Hosted viewing requires only a modern browser. Local analysis requires Go 1.24+; Python 3.10+ is needed only when Python source is analyzed.</p>
      </section>

      <section className="build-section" id="build-story">
        <div className="build-claim">
          <p className="eyebrow">Built with Codex + GPT-5.6</p>
          <h2>One continuous collaboration, from raw idea to runnable prototype.</h2>
          <p>
            Codex with GPT-5.6 in Sol Max mode was the primary engineering
            environment throughout ideation, architecture, implementation,
            visual critique, code review, testing, documentation, and submission.
          </p>
        </div>
        <div className="build-facts">
          <article><strong>6</strong><span>core prototype commits before submission packaging</span></article>
          <article><strong>9.9k</strong><span>tracked lines of Go before this site</span></article>
          <article><strong>28.8k</strong><span>tracked lines across code, schemas, tests, examples, and docs before this site</span></article>
        </div>
        <div className="roles-grid">
          <article>
            <h3>Human direction</h3>
            <p>The core problem, visual-city metaphor, critique of misleading views, product boundaries, and every consequential direction change came from the builder.</p>
          </article>
          <article>
            <h3>Codex + GPT-5.6 execution</h3>
            <p>Translated the vision into Go contracts, DSL schemas, analyzers, tests, the Three.js renderer, agent guidance, and this public demo—then repeatedly audited its own work.</p>
          </article>
          <article>
            <h3>Key decisions</h3>
            <p>Unknown must stay unknown; static imports are topology, not runtime communication; AI architecture claims carry provenance; generated visual clutter must not masquerade as code complexity.</p>
          </article>
        </div>
      </section>

      <section className="install-section">
        <div>
          <p className="eyebrow">Map a repository</p>
          <h2>Clone, build, then ask the checked-in agent skill to investigate.</h2>
          <p>The live artifact above needs no installation. To analyze your own source, build the CLI and run the end-to-end skill from Codex.</p>
        </div>
        <pre aria-label="Installation commands"><code>{`git clone https://github.com/mmirolim/vibecodemap.git
cd vibecodemap
make build

./bin/vibecodemap show \\
  examples/uzumtools/uzumtools.project.vcm.yaml`}</code></pre>
      </section>

      <footer>
        <div><span className="footer-mark">VCM</span><span>VibeCodeMap · experimental open-source prototype</span></div>
        <div><a href={githubUrl}>GitHub</a><a href="/demo.html">Live map</a><a href={`${githubUrl}/blob/main/LICENSE`}>Apache-2.0</a></div>
      </footer>
    </main>
  );
}
