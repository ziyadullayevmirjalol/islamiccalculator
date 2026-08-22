// Small DOM + component helpers shared by every page.

export function el(tag, attrs = {}, ...children) {
  const node = document.createElement(tag);
  for (const [k, v] of Object.entries(attrs)) {
    if (v === undefined || v === null) continue;
    if (k === 'class') node.className = v;
    else if (k.startsWith('on') && typeof v === 'function') node.addEventListener(k.slice(2), v);
    else if (k === 'checked' || k === 'disabled' || k === 'selected') { if (v) node[k] = true; }
    else if (k === 'value') node.value = v;
    else node.setAttribute(k, v);
  }
  for (const child of children.flat(Infinity)) {
    if (child === undefined || child === null || child === false) continue;
    node.append(child.nodeType ? child : document.createTextNode(child));
  }
  return node;
}

// Group thousands on a decimal STRING without ever parsing to float (C2).
export function fmtMoney(s) {
  if (typeof s !== 'string' || !/^-?\d+(\.\d+)?$/.test(s)) return s ?? '—';
  const neg = s.startsWith('-');
  const [int, frac] = (neg ? s.slice(1) : s).split('.');
  const grouped = int.replace(/\B(?=(\d{3})+(?!\d))/g, ' ');
  return (neg ? '−' : '') + grouped + (frac ? '.' + frac : '');
}

export function fmtPercent(s) {
  const n = Number(s);
  return Number.isFinite(n) ? (n * 100).toFixed(2).replace(/\.?0+$/, '') + '%' : s;
}

// --- shared class strings ----------------------------------------------------

export const CLS = {
  input: 'w-full rounded-lg border border-stone-300 bg-white px-3 py-2 text-sm ' +
         'focus:outline-none focus:ring-2 focus:ring-brand-600/40 focus:border-brand-600 ' +
         'placeholder:text-stone-400',
  label: 'block text-xs font-semibold uppercase tracking-wide text-stone-500 mb-1',
  btnPrimary: 'inline-flex items-center justify-center rounded-lg bg-brand-600 px-4 py-2.5 ' +
              'text-sm font-semibold text-white hover:bg-brand-700 focus:outline-none ' +
              'focus:ring-2 focus:ring-brand-600/50 disabled:opacity-50',
  btnGhost: 'inline-flex items-center justify-center rounded-lg border border-stone-300 px-3 py-1.5 ' +
            'text-sm font-medium text-stone-700 hover:bg-stone-100',
  card: 'bg-white rounded-xl border border-stone-200 shadow-sm',
  fieldError: 'mt-1 text-xs text-red-600',
};

// --- result building blocks --------------------------------------------------

export function kvRows(rows) {
  return el('dl', { class: 'divide-y divide-stone-100' },
    rows.filter(Boolean).map(([label, value, opts = {}]) =>
      el('div', { class: 'flex items-baseline justify-between gap-4 py-2' },
        el('dt', { class: 'text-sm text-stone-500' }, label),
        el('dd', {
          class: 'font-mono text-sm tabular-nums text-right ' +
                 (opts.emphasis ? 'text-lg font-semibold text-brand-700 ' : '') +
                 (opts.tone === 'neg' ? 'text-red-600 ' : '') +
                 (opts.tone === 'brass' ? 'text-brass-700 ' : ''),
        }, value),
      )));
}

export function dataTable(columns, rows) {
  return el('div', { class: 'overflow-x-auto rounded-lg border border-stone-200' },
    el('table', { class: 'w-full text-sm' },
      el('thead', {},
        el('tr', { class: 'bg-stone-50 text-left' },
          columns.map(c => el('th', {
            class: 'px-3 py-2 text-xs font-semibold uppercase tracking-wide text-stone-500 ' +
                   (c.align === 'right' ? 'text-right' : ''),
          }, c.label)))),
      el('tbody', { class: 'divide-y divide-stone-100' },
        rows.map(r => el('tr', { class: 'hover:bg-brand-50/40' },
          columns.map(c => el('td', {
            class: 'px-3 py-1.5 whitespace-nowrap ' +
                   (c.align === 'right' ? 'text-right font-mono tabular-nums' : ''),
          }, r[c.key] ?? '—')))))));
}

export function badge(text, tone = 'brand') {
  const tones = {
    brand: 'bg-brand-100 text-brand-800',
    brass: 'bg-brass-100 text-brass-700',
    red: 'bg-red-100 text-red-700',
    green: 'bg-emerald-100 text-emerald-800',
    stone: 'bg-stone-200 text-stone-700',
  };
  return el('span', {
    class: `inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-semibold ${tones[tone]}`,
  }, text);
}

export function notice(text, tone = 'warn') {
  const tones = {
    warn: 'border-brass-600/40 bg-brass-100/60 text-brass-700',
    info: 'border-brand-200 bg-brand-50 text-brand-800',
    error: 'border-red-300 bg-red-50 text-red-700',
  };
  return el('div', { class: `rounded-lg border px-3 py-2 text-sm ${tones[tone]}` }, text);
}

export function sectionTitle(text) {
  return el('h3', { class: 'text-sm font-semibold uppercase tracking-wide text-stone-500 mt-6 mb-2' }, text);
}
