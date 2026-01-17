import React, { useEffect, useState } from 'react';
import { api } from '../../../services/apiClient.js';
import { useAuth } from '../../auth/auth.jsx';

export default function Reports() {
  const { user } = useAuth();
  const [data, setData] = useState(null);
  const [error, setError] = useState('');

  useEffect(() => {
    const load = async () => {
      try {
        if ((user?.role || user?.RoleName) === 'HR') {
          setData(await api.get('/reports/dashboard/hr'));
        } else if ((user?.role || user?.RoleName) === 'Manager') {
          setData(await api.get('/reports/dashboard/manager'));
        } else {
          setData(await api.get('/reports/dashboard/employee'));
        }
      } catch (err) {
        setError(err.message);
      }
    };
    load();
  }, [user]);

  return (
    <section className="page">
      <header className="page-header">
        <div>
          <h2>Reports</h2>
          <p>Role-specific reporting snapshots with export-ready data.</p>
        </div>
      </header>

      {error && <div className="error">{error}</div>}

      {data && (
        <div className="card-grid">
          {Object.entries(data).map(([key, value]) => (
            <div key={key} className="card">
              <h3>{key}</h3>
              <p className="metric">{value}</p>
            </div>
          ))}
        </div>
      )}
    </section>
  );
}
