# Three.js interaction prototype

`uzumtools-evidence-landscape.fragment.html` is the current interaction and
visual-grammar experiment. It embeds a reduced Uzumtools scan so camera behavior,
semantic zoom, heat lenses, legends, side-effect contact, bounded dependency
views, and source navigation can be tested before a generic renderer exists.

It is intentionally an HTML fragment for the Codex visualization host rather
than a standalone application. Three.js and OrbitControls are version-pinned.
The next renderer iteration should replace the embedded fixture with validated
VCM JSON supplied by the Go core while preserving the same visual semantics.

Important interpretation rules:

- color belongs to the currently selected lens and always has a visible scale;
- grounded file plus roof sphere means a directly detected mutating effect;
- translucent floating file means no direct mutating site was detected, not
  that the file is pure through all dependencies;
- outgoing static dependencies terminate in spheres;
- incoming static dependencies terminate in diamonds;
- whole-map edges are never drawn at file level; detail uses a bounded ego view.
