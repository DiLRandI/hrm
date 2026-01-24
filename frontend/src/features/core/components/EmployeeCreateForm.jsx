import React from 'react';

export default function EmployeeCreateForm({ form, onChange, onSubmit, disabled }) {
  return (
    <form className="inline-form" onSubmit={onSubmit} aria-label="Add employee">
      <input
        aria-label="First name"
        placeholder="First name"
        value={form.firstName}
        onChange={(e) => onChange('firstName', e.target.value)}
      />
      <input
        aria-label="Last name"
        placeholder="Last name"
        value={form.lastName}
        onChange={(e) => onChange('lastName', e.target.value)}
      />
      <input
        aria-label="Email"
        placeholder="Email"
        type="email"
        value={form.email}
        onChange={(e) => onChange('email', e.target.value)}
      />
      <button type="submit" disabled={disabled}>Add employee</button>
    </form>
  );
}
