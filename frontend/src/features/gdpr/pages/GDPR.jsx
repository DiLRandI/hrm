import React, { useEffect, useState } from 'react';
import { api } from '../../../services/apiClient.js';

export default function GDPR() {
  const [dsars, setDsars] = useState([]);
  const [employeeId, setEmployeeId] = useState('');
  const [error, setError] = useState('');

  const load = async () => {
    try {
      const data = await api.get('/gdpr/dsar');
      setDsars(Array.isArray(data) ? data : []);
    } catch (err) {
      setError(err.message);
    }
  };

  useEffect(() => {
    load();
  }, []);

  const requestExport = async (e) => {
    e.preventDefault();
    setError('');
    try {
      await api.post('/gdpr/dsar', { employeeId });
      setEmployeeId('');
      await load();
    } catch (err) {
      setError(err.message);
    }
  };

  return (
    <section className="page">
      <header className="page-header">
        <div>
          <h2>GDPR Toolkit</h2>
          <p>Manage DSAR exports, retention policies, and anonymization workflows.</p>
        </div>
      </header>

      {error && <div className="error">{error}</div>}

      <form className="inline-form" onSubmit={requestExport}>
        <input placeholder="Employee ID" value={employeeId} onChange={(e) => setEmployeeId(e.target.value)} />
        <button type="submit">Request DSAR export</button>
      </form>

      <div className="table">
        <div className="table-row header">
          <span>Employee</span>
          <span>Status</span>
          <span>File</span>
        </div>
        {dsars.map((dsar) => (
          <div key={dsar.id} className="table-row">
            <span>{dsar.employeeId}</span>
            <span>{dsar.status}</span>
            <span>{dsar.fileUrl || 'Pending'}</span>
          </div>
        ))}
      </div>
    </section>
  );
}
