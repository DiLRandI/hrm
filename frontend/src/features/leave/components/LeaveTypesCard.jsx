import React from 'react';

export default function LeaveTypesCard({ types, form, onChange, onSubmit, disabled }) {
  return (
    <div className="card">
      <h3>Leave Types</h3>
      <form className="inline-form" onSubmit={onSubmit} aria-label="Add leave type">
        <input
          aria-label="Name"
          placeholder="Name"
          value={form.name}
          onChange={(e) => onChange('name', e.target.value)}
        />
        <input
          aria-label="Code"
          placeholder="Code"
          value={form.code}
          onChange={(e) => onChange('code', e.target.value)}
        />
        <select
          aria-label="Paid status"
          value={form.isPaid ? 'paid' : 'unpaid'}
          onChange={(e) => onChange('isPaid', e.target.value === 'paid')}
        >
          <option value="paid">Paid</option>
          <option value="unpaid">Unpaid</option>
        </select>
        <select
          aria-label="Documentation required"
          value={form.requiresDoc ? 'yes' : 'no'}
          onChange={(e) => onChange('requiresDoc', e.target.value === 'yes')}
        >
          <option value="no">No doc</option>
          <option value="yes">Requires doc</option>
        </select>
        <button type="submit" disabled={disabled}>Add type</button>
      </form>
      <div className="table">
        <div className="table-row header">
          <span>Name</span>
          <span>Code</span>
          <span>Paid</span>
        </div>
        {types.map((type) => (
          <div key={type.id} className="table-row">
            <span>{type.name}</span>
            <span>{type.code}</span>
            <span>{type.isPaid ? 'Yes' : 'No'}</span>
          </div>
        ))}
      </div>
    </div>
  );
}
