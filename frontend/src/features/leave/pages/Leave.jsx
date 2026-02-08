import React, { useCallback, useMemo, useState } from 'react';
import { NavLink, Navigate, Route, Routes, useLocation } from 'react-router-dom';
import { api } from '../../../services/apiClient.js';
import { useAuth } from '../../auth/auth.jsx';
import { ROLE_HR, ROLE_MANAGER } from '../../../shared/constants/roles.js';
import { getRole } from '../../../shared/utils/role.js';
import { useApiQuery } from '../../../shared/hooks/useApiQuery.js';
import { EmptyState, InlineError, PageStatus } from '../../../shared/components/PageStatus.jsx';
import LeaveRequestsCard from '../components/LeaveRequestsCard.jsx';
import LeaveBalancesCard from '../components/LeaveBalancesCard.jsx';
import LeaveAdjustCard from '../components/LeaveAdjustCard.jsx';
import LeaveTypesCard from '../components/LeaveTypesCard.jsx';
import LeavePoliciesCard from '../components/LeavePoliciesCard.jsx';
import LeaveHolidaysCard from '../components/LeaveHolidaysCard.jsx';
import LeaveCalendarCard from '../components/LeaveCalendarCard.jsx';
import LeaveReportsGrid from '../components/LeaveReportsGrid.jsx';

const REQUEST_LIMIT = 25;

const downloadBlob = ({ blob, filename }) => {
  const url = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  link.download = filename;
  link.click();
  URL.revokeObjectURL(url);
};

const normalizeArray = (value) => (Array.isArray(value) ? value : []);

export default function Leave() {
  const { user, employee } = useAuth();
  const role = getRole(user);
  const isHR = role === ROLE_HR;
  const isManager = role === ROLE_MANAGER;
  const location = useLocation();
  const activeSection = useMemo(() => {
    const segment = location.pathname.split('/')[2];
    return segment || 'requests';
  }, [location.pathname]);

  const [requestOffset, setRequestOffset] = useState(0);
  const [accrualSummary, setAccrualSummary] = useState(null);
  const [actionError, setActionError] = useState('');

  const [requestForm, setRequestForm] = useState({
    leaveTypeId: '',
    startDate: '',
    endDate: '',
    startHalf: false,
    endHalf: false,
    reason: '',
    documents: [],
  });
  const [typeForm, setTypeForm] = useState({
    name: '',
    code: '',
    isPaid: true,
    requiresDoc: false,
  });
  const [policyForm, setPolicyForm] = useState({
    leaveTypeId: '',
    accrualRate: '',
    accrualPeriod: 'monthly',
    entitlement: '',
    carryOverLimit: '',
    allowNegative: false,
    requiresHrApproval: false,
  });
  const [holidayForm, setHolidayForm] = useState({
    date: '',
    name: '',
    region: '',
  });
  const [adjustForm, setAdjustForm] = useState({
    employeeId: '',
    leaveTypeId: '',
    delta: '',
    reason: '',
  });

  const fetchLeaveData = useCallback(
    async ({ signal }) => {
      const needs = {
        types: true,
        policies: activeSection === 'policies',
        balances: activeSection === 'balances',
        holidays: activeSection === 'holidays',
        calendar: activeSection === 'holidays',
        balanceReport: activeSection === 'reports',
        usageReport: activeSection === 'reports',
      };

      const result = {
        types: [],
        policies: [],
        balances: [],
        holidays: [],
        calendar: [],
        balanceReport: [],
        usageReport: [],
      };

      const requests = [];

      if (needs.types) {
        requests.push(
          api.get('/leave/types', { signal }).then((data) => {
            result.types = normalizeArray(data);
          }),
        );
      }
      if (needs.policies) {
        requests.push(
          api.get('/leave/policies', { signal }).then((data) => {
            result.policies = normalizeArray(data);
          }),
        );
      }
      if (needs.balances) {
        requests.push(
          api.get('/leave/balances', { signal }).then((data) => {
            result.balances = normalizeArray(data);
          }),
        );
      }
      if (needs.holidays) {
        requests.push(
          api.get('/leave/holidays', { signal }).then((data) => {
            result.holidays = normalizeArray(data);
          }),
        );
      }
      if (needs.calendar) {
        requests.push(
          api.get('/leave/calendar', { signal }).then((data) => {
            result.calendar = normalizeArray(data);
          }),
        );
      }
      if (needs.balanceReport) {
        requests.push(
          api.get('/leave/reports/balances', { signal }).then((data) => {
            result.balanceReport = normalizeArray(data);
          }),
        );
      }
      if (needs.usageReport) {
        requests.push(
          api.get('/leave/reports/usage', { signal }).then((data) => {
            result.usageReport = normalizeArray(data);
          }),
        );
      }

      await Promise.all(requests);
      return result;
    },
    [activeSection],
  );

  const {
    data: leaveData,
    error: leaveError,
    loading: leaveLoading,
    reload: reloadLeave,
  } = useApiQuery(fetchLeaveData, [activeSection], {
    initialData: {
      types: [],
      policies: [],
      balances: [],
      holidays: [],
      calendar: [],
      balanceReport: [],
      usageReport: [],
    },
  });

  const fetchRequests = useCallback(
    ({ signal }) => api.getWithMeta(`/leave/requests?limit=${REQUEST_LIMIT}&offset=${requestOffset}`, { signal }),
    [requestOffset],
  );

  const {
    data: requestPage,
    error: requestError,
    loading: requestsLoading,
    reload: reloadRequests,
  } = useApiQuery(fetchRequests, [requestOffset, activeSection], {
    enabled: activeSection === 'requests',
    initialData: { data: [], total: 0 },
  });

  const requests = normalizeArray(requestPage?.data);
  const requestTotal = requestPage?.total ?? requests.length;
  const selectedLeaveType = useMemo(
    () => leaveData.types.find((type) => type.id === requestForm.leaveTypeId) || null,
    [leaveData.types, requestForm.leaveTypeId],
  );
  const requestRequiresDoc = Boolean(selectedLeaveType?.requiresDoc);

  const typeLookup = useMemo(() => {
    return leaveData.types.reduce((acc, type) => {
      acc[type.id] = type.name;
      return acc;
    }, {});
  }, [leaveData.types]);

  const reloadAll = async () => {
    await Promise.all([reloadLeave(), reloadRequests()]);
  };

  const nextRequests = () => {
    if (requestOffset + REQUEST_LIMIT >= requestTotal) {
      return;
    }
    setRequestOffset(requestOffset + REQUEST_LIMIT);
  };

  const prevRequests = () => {
    setRequestOffset(Math.max(0, requestOffset - REQUEST_LIMIT));
  };

  const submitRequest = async (event) => {
    event.preventDefault();
    setActionError('');
    if (requestRequiresDoc && requestForm.documents.length === 0) {
      setActionError('Supporting document is required for the selected leave type.');
      return;
    }

    const formData = new FormData();
    if (employee?.id) {
      formData.append('employeeId', employee.id);
    }
    formData.append('leaveTypeId', requestForm.leaveTypeId);
    formData.append('startDate', requestForm.startDate);
    formData.append('endDate', requestForm.endDate);
    formData.append('startHalf', String(requestForm.startHalf));
    formData.append('endHalf', String(requestForm.endHalf));
    formData.append('reason', requestForm.reason || '');
    requestForm.documents.forEach((file) => {
      formData.append('documents', file);
    });

    try {
      await api.postForm('/leave/requests', formData);
      setRequestForm({
        leaveTypeId: '',
        startDate: '',
        endDate: '',
        startHalf: false,
        endHalf: false,
        reason: '',
        documents: [],
      });
      await reloadAll();
    } catch (err) {
      setActionError(err.message);
    }
  };

  const createType = async (event) => {
    event.preventDefault();
    setActionError('');
    try {
      await api.post('/leave/types', typeForm);
      setTypeForm({ name: '', code: '', isPaid: true, requiresDoc: false });
      await reloadAll();
    } catch (err) {
      setActionError(err.message);
    }
  };

  const createPolicy = async (event) => {
    event.preventDefault();
    setActionError('');
    try {
      await api.post('/leave/policies', {
        leaveTypeId: policyForm.leaveTypeId,
        accrualRate: Number(policyForm.accrualRate || 0),
        accrualPeriod: policyForm.accrualPeriod,
        entitlement: Number(policyForm.entitlement || 0),
        carryOverLimit: Number(policyForm.carryOverLimit || 0),
        allowNegative: policyForm.allowNegative,
        requiresHrApproval: policyForm.requiresHrApproval,
      });
      setPolicyForm({
        leaveTypeId: '',
        accrualRate: '',
        accrualPeriod: 'monthly',
        entitlement: '',
        carryOverLimit: '',
        allowNegative: false,
        requiresHrApproval: false,
      });
      await reloadAll();
    } catch (err) {
      setActionError(err.message);
    }
  };

  const createHoliday = async (event) => {
    event.preventDefault();
    setActionError('');
    try {
      await api.post('/leave/holidays', holidayForm);
      setHolidayForm({ date: '', name: '', region: '' });
      await reloadAll();
    } catch (err) {
      setActionError(err.message);
    }
  };

  const deleteHoliday = async (holidayId) => {
    setActionError('');
    try {
      await api.del(`/leave/holidays/${holidayId}`);
      await reloadAll();
    } catch (err) {
      setActionError(err.message);
    }
  };

  const adjustBalance = async (event) => {
    event.preventDefault();
    setActionError('');
    try {
      await api.post('/leave/balances/adjust', {
        employeeId: adjustForm.employeeId,
        leaveTypeId: adjustForm.leaveTypeId,
        delta: Number(adjustForm.delta || 0),
        reason: adjustForm.reason,
      });
      setAdjustForm({ employeeId: '', leaveTypeId: '', delta: '', reason: '' });
      await reloadAll();
    } catch (err) {
      setActionError(err.message);
    }
  };

  const runAccruals = async () => {
    setActionError('');
    try {
      const summary = await api.post('/leave/accrual/run', {});
      setAccrualSummary(summary);
      await reloadAll();
    } catch (err) {
      setActionError(err.message);
    }
  };

  const approveRequest = async (requestId) => {
    setActionError('');
    try {
      await api.post(`/leave/requests/${requestId}/approve`, {});
      await reloadAll();
    } catch (err) {
      setActionError(err.message);
    }
  };

  const rejectRequest = async (requestId) => {
    setActionError('');
    try {
      await api.post(`/leave/requests/${requestId}/reject`, {});
      await reloadAll();
    } catch (err) {
      setActionError(err.message);
    }
  };

  const cancelRequest = async (requestId) => {
    setActionError('');
    try {
      await api.post(`/leave/requests/${requestId}/cancel`, {});
      await reloadAll();
    } catch (err) {
      setActionError(err.message);
    }
  };

  const downloadRequestDocument = async (requestId, documentId) => {
    setActionError('');
    try {
      const file = await api.download(`/leave/requests/${requestId}/documents/${documentId}/download`);
      downloadBlob(file);
    } catch (err) {
      setActionError(err.message);
    }
  };

  const exportCalendar = async (format) => {
    setActionError('');
    try {
      const file = await api.download(`/leave/calendar/export?format=${format}`);
      downloadBlob(file);
    } catch (err) {
      setActionError(err.message);
    }
  };

  const loading = leaveLoading || requestsLoading;
  const combinedError =
    actionError || leaveError || (activeSection === 'requests' ? requestError : '');

  return (
    <section className="page">
      <header className="page-header">
        <div>
          <h2>Leave & Absence</h2>
          <p>Request time off, manage policies, and review balances.</p>
        </div>
        {isHR && (
          <button type="button" onClick={runAccruals} disabled={loading}>
            Run accruals
          </button>
        )}
      </header>

      <InlineError message={combinedError} />

      <nav className="subnav">
        <NavLink to="/leave/requests">Requests</NavLink>
        <NavLink to="/leave/balances">Balances</NavLink>
        {isHR && <NavLink to="/leave/policies">Policies</NavLink>}
        {isHR && <NavLink to="/leave/holidays">Holidays</NavLink>}
        {isHR && <NavLink to="/leave/reports">Reports</NavLink>}
      </nav>

      {loading && leaveData.types.length === 0 && (
        <PageStatus title="Loading leave data" description="Gathering leave details." />
      )}

      <Routes>
        <Route path="/" element={<Navigate to="requests" replace />} />
        <Route
          path="requests"
          element={
            <>
              <LeaveRequestsCard
                types={leaveData.types}
                typeLookup={typeLookup}
                requests={requests}
                requestForm={requestForm}
                onFormChange={(field, value) => setRequestForm((prev) => ({ ...prev, [field]: value }))}
                onDocumentsChange={(files) => setRequestForm((prev) => ({ ...prev, documents: files }))}
                onSubmit={submitRequest}
                isManager={isManager}
                isHR={isHR}
                onApprove={approveRequest}
                onReject={rejectRequest}
                onCancel={cancelRequest}
                onDownloadDocument={downloadRequestDocument}
                requiresDocument={requestRequiresDoc}
                disabled={loading}
              />

              <div className="row-actions pagination">
                <button type="button" className="ghost" onClick={prevRequests} disabled={requestOffset === 0}>
                  Prev
                </button>
                <small>
                  {requestTotal ? `${Math.min(requestOffset + REQUEST_LIMIT, requestTotal)} of ${requestTotal}` : '—'}
                </small>
                <button
                  type="button"
                  className="ghost"
                  onClick={nextRequests}
                  disabled={requestTotal ? requestOffset + REQUEST_LIMIT >= requestTotal : requests.length < REQUEST_LIMIT}
                >
                  Next
                </button>
              </div>
            </>
          }
        />
        <Route
          path="balances"
          element={
            <>
              {accrualSummary && (
                <div className="card">
                  <h3>Accrual Run</h3>
                  <p>Updated balances: {accrualSummary.updated || 0}</p>
                  <p>Skipped: {accrualSummary.skipped || 0}</p>
                </div>
              )}

              {leaveData.balances.length === 0 && !loading ? (
                <EmptyState
                  title="No balances yet"
                  description="Balances and policy usage will appear once requests are created."
                />
              ) : (
                <div className="card-grid">
                  <LeaveBalancesCard balances={leaveData.balances} typeLookup={typeLookup} />
                  {isHR && (
                    <LeaveAdjustCard
                      types={leaveData.types}
                      form={adjustForm}
                      onChange={(field, value) => setAdjustForm((prev) => ({ ...prev, [field]: value }))}
                      onSubmit={adjustBalance}
                      disabled={loading}
                    />
                  )}
                </div>
              )}
            </>
          }
        />
        {isHR && (
          <Route
            path="policies"
            element={
              <div className="card-grid">
                <LeaveTypesCard
                  types={leaveData.types}
                  form={typeForm}
                  onChange={(field, value) => setTypeForm((prev) => ({ ...prev, [field]: value }))}
                  onSubmit={createType}
                  disabled={loading}
                />
                <LeavePoliciesCard
                  types={leaveData.types}
                  policies={leaveData.policies}
                  typeLookup={typeLookup}
                  form={policyForm}
                  onChange={(field, value) => setPolicyForm((prev) => ({ ...prev, [field]: value }))}
                  onSubmit={createPolicy}
                  disabled={loading}
                />
              </div>
            }
          />
        )}
        {isHR && (
          <Route
            path="holidays"
            element={
              <div className="card-grid">
                <LeaveHolidaysCard
                  holidays={leaveData.holidays}
                  form={holidayForm}
                  onChange={(field, value) => setHolidayForm((prev) => ({ ...prev, [field]: value }))}
                  onSubmit={createHoliday}
                  onDelete={deleteHoliday}
                  disabled={loading}
                />
                <LeaveCalendarCard
                  calendar={leaveData.calendar}
                  typeLookup={typeLookup}
                  onExport={exportCalendar}
                  disabled={loading}
                />
              </div>
            }
          />
        )}
        {isHR && (
          <Route
            path="reports"
            element={
              <LeaveReportsGrid
                balanceReport={leaveData.balanceReport}
                usageReport={leaveData.usageReport}
                typeLookup={typeLookup}
              />
            }
          />
        )}
        <Route path="*" element={<Navigate to="requests" replace />} />
      </Routes>
    </section>
  );
}
