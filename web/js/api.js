// API client honoring the shared contract (PROJECT_CONTEXT.md):
// envelope unwrapping (C1), anonymous-first with optional bearer (C3),
// and single-flight refresh-token rotation (C4).
import { API_BASE } from './config.js';

const TOKENS_KEY = 'ic.tokens';

export class ApiError extends Error {
  constructor(status, code, message, fields) {
    super(message);
    this.status = status;
    this.code = code;
    this.fields = fields || {};
  }
}

// --- auth state --------------------------------------------------------------

export function session() {
  try {
    return JSON.parse(localStorage.getItem(TOKENS_KEY));
  } catch {
    return null;
  }
}

function saveSession(s) {
  if (s) localStorage.setItem(TOKENS_KEY, JSON.stringify(s));
  else localStorage.removeItem(TOKENS_KEY);
  window.dispatchEvent(new Event('auth-changed'));
}

export function logout() {
  saveSession(null);
}

function adoptAuthPayload(data) {
  saveSession({
    userId: data.user.id,
    email: data.user.email || session()?.email || '',
    accessToken: data.accessToken,
    refreshToken: data.refreshToken,
  });
}

// --- refresh rotation: single flight ----------------------------------------

let refreshing = null;

async function refreshTokens() {
  if (!refreshing) {
    refreshing = (async () => {
      const s = session();
      if (!s?.refreshToken) throw new ApiError(401, 'UNAUTHORIZED', 'no session');
      const res = await fetch(`${API_BASE}/api/v1/auth/refresh`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ refreshToken: s.refreshToken }),
      });
      const body = await res.json();
      if (body.error) {
        saveSession(null); // rotated/expired token: the session is over
        throw new ApiError(res.status, body.error.code, body.error.message, body.error.fields);
      }
      adoptAuthPayload(body.data);
    })().finally(() => { refreshing = null; });
  }
  return refreshing;
}

// --- core request ------------------------------------------------------------

export async function api(path, { method = 'GET', body } = {}) {
  const doFetch = () => {
    const headers = { Accept: 'application/json' };
    if (body !== undefined) headers['Content-Type'] = 'application/json';
    const token = session()?.accessToken;
    if (token) headers.Authorization = `Bearer ${token}`;
    return fetch(API_BASE + path, {
      method,
      headers,
      body: body !== undefined ? JSON.stringify(body) : undefined,
    });
  };

  let res = await doFetch();
  if (res.status === 401 && session()?.refreshToken && !path.startsWith('/api/v1/auth/')) {
    await refreshTokens(); // throws (and clears session) if the refresh is dead
    res = await doFetch();
  }

  const text = await res.text();
  let parsed;
  try {
    parsed = JSON.parse(text);
  } catch {
    throw new ApiError(res.status, 'INTERNAL', `unexpected response (${res.status})`);
  }
  if (parsed.error) {
    throw new ApiError(res.status, parsed.error.code, parsed.error.message, parsed.error.fields);
  }
  return parsed.data;
}

// --- auth flows --------------------------------------------------------------

export async function register(email, password) {
  const data = await api('/api/v1/auth/register', { method: 'POST', body: { email, password } });
  adoptAuthPayload(data);
  return data;
}

export async function login(email, password) {
  const data = await api('/api/v1/auth/login', { method: 'POST', body: { email, password } });
  adoptAuthPayload(data);
  return data;
}
