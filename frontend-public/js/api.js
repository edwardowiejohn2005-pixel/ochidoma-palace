// Single source of truth for the API base URL. This is the ONE line to
// change when the backend gets deployed somewhere public — everything else
// in the site calls the functions below, never fetch() directly.
const API_BASE = 'http://localhost:8080';

async function apiGet(path) {
  const res = await fetch(`${API_BASE}${path}`);
  if (!res.ok) {
    throw new Error(`API error ${res.status} on ${path}`);
  }
  return res.json();
}

const api = {
  decrees: () => apiGet('/api/decrees'),
  decree: (decreeNumber) => apiGet(`/api/decrees/${encodeURIComponent(decreeNumber)}`),
  announcements: () => apiGet('/api/announcements'),
  announcement: (id) => apiGet(`/api/announcements/${id}`),
  articles: (params = {}) => {
    const qs = new URLSearchParams(params).toString();
    return apiGet(`/api/articles${qs ? '?' + qs : ''}`);
  },
  article: (slug) => apiGet(`/api/articles/${slug}`),
  foods: () => apiGet('/api/foods'),
  food: (slug) => apiGet(`/api/foods/${slug}`),
  events: (status) => apiGet(`/api/events${status ? '?status=' + status : ''}`),
  event: (slug) => apiGet(`/api/events/${slug}`),
};

// Small helpers reused across pages.
function formatDate(isoString) {
  if (!isoString) return '';
  return new Date(isoString).toLocaleDateString('en-GB', {
    day: 'numeric', month: 'long', year: 'numeric',
  });
}

function escapeHTML(str) {
  const div = document.createElement('div');
  div.textContent = str ?? '';
  return div.innerHTML;
}

function renderState(container, message) {
  container.innerHTML = `<div class="state-message">${escapeHTML(message)}</div>`;
}
