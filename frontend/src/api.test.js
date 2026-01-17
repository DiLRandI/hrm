import { describe, it, expect, vi, beforeEach } from 'vitest';
import { api, setToken } from './api';

const mockResponse = (payload, ok = true) => ({
  ok,
  json: vi.fn().mockResolvedValue(payload),
});

describe('api client', () => {
  beforeEach(() => {
    localStorage.clear();
    global.fetch = vi.fn();
  });

  it('adds authorization header when token is set', async () => {
    setToken('token-123');
    global.fetch.mockResolvedValue(mockResponse({ data: { ok: true } }));

    await api.get('/me');

    expect(global.fetch).toHaveBeenCalledOnce();
    const [url, options] = global.fetch.mock.calls[0];
    expect(url).toContain('/api/v1/me');
    expect(options.headers.Authorization).toBe('Bearer token-123');
  });

  it('throws error when response is not ok', async () => {
    global.fetch.mockResolvedValue(mockResponse({ error: { message: 'fail' } }, false));

    await expect(api.get('/me')).rejects.toThrow('fail');
  });
});
