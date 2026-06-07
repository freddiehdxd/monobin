import "./style.css"; // bundled into /assets/style.css; in dev Vite injects it (HMR)
import { hydrate, h } from "preact";
import Counter from "./counter.jsx";

// Registry of available islands. Add components here as you create them.
const islands = {
  Counter,
};

// Find every server-rendered island placeholder and hydrate it in place.
// Only these mounts get JavaScript — the rest of the page stays static HTML.
for (const el of document.querySelectorAll("[data-island]")) {
  const name = el.dataset.island;
  const Component = islands[name];
  if (!Component) {
    console.warn(`monobin: no island registered for "${name}"`);
    continue;
  }
  let props = {};
  try {
    props = JSON.parse(el.dataset.props || "{}");
  } catch (e) {
    console.warn(`monobin: bad props for "${name}"`, e);
  }
  hydrate(h(Component, props), el);
}
