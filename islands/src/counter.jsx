import { useState } from "preact/hooks";

// Note: styled with a neutral (slate) palette on purpose. The brand accent
// color is reserved for the Go templates so the Tailwind @source proof stays
// unambiguous — its rules in the built CSS can only come from scanning app/.
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
