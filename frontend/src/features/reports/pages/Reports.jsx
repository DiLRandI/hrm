import React, { useEffect, useState } from 'react';
import { api } from '../../../services/apiClient.js';
import { useAuth } from '../../auth/auth.jsx';
import { ROLE_HR, ROLE_MANAGER, ROLE_EMPLOYEE } from '../../../shared/constants/roles.js';

const downloadBlob = ({ blob, filename }) => {
  const url = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  link.download = filename;
  link.click();
  URL.revokeObjectURL(url);
};

export default function Reports() {
  const { user } = useAuth();
  const role = user?.role || user?.RoleName || ROLE_EMPLOYEE;
  const isHR = role === ROLE_HR;
  const isManager = role === ROLE_MANAGER;
  const [data, setData] = useState(null);
  const [jobRuns, setJobRuns] = useState([]);
  const [jobFilter, setJobFilter] = useState('');
  const [error, setError] = useState('');

  useEffect(() => {
    const load = async () => {
      try {
        if (isHR) {
          setData(await api.get('/reports/dashboard/hr'));
        } else if (isManager) {
          setData(await api.get('/reports/dashboard/manager'));
        } else {
          setData(await api.get('/reports/dashboard/employee'));
        }
        if (isHR) {
          const runs = await api.get(`/reports/jobs${jobFilter ? `?jobType=${jobFilter}` : ''}`);
          setJobRuns(Array.isArray(runs) ? runs : []);
        }
      } catch (err) {
        setError(err.message);
      }
    };
    load();
  }, [role, isHR, isManager, jobFilter]);

  const exportDashboard = async () => {
    try {
      const path = isHR
        ? '/reports/dashboard/hr/export'
        : isManager
          ? '/reports/dashboard/manager/export'
          : '/reports/dashboard/employee/export';
      const result = await api.download(path);
      downloadBlob(result);
    } catch (err) {
      setError(err.message);
    }
  };

  return (
    <section className="page">
      <header className="page-header">
        <div>
          <h2>Reports</h2>
          <p>Role-specific reporting snapshots with export-ready data.</p>
        </div>
        <button type="button" onClick={exportDashboard}>Export CSV</button>
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

      {isHR && (
        <div className="card">
          <h3>Job runs</h3>
          <div className="row-actions">
            <select value={jobFilter} onChange={(e) => setJobFilter(e.target.value)}>
              <option value="">All job types</option>
              <option value="leave_accrual">Leave accrual</option>
              <option value="gdpr_retention">GDPR retention</option>
              <option value="payroll_run">Payroll run</option>
            </select>
          </div>
          <div className="table">
            <div className="table-row header">
              <span>Job</span>
              <span>Status</span>
              <span>Started</span>
              <span>Completed</span>
            </div>
            {jobRuns.map((run) => (
              <div key={run.id} className="table-row">
                <span>{run.jobType}</span>
                <span>{run.status}</span>
                <span>{run.startedAt?.slice(0, 10)}</span>
                <span>{run.completedAt ? run.completedAt.slice(0, 10) : '—'}</span>
              </div>
            ))}
          </div>
        </div>
      )}
    </section>
  );
}
