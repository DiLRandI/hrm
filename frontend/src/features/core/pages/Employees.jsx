import React, { useEffect, useState } from 'react';
import { api } from '../../../services/apiClient.js';
import { useAuth } from '../../auth/auth.jsx';
import { ROLE_HR } from '../../../shared/constants/roles.js';
import { EMPLOYEE_STATUS_ACTIVE } from '../../../shared/constants/statuses.js';

export default function Employees() {
  const { user } = useAuth();
  const [employees, setEmployees] = useState([]);
  const [employeeOptions, setEmployeeOptions] = useState([]);
  const [employeeOffset, setEmployeeOffset] = useState(0);
  const [employeeTotal, setEmployeeTotal] = useState(0);
  const [departments, setDepartments] = useState([]);
  const [payGroups, setPayGroups] = useState([]);
  const [orgChart, setOrgChart] = useState([]);
  const [roles, setRoles] = useState([]);
  const [permissions, setPermissions] = useState([]);
  const [selectedRole, setSelectedRole] = useState('');
  const [selectedPerms, setSelectedPerms] = useState([]);
  const [editEmployee, setEditEmployee] = useState(null);
  const [managerHistory, setManagerHistory] = useState([]);
  const [error, setError] = useState('');
  const [form, setForm] = useState({ firstName: '', lastName: '', email: '' });
  const isHR = (user?.role || user?.RoleName) === ROLE_HR;
  const EMPLOYEE_LIMIT = 25;

  const load = async () => {
    try {
      const { data, total } = await api.getWithMeta(`/employees?limit=${EMPLOYEE_LIMIT}&offset=${employeeOffset}`);
      const employeeList = Array.isArray(data) ? data : [];
      setEmployees(employeeList);
      setEmployeeTotal(total ?? employeeList.length);
      if (isHR) {
        const options = await api.get('/employees?limit=500&offset=0');
        setEmployeeOptions(Array.isArray(options) ? options : employeeList);
      } else {
        setEmployeeOptions(employeeList);
      }
      const deptData = await api.get('/departments');
      setDepartments(Array.isArray(deptData) ? deptData : []);
      const orgData = await api.get('/org/chart');
      setOrgChart(Array.isArray(orgData) ? orgData : []);
      if (isHR) {
        const groupsData = await api.get('/payroll/groups');
        setPayGroups(Array.isArray(groupsData) ? groupsData : []);
        const roleData = await api.get('/roles');
        setRoles(Array.isArray(roleData) ? roleData : []);
        const permData = await api.get('/permissions');
        setPermissions(Array.isArray(permData) ? permData : []);
      }
    } catch (err) {
      setError(err.message);
    }
  };

  useEffect(() => {
    load();
  }, [isHR, employeeOffset]);

  const nextEmployees = () => {
    if (employeeOffset + EMPLOYEE_LIMIT >= employeeTotal) {
      return;
    }
    setEmployeeOffset(employeeOffset + EMPLOYEE_LIMIT);
  };

  const prevEmployees = () => {
    setEmployeeOffset(Math.max(0, employeeOffset - EMPLOYEE_LIMIT));
  };

  const handleSubmit = async (e) => {
    e.preventDefault();
    setError('');
    try {
      await api.post('/employees', {
        firstName: form.firstName,
        lastName: form.lastName,
        email: form.email,
        status: EMPLOYEE_STATUS_ACTIVE,
      });
      setForm({ firstName: '', lastName: '', email: '' });
      await load();
    } catch (err) {
      setError(err.message);
    }
  };

  const handleSelectEmployee = async (employeeId) => {
    if (!employeeId) {
      setEditEmployee(null);
      setManagerHistory([]);
      return;
    }
    try {
      const detail = await api.get(`/employees/${employeeId}`);
      setEditEmployee(detail);
      const history = await api.get(`/employees/${employeeId}/manager-history`);
      setManagerHistory(Array.isArray(history) ? history : []);
    } catch (err) {
      setError(err.message);
    }
  };

  const handleUpdateEmployee = async (e) => {
    e.preventDefault();
    if (!editEmployee?.id) {
      setError('Select an employee to update');
      return;
    }
    setError('');
    try {
      await api.put(`/employees/${editEmployee.id}`, editEmployee);
      await load();
    } catch (err) {
      setError(err.message);
    }
  };

  const handleSaveRole = async () => {
    if (!selectedRole) {
      setError('Select a role to update');
      return;
    }
    setError('');
    try {
      await api.put(`/roles/${selectedRole}`, { permissions: selectedPerms });
      await load();
    } catch (err) {
      setError(err.message);
    }
  };

  return (
    <section className="page">
      <header className="page-header">
        <div>
          <h2>People</h2>
          <p>Manage employee profiles and reporting lines.</p>
        </div>
      </header>

      {error && <div className="error">{error}</div>}

      {isHR && (
        <form className="inline-form" onSubmit={handleSubmit}>
          <input placeholder="First name" value={form.firstName} onChange={(e) => setForm({ ...form, firstName: e.target.value })} />
          <input placeholder="Last name" value={form.lastName} onChange={(e) => setForm({ ...form, lastName: e.target.value })} />
          <input placeholder="Email" value={form.email} onChange={(e) => setForm({ ...form, email: e.target.value })} />
          <button type="submit">Add employee</button>
        </form>
      )}

      {isHR && (
        <form className="inline-form" onSubmit={handleUpdateEmployee}>
          <select value={editEmployee?.id || ''} onChange={(e) => handleSelectEmployee(e.target.value)}>
            <option value="">Select employee to edit</option>
            {employeeOptions.map((emp) => (
              <option key={emp.id} value={emp.id}>{emp.firstName} {emp.lastName}</option>
            ))}
          </select>
          <select value={editEmployee?.departmentId || ''} onChange={(e) => setEditEmployee({ ...editEmployee, departmentId: e.target.value })}>
            <option value="">Department</option>
            {departments.map((dep) => (
              <option key={dep.id} value={dep.id}>{dep.name}</option>
            ))}
          </select>
          <select value={editEmployee?.managerId || ''} onChange={(e) => setEditEmployee({ ...editEmployee, managerId: e.target.value })}>
            <option value="">Manager</option>
            {employeeOptions.map((emp) => (
              <option key={emp.id} value={emp.id}>{emp.firstName} {emp.lastName}</option>
            ))}
          </select>
          <select value={editEmployee?.payGroupId || ''} onChange={(e) => setEditEmployee({ ...editEmployee, payGroupId: e.target.value })}>
            <option value="">Pay group</option>
            {payGroups.map((group) => (
              <option key={group.id} value={group.id}>{group.name}</option>
            ))}
          </select>
          <button type="submit">Save changes</button>
        </form>
      )}

      {isHR && managerHistory.length > 0 && (
        <div className="card">
          <h3>Manager history</h3>
          {managerHistory.map((item, idx) => (
            <div key={`${item.managerId}-${idx}`} className="table-row">
              <span>{item.name || item.managerId}</span>
              <span>{item.startDate ? new Date(item.startDate).toLocaleDateString() : '-'}</span>
              <span>{item.endDate ? new Date(item.endDate).toLocaleDateString() : 'Present'}</span>
            </div>
          ))}
        </div>
      )}

      <div className="table">
        <div className="table-row header">
          <span>Name</span>
          <span>Email</span>
          <span>Status</span>
          {isHR && <span>Pay group</span>}
        </div>
        {employees.map((emp) => (
          <div key={emp.id} className="table-row">
            <span>{emp.firstName} {emp.lastName}</span>
            <span>{emp.email}</span>
            <span>{emp.status}</span>
            {isHR && <span>{payGroups.find((g) => g.id === emp.payGroupId)?.name || '—'}</span>}
          </div>
        ))}
      </div>

      <div className="row-actions pagination">
        <button type="button" className="ghost" onClick={prevEmployees} disabled={employeeOffset === 0}>
          Prev
        </button>
        <small>
          {employeeTotal ? `${Math.min(employeeOffset + EMPLOYEE_LIMIT, employeeTotal)} of ${employeeTotal}` : '—'}
        </small>
        <button
          type="button"
          className="ghost"
          onClick={nextEmployees}
          disabled={employeeTotal ? employeeOffset + EMPLOYEE_LIMIT >= employeeTotal : employees.length < EMPLOYEE_LIMIT}
        >
          Next
        </button>
      </div>

      {orgChart.length > 0 && (
        <div className="card">
          <h3>Org chart</h3>
          {orgChart.map((node) => (
            <div key={node.id} className="table-row">
              <span>{node.name}</span>
              <span>Manager: {orgChart.find((n) => n.id === node.managerId)?.name || '—'}</span>
            </div>
          ))}
        </div>
      )}

      {isHR && (
        <div className="card">
          <h3>Roles & permissions</h3>
          <div className="inline-form">
            <select value={selectedRole} onChange={(e) => {
              setSelectedRole(e.target.value);
              const role = roles.find((r) => r.id === e.target.value);
              setSelectedPerms(role?.permissions || []);
            }}>
              <option value="">Select role</option>
              {roles.map((role) => (
                <option key={role.id} value={role.id}>{role.name}</option>
              ))}
            </select>
          </div>
          {selectedRole && (
            <div className="checkbox-grid">
              {permissions.map((perm) => (
                <label key={perm.key} className="checkbox">
                  <input
                    type="checkbox"
                    checked={selectedPerms.includes(perm.key)}
                    onChange={(e) => {
                      if (e.target.checked) {
                        setSelectedPerms([...selectedPerms, perm.key]);
                      } else {
                        setSelectedPerms(selectedPerms.filter((p) => p !== perm.key));
                      }
                    }}
                  />
                  {perm.key}
                </label>
              ))}
            </div>
          )}
          <button type="button" onClick={handleSaveRole}>Save role permissions</button>
        </div>
      )}
    </section>
  );
}
