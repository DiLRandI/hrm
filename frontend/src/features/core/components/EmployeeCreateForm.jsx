import React from 'react';
import { ROLE_EMPLOYEE, ROLE_MANAGER } from '../../../shared/constants/roles.js';

export default function EmployeeCreateForm({ form, availableRoles = [ROLE_EMPLOYEE], onChange, onSubmit, disabled }) {
  const requiresProfile = form.role === ROLE_EMPLOYEE || form.role === ROLE_MANAGER;

  return (
    <form className="inline-form" onSubmit={onSubmit} aria-label="Add employee">
      <select
        aria-label="Role"
        value={form.role}
        onChange={(e) => onChange('role', e.target.value)}
      >
        {availableRoles.map((role) => (
          <option key={role} value={role}>{role}</option>
        ))}
      </select>
      <input
        aria-label="First name"
        placeholder="First name"
        value={form.firstName}
        onChange={(e) => onChange('firstName', e.target.value)}
        required={requiresProfile}
      />
      <input
        aria-label="Last name"
        placeholder="Last name"
        value={form.lastName}
        onChange={(e) => onChange('lastName', e.target.value)}
        required={requiresProfile}
      />
      <input
        aria-label="Email"
        placeholder="Email"
        type="email"
        value={form.email}
        onChange={(e) => onChange('email', e.target.value)}
        required
      />
      <button type="submit" disabled={disabled}>Create user</button>
    </form>
  );
}
