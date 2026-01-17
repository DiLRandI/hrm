import React, { useEffect, useState } from 'react';
import { api } from '../../../services/apiClient.js';
import { useAuth } from '../../auth/auth.jsx';

export default function Payroll() {
  const { user, employee } = useAuth();
  const [periods, setPeriods] = useState([]);
  const [payslips, setPayslips] = useState([]);
  const [error, setError] = useState('');
  const [form, setForm] = useState({ scheduleId: '', startDate: '', endDate: '' });

  const load = async () => {
    try {
      const [periodData, payslipData] = await Promise.all([
        api.get('/payroll/periods'),
        api.get('/payroll/payslips?employeeId=' + (employee?.id || '')),
      ]);
      setPeriods(periodData);
      setPayslips(payslipData);
    } catch (err) {
      setError(err.message);
    }
  };

  useEffect(() => {
    load();
  }, [employee]);

  const createPeriod = async (e) => {
    e.preventDefault();
    setError('');
    try {
      await api.post('/payroll/periods', form);
      setForm({ scheduleId: '', startDate: '', endDate: '' });
      await load();
    } catch (err) {
      setError(err.message);
    }
  };

  const runPayroll = async (id) => {
    await api.post(`/payroll/periods/${id}/run`, {});
    await load();
  };

  const finalizePayroll = async (id) => {
    await api.post(`/payroll/periods/${id}/finalize`, {});
    await load();
  };

  return (
    <section className="page">
      <header className="page-header">
        <div>
          <h2>Payroll</h2>
          <p>Draft, run, and finalize payroll periods with payslips.</p>
        </div>
      </header>

      {error && <div className="error">{error}</div>}

      {(user?.role || user?.RoleName) === 'HR' && (
        <form className="inline-form" onSubmit={createPeriod}>
          <input placeholder="Schedule ID" value={form.scheduleId} onChange={(e) => setForm({ ...form, scheduleId: e.target.value })} />
          <input type="date" value={form.startDate} onChange={(e) => setForm({ ...form, startDate: e.target.value })} />
          <input type="date" value={form.endDate} onChange={(e) => setForm({ ...form, endDate: e.target.value })} />
          <button type="submit">Create period</button>
        </form>
      )}

      <div className="table">
        <div className="table-row header">
          <span>Period</span>
          <span>Status</span>
          <span>Actions</span>
        </div>
        {periods.map((period) => (
          <div key={period.id} className="table-row">
            <span>{period.startDate?.slice(0, 10)} → {period.endDate?.slice(0, 10)}</span>
            <span>{period.status}</span>
            <span className="row-actions">
              {(user?.role || user?.RoleName) === 'HR' && (
                <>
                  <button onClick={() => runPayroll(period.id)}>Run</button>
                  <button onClick={() => finalizePayroll(period.id)}>Finalize</button>
                </>
              )}
            </span>
          </div>
        ))}
      </div>

      <h3>Payslips</h3>
      <div className="table">
        <div className="table-row header">
          <span>Period</span>
          <span>Net</span>
          <span>Currency</span>
        </div>
        {payslips.map((slip) => (
          <div key={slip.id} className="table-row">
            <span>{slip.periodId}</span>
            <span>{slip.net}</span>
            <span>{slip.currency}</span>
          </div>
        ))}
      </div>
    </section>
  );
}
