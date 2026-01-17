const API_BASE = '/api/v1';

export function getToken() {
  return localStorage.getItem('hrm_token');
}

export function setToken(token) {
  if (token) {
    localStorage.setItem('hrm_token', token);
  } else {
    localStorage.removeItem('hrm_token');
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

export const api = {
  get: (path) => request(path),
  post: (path, body) => request(path, { method: 'POST', body: JSON.stringify(body) }),
  put: (path, body) => request(path, { method: 'PUT', body: JSON.stringify(body) }),
};
