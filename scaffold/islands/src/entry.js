import "./style.css"; // bundled into /assets/style.css; in dev Vite injects it (HMR)
import { render, h } from "preact";
import Counter from "./counter.jsx";

// Registry of available islands. Add components here as you create them.
// `monobin check` flags any island referenced in a template but missing here.
const islands = {
  Counter,
};

// Mount every server-rendered island placeholder. The placeholder is empty, so
// render() is the correct API. Each mount is isolated so one failure can't take
// down the rest of the page.
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
  try {
    render(h(Component, props), el);
  } catch (e) {
    console.error(`monobin: island "${name}" failed to mount`, e);
  }
}
