import { useCallback, useEffect, useRef, useState } from 'react';

export function useApiQuery(fetcher, deps = [], options = {}) {
  const { enabled = true, initialData = null } = options;
  const [data, setData] = useState(initialData);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(enabled);
  const abortRef = useRef(null);

  const run = useCallback(async () => {
    if (!enabled) {
      return;
    }
    if (abortRef.current) {
      abortRef.current.abort();
    }
    const controller = new AbortController();
    abortRef.current = controller;
    setLoading(true);
    setError('');
    try {
      const result = await fetcher({ signal: controller.signal });
      if (!controller.signal.aborted) {
        setData(result);
      }
    } catch (err) {
      if (!controller.signal.aborted) {
        setError(err.message || 'Request failed');
      }
    } finally {
      if (!controller.signal.aborted) {
        setLoading(false);
      }
    }
  }, [enabled, fetcher]);

  useEffect(() => {
    run();
    return () => {
      if (abortRef.current) {
        abortRef.current.abort();
      }
    };
  }, [run, ...deps]);

  return { data, error, loading, reload: run, setData, setError };
}
