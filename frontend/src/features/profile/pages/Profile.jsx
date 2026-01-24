import React, { useEffect, useMemo, useState } from 'react';
import { NavLink, Navigate, Route, Routes } from 'react-router-dom';
import { api } from '../../../services/apiClient.js';
import { useAuth } from '../../auth/auth.jsx';
import { InlineError } from '../../../shared/components/PageStatus.jsx';

const emptyContact = () => ({
  fullName: '',
  relationship: '',
  phone: '',
  email: '',
  address: '',
  isPrimary: false,
});

export default function Profile() {
  const { user, employee, refresh } = useAuth();
  const [error, setError] = useState('');
  const [message, setMessage] = useState('');
  const [saving, setSaving] = useState(false);
  const [contacts, setContacts] = useState([]);
  const [contactsLoading, setContactsLoading] = useState(false);

  const [form, setForm] = useState({
    preferredName: '',
    pronouns: '',
    personalEmail: '',
    phone: '',
    address: '',
    dateOfBirth: '',
  });

  const [mfaSecret, setMfaSecret] = useState('');
  const [mfaUrl, setMfaUrl] = useState('');
  const [mfaCode, setMfaCode] = useState('');
  const [mfaMessage, setMfaMessage] = useState('');

  useEffect(() => {
    if (!employee) {
      return;
    }
    setForm({
      preferredName: employee.preferredName || '',
      pronouns: employee.pronouns || '',
      personalEmail: employee.personalEmail || '',
      phone: employee.phone || '',
      address: employee.address || '',
      dateOfBirth: employee.dateOfBirth ? employee.dateOfBirth.slice(0, 10) : '',
    });
  }, [employee]);

  const loadContacts = async () => {
    setContactsLoading(true);
    setError('');
    try {
      const data = await api.get('/profile/emergency-contacts');
      setContacts(Array.isArray(data) ? data : []);
    } catch (err) {
      setError(err.message);
    } finally {
      setContactsLoading(false);
    }
  };

  useEffect(() => {
    loadContacts();
  }, []);

  const profilePayload = useMemo(() => {
    if (!employee) {
      return null;
    }
    return {
      ...employee,
      preferredName: form.preferredName,
      pronouns: form.pronouns,
      personalEmail: form.personalEmail,
      phone: form.phone,
      address: form.address,
      dateOfBirth: form.dateOfBirth ? form.dateOfBirth : null,
    };
  }, [employee, form]);

  const saveProfile = async (e) => {
    e.preventDefault();
    if (!employee || !profilePayload) {
      return;
    }
    setSaving(true);
    setMessage('');
    setError('');
    try {
      await api.put(`/employees/${employee.id}`, profilePayload);
      await refresh();
      setMessage('Profile updated.');
    } catch (err) {
      setError(err.message);
    } finally {
      setSaving(false);
    }
  };

  const saveContacts = async () => {
    setSaving(true);
    setMessage('');
    setError('');
    try {
      await api.put('/profile/emergency-contacts', { contacts });
      await loadContacts();
      setMessage('Emergency contacts updated.');
    } catch (err) {
      setError(err.message);
    } finally {
      setSaving(false);
    }
  };

  return (
    <section className="page">
      <header className="page-header">
        <div>
          <h2>Profile</h2>
          <p>Manage your personal details, emergency contacts, and security.</p>
        </div>
      </header>

      <InlineError message={error} />
      {message && <div className="success">{message}</div>}

      <nav className="subnav">
        <NavLink to="/profile/overview">Overview</NavLink>
        <NavLink to="/profile/personal">Personal</NavLink>
        <NavLink to="/profile/contact">Contact</NavLink>
        <NavLink to="/profile/emergency">Emergency</NavLink>
        <NavLink to="/profile/security">Security</NavLink>
      </nav>

      <Routes>
        <Route path="/" element={<Navigate to="overview" replace />} />
        <Route
          path="overview"
          element={
            <div className="card-grid">
              <div className="card">
                <h3>Name</h3>
                <p className="metric">{employee ? `${employee.firstName} ${employee.lastName}` : '—'}</p>
                <p className="inline-note">{employee?.preferredName ? `Preferred: ${employee.preferredName}` : '—'}</p>
              </div>
              <div className="card">
                <h3>Contact</h3>
                <p className="metric">{employee?.phone || '—'}</p>
                <p className="inline-note">{employee?.personalEmail || employee?.email || '—'}</p>
              </div>
              <div className="card">
                <h3>Quick actions</h3>
                <div className="row-actions">
                  <NavLink className="ghost-link" to="/profile/personal">Update details</NavLink>
                  <NavLink className="ghost-link" to="/profile/emergency">Emergency contacts</NavLink>
                </div>
              </div>
            </div>
          }
        />
        <Route
          path="personal"
          element={
            <form className="stack card" onSubmit={saveProfile}>
              <h3>Personal details</h3>
              <input
                placeholder="Preferred name"
                value={form.preferredName}
                onChange={(e) => setForm({ ...form, preferredName: e.target.value })}
              />
              <input
                placeholder="Pronouns"
                value={form.pronouns}
                onChange={(e) => setForm({ ...form, pronouns: e.target.value })}
              />
              <input
                type="date"
                value={form.dateOfBirth}
                onChange={(e) => setForm({ ...form, dateOfBirth: e.target.value })}
              />
              <button type="submit" disabled={saving}>Save personal</button>
            </form>
          }
        />
        <Route
          path="contact"
          element={
            <form className="stack card" onSubmit={saveProfile}>
              <h3>Contact info</h3>
              <input
                placeholder="Personal email"
                value={form.personalEmail}
                onChange={(e) => setForm({ ...form, personalEmail: e.target.value })}
              />
              <input
                placeholder="Phone"
                value={form.phone}
                onChange={(e) => setForm({ ...form, phone: e.target.value })}
              />
              <input
                placeholder="Address"
                value={form.address}
                onChange={(e) => setForm({ ...form, address: e.target.value })}
              />
              <button type="submit" disabled={saving}>Save contact</button>
            </form>
          }
        />
        <Route
          path="emergency"
          element={
            <div className="card stack">
              <h3>Emergency contacts</h3>
              {contactsLoading ? (
                <p className="hint">Loading contacts…</p>
              ) : (
                contacts.map((contact, idx) => (
                  <div key={contact.id || idx} className="inline-form">
                    <input
                      placeholder="Full name"
                      value={contact.fullName || ''}
                      onChange={(e) => {
                        const next = [...contacts];
                        next[idx] = { ...next[idx], fullName: e.target.value };
                        setContacts(next);
                      }}
                    />
                    <input
                      placeholder="Relationship"
                      value={contact.relationship || ''}
                      onChange={(e) => {
                        const next = [...contacts];
                        next[idx] = { ...next[idx], relationship: e.target.value };
                        setContacts(next);
                      }}
                    />
                    <input
                      placeholder="Phone"
                      value={contact.phone || ''}
                      onChange={(e) => {
                        const next = [...contacts];
                        next[idx] = { ...next[idx], phone: e.target.value };
                        setContacts(next);
                      }}
                    />
                    <input
                      placeholder="Email"
                      value={contact.email || ''}
                      onChange={(e) => {
                        const next = [...contacts];
                        next[idx] = { ...next[idx], email: e.target.value };
                        setContacts(next);
                      }}
                    />
                    <input
                      placeholder="Address"
                      value={contact.address || ''}
                      onChange={(e) => {
                        const next = [...contacts];
                        next[idx] = { ...next[idx], address: e.target.value };
                        setContacts(next);
                      }}
                    />
                    <label className="checkbox">
                      <input
                        type="checkbox"
                        checked={Boolean(contact.isPrimary)}
                        onChange={(e) => {
                          const next = contacts.map((item, idy) => ({
                            ...item,
                            isPrimary: idy === idx ? e.target.checked : false,
                          }));
                          setContacts(next);
                        }}
                      />
                      Primary
                    </label>
                    <button
                      type="button"
                      className="ghost"
                      onClick={() => setContacts(contacts.filter((_, idy) => idy !== idx))}
                    >
                      Remove
                    </button>
                  </div>
                ))
              )}
              <div className="row-actions">
                <button type="button" className="ghost" onClick={() => setContacts([...contacts, emptyContact()])}>
                  Add contact
                </button>
                <button type="button" onClick={saveContacts} disabled={saving}>
                  Save contacts
                </button>
              </div>
            </div>
          }
        />
        <Route
          path="security"
          element={
            <div className="card stack">
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
          }
        />
        <Route path="*" element={<Navigate to="overview" replace />} />
      </Routes>
    </section>
  );
}
