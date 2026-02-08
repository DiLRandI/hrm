import React, { useMemo, useState } from 'react';
import { Link, useNavigate, useSearchParams } from 'react-router-dom';
import { api } from '../../../services/apiClient.js';

const PASSWORD_RULES = {
  minLength: 'At least 10 characters',
  uppercase: 'At least 1 uppercase letter',
  lowercase: 'At least 1 lowercase letter',
  number: 'At least 1 number',
};

function evaluatePassword(password) {
  return {
    minLength: password.length >= 10,
    uppercase: /[A-Z]/.test(password),
    lowercase: /[a-z]/.test(password),
    number: /[0-9]/.test(password),
  };
}

export default function ResetPassword() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const tokenFromUrl = useMemo(() => searchParams.get('token') || '', [searchParams]);
  const [token, setToken] = useState(tokenFromUrl);
  const [useCustomToken, setUseCustomToken] = useState(false);
  const [newPassword, setNewPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [error, setError] = useState('');
  const [status, setStatus] = useState('');
  const [loading, setLoading] = useState(false);
  const passwordChecks = useMemo(() => evaluatePassword(newPassword), [newPassword]);
  const passwordValid = useMemo(() => Object.values(passwordChecks).every(Boolean), [passwordChecks]);
  const tokenLocked = Boolean(tokenFromUrl) && !useCustomToken;

  const toggleCustomToken = () => {
    if (useCustomToken) {
      setToken(tokenFromUrl);
    }
    setUseCustomToken((prev) => !prev);
  };

  const handleSubmit = async (e) => {
    e.preventDefault();
    setError('');
    setStatus('');
    if (!token.trim()) {
      setError('Reset token is required');
      return;
    }
    if (!passwordValid) {
      setError('Password must be at least 10 characters and include uppercase, lowercase, and a number.');
      return;
    }
    if (newPassword !== confirmPassword) {
      setError('Passwords do not match');
      return;
    }
    setLoading(true);
    try {
      await api.post('/auth/reset', { token, newPassword });
      setStatus('Password updated. You can sign in now.');
      setTimeout(() => navigate('/login'), 800);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="login-page">
      <div className="login-card">
        <h1>Set a new password</h1>
        <p>
          {tokenFromUrl
            ? 'Your reset link token is already loaded. Set your new password below.'
            : 'Enter the reset token and choose a new password.'}
        </p>
        <form onSubmit={handleSubmit}>
          <label>
            Reset token
            <input
              type="text"
              value={token}
              onChange={(e) => setToken(e.target.value)}
              required
              disabled={tokenLocked}
            />
          </label>
          {tokenFromUrl && (
            <div className="row-actions">
              <small className="hint">
                {tokenLocked ? 'Using token from reset link.' : 'Using custom token.'}
              </small>
              <button type="button" className="ghost" onClick={toggleCustomToken}>
                {tokenLocked ? 'Use different token' : 'Use link token'}
              </button>
            </div>
          )}
          <label>
            New password
            <input type="password" value={newPassword} onChange={(e) => setNewPassword(e.target.value)} required />
          </label>
          <div className="hint" role="status">
            Password requirements:
            <ul>
              {Object.entries(PASSWORD_RULES).map(([key, label]) => (
                <li key={key}>
                  {passwordChecks[key] ? 'OK' : 'Required'}: {label}
                </li>
              ))}
            </ul>
          </div>
          <label>
            Confirm password
            <input type="password" value={confirmPassword} onChange={(e) => setConfirmPassword(e.target.value)} required />
          </label>
          {error && <div className="error">{error}</div>}
          {status && <div className="hint">{status}</div>}
          <button type="submit" disabled={loading}>{loading ? 'Updating...' : 'Update password'}</button>
        </form>
        <div className="auth-links">
          <Link to="/login">Back to sign in</Link>
        </div>
      </div>
    </div>
  );
}
