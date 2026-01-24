import React, { useCallback, useMemo, useState } from 'react';
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
import ManagerHistory from '../components/ManagerHistory.jsx';
import OrgChart from '../components/OrgChart.jsx';
import RolePermissions from '../components/RolePermissions.jsx';

const EMPLOYEE_LIMIT = 25;

export default function Employees() {
  const { user } = useAuth();
  const isHR = getRole(user) === ROLE_HR;
  const [employeeOffset, setEmployeeOffset] = useState(0);
  const [selectedRole, setSelectedRole] = useState('');
  const [selectedPerms, setSelectedPerms] = useState([]);
  const [editEmployee, setEditEmployee] = useState(null);
  const [managerHistory, setManagerHistory] = useState([]);
  const [actionError, setActionError] = useState('');
  const [form, setForm] = useState({ firstName: '', lastName: '', email: '' });
  const [tempCredentials, setTempCredentials] = useState(null);

  const fetchEmployees = useCallback(
    ({ signal }) => api.getWithMeta(`/employees?limit=${EMPLOYEE_LIMIT}&offset=${employeeOffset}`, { signal }),
    [employeeOffset],
  );

  const {
    data: employeePage,
    error: employeeError,
    loading: employeesLoading,
    reload: reloadEmployees,
  } = useApiQuery(fetchEmployees, [employeeOffset], { initialData: { data: [], total: 0 } });

  const employees = Array.isArray(employeePage?.data) ? employeePage.data : [];
  const employeeTotal = employeePage?.total ?? employees.length;

  const fetchLookups = useCallback(
    async ({ signal }) => {
      const baseRequests = [
        api.get('/departments', { signal }),
        api.get('/org/chart', { signal }),
      ];

      if (isHR) {
        const [departments, orgChart, payGroups, roles, permissions, employeeOptions] = await Promise.all([
          ...baseRequests,
          api.get('/payroll/groups', { signal }),
          api.get('/roles', { signal }),
          api.get('/permissions', { signal }),
          api.get('/employees?limit=500&offset=0', { signal }),
        ]);

        return {
          departments: Array.isArray(departments) ? departments : [],
          orgChart: Array.isArray(orgChart) ? orgChart : [],
          payGroups: Array.isArray(payGroups) ? payGroups : [],
          roles: Array.isArray(roles) ? roles : [],
          permissions: Array.isArray(permissions) ? permissions : [],
          employeeOptions: Array.isArray(employeeOptions) ? employeeOptions : [],
        };
      }

      const [departments, orgChart] = await Promise.all(baseRequests);
      return {
        departments: Array.isArray(departments) ? departments : [],
        orgChart: Array.isArray(orgChart) ? orgChart : [],
        payGroups: [],
        roles: [],
        permissions: [],
        employeeOptions: [],
      };
    },
    [isHR],
  );

  const {
    data: lookups,
    error: lookupError,
    loading: lookupsLoading,
    reload: reloadLookups,
  } = useApiQuery(
    fetchLookups,
    [isHR],
    {
      initialData: {
        departments: [],
        orgChart: [],
        payGroups: [],
        roles: [],
        permissions: [],
        employeeOptions: [],
      },
    },
  );

  const employeeOptions = isHR ? lookups.employeeOptions : employees;

  const payGroupById = useMemo(() => {
    return (lookups.payGroups || []).reduce((acc, group) => {
      acc[group.id] = group.name;
      return acc;
    }, {});
  }, [lookups.payGroups]);

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
  const combinedError = actionError || employeeError || lookupError;

  return (
    <section className="page">
      <header className="page-header">
        <div>
          <h2>People</h2>
          <p>Manage employee profiles and reporting lines.</p>
        </div>
      </header>

      <InlineError message={combinedError} />

      {loading && employees.length === 0 && (
        <PageStatus title="Loading employees" description="Preparing people records and org data." />
      )}

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

      {isHR && (
        <EmployeeCreateForm
          form={form}
          onChange={handleFormChange}
          onSubmit={handleSubmit}
          disabled={loading}
        />
      )}

      {isHR && (
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
      )}

      <ManagerHistory history={managerHistory} />

      {!loading && employees.length === 0 ? (
        <EmptyState
          title="No employees yet"
          description="Once employees are added, they will appear here with contact and status details."
        />
      ) : (
        <EmployeeTable employees={employees} showPayGroup={isHR} payGroupById={payGroupById} />
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

      <OrgChart orgChart={lookups.orgChart} />

      {isHR && (
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
      )}
    </section>
  );
}
