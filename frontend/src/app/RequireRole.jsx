import React from 'react';
import { Navigate } from 'react-router-dom';
import { useAuth } from '../features/auth/auth.jsx';
import { getRole } from '../shared/utils/role.js';

export default function RequireRole({ allowed, children }) {
  const { user, loading } = useAuth();
  if (loading) {
    return <div className="page-loading">Loading...</div>;
  }
  if (!user) {
    return <Navigate to="/login" replace />;
  }
  const role = getRole(user);
  if (Array.isArray(allowed) && allowed.length > 0 && !allowed.includes(role)) {
    return <Navigate to="/" replace />;
  }
  return children;
}
