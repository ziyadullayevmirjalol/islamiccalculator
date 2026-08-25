// Page renderers: dashboard, generic calculator page (form engine),
// auth, history, rates, and the livestock reference table.
import { api, ApiError, login, register, session } from './api.js';
import { CALCS, GROUPS, calcById } from './calculators.js';
import { el, CLS, kvRows, dataTable, badge, notice, sectionTitle, fmtMoney } from './ui.js';
import { t } from './i18n.js';

// --- generic form engine -----------------------------------------------------

function fieldControl(f, values, rerender) {
  const set = v => { values[f.name] = v; };
  switch (f.type) {
    case 'select':
      return el('select', {
        class: CLS.input, value: values[f.name],
        onchange: e => { set(e.target.value); rerender(); },
      }, f.options.map(o => el('option', { value: o.value, selected: values[f.name] === o.value }, t(o.label))));
    case 'check':
      return el('label', { class: 'flex items-center gap-2 py-1 cursor-pointer text-sm' },
        el('input', {
          type: 'checkbox', class: 'h-4 w-4 rounded border-stone-300 text-brand-600 focus:ring-brand-600',
          checked: !!values[f.name], onchange: e => set(e.target.checked),
        }), t(f.label));
    case 'multicheck':
      values[f.name] ||= [];
      return el('div', { class: 'grid grid-cols-2 gap-1' },
        f.options.map(o => el('label', { class: 'flex items-center gap-2 text-sm py-0.5 cursor-pointer' },
          el('input', {
            type: 'checkbox', class: 'h-4 w-4 rounded border-stone-300 text-brand-600 focus:ring-brand-600',
            checked: values[f.name].includes(o.value),
            onchange: e => {
              const list = values[f.name];
              if (e.target.checked) list.push(o.value);
              else values[f.name] = list.filter(x => x !== o.value);
            },
          }), t(o.label))));
    case 'date':
      return el('input', { type: 'date', class: CLS.input, value: values[f.name] || '', onchange: e => set(e.target.value) });
    case 'percent':
      return el('div', { class: 'relative' },
        el('input', {
          type: 'text', class: CLS.input + ' font-mono pr-8', inputmode: 'decimal',
          value: values[f.name] || '', placeholder: f.placeholder || '', oninput: e => set(e.target.value),
        }),
        el('span', { class: 'absolute right-3 top-1/2 -translate-y-1/2 text-stone-400 text-sm pointer-events-none' }, '%'));
    case 'int':
      return el('input', {
        type: 'number', step: '1', min: '0', class: CLS.input, inputmode: 'numeric',
        value: values[f.name] || '', placeholder: f.placeholder || '', oninput: e => set(e.target.value),
      });
    default: // money / text — money stays a string end to end (contract C2)
      return el('input', {
        type: 'text', class: CLS.input + ' font-mono', inputmode: f.type === 'money' ? 'decimal' : 'text',
        value: values[f.name] || '', placeholder: f.placeholder || '', oninput: e => set(e.target.value),
      });
  }
}

function fieldWrapper(f, values, rerender) {
  if (f.type === 'check') return el('div', { 'data-field': f.name }, fieldControl(f, values, rerender));
  const hint = typeof f.hint === 'function' ? f.hint(values) : f.hint;
  return el('div', { 'data-field': f.name },
    el('label', { class: CLS.label }, t(f.label)),
    fieldControl(f, values, rerender),
    hint && el('p', { class: 'mt-1 text-xs text-stone-400' }, t(hint)),
    el('p', { class: CLS.fieldError + ' hidden', 'data-error-for': f.name }));
}

function listEditor(list, values, rerender) {
  values[list.name] ||= structuredClone(list.defaultRows || []);
  const rows = values[list.name];

  const cellControl = (col, row) => {
    if (col.type === 'select') {
      return el('select', {
        class: CLS.input, onchange: e => { row[col.name] = e.target.value; },
      }, col.options.map(o => el('option', { value: o.value, selected: (row[col.name] ?? col.options[0].value) === o.value }, o.label)));
    }
    if (col.type === 'percent') {
      return el('div', { class: 'relative' },
        el('input', {
          type: 'text', class: CLS.input + ' font-mono pr-6', inputmode: 'decimal',
          value: row[col.name] || '', placeholder: col.placeholder || '',
          oninput: e => { row[col.name] = e.target.value; },
        }),
        el('span', { class: 'absolute right-2 top-1/2 -translate-y-1/2 text-stone-400 text-xs pointer-events-none' }, '%'));
    }
    return el('input', {
      type: col.type === 'date' ? 'date' : 'text',
      class: CLS.input + (col.type === 'money' ? ' font-mono' : ''),
      inputmode: col.type === 'money' ? 'decimal' : undefined,
      value: row[col.name] || '', placeholder: col.placeholder || '',
      oninput: e => { row[col.name] = e.target.value; },
      onchange: e => { row[col.name] = e.target.value; },
    });
  };

  return el('div', { class: 'space-y-2' },
    el('label', { class: CLS.label }, t(list.label)),
    el('div', { class: 'space-y-2' },
      rows.map((row, i) => el('div', { class: 'flex items-end gap-2' },
        el('div', { class: 'grid gap-2 flex-1', style: `grid-template-columns: repeat(${list.columns.length}, minmax(0,1fr))` },
          list.columns.map(col => el('div', {},
            i === 0 && el('span', { class: 'block text-[10px] font-semibold uppercase tracking-wide text-stone-400 mb-1' }, t(col.label)),
            cellControl(col, row)))),
        el('button', {
          type: 'button', class: 'text-stone-400 hover:text-red-600 pb-2 text-lg leading-none', title: 'Remove',
          onclick: () => { rows.splice(i, 1); rerender(); },
        }, '×')))),
    el('button', {
      type: 'button', class: CLS.btnGhost, onclick: () => { rows.push({}); rerender(); },
    }, t('+ Add row')));
}

function showFieldErrors(form, fields) {
  form.querySelectorAll('[data-error-for]').forEach(p => { p.classList.add('hidden'); p.textContent = ''; });
  const unmatched = [];
  for (const [key, code] of Object.entries(fields || {})) {
    const tail = key.split('.').pop().replace(/\[\d+\]/g, '');
    const slot = form.querySelector(`[data-error-for="${key}"]`) || form.querySelector(`[data-error-for="${tail}"]`);
    const translated = t(code);
    const text = translated !== code ? translated : code.replaceAll('_', ' ');
    if (slot) { slot.textContent = text; slot.classList.remove('hidden'); }
    else unmatched.push(`${key}: ${text}`);
  }
  return unmatched;
}

// --- calculator page ---------------------------------------------------------

export function renderCalcPage(root, id, prefill) {
  const def = calcById(id);
  if (!def) { root.replaceChildren(notice(`Unknown calculator: ${id}`, 'error')); return; }

  const values = prefill ? { ...prefill } : {};
  for (const f of def.fields) if (f.default !== undefined && values[f.name] === undefined) values[f.name] = f.default;

  const resultPanel = el('div', { class: CLS.card + ' p-5 min-h-[8rem]' },
    el('p', { class: 'text-sm text-stone-400' }, t('Fill the form and calculate — results come from the backend, never computed here.')));
  const errorBanner = el('div', { class: 'hidden' });
  let form;

  const rebuildForm = () => {
    const visible = def.fields.filter(f => !f.showIf || f.showIf(values));
    const listVisible = def.list && (!def.list.showIf || def.list.showIf(values));
    const inner = el('div', { class: 'space-y-4' },
      visible.map(f => fieldWrapper(f, values, rebuildForm)),
      listVisible && listEditor(def.list, values, rebuildForm),
      errorBanner,
      el('button', { class: CLS.btnPrimary + ' w-full', type: 'submit' }, t('Calculate')));
    form.replaceChildren(inner);
  };

  form = el('form', {
    class: CLS.card + ' p-5',
    onsubmit: async e => {
      e.preventDefault();
      errorBanner.className = 'hidden';
      const button = form.querySelector('button[type=submit]');
      button.disabled = true; button.textContent = t('Calculating…');
      try {
        const data = await api(def.endpoint, { method: 'POST', body: def.payload(values) });
        showFieldErrors(form, {});
        resultPanel.replaceChildren(...[def.render(data)].flat(Infinity).filter(Boolean));
      } catch (err) {
        if (err instanceof ApiError) {
          const unmatched = showFieldErrors(form, err.fields);
          const lines = [err.message, ...unmatched];
          errorBanner.className = '';
          errorBanner.replaceChildren(notice(lines.join(' · '), 'error'));
        } else {
          errorBanner.className = '';
          errorBanner.replaceChildren(notice(`Backend unreachable — is it running? (${err.message})`, 'error'));
        }
      } finally {
        button.disabled = false; button.textContent = t('Calculate');
      }
    },
  });
  rebuildForm();

  root.replaceChildren(
    el('a', { href: '#/', class: 'text-sm text-brand-600 hover:underline' }, t('← All calculators')),
    el('div', { class: 'mt-2 mb-6' },
      el('h1', { class: 'text-2xl font-bold' }, t(def.title)),
      el('p', { class: 'text-stone-500' }, t(def.blurb))),
    el('div', { class: 'grid gap-6 lg:grid-cols-2 items-start' }, form, resultPanel),
  );
}

// --- dashboard ---------------------------------------------------------------

export function renderHome(root) {
  const ratesBox = el('div', { class: CLS.card + ' p-5' },
    el('p', { class: 'text-sm text-stone-400' }, 'Loading metal prices…'));

  api('/api/v1/rates/metals').then(d => {
    const metal = (label, p) => el('div', { class: 'flex items-center justify-between py-1.5 text-sm' },
      el('span', { class: 'text-stone-500' }, label),
      el('span', { class: 'flex items-center gap-2' },
        el('span', { class: 'font-mono tabular-nums font-semibold' }, `${fmtMoney(p.pricePerGram)} ${p.currency}/g`),
        p.stale && badge('STALE', 'red')));
    ratesBox.replaceChildren(...[
      el('h2', { class: 'font-semibold mb-2' }, t('Metal prices & nisab')),
      metal(t('Gold'), d.gold), metal(t('Silver'), d.silver),
      el('div', { class: 'mt-2 pt-2 border-t border-stone-100 text-sm flex justify-between' },
        el('span', { class: 'text-stone-500' }, `${t('Nisab')} (${t(d.nisab.basis === 'silver' ? 'Silver' : 'Gold')})`),
        el('span', { class: 'font-mono tabular-nums font-semibold text-brand-700' }, fmtMoney(d.nisab.applied))),
      (d.gold.stale || d.silver.stale) && el('div', { class: 'mt-2' },
        notice('Prices are outdated — zakat figures may not reflect current markets.')),
    ].filter(Boolean));
  }).catch(() => {
    ratesBox.replaceChildren(notice(t('Backend unreachable — start it with `make docker-up && make run` in backend/.'), 'error'));
  });

  const groupIcons = { 'Retail finance': '🏠', 'Business finance': '🏭', 'Zakat & religious': '🕌', 'Investment': '📈' };

  root.replaceChildren(
    el('div', { class: 'mb-8' },
      el('h1', { class: 'text-3xl font-bold tracking-tight' }, t('Islamic Calculator')),
      el('p', { class: 'text-stone-500 mt-1' },
        t('Shariah-compliant financial calculators. Sign in to keep a history of your calculations.'))),
    el('div', { class: 'grid gap-6 lg:grid-cols-[2fr_1fr] items-start' },
      el('div', { class: 'space-y-8' },
        GROUPS.map(group => el('section', {},
          el('h2', { class: 'text-sm font-semibold uppercase tracking-wide text-stone-500 mb-3' },
            `${groupIcons[group]}  ${t(group)}`),
          el('div', { class: 'grid gap-3 sm:grid-cols-2' },
            CALCS.filter(c => c.group === group).map(c =>
              el('a', {
                href: `#/calc/${c.id}`,
                class: CLS.card + ' p-4 block hover:border-brand-600 hover:shadow transition-colors',
              },
                el('h3', { class: 'font-semibold text-brand-800' }, t(c.title)),
                el('p', { class: 'text-sm text-stone-500 mt-0.5' }, t(c.blurb)))))))),
      el('div', { class: 'space-y-4' },
        ratesBox,
        el('a', { href: '#/reference/livestock', class: CLS.card + ' p-4 block hover:border-brand-600' },
          el('h3', { class: 'font-semibold' }, t('Livestock zakat tiers')),
          el('p', { class: 'text-sm text-stone-500' }, 'The seeded Hanafi rule table.')))));
}

// --- auth pages --------------------------------------------------------------

function authForm(root, { title, submitLabel, action, alt }) {
  const values = {};
  const errorBanner = el('div', { class: 'hidden' });
  const form = el('form', {
    class: CLS.card + ' p-6 space-y-4',
    onsubmit: async e => {
      e.preventDefault();
      errorBanner.className = 'hidden';
      try {
        await action(values.email || '', values.password || '');
        location.hash = '#/';
      } catch (err) {
        const fields = err instanceof ApiError ? err.fields : {};
        const unmatched = showFieldErrors(form, fields);
        errorBanner.className = '';
        errorBanner.replaceChildren(notice([err.message, ...unmatched].join(' · '), 'error'));
      }
    },
  },
    fieldWrapper({ name: 'email', label: t('Email'), type: 'text', placeholder: 'you@example.com' }, values, () => {}),
    el('div', { 'data-field': 'password' },
      el('label', { class: CLS.label }, t('Password')),
      el('input', {
        type: 'password', class: CLS.input, placeholder: t('8–72 characters'),
        oninput: e => { values.password = e.target.value; },
      }),
      el('p', { class: CLS.fieldError + ' hidden', 'data-error-for': 'password' })),
    errorBanner,
    el('button', { class: CLS.btnPrimary + ' w-full', type: 'submit' }, submitLabel));

  root.replaceChildren(
    el('div', { class: 'max-w-sm mx-auto mt-8' },
      el('h1', { class: 'text-2xl font-bold mb-4 text-center' }, title),
      form,
      el('p', { class: 'text-sm text-stone-500 text-center mt-4' }, alt)));
}

export function renderLogin(root) {
  authForm(root, {
    title: t('Sign in'), submitLabel: t('Sign in'), action: login,
    alt: el('span', {}, t('No account? '), el('a', { href: '#/register', class: 'text-brand-600 hover:underline' }, t('Register'))),
  });
}

export function renderRegister(root) {
  authForm(root, {
    title: t('Create account'), submitLabel: t('Register'), action: register,
    alt: el('span', {}, t('Already registered? '), el('a', { href: '#/login', class: 'text-brand-600 hover:underline' }, t('Sign in'))),
  });
}

// --- history -----------------------------------------------------------------

export function renderHistory(root) {
  if (!session()) { location.hash = '#/login'; return; }

  const listBox = el('div', { class: 'space-y-3' }, el('p', { class: 'text-sm text-stone-400' }, 'Loading…'));
  const filter = el('select', { class: CLS.input + ' max-w-xs', onchange: () => load() },
    el('option', { value: '' }, t('All calculators')),
    CALCS.map(c => {
      const type = c.endpoint.includes('/zakat/') ? 'zakat.' : c.endpoint.includes('/invest/') ? 'invest.' : 'finance.';
      const key = type + c.endpoint.split('/').pop().replaceAll('-', '_');
      return el('option', { value: key }, t(c.title));
    }));

  async function load() {
    try {
      const q = filter.value ? `&calcType=${filter.value}` : '';
      const d = await api(`/api/v1/history/?limit=50${q}`);
      const entries = d.entries || [];
      if (!entries.length) {
        listBox.replaceChildren(el('p', { class: 'text-sm text-stone-400' },
          t('No saved calculations yet — use any calculator while signed in and it lands here.')));
        return;
      }
      listBox.replaceChildren(...entries.map(e2 => {
        const calc = CALCS.find(c => {
          const tail = c.endpoint.split('/').pop().replaceAll('-', '_');
          return e2.calcType.endsWith(tail);
        });
        return el('div', { class: CLS.card + ' p-4 flex items-center justify-between gap-4' },
          el('div', {},
            el('div', { class: 'flex items-center gap-2' },
              el('span', { class: 'font-semibold' }, calc ? t(calc.title) : e2.calcType),
              badge(e2.calcType, 'stone')),
            el('p', { class: 'text-xs text-stone-400 mt-0.5 font-mono' }, e2.createdAt)),
          el('div', { class: 'flex gap-2' },
            calc && el('button', {
              class: CLS.btnGhost,
              onclick: () => alert('Inputs:\n' + JSON.stringify(e2.inputs, null, 2)),
            }, t('View inputs')),
            el('button', {
              class: CLS.btnGhost + ' text-red-600 border-red-200 hover:bg-red-50',
              onclick: async () => {
                await api(`/api/v1/history/${e2.id}`, { method: 'DELETE' });
                load();
              },
            }, t('Delete'))));
      }));
    } catch (err) {
      listBox.replaceChildren(notice(err.message, 'error'));
    }
  }
  load();

  root.replaceChildren(
    el('h1', { class: 'text-2xl font-bold mb-4' }, t('My calculations')),
    el('div', { class: 'mb-4' }, filter),
    listBox);
}

// --- livestock reference -----------------------------------------------------

export function renderLivestockReference(root) {
  const box = el('div', {}, el('p', { class: 'text-sm text-stone-400' }, 'Loading…'));
  api('/api/v1/reference/livestock-rules').then(d => {
    const bySpecies = {};
    for (const r of d.rules) (bySpecies[r.species] ||= []).push(r);
    const label = { sheep_goats: 'Sheep & goats', cattle: 'Cattle', camels: 'Camels' };
    box.replaceChildren(...Object.entries(bySpecies).map(([species, rules]) => el('section', { class: 'mb-8' },
      el('h2', { class: 'font-semibold text-lg mb-2' }, t(label[species] || species)),
      dataTable(
        [{ key: 'range', label: 'Head count' }, { key: 'due', label: 'Zakat due' }, { key: 'note', label: 'Note' }],
        rules.map(r => ({
          range: r.maxCount ? `${r.minCount} – ${r.maxCount}` : `${r.minCount}+`,
          due: (r.due || []).map(x => `${x.count} × ${x.animal}`).join(' + ') || '(computed)',
          note: (r.note || '').replaceAll('_', ' ') + (r.perExtraEvery ? ` (+1 per ${r.perExtraEvery} above)` : ''),
        }))))));
  }).catch(err => box.replaceChildren(notice(err.message, 'error')));

  root.replaceChildren(
    el('a', { href: '#/', class: 'text-sm text-brand-600 hover:underline' }, t('← Home')),
    el('h1', { class: 'text-2xl font-bold my-3' }, t('Livestock zakat tiers (Hanafi)')),
    el('p', { class: 'text-stone-500 mb-4 text-sm' },
      t('Served from the backend rule table. Cattle at 90+ head are computed by the per-30/per-40 combination rule.')),
    box);
}
