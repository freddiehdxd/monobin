import { useState } from "preact/hooks";

export default function Counter({ start = 0, label = "count" }) {
  const [n, setN] = useState(start);
  return (
    <button
      onClick={() => setN(n + 1)}
      class="rounded-lg bg-slate-900 px-4 py-2 text-sm font-medium text-white shadow-sm transition hover:bg-slate-700 active:scale-95"
    >
      {n} {label}
    </button>
  );
}
