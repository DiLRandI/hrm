const listeners = new Set();

export const TOAST_ERROR = 'error';
export const TOAST_SUCCESS = 'success';

export function subscribeToast(listener) {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

export function pushToast(payload) {
  if (!payload) {
    return;
  }
  const normalized = typeof payload === 'string' ? { message: payload } : payload;
  if (!normalized.message) {
    return;
  }
  listeners.forEach((listener) => listener(normalized));
}
