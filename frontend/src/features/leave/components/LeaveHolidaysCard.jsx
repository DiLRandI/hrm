import React from 'react';

export default function LeaveHolidaysCard({ holidays, form, onChange, onSubmit, onDelete, disabled }) {
  return (
    <div className="card">
      <h3>Holidays</h3>
      <form className="inline-form" onSubmit={onSubmit} aria-label="Add holiday">
        <input
          aria-label="Date"
          type="date"
          value={form.date}
          onChange={(e) => onChange('date', e.target.value)}
        />
        <input
          aria-label="Name"
          placeholder="Name"
          value={form.name}
          onChange={(e) => onChange('name', e.target.value)}
        />
        <input
          aria-label="Region"
          placeholder="Region"
          value={form.region}
          onChange={(e) => onChange('region', e.target.value)}
        />
        <button type="submit" disabled={disabled}>Add holiday</button>
      </form>
      <div className="table">
        <div className="table-row header">
          <span>Date</span>
          <span>Name</span>
          <span>Region</span>
          <span>Actions</span>
        </div>
        {holidays.map((holiday) => (
          <div key={holiday.id} className="table-row">
            <span>{holiday.date?.slice(0, 10)}</span>
            <span>{holiday.name}</span>
            <span>{holiday.region || '-'}</span>
            <span>
              <button type="button" className="ghost" onClick={() => onDelete(holiday.id)} disabled={disabled}>
                Delete
              </button>
            </span>
          </div>
        ))}
      </div>
    </div>
  );
}
