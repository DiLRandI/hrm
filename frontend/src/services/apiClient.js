import { API_BASE } from '../shared/constants/api.js';
import { TOKEN_STORAGE_KEY } from '../shared/constants/storage.js';

export function getToken() {
  return localStorage.getItem(TOKEN_STORAGE_KEY);
}

export function setToken(token) {
  if (token) {
    localStorage.setItem(TOKEN_STORAGE_KEY, token);
  } else {
    localStorage.removeItem(TOKEN_STORAGE_KEY);
  }
}

async function request(path, options = {}) {
  const headers = options.headers || {};
  const token = getToken();
  if (token) {
    headers.Authorization = `Bearer ${token}`;
  }
  if (!(options.body instanceof FormData)) {
    headers['Content-Type'] = 'application/json';
  }

  const response = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers,
  });

  const data = await response.json().catch(() => ({}));
  if (!response.ok) {
    const message = data?.error?.message || 'Request failed';
    throw new Error(message);
  }
  return data.data ?? data;
}

async function requestRaw(path, options = {}) {
  const headers = options.headers || {};
  const token = getToken();
  if (token) {
    headers.Authorization = `Bearer ${token}`;
  }
  if (options.contentType) {
    headers['Content-Type'] = options.contentType;
  }

  const response = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers,
  });

  const data = await response.json().catch(() => ({}));
  if (!response.ok) {
    const message = data?.error?.message || 'Request failed';
    throw new Error(message);
  }
  return data.data ?? data;
}

async function download(path, options = {}) {
  const headers = options.headers || {};
  const token = getToken();
  if (token) {
    headers.Authorization = `Bearer ${token}`;
  }

  const response = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers,
  });

  if (!response.ok) {
    const data = await response.json().catch(() => ({}));
    const message = data?.error?.message || 'Download failed';
    throw new Error(message);
  }

  const blob = await response.blob();
  const disposition = response.headers.get('Content-Disposition') || '';
  const match = disposition.match(/filename="?([^"]+)"?/i);
  const filename = match ? match[1] : 'download';
  return { blob, filename };
}

export const api = {
  get: (path) => request(path),
  post: (path, body) => request(path, { method: 'POST', body: JSON.stringify(body) }),
  put: (path, body) => request(path, { method: 'PUT', body: JSON.stringify(body) }),
  del: (path) => request(path, { method: 'DELETE' }),
  postRaw: (path, body, contentType) => requestRaw(path, { method: 'POST', body, contentType }),
  download: (path, options) => download(path, options),
};
