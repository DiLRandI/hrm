import React from 'react';
import { Routes, Route, Navigate, NavLink } from 'react-router-dom';
import { AuthProvider, useAuth } from '../features/auth/auth.jsx';
import Login from '../features/auth/pages/Login.jsx';
import Dashboard from '../features/core/pages/Dashboard.jsx';
import Employees from '../features/core/pages/Employees.jsx';
import Leave from '../features/leave/pages/Leave.jsx';
import Payroll from '../features/payroll/pages/Payroll.jsx';
import Performance from '../features/performance/pages/Performance.jsx';
import GDPR from '../features/gdpr/pages/GDPR.jsx';
import Reports from '../features/reports/pages/Reports.jsx';
import Notifications from '../features/notifications/pages/Notifications.jsx';
import Audit from '../features/audit/pages/Audit.jsx';
import { ROLE_EMPLOYEE, ROLE_HR } from '../shared/constants/roles.js';

function AppShell() {
  const { user, logout } = useAuth();

  if (!user) {
    return <Navigate to="/login" replace />;
  }

  const role = user?.role || user?.RoleName || ROLE_EMPLOYEE;

  return (
    <div className="app-shell">
      <aside className="sidebar">
        <div className="brand">
          <span>PulseHR</span>
          <small>{role}</small>
        </div>
        <nav>
          <NavLink to="/" end>Dashboard</NavLink>
          <NavLink to="/employees">People</NavLink>
          <NavLink to="/leave">Leave</NavLink>
          <NavLink to="/payroll">Payroll</NavLink>
          <NavLink to="/performance">Performance</NavLink>
          <NavLink to="/gdpr">GDPR</NavLink>
          <NavLink to="/reports">Reports</NavLink>
          <NavLink to="/notifications">Notifications</NavLink>
          {role === ROLE_HR && <NavLink to="/audit">Audit</NavLink>}
        </nav>
        <button className="ghost" onClick={logout}>Log out</button>
      </aside>
      <main className="content">
        <Routes>
          <Route path="/" element={<Dashboard />} />
          <Route path="/employees" element={<Employees />} />
          <Route path="/leave" element={<Leave />} />
          <Route path="/payroll" element={<Payroll />} />
          <Route path="/performance" element={<Performance />} />
          <Route path="/gdpr" element={<GDPR />} />
          <Route path="/reports" element={<Reports />} />
          <Route path="/notifications" element={<Notifications />} />
          <Route path="/audit" element={<Audit />} />
        </Routes>
      </main>
    </div>
  );
}

export default function App() {
  return (
    <AuthProvider>
      <Routes>
        <Route path="/login" element={<Login />} />
        <Route path="/*" element={<AppShell />} />
      </Routes>
    </AuthProvider>
  );
}
