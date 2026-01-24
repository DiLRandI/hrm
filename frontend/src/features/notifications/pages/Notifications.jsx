import React, { useEffect, useState } from 'react';
import { api } from '../../../services/apiClient.js';
import { useAuth } from '../../auth/auth.jsx';
import { ROLE_HR } from '../../../shared/constants/roles.js';
import { getRole } from '../../../shared/utils/role.js';

export default function Notifications() {
  const { user } = useAuth();
  const isHR = getRole(user) === ROLE_HR;
  const [items, setItems] = useState([]);
  const [settings, setSettings] = useState(null);
  const [settingsForm, setSettingsForm] = useState({ emailEnabled: false, emailFrom: '' });
  const [message, setMessage] = useState('');
  const [error, setError] = useState('');

  const load = async () => {
    try {
      const data = await api.get('/notifications');
      setItems(Array.isArray(data) ? data : []);
      if (isHR) {
        const settingsData = await api.get('/notifications/settings');
        setSettings(settingsData);
        setSettingsForm({
          emailEnabled: Boolean(settingsData?.emailEnabled),
          emailFrom: settingsData?.emailFrom || '',
        });
      }
    } catch (err) {
      setError(err.message);
    }
  };

  useEffect(() => {
    load();
  }, [isHR]);

  const markRead = async (id) => {
    try {
      await api.post(`/notifications/${id}/read`, {});
      await load();
    } catch (err) {
      setError(err.message);
    }
  };

  const saveSettings = async (e) => {
    e.preventDefault();
    setMessage('');
    try {
      await api.put('/notifications/settings', settingsForm);
      setMessage('Notification settings saved.');
      await load();
    } catch (err) {
      setError(err.message);
    }
  };

  return (
    <section className="page">
      <header className="page-header">
        <div>
          <h2>Notifications</h2>
          <p>All workflow alerts and reminders.</p>
        </div>
      </header>

      {error && <div className="error">{error}</div>}
      {message && <div className="success">{message}</div>}

      {isHR && (
        <div className="card">
          <h3>Email notifications</h3>
          <form className="stack" onSubmit={saveSettings}>
            <label className="checkbox">
              <input
                type="checkbox"
                checked={settingsForm.emailEnabled}
                onChange={(e) => setSettingsForm({ ...settingsForm, emailEnabled: e.target.checked })}
              />
              Enable tenant email notifications
            </label>
            <input
              placeholder="From address"
              value={settingsForm.emailFrom}
              onChange={(e) => setSettingsForm({ ...settingsForm, emailFrom: e.target.value })}
            />
            <button type="submit">Save settings</button>
          </form>
          {settings && (
            <small className="hint">Current status: {settings.emailEnabled ? 'Enabled' : 'Disabled'}</small>
          )}
        </div>
      )}

      <div className="list">
        {items.map((item) => (
          <div key={item.id} className="list-item">
            <div>
              <strong>{item.title}</strong>
              <p>{item.body}</p>
              <small>{item.readAt ? 'Read' : 'Unread'}</small>
            </div>
            <div className="row-actions">
              <small>{item.createdAt?.slice(0, 10)}</small>
              {!item.readAt && <button onClick={() => markRead(item.id)}>Mark read</button>}
            </div>
          </div>
        ))}
      </div>
    </section>
  );
}
