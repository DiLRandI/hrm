import React, { useMemo } from 'react';

const formatName = (employee) => {
  if (!employee) {
    return 'N/A';
  }
  return `${employee.firstName} ${employee.lastName}`.trim();
};

export default function DepartmentsCard({
  departments,
  employees,
  form,
  editingId,
  onChange,
  onSubmit,
  onEdit,
  onCancelEdit,
  onDelete,
  disabled,
}) {
  const departmentById = useMemo(() => {
    return departments.reduce((acc, dep) => {
      acc[dep.id] = dep;
      return acc;
    }, {});
  }, [departments]);

  const employeeByUserId = useMemo(() => {
    return employees.reduce((acc, emp) => {
      if (emp.userId) {
        acc[emp.userId] = emp;
      }
      return acc;
    }, {});
  }, [employees]);

  const managerOptions = employees.filter((emp) => emp.userId);
  const parentOptions = editingId
    ? departments.filter((dep) => dep.id !== editingId)
    : departments;

  return (
    <div className="card">
      <h3>Departments</h3>
      <form className="inline-form" onSubmit={onSubmit} aria-label="Manage departments">
        <input
          aria-label="Department name"
          placeholder="Name"
          value={form.name}
          onChange={(e) => onChange('name', e.target.value)}
          required
        />
        <input
          aria-label="Department code"
          placeholder="Code"
          value={form.code}
          onChange={(e) => onChange('code', e.target.value)}
          required
        />
        <select
          aria-label="Parent department"
          value={form.parentId}
          onChange={(e) => onChange('parentId', e.target.value)}
        >
          <option value="">Parent department</option>
          {parentOptions.map((dep) => (
            <option key={dep.id} value={dep.id}>
              {dep.name}
            </option>
          ))}
        </select>
        <select
          aria-label="Department manager"
          value={form.managerId}
          onChange={(e) => onChange('managerId', e.target.value)}
        >
          <option value="">Manager</option>
          {managerOptions.map((emp) => (
            <option key={emp.userId} value={emp.userId}>
              {emp.firstName} {emp.lastName}
            </option>
          ))}
        </select>
        <button type="submit" disabled={disabled}>
          {editingId ? 'Save changes' : 'Add department'}
        </button>
        {editingId && (
          <button type="button" className="ghost" onClick={onCancelEdit} disabled={disabled}>
            Cancel
          </button>
        )}
      </form>

      <div className="table">
        <div className="table-row header">
          <span>Name</span>
          <span>Code</span>
          <span>Parent</span>
          <span>Manager</span>
          <span></span>
        </div>
        {departments.map((dep) => (
          <div key={dep.id} className="table-row">
            <span>{dep.name}</span>
            <span>{dep.code || 'N/A'}</span>
            <span>{departmentById[dep.parentId]?.name || 'N/A'}</span>
            <span>{formatName(employeeByUserId[dep.managerId])}</span>
            <span className="row-actions">
              <button type="button" className="ghost" onClick={() => onEdit(dep)} disabled={disabled}>
                Edit
              </button>
              <button type="button" className="ghost" onClick={() => onDelete(dep)} disabled={disabled}>
                Delete
              </button>
            </span>
          </div>
        ))}
      </div>
    </div>
  );
}
