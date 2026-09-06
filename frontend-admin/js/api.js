// Single place to change the API URL when the backend is deployed.
const API_BASE = 'http://localhost:8080';

// Session state lives in sessionStorage (cleared when the tab closes) —
// this is a real deployed app, not a sandboxed preview, so normal browser
// storage is the right call here (unlike in-chat design previews).
const SESSION_KEY = 'ochidoma_admin_session';

function getSession() {
  const raw = sessionStorage.getItem(SESSION_KEY);
  return raw ? JSON.parse(raw) : null;
}

function setSession(session) {
  sessionStorage.setItem(SESSION_KEY, JSON.stringify(session));
}

function clearSession() {
  sessionStorage.removeItem(SESSION_KEY);
}

function requireLogin() {
  if (!getSession()) {
    window.location.href = 'login.html';
  }
}

async function login(email, password) {
  const res = await fetch(`${API_BASE}/api/auth/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    credentials: 'include', // needed so the refresh-token cookie gets set
    body: JSON.stringify({ email, password }),
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body.error || 'Login failed');
  }
  const data = await res.json();
  setSession({ accessToken: data.access_token, user: data.user, expiresAt: Date.now() + data.expires_in * 1000 });
  return data.user;
}

async function refreshAccessToken() {
  const res = await fetch(`${API_BASE}/api/auth/refresh`, {
    method: 'POST',
    credentials: 'include',
  });
  if (!res.ok) {
    clearSession();
    return null;
  }
  const data = await res.json();
  const session = getSession();
  if (session) {
    session.accessToken = data.access_token;
    session.expiresAt = Date.now() + data.expires_in * 1000;
    setSession(session);
  }
  return data.access_token;
}

async function logout() {
  await fetch(`${API_BASE}/api/auth/logout`, { method: 'POST', credentials: 'include' }).catch(() => {});
  clearSession();
  window.location.href = 'login.html';
}

// authFetch: attaches the access token, and transparently refreshes once on
// a 401 before giving up — so a page doesn't just break when a 15-minute
// token expires mid-session.
async function authFetch(path, options = {}) {
  let session = getSession();
  if (!session) {
    window.location.href = 'login.html';
    throw new Error('not logged in');
  }

  const doFetch = (token) => fetch(`${API_BASE}${path}`, {
    ...options,
    headers: {
      ...(options.body ? { 'Content-Type': 'application/json' } : {}),
      ...(options.headers || {}),
      'Authorization': `Bearer ${token}`,
    },
  });

  let res = await doFetch(session.accessToken);
  if (res.status === 401) {
    const newToken = await refreshAccessToken();
    if (!newToken) {
      window.location.href = 'login.html';
      throw new Error('session expired');
    }
    res = await doFetch(newToken);
  }
  return res;
}

async function authGet(path) {
  const res = await authFetch(path);
  if (!res.ok) throw await apiError(res);
  return res.json();
}

async function authPost(path, body) {
  const res = await authFetch(path, { method: 'POST', body: body !== undefined ? JSON.stringify(body) : undefined });
  if (!res.ok) throw await apiError(res);
  return res.json();
}

async function authPut(path, body) {
  const res = await authFetch(path, { method: 'PUT', body: JSON.stringify(body) });
  if (!res.ok) throw await apiError(res);
  return res.json();
}

async function authDelete(path) {
  const res = await authFetch(path, { method: 'DELETE' });
  if (!res.ok && res.status !== 204) throw await apiError(res);
}

async function apiError(res) {
  const body = await res.json().catch(() => ({}));
  return new Error(body.error || `Request failed (${res.status})`);
}

// role helpers — the SAME roles are enforced server-side via RequireRole;
// this only controls which buttons are shown, never what's actually allowed.
function currentRole() {
  const session = getSession();
  return session ? session.user.role : null;
}

function hasRole(...roles) {
  return roles.includes(currentRole());
}

function isPublisher() { return hasRole('super_admin', 'palace_publisher'); }
function isCulturalEditor() { return hasRole('super_admin', 'cultural_editor'); }
function canEditContent() { return hasRole('super_admin', 'palace_editor', 'cultural_editor'); }

function escapeHTML(str) {
  const div = document.createElement('div');
  div.textContent = str ?? '';
  return div.innerHTML;
}

function formatDate(isoString) {
  if (!isoString) return '—';
  return new Date(isoString).toLocaleString('en-GB', {
    day: 'numeric', month: 'short', year: 'numeric', hour: '2-digit', minute: '2-digit',
  });
}

function renderSidebarUser() {
  const el = document.getElementById('sidebar-user');
  if (!el) return;
  const session = getSession();
  if (!session) return;
  el.innerHTML = `
    ${escapeHTML(session.user.full_name)}
    <div class="role-badge">${escapeHTML(session.user.role)}</div>
    <div class="logout-link" onclick="logout()">Log out</div>
  `;
}
