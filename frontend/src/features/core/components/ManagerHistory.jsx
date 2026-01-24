import React from 'react';

export default function ManagerHistory({ history }) {
  if (!history?.length) {
    return null;
  }

  return (
    <div className="card">
      <h3>Manager history</h3>
      {history.map((item, idx) => (
        <div key={`${item.managerId}-${idx}`} className="table-row">
          <span>{item.name || item.managerId}</span>
          <span>{item.startDate ? new Date(item.startDate).toLocaleDateString() : '-'}</span>
          <span>{item.endDate ? new Date(item.endDate).toLocaleDateString() : 'Present'}</span>
        </div>
      ))}
    </div>
  );
}
