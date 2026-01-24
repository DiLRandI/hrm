import React, { useCallback, useMemo, useState } from 'react';
import { NavLink, Navigate, Route, Routes, useLocation } from 'react-router-dom';
import { api } from '../../../services/apiClient.js';
import { useAuth } from '../../auth/auth.jsx';
import { ROLE_HR } from '../../../shared/constants/roles.js';
import { EMPLOYEE_STATUS_ACTIVE } from '../../../shared/constants/statuses.js';
import { getRole } from '../../../shared/utils/role.js';
import { useApiQuery } from '../../../shared/hooks/useApiQuery.js';
import { EmptyState, InlineError, PageStatus } from '../../../shared/components/PageStatus.jsx';
import EmployeeCreateForm from '../components/EmployeeCreateForm.jsx';
import EmployeeEditForm from '../components/EmployeeEditForm.jsx';
import EmployeeTable from '../components/EmployeeTable.jsx';
import DepartmentsCard from '../components/DepartmentsCard.jsx';
import ManagerHistory from '../components/ManagerHistory.jsx';
import OrgChart from '../components/OrgChart.jsx';
import RolePermissions from '../components/RolePermissions.jsx';

const EMPLOYEE_LIMIT = 25;

export default function Employees() {
  const { user } = useAuth();
  const isHR = getRole(user) === ROLE_HR;
  const location = useLocation();
  const activeSection = useMemo(() => {
    const segment = location.pathname.split('/')[2];
    return segment || 'overview';
  }, [location.pathname]);
  const [employeeOffset, setEmployeeOffset] = useState(0);
  const [selectedRole, setSelectedRole] = useState('');
  const [selectedPerms, setSelectedPerms] = useState([]);
  const [editEmployee, setEditEmployee] = useState(null);
  const [managerHistory, setManagerHistory] = useState([]);
  const [actionError, setActionError] = useState('');
  const [form, setForm] = useState({ firstName: '', lastName: '', email: '' });
  const [tempCredentials, setTempCredentials] = useState(null);
  const [departmentForm, setDepartmentForm] = useState({
    name: '',
    code: '',
    parentId: '',
    managerId: '',
  });
  const [editingDepartmentId, setEditingDepartmentId] = useState('');

  const fetchEmployees = useCallback(
    ({ signal }) => api.getWithMeta(`/employees?limit=${EMPLOYEE_LIMIT}&offset=${employeeOffset}`, { signal }),
    [employeeOffset],
  );

  const {
    data: employeePage,
    error: employeeError,
    loading: employeesLoading,
    reload: reloadEmployees,
  } = useApiQuery(fetchEmployees, [employeeOffset, activeSection], {
    enabled: activeSection === 'directory' || activeSection === 'overview',
    initialData: { data: [], total: 0 },
  });

  const employees = Array.isArray(employeePage?.data) ? employeePage.data : [];
  const employeeTotal = employeePage?.total ?? employees.length;

  const lookupNeeds = useMemo(() => {
    return {
      departments: activeSection === 'directory' || (isHR && activeSection === 'manage'),
      orgChart: activeSection === 'org',
      payGroups: isHR && (activeSection === 'manage' || activeSection === 'directory'),
      roles: isHR && activeSection === 'access',
      permissions: isHR && activeSection === 'access',
      employeeOptions: isHR && activeSection === 'manage',
    };
  }, [activeSection, isHR]);

  const lookupsEnabled = Object.values(lookupNeeds).some(Boolean);

  const fetchLookups = useCallback(
    async ({ signal }) => {
      const result = {
        departments: [],
        orgChart: [],
        payGroups: [],
        roles: [],
        permissions: [],
        employeeOptions: [],
      };
      const requests = [];

      if (lookupNeeds.departments) {
        requests.push(
          api.get('/departments', { signal }).then((data) => {
            result.departments = Array.isArray(data) ? data : [];
          }),
        );
      }
      if (lookupNeeds.orgChart) {
        requests.push(
          api.get('/org/chart', { signal }).then((data) => {
            result.orgChart = Array.isArray(data) ? data : [];
          }),
        );
      }
      if (lookupNeeds.payGroups) {
        requests.push(
          api.get('/payroll/groups', { signal }).then((data) => {
            result.payGroups = Array.isArray(data) ? data : [];
          }),
        );
      }
      if (lookupNeeds.roles) {
        requests.push(
          api.get('/roles', { signal }).then((data) => {
            result.roles = Array.isArray(data) ? data : [];
          }),
        );
      }
      if (lookupNeeds.permissions) {
        requests.push(
          api.get('/permissions', { signal }).then((data) => {
            result.permissions = Array.isArray(data) ? data : [];
          }),
        );
      }
      if (lookupNeeds.employeeOptions) {
        requests.push(
          api.get('/employees?limit=500&offset=0', { signal }).then((data) => {
            result.employeeOptions = Array.isArray(data) ? data : [];
          }),
        );
      }

      await Promise.all(requests);
      return result;
    },
    [lookupNeeds],
  );

  const {
    data: lookups,
    error: lookupError,
    loading: lookupsLoading,
    reload: reloadLookups,
  } = useApiQuery(fetchLookups, [isHR, activeSection], {
    enabled: lookupsEnabled,
    initialData: {
      departments: [],
      orgChart: [],
      payGroups: [],
      roles: [],
      permissions: [],
      employeeOptions: [],
    },
  });

  const employeeOptions = isHR ? (lookups.employeeOptions.length ? lookups.employeeOptions : employees) : employees;

  const payGroupById = useMemo(() => {
    return (lookups.payGroups || []).reduce((acc, group) => {
      acc[group.id] = group.name;
      return acc;
    }, {});
  }, [lookups.payGroups]);

  const departmentById = useMemo(() => {
    return (lookups.departments || []).reduce((acc, dep) => {
      acc[dep.id] = dep.name;
      return acc;
    }, {});
  }, [lookups.departments]);

  const reloadAll = async () => {
    await Promise.all([reloadEmployees(), reloadLookups()]);
  };

  const nextEmployees = () => {
    if (employeeOffset + EMPLOYEE_LIMIT >= employeeTotal) {
      return;
    }
    setEmployeeOffset(employeeOffset + EMPLOYEE_LIMIT);
  };

  const prevEmployees = () => {
    setEmployeeOffset(Math.max(0, employeeOffset - EMPLOYEE_LIMIT));
  };

  const handleFormChange = (field, value) => {
    setForm((prev) => ({ ...prev, [field]: value }));
  };

  const handleDepartmentChange = (field, value) => {
    setDepartmentForm((prev) => ({ ...prev, [field]: value }));
  };

  const resetDepartmentForm = () => {
    setDepartmentForm({ name: '', code: '', parentId: '', managerId: '' });
    setEditingDepartmentId('');
  };

  const handleDepartmentSubmit = async (e) => {
    e.preventDefault();
    setActionError('');
    try {
      if (editingDepartmentId) {
        await api.put(`/departments/${editingDepartmentId}`, departmentForm);
      } else {
        await api.post('/departments', departmentForm);
      }
      resetDepartmentForm();
      await reloadLookups();
    } catch (err) {
      setActionError(err.message);
    }
  };

  const handleEditDepartment = (dep) => {
    setEditingDepartmentId(dep.id);
    setDepartmentForm({
      name: dep.name || '',
      code: dep.code || '',
      parentId: dep.parentId || '',
      managerId: dep.managerId || '',
    });
  };

  const handleDeleteDepartment = async (dep) => {
    if (!window.confirm(`Delete ${dep.name}? Employees assigned to this department will block deletion.`)) {
      return;
    }
    setActionError('');
    try {
      await api.del(`/departments/${dep.id}`);
      if (editingDepartmentId === dep.id) {
        resetDepartmentForm();
      }
      await reloadLookups();
    } catch (err) {
      setActionError(err.message);
    }
  };

  const handleSubmit = async (e) => {
    e.preventDefault();
    setActionError('');
    try {
      const result = await api.post('/employees', {
        firstName: form.firstName,
        lastName: form.lastName,
        email: form.email,
        status: EMPLOYEE_STATUS_ACTIVE,
      });
      if (result?.tempPassword) {
        setTempCredentials({ email: form.email, password: result.tempPassword });
      } else {
        setTempCredentials(null);
      }
      setForm({ firstName: '', lastName: '', email: '' });
      await reloadAll();
    } catch (err) {
      setActionError(err.message);
    }
  };

  const handleSelectEmployee = async (employeeId) => {
    if (!employeeId) {
      setEditEmployee(null);
      setManagerHistory([]);
      return;
    }
    setActionError('');
    try {
      const detail = await api.get(`/employees/${employeeId}`);
      setEditEmployee(detail);
      const history = await api.get(`/employees/${employeeId}/manager-history`);
      setManagerHistory(Array.isArray(history) ? history : []);
    } catch (err) {
      setActionError(err.message);
    }
  };

  const handleUpdateEmployee = async (e) => {
    e.preventDefault();
    if (!editEmployee?.id) {
      setActionError('Select an employee to update');
      return;
    }
    setActionError('');
    try {
      await api.put(`/employees/${editEmployee.id}`, editEmployee);
      await reloadAll();
    } catch (err) {
      setActionError(err.message);
    }
  };

  const handleSaveRole = async () => {
    if (!selectedRole) {
      setActionError('Select a role to update');
      return;
    }
    setActionError('');
    try {
      await api.put(`/roles/${selectedRole}`, { permissions: selectedPerms });
      await reloadAll();
    } catch (err) {
      setActionError(err.message);
    }
  };

  const handleSelectRole = (roleId) => {
    setSelectedRole(roleId);
    const role = lookups.roles.find((item) => item.id === roleId);
    setSelectedPerms(role?.permissions || []);
  };

  const togglePermission = (permKey) => {
    setSelectedPerms((prev) => {
      if (prev.includes(permKey)) {
        return prev.filter((perm) => perm !== permKey);
      }
      return [...prev, permKey];
    });
  };

  const loading = employeesLoading || lookupsLoading;
  const combinedError =
    actionError ||
    (activeSection === 'directory' ? employeeError : '') ||
    (lookupsEnabled ? lookupError : '');

  return (
    <section className="page">
      <header className="page-header">
        <div>
          <h2>People</h2>
          <p>Manage employee profiles and reporting lines.</p>
        </div>
      </header>

      <InlineError message={combinedError} />

      <nav className="subnav">
        <NavLink to="/employees/overview">Overview</NavLink>
        <NavLink to="/employees/directory">Directory</NavLink>
        {isHR && <NavLink to="/employees/manage">Add & Edit</NavLink>}
        <NavLink to="/employees/org">Org chart</NavLink>
        {isHR && <NavLink to="/employees/access">Roles & Access</NavLink>}
      </nav>

      <Routes>
        <Route path="/" element={<Navigate to="overview" replace />} />
        <Route
          path="overview"
          element={
            <div className="card-grid">
              <div className="card">
                <h3>Employees</h3>
                <p className="metric">{employeeTotal || 0}</p>
                <p className="inline-note">Total employee records</p>
              </div>
              <div className="card">
                <h3>Visible now</h3>
                <p className="metric">{employees.length}</p>
                <p className="inline-note">Loaded in this view</p>
              </div>
              <div className="card">
                <h3>Quick actions</h3>
                <div className="row-actions">
                  {isHR && <NavLink className="ghost-link" to="/employees/manage">Add employee</NavLink>}
                  <NavLink className="ghost-link" to="/employees/org">View org chart</NavLink>
                </div>
              </div>
            </div>
          }
        />
        <Route
          path="directory"
          element={
            <>
              {loading && employees.length === 0 && (
                <PageStatus title="Loading employees" description="Preparing people records and org data." />
              )}

              {!loading && employees.length === 0 ? (
                <EmptyState
                  title="No employees yet"
                  description="Once employees are added, they will appear here with contact and status details."
                />
              ) : (
                <EmployeeTable
                  employees={employees}
                  showDepartment
                  departmentById={departmentById}
                  showPayGroup={isHR}
                  payGroupById={payGroupById}
                />
              )}

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
            </>
          }
        />
        {isHR && (
          <Route
            path="manage"
            element={
              <>
                {tempCredentials && (
                  <div className="card">
                    <h3>Temporary login created</h3>
                    <p>
                      Share this once with the employee. It will not be shown again.
                    </p>
                    <div className="inline-form" style={{ marginTop: '12px' }}>
                      <input readOnly value={tempCredentials.email} aria-label="Employee email" />
                      <input readOnly value={tempCredentials.password} aria-label="Temporary password" />
                      <button
                        type="button"
                        onClick={async () => {
                          try {
                            await navigator.clipboard.writeText(tempCredentials.password);
                          } catch {
                            // Ignore clipboard failures (permissions, insecure context).
                          }
                        }}
                      >
                        Copy password
                      </button>
                      <button type="button" className="ghost" onClick={() => setTempCredentials(null)}>
                        Dismiss
                      </button>
                    </div>
                  </div>
                )}

                <EmployeeCreateForm
                  form={form}
                  onChange={handleFormChange}
                  onSubmit={handleSubmit}
                  disabled={loading}
                />

                <EmployeeEditForm
                  employeeOptions={employeeOptions}
                  departments={lookups.departments}
                  payGroups={lookups.payGroups}
                  editEmployee={editEmployee}
                  onSelectEmployee={handleSelectEmployee}
                  onFieldChange={(field, value) => setEditEmployee((prev) => ({ ...prev, [field]: value }))}
                  onSubmit={handleUpdateEmployee}
                  disabled={loading}
                />

                <DepartmentsCard
                  departments={lookups.departments}
                  employees={employeeOptions}
                  form={departmentForm}
                  editingId={editingDepartmentId}
                  onChange={handleDepartmentChange}
                  onSubmit={handleDepartmentSubmit}
                  onEdit={handleEditDepartment}
                  onCancelEdit={resetDepartmentForm}
                  onDelete={handleDeleteDepartment}
                  disabled={loading}
                />

                <ManagerHistory history={managerHistory} />
              </>
            }
          />
        )}
        <Route
          path="org"
          element={
            <OrgChart orgChart={lookups.orgChart} />
          }
        />
        {isHR && (
          <Route
            path="access"
            element={
              <RolePermissions
                roles={lookups.roles}
                permissions={lookups.permissions}
                selectedRole={selectedRole}
                selectedPerms={selectedPerms}
                onSelectRole={handleSelectRole}
                onTogglePerm={togglePermission}
                onSave={handleSaveRole}
                disabled={loading}
              />
            }
          />
        )}
        <Route path="*" element={<Navigate to="overview" replace />} />
      </Routes>
    </section>
  );
}
