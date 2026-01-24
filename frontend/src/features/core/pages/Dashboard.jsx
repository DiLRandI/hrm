import React, { useCallback, useMemo, useState } from 'react';
import { useAuth } from '../../auth/auth.jsx';
import { api } from '../../../services/apiClient.js';
import { ROLE_HR, ROLE_MANAGER } from '../../../shared/constants/roles.js';
import { getRole } from '../../../shared/utils/role.js';
import { useApiQuery } from '../../../shared/hooks/useApiQuery.js';
import { InlineError, PageStatus } from '../../../shared/components/PageStatus.jsx';

export default function Dashboard() {
  const { user, employee, refresh } = useAuth();
  const [mfaSecret, setMfaSecret] = useState('');
  const [mfaUrl, setMfaUrl] = useState('');
  const [mfaCode, setMfaCode] = useState('');
  const [mfaMessage, setMfaMessage] = useState('');
  const role = getRole(user);
  const dashboardEndpoint = useMemo(() => {
    if (role === ROLE_HR) {
      return '/reports/dashboard/hr';
    }
    if (role === ROLE_MANAGER) {
      return '/reports/dashboard/manager';
    }
    return '/reports/dashboard/employee';
  }, [role]);

  const fetchDashboard = useCallback(
    ({ signal }) => api.get(dashboardEndpoint, { signal }),
    [dashboardEndpoint],
  );

  const { data, error, loading } = useApiQuery(fetchDashboard, [dashboardEndpoint], { enabled: Boolean(user) });

  return (
    <section className="page">
      <header className="page-header">
        <div>
          <h2>Welcome back, {employee?.firstName || role}</h2>
          <p>Here’s your live snapshot across leave, payroll, and performance.</p>
        </div>
      </header>

      <InlineError message={error} />

      {loading && (
        <PageStatus title="Loading dashboard" description="Fetching your latest HR snapshot." />
      )}

      {data && !loading && (
        <div className="card-grid">
          {Object.entries(data).map(([key, value]) => (
            <div key={key} className="card">
              <h3>{key}</h3>
              <p className="metric">{value}</p>
            </div>
          ))}
        </div>
      )}

      <div className="card">
        <h3>Multi-factor authentication</h3>
        <p>Protect your account with a time-based code.</p>
        {user?.mfaEnabled ? (
          <>
            <p className="status-tag success">MFA enabled</p>
            <div className="inline-form">
              <input placeholder="MFA code" value={mfaCode} onChange={(e) => setMfaCode(e.target.value)} />
              <button
                type="button"
                onClick={async () => {
                  setMfaMessage('');
                  try {
                    await api.post('/auth/mfa/disable', { code: mfaCode });
                    setMfaCode('');
                    await refresh();
                    setMfaMessage('MFA disabled.');
                  } catch (err) {
                    setMfaMessage(err.message);
                  }
                }}
              >
                Disable MFA
              </button>
            </div>
          </>
        ) : (
          <>
            <p className="status-tag">MFA not enabled</p>
            <div className="inline-form">
              <button
                type="button"
                onClick={async () => {
                  setMfaMessage('');
                  try {
                    const result = await api.post('/auth/mfa/setup', {});
                    setMfaSecret(result.secret);
                    setMfaUrl(result.otpauthUrl);
                  } catch (err) {
                    setMfaMessage(err.message);
                  }
                }}
              >
                Generate MFA secret
              </button>
              {mfaSecret && (
                <div className="inline-note">
                  <strong>Secret:</strong> {mfaSecret}
                  {mfaUrl && <div className="hint">otpauth URL: {mfaUrl}</div>}
                </div>
              )}
            </div>
            <div className="inline-form">
              <input placeholder="MFA code" value={mfaCode} onChange={(e) => setMfaCode(e.target.value)} />
              <button
                type="button"
                onClick={async () => {
                  setMfaMessage('');
                  try {
                    await api.post('/auth/mfa/enable', { code: mfaCode });
                    setMfaCode('');
                    await refresh();
                    setMfaMessage('MFA enabled.');
                  } catch (err) {
                    setMfaMessage(err.message);
                  }
                }}
              >
                Enable MFA
              </button>
            </div>
          </>
        )}
        {mfaMessage && <div className="hint">{mfaMessage}</div>}
      </div>
    </section>
  );
}
