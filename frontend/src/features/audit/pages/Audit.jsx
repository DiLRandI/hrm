import React, { useEffect, useState } from 'react';
import { api } from '../../../services/apiClient.js';

const downloadBlob = ({ blob, filename }) => {
  const url = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  link.download = filename;
  link.click();
  URL.revokeObjectURL(url);
};

export default function Audit() {
  const [events, setEvents] = useState([]);
  const [filters, setFilters] = useState({
    action: '',
    entityType: '',
    actorUserId: '',
    includeDetails: false,
  });
  const [offset, setOffset] = useState(0);
  const [total, setTotal] = useState(0);
  const [error, setError] = useState('');

  const LIMIT = 25;

  const load = async (nextOffset = offset) => {
    setError('');
    try {
      const params = new URLSearchParams();
      params.set('limit', String(LIMIT));
      params.set('offset', String(nextOffset));
      if (filters.action) params.set('action', filters.action);
      if (filters.entityType) params.set('entityType', filters.entityType);
      if (filters.actorUserId) params.set('actorUserId', filters.actorUserId);
      if (filters.includeDetails) params.set('includeDetails', 'true');
      const { data, total: totalCount } = await api.getWithMeta(`/audit/events?${params.toString()}`);
      setEvents(Array.isArray(data) ? data : []);
      setTotal(totalCount ?? (Array.isArray(data) ? data.length : 0));
      setOffset(nextOffset);
    } catch (err) {
      setError(err.message);
    }
  };

  useEffect(() => {
    load(0);
  }, [filters.action, filters.entityType, filters.actorUserId, filters.includeDetails]);

  const exportCsv = async () => {
    try {
      const result = await api.download('/audit/events/export');
      downloadBlob(result);
    } catch (err) {
      setError(err.message);
    }
  };

  const nextPage = async () => {
    const nextOffset = offset + LIMIT;
    if (nextOffset >= total) {
      return;
    }
    await load(nextOffset);
  };

  const prevPage = async () => {
    const prevOffset = Math.max(0, offset - LIMIT);
    await load(prevOffset);
  };

  return (
    <section className="page">
      <header className="page-header">
        <div>
          <h2>Audit log</h2>
          <p>Track sensitive actions across payroll, leave, and performance.</p>
        </div>
        <button type="button" onClick={exportCsv}>Export CSV</button>
      </header>

      {error && <div className="error">{error}</div>}

      <form className="inline-form" onSubmit={(e) => e.preventDefault()}>
        <input
          placeholder="Action (e.g. payroll.finalize)"
          value={filters.action}
          onChange={(e) => setFilters({ ...filters, action: e.target.value })}
        />
        <input
          placeholder="Entity type"
          value={filters.entityType}
          onChange={(e) => setFilters({ ...filters, entityType: e.target.value })}
        />
        <input
          placeholder="Actor user ID"
          value={filters.actorUserId}
          onChange={(e) => setFilters({ ...filters, actorUserId: e.target.value })}
        />
        <label className="checkbox">
          <input
            type="checkbox"
            checked={filters.includeDetails}
            onChange={(e) => setFilters({ ...filters, includeDetails: e.target.checked })}
          />
          Include before/after
        </label>
      </form>

      <div className="table">
        <div className="table-row header">
          <span>Action</span>
          <span>Actor</span>
          <span>Entity</span>
          <span>When</span>
        </div>
        {events.map((event) => (
          <div key={event.id} className="table-row">
            <span>{event.action}</span>
            <span>{event.actorId}</span>
            <span>{event.entityType}:{event.entityId}</span>
            <span>{event.createdAt?.slice(0, 10)}</span>
          </div>
        ))}
      </div>

      {filters.includeDetails && events.length > 0 && (
        <div className="card">
          <h3>Event details</h3>
          {events.map((event) => (
            <div key={event.id} className="list-item">
              <div>
                <strong>{event.action}</strong>
                <p className="hint">Entity: {event.entityType} · {event.entityId}</p>
              </div>
              <pre className="code-block">
                {JSON.stringify({ before: event.before, after: event.after }, null, 2)}
              </pre>
            </div>
          ))}
        </div>
      )}

      <div className="row-actions pagination">
        <button type="button" className="ghost" onClick={prevPage} disabled={offset === 0}>
          Prev
        </button>
        <small>
          {total ? `${Math.min(offset + LIMIT, total)} of ${total}` : '—'}
        </small>
        <button type="button" className="ghost" onClick={nextPage} disabled={total ? offset + LIMIT >= total : events.length < LIMIT}>
          Next
        </button>
      </div>
    </section>
  );
}
