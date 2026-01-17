import React, { useEffect, useState } from 'react';
import { useAuth } from '../../auth/auth.jsx';
import { api } from '../../../services/apiClient.js';
import { ROLE_HR, ROLE_MANAGER } from '../../../shared/constants/roles.js';

export default function Dashboard() {
  const { user, employee, refresh } = useAuth();
  const [data, setData] = useState(null);
  const [error, setError] = useState('');
  const [mfaSecret, setMfaSecret] = useState('');
  const [mfaUrl, setMfaUrl] = useState('');
  const [mfaCode, setMfaCode] = useState('');
  const [mfaMessage, setMfaMessage] = useState('');

  useEffect(() => {
    const load = async () => {
      try {
        if ((user?.role || user?.RoleName) === ROLE_HR) {
          setData(await api.get('/reports/dashboard/hr'));
        } else if ((user?.role || user?.RoleName) === ROLE_MANAGER) {
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
