import React, { useEffect, useState } from 'react';
import { api } from '../../../services/apiClient.js';
import { useAuth } from '../../auth/auth.jsx';
import { ROLE_HR } from '../../../shared/constants/roles.js';
import { GDPR_DATA_CATEGORIES } from '../../../shared/constants/gdpr.js';
import { ANONYMIZATION_STATUS_REQUESTED } from '../../../shared/constants/statuses.js';

const downloadBlob = ({ blob, filename }) => {
  const url = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  link.download = filename;
  link.click();
  URL.revokeObjectURL(url);
};

export default function GDPR() {
  const { user } = useAuth();
  const isHR = (user?.role || user?.RoleName) === ROLE_HR;

  const [dsars, setDsars] = useState([]);
  const [retentionPolicies, setRetentionPolicies] = useState([]);
  const [retentionRuns, setRetentionRuns] = useState([]);
  const [anonymizationJobs, setAnonymizationJobs] = useState([]);
  const [accessLogs, setAccessLogs] = useState([]);
  const [error, setError] = useState('');
  const [message, setMessage] = useState('');

  const [dsarEmployeeId, setDsarEmployeeId] = useState('');
  const [retentionForm, setRetentionForm] = useState({ dataCategory: 'leave', retentionDays: '' });
  const [retentionRun, setRetentionRun] = useState({ dataCategory: '' });
  const [anonymizeForm, setAnonymizeForm] = useState({ employeeId: '', reason: '' });

  const load = async () => {
    setError('');
    try {
      const requests = [api.get('/gdpr/dsar')];
      if (isHR) {
        requests.push(
          api.get('/gdpr/retention-policies'),
          api.get('/gdpr/retention/runs'),
          api.get('/gdpr/anonymize'),
          api.get('/gdpr/access-logs')
        );
      }
      const results = await Promise.allSettled(requests);
      if (results[0]?.status === 'fulfilled') {
        setDsars(Array.isArray(results[0].value) ? results[0].value : []);
      }
      if (isHR) {
        if (results[1]?.status === 'fulfilled') {
          setRetentionPolicies(Array.isArray(results[1].value) ? results[1].value : []);
        }
        if (results[2]?.status === 'fulfilled') {
          setRetentionRuns(Array.isArray(results[2].value) ? results[2].value : []);
        }
        if (results[3]?.status === 'fulfilled') {
          setAnonymizationJobs(Array.isArray(results[3].value) ? results[3].value : []);
        }
        if (results[4]?.status === 'fulfilled') {
          setAccessLogs(Array.isArray(results[4].value) ? results[4].value : []);
        }
      }
      const failure = results.find((res) => res.status === 'rejected');
      if (failure) {
        setError(failure.reason?.message || 'Failed to load GDPR data');
      }
    } catch (err) {
      setError(err.message);
    }
  };

  useEffect(() => {
    load();
  }, [isHR]);

  const requestExport = async (e) => {
    e.preventDefault();
    setError('');
    try {
      await api.post('/gdpr/dsar', { employeeId: dsarEmployeeId });
      setDsarEmployeeId('');
      await load();
    } catch (err) {
      setError(err.message);
    }
  };

  const downloadExport = async (exportId) => {
    try {
      const result = await api.download(`/gdpr/dsar/${exportId}/download`);
      downloadBlob(result);
    } catch (err) {
      setError(err.message);
    }
  };

  const saveRetentionPolicy = async (e) => {
    e.preventDefault();
    setError('');
    try {
      await api.post('/gdpr/retention-policies', {
        dataCategory: retentionForm.dataCategory,
        retentionDays: Number(retentionForm.retentionDays || 0),
      });
      setRetentionForm({ dataCategory: 'leave', retentionDays: '' });
      await load();
    } catch (err) {
      setError(err.message);
    }
  };

  const runRetention = async (e) => {
    e.preventDefault();
    setMessage('');
    try {
      const response = await api.post('/gdpr/retention/run', {
        dataCategory: retentionRun.dataCategory,
      });
      setMessage(`Retention run completed for ${response.length || 0} categories.`);
      setRetentionRun({ dataCategory: '' });
      await load();
    } catch (err) {
      setError(err.message);
    }
  };

  const requestAnonymization = async (e) => {
    e.preventDefault();
    try {
      await api.post('/gdpr/anonymize', anonymizeForm);
      setAnonymizeForm({ employeeId: '', reason: '' });
      await load();
    } catch (err) {
      setError(err.message);
    }
  };

  const executeAnonymization = async (jobId) => {
    try {
      await api.post(`/gdpr/anonymize/${jobId}/execute`, {});
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
          <p>DSAR exports, retention policies, anonymization, and access logs.</p>
        </div>
      </header>

      {error && <div className="error">{error}</div>}
      {message && <div className="success">{message}</div>}

      <div className="card-grid">
        <div className="card">
          <h3>DSAR exports</h3>
          <form className="inline-form" onSubmit={requestExport}>
            <input
              placeholder="Employee ID (optional)"
              value={dsarEmployeeId}
              onChange={(e) => setDsarEmployeeId(e.target.value)}
            />
            <button type="submit">Request export</button>
          </form>
          <div className="table">
            <div className="table-row header">
              <span>Employee</span>
              <span>Status</span>
              <span>Actions</span>
            </div>
            {dsars.map((dsar) => (
              <div key={dsar.id} className="table-row">
                <span>{dsar.employeeId}</span>
                <span>{dsar.status}</span>
                <span className="row-actions">
                  {dsar.fileUrl ? (
                    <button onClick={() => downloadExport(dsar.id)}>Download</button>
                  ) : (
                    <small>Pending</small>
                  )}
                </span>
              </div>
            ))}
          </div>
        </div>

        {isHR && (
          <>
            <div className="card">
              <h3>Retention policies</h3>
              <form className="stack" onSubmit={saveRetentionPolicy}>
                <select
                  value={retentionForm.dataCategory}
                  onChange={(e) => setRetentionForm({ ...retentionForm, dataCategory: e.target.value })}
                >
                  {GDPR_DATA_CATEGORIES.map((category) => (
                    <option key={category.value} value={category.value}>{category.label}</option>
                  ))}
                </select>
                <input
                  type="number"
                  min="0"
                  placeholder="Retention days"
                  value={retentionForm.retentionDays}
                  onChange={(e) => setRetentionForm({ ...retentionForm, retentionDays: e.target.value })}
                />
                <button type="submit">Save policy</button>
              </form>
              <div className="table">
                <div className="table-row header">
                  <span>Category</span>
                  <span>Days</span>
                </div>
                {retentionPolicies.map((policy) => (
                  <div key={policy.id} className="table-row">
                    <span>{policy.dataCategory}</span>
                    <span>{policy.retentionDays}</span>
                  </div>
                ))}
              </div>
            </div>

            <div className="card">
              <h3>Retention runs</h3>
              <form className="stack" onSubmit={runRetention}>
                <select
                  value={retentionRun.dataCategory}
                  onChange={(e) => setRetentionRun({ dataCategory: e.target.value })}
                >
                  <option value="">All categories</option>
                  {GDPR_DATA_CATEGORIES.map((category) => (
                    <option key={category.value} value={category.value}>{category.label}</option>
                  ))}
                </select>
                <button type="submit">Run retention</button>
              </form>
              <div className="table">
                <div className="table-row header">
                  <span>Category</span>
                  <span>Status</span>
                  <span>Deleted</span>
                </div>
                {retentionRuns.map((run) => (
                  <div key={run.id} className="table-row">
                    <span>{run.dataCategory}</span>
                    <span>{run.status}</span>
                    <span>{run.deletedCount}</span>
                  </div>
                ))}
              </div>
            </div>

            <div className="card">
              <h3>Anonymization</h3>
              <form className="stack" onSubmit={requestAnonymization}>
                <input
                  placeholder="Employee ID"
                  value={anonymizeForm.employeeId}
                  onChange={(e) => setAnonymizeForm({ ...anonymizeForm, employeeId: e.target.value })}
                  required
                />
                <input
                  placeholder="Reason"
                  value={anonymizeForm.reason}
                  onChange={(e) => setAnonymizeForm({ ...anonymizeForm, reason: e.target.value })}
                />
                <button type="submit">Request anonymization</button>
              </form>
              <div className="table">
                <div className="table-row header">
                  <span>Employee</span>
                  <span>Status</span>
                  <span>Actions</span>
                </div>
                {anonymizationJobs.map((job) => (
                  <div key={job.id} className="table-row">
                    <span>{job.employeeId}</span>
                    <span>{job.status}</span>
                    <span className="row-actions">
                      {job.status === ANONYMIZATION_STATUS_REQUESTED && (
                        <button onClick={() => executeAnonymization(job.id)}>Execute</button>
                      )}
                    </span>
                  </div>
                ))}
              </div>
            </div>

            <div className="card">
              <h3>Access logs</h3>
              <div className="table">
                <div className="table-row header">
                  <span>Actor</span>
                  <span>Employee</span>
                  <span>Fields</span>
                </div>
                {accessLogs.map((log) => (
                  <div key={log.id} className="table-row">
                    <span>{log.actorUserId}</span>
                    <span>{log.employeeId}</span>
                    <span>{Array.isArray(log.fields) ? log.fields.join(', ') : ''}</span>
                  </div>
                ))}
              </div>
            </div>
          </>
        )}
      </div>
    </section>
  );
}
