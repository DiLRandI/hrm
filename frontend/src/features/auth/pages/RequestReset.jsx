import React, { useState } from 'react';
import { Link } from 'react-router-dom';
import { api } from '../../../services/apiClient.js';

export default function RequestReset() {
  const [email, setEmail] = useState('');
  const [status, setStatus] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  const handleSubmit = async (e) => {
    e.preventDefault();
    setLoading(true);
    setError('');
    setStatus('');
    try {
      await api.post('/auth/request-reset', { email });
      setStatus('If the account exists, a reset link has been sent.');
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="login-page">
      <div className="login-card">
        <h1>Reset password</h1>
        <p>Enter your email to receive a password reset link.</p>
        <form onSubmit={handleSubmit}>
          <label>
            Email
            <input type="email" value={email} onChange={(e) => setEmail(e.target.value)} required />
          </label>
          {error && <div className="error">{error}</div>}
          {status && <div className="hint">{status}</div>}
          <button type="submit" disabled={loading}>{loading ? 'Sending...' : 'Send reset link'}</button>
        </form>
        <div className="auth-links">
          <Link to="/login">Back to sign in</Link>
        </div>
      </div>
    </div>
  );
}
