import React from 'react';

export default function EmployeeEditForm({
  employeeOptions,
  departments,
  payGroups,
  editEmployee,
  onSelectEmployee,
  onFieldChange,
  onSubmit,
  disabled,
}) {
  return (
    <form className="inline-form" onSubmit={onSubmit} aria-label="Edit employee">
      <select
        aria-label="Select employee"
        value={editEmployee?.id || ''}
        onChange={(e) => onSelectEmployee(e.target.value)}
      >
        <option value="">Select employee to edit</option>
        {employeeOptions.map((emp) => (
          <option key={emp.id} value={emp.id}>
            {emp.firstName} {emp.lastName}
          </option>
        ))}
      </select>
      <select
        aria-label="Department"
        value={editEmployee?.departmentId || ''}
        onChange={(e) => onFieldChange('departmentId', e.target.value)}
      >
        <option value="">Department</option>
        {departments.map((dep) => (
          <option key={dep.id} value={dep.id}>
            {dep.name}
          </option>
        ))}
      </select>
      <select
        aria-label="Manager"
        value={editEmployee?.managerId || ''}
        onChange={(e) => onFieldChange('managerId', e.target.value)}
      >
        <option value="">Manager</option>
        {employeeOptions.map((emp) => (
          <option key={emp.id} value={emp.id}>
            {emp.firstName} {emp.lastName}
          </option>
        ))}
      </select>
      <select
        aria-label="Pay group"
        value={editEmployee?.payGroupId || ''}
        onChange={(e) => onFieldChange('payGroupId', e.target.value)}
      >
        <option value="">Pay group</option>
        {payGroups.map((group) => (
          <option key={group.id} value={group.id}>
            {group.name}
          </option>
        ))}
      </select>
      <button type="submit" disabled={disabled || !editEmployee?.id}>Save changes</button>
    </form>
  );
}
