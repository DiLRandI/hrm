import React from 'react';

export default function EmployeeTable({ employees, showDepartment, departmentById, showPayGroup, payGroupById }) {
  return (
    <div className="table" role="table" aria-label="Employee list">
      <div className="table-row header" role="row">
        <span role="columnheader">Name</span>
        <span role="columnheader">Email</span>
        <span role="columnheader">Status</span>
        {showDepartment && <span role="columnheader">Department</span>}
        {showPayGroup && <span role="columnheader">Pay group</span>}
      </div>
      {employees.map((emp) => (
        <div key={emp.id} className="table-row" role="row">
          <span role="cell">{emp.firstName} {emp.lastName}</span>
          <span role="cell">{emp.email}</span>
          <span role="cell">{emp.status}</span>
          {showDepartment && <span role="cell">{departmentById[emp.departmentId] || 'N/A'}</span>}
          {showPayGroup && <span role="cell">{payGroupById[emp.payGroupId] || '—'}</span>}
        </div>
      ))}
    </div>
  );
}
