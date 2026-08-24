// Runtime configuration, loaded from the `env` file next to index.html
// (same KEY=VALUE format as backend/.env — edit it at deploy time, the
// JS never changes). Precedence:
//   1. localStorage override:  localStorage.setItem('ic.apiBase', '…')
//   2. the `env` file:         API_BASE_URL=…
//   3. built-in default:       http://localhost:8080

async function loadEnvFile() {
  try {
    const res = await fetch('env', { cache: 'no-store' });
    if (!res.ok) return {};
    const text = await res.text();
    if (text.trimStart().startsWith('<')) return {}; // host served index.html fallback
    const vars = {};
    for (const line of text.split('\n')) {
      const t = line.trim();
      if (!t || t.startsWith('#')) continue;
      const i = t.indexOf('=');
      if (i > 0) vars[t.slice(0, i).trim()] = t.slice(i + 1).trim().replace(/^["']|["']$/g, '');
    }
    return vars;
  } catch {
    return {};
  }
}

const env = await loadEnvFile();

export const API_BASE =
  (localStorage.getItem('ic.apiBase') || env.API_BASE_URL || 'http://localhost:8080')
    .replace(/\/+$/, '');
