import React, { useEffect, useState } from 'react';
import { api } from '../../../services/apiClient.js';
import { useAuth } from '../../auth/auth.jsx';

export default function Performance() {
  const { employee } = useAuth();
  const [goals, setGoals] = useState([]);
  const [error, setError] = useState('');
  const [form, setForm] = useState({ title: '', metric: '', dueDate: '' });

  const load = async () => {
    try {
      const data = await api.get('/performance/goals');
      setGoals(Array.isArray(data) ? data : []);
    } catch (err) {
      setError(err.message);
    }
  };

  useEffect(() => {
    load();
  }, []);

  const submit = async (e) => {
    e.preventDefault();
    try {
      await api.post('/performance/goals', {
        employeeId: employee?.id,
        title: form.title,
        metric: form.metric,
        dueDate: form.dueDate,
        status: 'active',
        progress: 0,
      });
      setForm({ title: '', metric: '', dueDate: '' });
      await load();
    } catch (err) {
      setError(err.message);
    }
  };

  return (
    <section className="page">
      <header className="page-header">
        <div>
          <h2>Performance</h2>
          <p>Track goals, feedback, and review cycles.</p>
        </div>
      </header>

      {error && <div className="error">{error}</div>}

      <form className="inline-form" onSubmit={submit}>
        <input placeholder="Goal title" value={form.title} onChange={(e) => setForm({ ...form, title: e.target.value })} />
        <input placeholder="Metric" value={form.metric} onChange={(e) => setForm({ ...form, metric: e.target.value })} />
        <input type="date" value={form.dueDate} onChange={(e) => setForm({ ...form, dueDate: e.target.value })} />
        <button type="submit">Add goal</button>
      </form>

      <div className="table">
        <div className="table-row header">
          <span>Title</span>
          <span>Due</span>
          <span>Status</span>
        </div>
        {goals.map((goal) => (
          <div key={goal.id} className="table-row">
            <span>{goal.title}</span>
            <span>{goal.dueDate?.slice(0, 10)}</span>
            <span>{goal.status}</span>
          </div>
        ))}
      </div>
    </section>
  );
}
