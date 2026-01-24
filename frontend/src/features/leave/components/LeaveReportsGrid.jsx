import React from 'react';

export default function LeaveReportsGrid({ balanceReport, usageReport, typeLookup }) {
  return (
    <div className="card-grid">
      <div className="card">
        <h3>Balance Report</h3>
        <div className="table">
          <div className="table-row header">
            <span>Employee</span>
            <span>Type</span>
            <span>Balance</span>
            <span>Pending</span>
            <span>Used</span>
          </div>
          {balanceReport.map((row) => (
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

      <div className="card">
        <h3>Usage Report</h3>
        <div className="table">
          <div className="table-row header">
            <span>Type</span>
            <span>Total days</span>
          </div>
          {usageReport.map((row) => (
            <div key={row.leaveTypeId} className="table-row">
              <span>{typeLookup[row.leaveTypeId] || row.leaveTypeId}</span>
              <span>{row.totalDays}</span>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
