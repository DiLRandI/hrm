import React from 'react';
import { Routes, Route, Navigate, NavLink } from 'react-router-dom';
import { AuthProvider, useAuth } from './auth.jsx';
import Login from './pages/Login.jsx';
import Dashboard from './pages/Dashboard.jsx';
import Employees from './pages/Employees.jsx';
import Leave from './pages/Leave.jsx';
import Payroll from './pages/Payroll.jsx';
import Performance from './pages/Performance.jsx';
import GDPR from './pages/GDPR.jsx';
import Reports from './pages/Reports.jsx';
import Notifications from './pages/Notifications.jsx';

function AppShell() {
  const { user, logout } = useAuth();

  if (!user) {
    return <Navigate to="/login" replace />;
  }

  const role = user.role || 'Employee';

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
