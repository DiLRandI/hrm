import React from 'react';

export default function LeaveCalendarCard({ calendar, typeLookup, onExport, disabled }) {
  return (
    <div className="card">
      <h3>Calendar</h3>
      <div className="row-actions">
        <button type="button" onClick={() => onExport('csv')} disabled={disabled}>Export CSV</button>
        <button type="button" className="ghost" onClick={() => onExport('ics')} disabled={disabled}>Export ICS</button>
      </div>
      <div className="table">
        <div className="table-row header">
          <span>Employee</span>
          <span>Type</span>
          <span>Dates</span>
          <span>Status</span>
        </div>
        {calendar.map((item) => (
          <div key={item.id} className="table-row">
            <span>{item.employeeId}</span>
            <span>{typeLookup[item.leaveTypeId] || item.leaveTypeId}</span>
            <span>{item.start?.slice(0, 10)} → {item.end?.slice(0, 10)}</span>
            <span>{item.status}</span>
          </div>
        ))}
      </div>
    </div>
  );
}
