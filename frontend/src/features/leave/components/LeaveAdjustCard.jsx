import React from 'react';

export default function LeaveAdjustCard({ types, form, onChange, onSubmit, disabled }) {
  return (
    <div className="card">
      <h3>Adjust Balance</h3>
      <form className="inline-form" onSubmit={onSubmit} aria-label="Adjust leave balance">
        <input
          aria-label="Employee ID"
          placeholder="Employee ID"
          value={form.employeeId}
          onChange={(e) => onChange('employeeId', e.target.value)}
        />
        <select
          aria-label="Leave type"
          value={form.leaveTypeId}
          onChange={(e) => onChange('leaveTypeId', e.target.value)}
        >
          <option value="">Leave type</option>
          {types.map((type) => (
            <option key={type.id} value={type.id}>{type.name}</option>
          ))}
        </select>
        <input
          aria-label="Delta"
          placeholder="Delta"
          value={form.delta}
          onChange={(e) => onChange('delta', e.target.value)}
        />
        <input
          aria-label="Reason"
          placeholder="Reason"
          value={form.reason}
          onChange={(e) => onChange('reason', e.target.value)}
        />
        <button type="submit" disabled={disabled}>Apply</button>
      </form>
    </div>
  );
}
