import React from 'react';

export default function OrgChart({ orgChart }) {
  if (!orgChart?.length) {
    return null;
  }

  const lookup = orgChart.reduce((acc, node) => {
    acc[node.id] = node.name;
    return acc;
  }, {});

  return (
    <div className="card">
      <h3>Org chart</h3>
      {orgChart.map((node) => (
        <div key={node.id} className="table-row">
          <span>{node.name}</span>
          <span>Manager: {lookup[node.managerId] || '—'}</span>
        </div>
      ))}
    </div>
  );
}
