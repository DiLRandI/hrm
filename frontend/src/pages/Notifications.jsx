import React, { useEffect, useState } from 'react';
import { api } from '../api.js';

export default function Notifications() {
  const [items, setItems] = useState([]);
  const [error, setError] = useState('');

  const load = async () => {
    try {
      const data = await api.get('/notifications');
      setItems(data);
    } catch (err) {
      setError(err.message);
    }
  };

  useEffect(() => {
    load();
  }, []);

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
            </div>
            <small>{item.createdAt?.slice(0, 10)}</small>
          </div>
        ))}
      </div>
    </section>
  );
}
