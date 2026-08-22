// Router + header. Hash-based routes:
//   #/               dashboard
//   #/calc/<id>      calculator page
//   #/login  #/register  #/history  #/reference/livestock
import { API_BASE } from './config.js';
import { logout, session } from './api.js';
import { el, CLS } from './ui.js';
import {
  renderCalcPage, renderHistory, renderHome,
  renderLivestockReference, renderLogin, renderRegister,
} from './pages.js';

const app = document.getElementById('app');
const header = document.getElementById('header');
document.getElementById('api-base-label').textContent = API_BASE;

function renderHeader() {
  const s = session();
  header.replaceChildren(
    el('div', { class: 'max-w-6xl mx-auto px-4 h-14 flex items-center justify-between gap-4' },
      el('a', { href: '#/', class: 'flex items-center gap-2 font-bold text-brand-800' },
        el('span', { class: 'inline-flex h-7 w-7 items-center justify-center rounded-lg bg-brand-600 text-white text-xs font-bold tracking-tight' }, 'IC'),
        'Islamic Calculator'),
      el('nav', { class: 'flex items-center gap-2 text-sm' },
        el('a', { href: '#/', class: 'px-2 py-1 rounded hover:bg-stone-100' }, 'Calculators'),
        s && el('a', { href: '#/history', class: 'px-2 py-1 rounded hover:bg-stone-100' }, 'History'),
        s
          ? el('span', { class: 'flex items-center gap-2 pl-2 border-l border-stone-200' },
              el('span', { class: 'text-stone-500 hidden sm:inline' }, s.email),
              el('button', { class: CLS.btnGhost, onclick: () => { logout(); location.hash = '#/'; } }, 'Sign out'))
          : el('a', { href: '#/login', class: CLS.btnPrimary }, 'Sign in'))));
}

function route() {
  const hash = location.hash || '#/';
  const parts = hash.slice(2).split('/').filter(Boolean); // '#/calc/x' → ['calc','x']
  window.scrollTo(0, 0);

  if (parts.length === 0) return renderHome(app);
  if (parts[0] === 'calc' && parts[1]) return renderCalcPage(app, parts[1]);
  if (parts[0] === 'login') return renderLogin(app);
  if (parts[0] === 'register') return renderRegister(app);
  if (parts[0] === 'history') return renderHistory(app);
  if (parts[0] === 'reference' && parts[1] === 'livestock') return renderLivestockReference(app);
  renderHome(app);
}

window.addEventListener('hashchange', route);
window.addEventListener('auth-changed', () => { renderHeader(); });

renderHeader();
route();
