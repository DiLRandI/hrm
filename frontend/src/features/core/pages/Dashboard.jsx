import React, { useCallback, useMemo } from 'react';
import { api } from '../../../services/apiClient.js';
import { useAuth } from '../../auth/auth.jsx';
import { ROLE_ADMIN, ROLE_HR, ROLE_HR_MANAGER, ROLE_MANAGER, ROLE_SYSTEM_ADMIN } from '../../../shared/constants/roles.js';
import { getRole } from '../../../shared/utils/role.js';
import { useApiQuery } from '../../../shared/hooks/useApiQuery.js';
import { InlineError, PageStatus } from '../../../shared/components/PageStatus.jsx';

export default function Dashboard() {
  const { user, employee } = useAuth();
  const role = getRole(user);
  const dashboardEndpoint = useMemo(() => {
    if (role === ROLE_HR || role === ROLE_HR_MANAGER) {
      return '/reports/dashboard/hr';
    }
    if (role === ROLE_MANAGER) {
      return '/reports/dashboard/manager';
    }
    if (role === ROLE_SYSTEM_ADMIN || role === ROLE_ADMIN) {
      return '';
    }
    return '/reports/dashboard/employee';
  }, [role]);

  const fetchDashboard = useCallback(
    ({ signal }) => api.get(dashboardEndpoint, { signal }),
    [dashboardEndpoint],
  );

  const { data, error, loading } = useApiQuery(fetchDashboard, [dashboardEndpoint], {
    enabled: Boolean(user && dashboardEndpoint),
    initialData: null,
  });

  return (
    <section className="page">
      <header className="page-header">
        <div>
          <h2>Welcome back, {employee?.firstName || role}</h2>
          <p>Here’s your live snapshot across leave, payroll, and performance.</p>
        </div>
      </header>

      <InlineError message={error} />

      {loading && (
        <PageStatus title="Loading dashboard" description="Fetching your latest HR snapshot." />
      )}

      {!dashboardEndpoint && (
        <div className="card">
          <h3>Admin workspace</h3>
          <p>Use People management to provision and maintain organization access.</p>
        </div>
      )}

      {data && !loading && (
        <div className="card-grid">
          {Object.entries(data).map(([key, value]) => (
            <div key={key} className="card">
              <h3>{key}</h3>
              <p className="metric">{value}</p>
            </div>
          ))}
        </div>
      )}

    </section>
  );
}
