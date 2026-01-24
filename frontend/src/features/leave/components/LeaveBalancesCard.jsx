import React from 'react';

export default function LeaveBalancesCard({ balances, typeLookup }) {
  return (
    <div className="card">
      <h3>Balances</h3>
      <div className="table">
        <div className="table-row header">
          <span>Employee</span>
          <span>Type</span>
          <span>Balance</span>
          <span>Pending</span>
          <span>Used</span>
        </div>
        {balances.map((row) => (
          <div key={`${row.employeeId}-${row.leaveTypeId}`} className="table-row">
            <span>{row.employeeId}</span>
            <span>{typeLookup[row.leaveTypeId] || row.leaveTypeId}</span>
            <span>{row.balance}</span>
            <span>{row.pending}</span>
            <span>{row.used}</span>
          </div>
        ))}
      </div>
    </div>
  );
}
