import { useState, useEffect, useCallback } from 'react';
import { api } from '../api/client.js';

// useApi fetches a GET endpoint and tracks loading/error/data with a reload().
// Pass a falsy path to skip fetching (e.g. a detail route with no id yet).
export function useApi(path, deps = []) {
  const [data, setData] = useState(null);
  const [error, setError] = useState(null);
  const [loading, setLoading] = useState(Boolean(path));

  const load = useCallback(async () => {
    if (!path) return;
    setLoading(true);
    setError(null);
    try {
      const d = await api.get(path);
      setData(d);
    } catch (e) {
      setError(e);
    } finally {
      setLoading(false);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, deps);

  useEffect(() => {
    load();
  }, [load]);

  return { data, error, loading, reload: load, setData };
}
