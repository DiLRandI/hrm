import React from 'react';

export function PageStatus({ title, description }) {
  return (
    <div className="status-panel" role="status" aria-live="polite">
      <strong>{title}</strong>
      {description && <p>{description}</p>}
    </div>
  );
}

export function InlineError({ message }) {
  if (!message) {
    return null;
  }
  return (
    <div className="error" role="alert">
      {message}
    </div>
  );
}

export function EmptyState({ title, description, action }) {
  return (
    <div className="empty-state">
      <strong>{title}</strong>
      {description && <p>{description}</p>}
      {action}
    </div>
  );
}
