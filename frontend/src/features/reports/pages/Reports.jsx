import React, { useEffect, useMemo, useState } from 'react';
import { NavLink, Navigate, Route, Routes, useLocation } from 'react-router-dom';
import { api } from '../../../services/apiClient.js';
import { useAuth } from '../../auth/auth.jsx';
import { ROLE_HR, ROLE_MANAGER } from '../../../shared/constants/roles.js';
import { getRole } from '../../../shared/utils/role.js';

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
  const role = getRole(user);
  const isHR = role === ROLE_HR;
  const isManager = role === ROLE_MANAGER;
  const location = useLocation();
  const activeSection = useMemo(() => {
    const segment = location.pathname.split('/')[2];
    return segment || 'overview';
  }, [location.pathname]);
  const [data, setData] = useState(null);
  const [jobRuns, setJobRuns] = useState([]);
  const [jobRunsTotal, setJobRunsTotal] = useState(0);
  const [jobFilter, setJobFilter] = useState('');
  const [statusFilter, setStatusFilter] = useState('');
  const [startedFrom, setStartedFrom] = useState('');
  const [startedTo, setStartedTo] = useState('');
  const [selectedRun, setSelectedRun] = useState(null);
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
        if (isHR && activeSection === 'jobs') {
          const params = new URLSearchParams();
          if (jobFilter) params.set('jobType', jobFilter);
          if (statusFilter) params.set('status', statusFilter);
          if (startedFrom) params.set('startedFrom', startedFrom);
          if (startedTo) params.set('startedTo', startedTo);
          const query = params.toString();
          const result = await api.getWithMeta(`/reports/jobs${query ? `?${query}` : ''}`);
          const runs = Array.isArray(result?.data) ? result.data : [];
          setJobRuns(runs);
          setJobRunsTotal(result?.total ?? runs.length);
          setSelectedRun(null);
        }
      } catch (err) {
        setError(err.message);
      }
    };
    load();
  }, [role, isHR, isManager, jobFilter, statusFilter, startedFrom, startedTo, activeSection]);

  const viewRunDetails = async (runId) => {
    try {
      setSelectedRun(await api.get(`/reports/jobs/${runId}`));
    } catch (err) {
      setError(err.message);
    }
  };

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

  const summarizeRunDetails = (details) => {
    if (!details || typeof details !== 'object') return '—';
    if (typeof details.error === 'string' && details.error) return details.error;
    if (typeof details.deleted === 'number') return `Deleted ${details.deleted}`;
    if (typeof details.processed === 'number') return `Processed ${details.processed}`;
    const asText = JSON.stringify(details);
    if (!asText || asText === '{}') return '—';
    return asText.length > 80 ? `${asText.slice(0, 80)}…` : asText;
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

      <nav className="subnav">
        <NavLink to="/reports/overview">Overview</NavLink>
        {isHR && <NavLink to="/reports/jobs">Job runs</NavLink>}
      </nav>

      <Routes>
        <Route path="/" element={<Navigate to="overview" replace />} />
        <Route
          path="overview"
          element={
            <div className="card-grid">
              {data &&
                Object.entries(data).map(([key, value]) => (
                  <div key={key} className="card">
                    <h3>{key}</h3>
                    <p className="metric">{value}</p>
                  </div>
                ))}
              {isHR && (
                <div className="card">
                  <h3>Quick actions</h3>
                  <div className="row-actions">
                    <NavLink className="ghost-link" to="/reports/jobs">View job runs</NavLink>
                  </div>
                </div>
              )}
            </div>
          }
        />
        {isHR && (
          <Route
            path="jobs"
            element={
              <div className="card">
                <h3>Job runs</h3>
                <div className="row-actions">
                  <select value={jobFilter} onChange={(e) => setJobFilter(e.target.value)}>
                    <option value="">All job types</option>
                    <option value="leave_accrual">Leave accrual</option>
                    <option value="gdpr_retention">GDPR retention</option>
                    <option value="payroll_run">Payroll run</option>
                  </select>
                  <select value={statusFilter} onChange={(e) => setStatusFilter(e.target.value)}>
                    <option value="">All statuses</option>
                    <option value="running">Running</option>
                    <option value="completed">Completed</option>
                    <option value="failed">Failed</option>
                  </select>
                  <input type="date" value={startedFrom} onChange={(e) => setStartedFrom(e.target.value)} aria-label="Started from" />
                  <input type="date" value={startedTo} onChange={(e) => setStartedTo(e.target.value)} aria-label="Started to" />
                  <button
                    type="button"
                    onClick={() => {
                      setJobFilter('');
                      setStatusFilter('');
                      setStartedFrom('');
                      setStartedTo('');
                    }}
                  >
                    Clear
                  </button>
                </div>
                <p>{jobRunsTotal} run(s)</p>
                <div className="table">
                  <div className="table-row header">
                    <span>Job</span>
                    <span>Status</span>
                    <span>Started</span>
                    <span>Completed</span>
                    <span>Details</span>
                    <span>Actions</span>
                  </div>
                  {jobRuns.map((run) => (
                    <div key={run.id} className="table-row">
                      <span>{run.jobType}</span>
                      <span>{run.status}</span>
                      <span>{run.startedAt?.slice(0, 10)}</span>
                      <span>{run.completedAt ? run.completedAt.slice(0, 10) : '—'}</span>
                      <span>{summarizeRunDetails(run.details)}</span>
                      <span>
                        <button type="button" onClick={() => viewRunDetails(run.id)}>View</button>
                      </span>
                    </div>
                  ))}
                </div>
                {selectedRun && (
                  <div className="card">
                    <h4>Run details: {selectedRun.id}</h4>
                    <pre>{JSON.stringify(selectedRun.details || {}, null, 2)}</pre>
                  </div>
                )}
              </div>
            }
          />
        )}
        <Route path="*" element={<Navigate to="overview" replace />} />
      </Routes>
    </section>
  );
}
