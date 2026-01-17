import React, { useEffect, useState } from 'react';
import { useAuth } from '../../auth/auth.jsx';
import { api } from '../../../services/apiClient.js';

export default function Dashboard() {
  const { user, employee } = useAuth();
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
          <h2>Welcome back, {employee?.firstName || (user?.role || user?.RoleName)}</h2>
          <p>Here’s your live snapshot across leave, payroll, and performance.</p>
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
