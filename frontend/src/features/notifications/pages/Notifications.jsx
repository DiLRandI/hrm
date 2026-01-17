import React, { useEffect, useState } from 'react';
import { api } from '../../../services/apiClient.js';

export default function Notifications() {
  const [items, setItems] = useState([]);
  const [error, setError] = useState('');

  const load = async () => {
    try {
      const data = await api.get('/notifications');
      setItems(Array.isArray(data) ? data : []);
    } catch (err) {
      setError(err.message);
    }
  };

  useEffect(() => {
    load();
  }, []);

  const markRead = async (id) => {
    try {
      await api.post(`/notifications/${id}/read`, {});
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
