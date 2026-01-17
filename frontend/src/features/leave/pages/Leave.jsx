import React, { useEffect, useState } from 'react';
import { api } from '../../../services/apiClient.js';
import { useAuth } from '../../auth/auth.jsx';

export default function Leave() {
  const { employee } = useAuth();
  const [types, setTypes] = useState([]);
  const [requests, setRequests] = useState([]);
  const [error, setError] = useState('');
  const [form, setForm] = useState({ leaveTypeId: '', startDate: '', endDate: '', reason: '' });

  const load = async () => {
    try {
      const [typesData, requestsData] = await Promise.all([
        api.get('/leave/types'),
        api.get('/leave/requests'),
      ]);
      setTypes(Array.isArray(typesData) ? typesData : []);
      setRequests(Array.isArray(requestsData) ? requestsData : []);
    } catch (err) {
      setError(err.message);
    }
  };

  useEffect(() => {
    load();
  }, []);

  const handleSubmit = async (e) => {
    e.preventDefault();
    setError('');
    try {
      await api.post('/leave/requests', {
        employeeId: employee?.id,
        leaveTypeId: form.leaveTypeId,
        startDate: form.startDate,
        endDate: form.endDate,
        reason: form.reason,
      });
      setForm({ leaveTypeId: '', startDate: '', endDate: '', reason: '' });
      await load();
    } catch (err) {
      setError(err.message);
    }
  };

  return (
    <section className="page">
      <header className="page-header">
        <div>
          <h2>Leave</h2>
          <p>Request time off and review approvals.</p>
        </div>
      </header>

      {error && <div className="error">{error}</div>}

      <form className="inline-form" onSubmit={handleSubmit}>
        <select value={form.leaveTypeId} onChange={(e) => setForm({ ...form, leaveTypeId: e.target.value })}>
          <option value="">Leave type</option>
          {types.map((type) => (
            <option key={type.id} value={type.id}>{type.name}</option>
          ))}
        </select>
        <input type="date" value={form.startDate} onChange={(e) => setForm({ ...form, startDate: e.target.value })} />
        <input type="date" value={form.endDate} onChange={(e) => setForm({ ...form, endDate: e.target.value })} />
        <input placeholder="Reason" value={form.reason} onChange={(e) => setForm({ ...form, reason: e.target.value })} />
        <button type="submit">Request leave</button>
      </form>

      <div className="table">
        <div className="table-row header">
          <span>Type</span>
          <span>Dates</span>
          <span>Status</span>
        </div>
        {requests.map((req) => (
          <div key={req.id} className="table-row">
            <span>{types.find((t) => t.id === req.leaveTypeId)?.name || req.leaveTypeId}</span>
            <span>{req.startDate?.slice(0, 10)} → {req.endDate?.slice(0, 10)}</span>
            <span>{req.status}</span>
          </div>
        ))}
      </div>
    </section>
  );
}
