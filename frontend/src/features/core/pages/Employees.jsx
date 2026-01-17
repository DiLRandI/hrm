import React, { useEffect, useState } from 'react';
import { api } from '../../../services/apiClient.js';
import { useAuth } from '../../auth/auth.jsx';

export default function Employees() {
  const { user } = useAuth();
  const [employees, setEmployees] = useState([]);
  const [error, setError] = useState('');
  const [form, setForm] = useState({ firstName: '', lastName: '', email: '' });

  const load = async () => {
    try {
      const data = await api.get('/employees');
      setEmployees(data);
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
      await api.post('/employees', {
        firstName: form.firstName,
        lastName: form.lastName,
        email: form.email,
        status: 'active',
      });
      setForm({ firstName: '', lastName: '', email: '' });
      await load();
    } catch (err) {
      setError(err.message);
    }
  };

  return (
    <section className="page">
      <header className="page-header">
        <div>
          <h2>People</h2>
          <p>Manage employee profiles and reporting lines.</p>
        </div>
      </header>

      {error && <div className="error">{error}</div>}

      {user?.role === 'HR' && (
        <form className="inline-form" onSubmit={handleSubmit}>
          <input placeholder="First name" value={form.firstName} onChange={(e) => setForm({ ...form, firstName: e.target.value })} />
          <input placeholder="Last name" value={form.lastName} onChange={(e) => setForm({ ...form, lastName: e.target.value })} />
          <input placeholder="Email" value={form.email} onChange={(e) => setForm({ ...form, email: e.target.value })} />
          <button type="submit">Add employee</button>
        </form>
      )}

      <div className="table">
        <div className="table-row header">
          <span>Name</span>
          <span>Email</span>
          <span>Status</span>
        </div>
        {employees.map((emp) => (
          <div key={emp.id} className="table-row">
            <span>{emp.firstName} {emp.lastName}</span>
            <span>{emp.email}</span>
            <span>{emp.status}</span>
          </div>
        ))}
      </div>
    </section>
  );
}
