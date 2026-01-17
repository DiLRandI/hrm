import React, { useEffect, useMemo, useState } from 'react';
import { api } from '../../../services/apiClient.js';
import { useAuth } from '../../auth/auth.jsx';
import { ROLE_HR, ROLE_MANAGER } from '../../../shared/constants/roles.js';
import {
  LEAVE_STATUS_PENDING,
  LEAVE_STATUS_PENDING_HR,
} from '../../../shared/constants/statuses.js';

const downloadBlob = ({ blob, filename }) => {
  const url = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  link.download = filename;
  link.click();
  URL.revokeObjectURL(url);
};

export default function Leave() {
  const { user, employee } = useAuth();
  const role = user?.role || user?.RoleName;
  const isHR = role === ROLE_HR;
  const isManager = role === ROLE_MANAGER;

  const [types, setTypes] = useState([]);
  const [policies, setPolicies] = useState([]);
  const [balances, setBalances] = useState([]);
  const [requests, setRequests] = useState([]);
  const [requestOffset, setRequestOffset] = useState(0);
  const [requestTotal, setRequestTotal] = useState(0);
  const [holidays, setHolidays] = useState([]);
  const [calendar, setCalendar] = useState([]);
  const [balanceReport, setBalanceReport] = useState([]);
  const [usageReport, setUsageReport] = useState([]);
  const [accrualSummary, setAccrualSummary] = useState(null);
  const [error, setError] = useState('');

  const [requestForm, setRequestForm] = useState({
    leaveTypeId: '',
    startDate: '',
    endDate: '',
    reason: '',
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

  const REQUEST_LIMIT = 25;

  const typeLookup = useMemo(() => {
    return types.reduce((acc, t) => {
      acc[t.id] = t.name;
      return acc;
    }, {});
  }, [types]);

  const load = async () => {
    setError('');
    setAccrualSummary(null);
    try {
      const results = await Promise.allSettled([
        api.get('/leave/types'),
        api.get('/leave/policies'),
        api.get('/leave/balances'),
        api.get('/leave/holidays'),
        api.get('/leave/calendar'),
        api.get('/leave/reports/balances'),
        api.get('/leave/reports/usage'),
      ]);

      const setters = [
        setTypes,
        setPolicies,
        setBalances,
        setHolidays,
        setCalendar,
        setBalanceReport,
        setUsageReport,
      ];

      results.forEach((result, idx) => {
        if (result.status === 'fulfilled') {
          setters[idx](Array.isArray(result.value) ? result.value : []);
        } else if (!error) {
          setError(result.reason?.message || 'Failed to load leave data');
        }
      });

      const { data, total } = await api.getWithMeta(`/leave/requests?limit=${REQUEST_LIMIT}&offset=${requestOffset}`);
      const reqList = Array.isArray(data) ? data : [];
      setRequests(reqList);
      setRequestTotal(total ?? reqList.length);
    } catch (err) {
      setError(err.message);
    }
  };

  useEffect(() => {
    load();
  }, [requestOffset]);

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
    try {
      await api.post('/leave/requests', {
        employeeId: employee?.id,
        leaveTypeId: requestForm.leaveTypeId,
        startDate: requestForm.startDate,
        endDate: requestForm.endDate,
        reason: requestForm.reason,
      });
      setRequestForm({ leaveTypeId: '', startDate: '', endDate: '', reason: '' });
      await load();
    } catch (err) {
      setError(err.message);
    }
  };

  const createType = async (event) => {
    event.preventDefault();
    try {
      await api.post('/leave/types', typeForm);
      setTypeForm({ name: '', code: '', isPaid: true, requiresDoc: false });
      await load();
    } catch (err) {
      setError(err.message);
    }
  };

  const createPolicy = async (event) => {
    event.preventDefault();
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
      await load();
    } catch (err) {
      setError(err.message);
    }
  };

  const createHoliday = async (event) => {
    event.preventDefault();
    try {
      await api.post('/leave/holidays', holidayForm);
      setHolidayForm({ date: '', name: '', region: '' });
      await load();
    } catch (err) {
      setError(err.message);
    }
  };

  const deleteHoliday = async (holidayId) => {
    try {
      await api.del(`/leave/holidays/${holidayId}`);
      await load();
    } catch (err) {
      setError(err.message);
    }
  };

  const adjustBalance = async (event) => {
    event.preventDefault();
    try {
      await api.post('/leave/balances/adjust', {
        employeeId: adjustForm.employeeId,
        leaveTypeId: adjustForm.leaveTypeId,
        delta: Number(adjustForm.delta || 0),
        reason: adjustForm.reason,
      });
      setAdjustForm({ employeeId: '', leaveTypeId: '', delta: '', reason: '' });
      await load();
    } catch (err) {
      setError(err.message);
    }
  };

  const runAccruals = async () => {
    try {
      const summary = await api.post('/leave/accrual/run', {});
      setAccrualSummary(summary);
      await load();
    } catch (err) {
      setError(err.message);
    }
  };

  const approveRequest = async (requestId) => {
    try {
      await api.post(`/leave/requests/${requestId}/approve`, {});
      await load();
    } catch (err) {
      setError(err.message);
    }
  };

  const rejectRequest = async (requestId) => {
    try {
      await api.post(`/leave/requests/${requestId}/reject`, {});
      await load();
    } catch (err) {
      setError(err.message);
    }
  };

  const cancelRequest = async (requestId) => {
    try {
      await api.post(`/leave/requests/${requestId}/cancel`, {});
      await load();
    } catch (err) {
      setError(err.message);
    }
  };

  const exportCalendar = async (format) => {
    try {
      const file = await api.download(`/leave/calendar/export?format=${format}`);
      downloadBlob(file);
    } catch (err) {
      setError(err.message);
    }
  };

  return (
    <section className="page">
      <header className="page-header">
        <div>
          <h2>Leave & Absence</h2>
          <p>Request time off, manage policies, and review balances.</p>
        </div>
        {isHR && (
          <button type="button" onClick={runAccruals}>
            Run accruals
          </button>
        )}
      </header>

      {error && <div className="error">{error}</div>}
      {accrualSummary && (
        <div className="card">
          <h3>Accrual Run</h3>
          <p>Updated balances: {accrualSummary.updated || 0}</p>
          <p>Skipped: {accrualSummary.skipped || 0}</p>
        </div>
      )}

      <div className="card">
        <h3>Request Leave</h3>
        <form className="inline-form" onSubmit={submitRequest}>
          <select value={requestForm.leaveTypeId} onChange={(e) => setRequestForm({ ...requestForm, leaveTypeId: e.target.value })}>
            <option value="">Leave type</option>
            {types.map((type) => (
              <option key={type.id} value={type.id}>{type.name}</option>
            ))}
          </select>
          <input type="date" value={requestForm.startDate} onChange={(e) => setRequestForm({ ...requestForm, startDate: e.target.value })} />
          <input type="date" value={requestForm.endDate} onChange={(e) => setRequestForm({ ...requestForm, endDate: e.target.value })} />
          <input placeholder="Reason" value={requestForm.reason} onChange={(e) => setRequestForm({ ...requestForm, reason: e.target.value })} />
          <button type="submit">Submit request</button>
        </form>
      </div>

      <div className="table">
        <div className="table-row header">
          <span>Employee</span>
          <span>Type</span>
          <span>Dates</span>
          <span>Status</span>
          <span>Actions</span>
        </div>
        {requests.map((req) => (
          <div key={req.id} className="table-row">
            <span>{req.employeeId}</span>
            <span>{typeLookup[req.leaveTypeId] || req.leaveTypeId}</span>
            <span>{req.startDate?.slice(0, 10)} → {req.endDate?.slice(0, 10)}</span>
            <span>{req.status}</span>
            <span className="row-actions">
              {(isManager || isHR) && req.status === LEAVE_STATUS_PENDING && (
                <>
                  <button type="button" onClick={() => approveRequest(req.id)}>Approve</button>
                  <button type="button" className="ghost" onClick={() => rejectRequest(req.id)}>Reject</button>
                </>
              )}
              {isHR && req.status === LEAVE_STATUS_PENDING_HR && (
                <>
                  <button type="button" onClick={() => approveRequest(req.id)}>Approve</button>
                  <button type="button" className="ghost" onClick={() => rejectRequest(req.id)}>Reject</button>
                </>
              )}
              {!isHR && req.status === LEAVE_STATUS_PENDING && (
                <button type="button" className="ghost" onClick={() => cancelRequest(req.id)}>Cancel</button>
              )}
            </span>
          </div>
        ))}
      </div>
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

      <div className="card-grid">
        <div className="card">
          <h3>Balances</h3>
          <div className="table">
            <div className="table-row header">
              <span>Employee</span>
              <span>Type</span>
              <span>Balance</span>
              <span>Pending</span>
              <span>Used</span>
            </div>
            {balances.map((row) => (
              <div key={`${row.employeeId}-${row.leaveTypeId}`} className="table-row">
                <span>{row.employeeId}</span>
                <span>{typeLookup[row.leaveTypeId] || row.leaveTypeId}</span>
                <span>{row.balance}</span>
                <span>{row.pending}</span>
                <span>{row.used}</span>
              </div>
            ))}
          </div>
        </div>

        {isHR && (
          <div className="card">
            <h3>Adjust Balance</h3>
            <form className="inline-form" onSubmit={adjustBalance}>
              <input placeholder="Employee ID" value={adjustForm.employeeId} onChange={(e) => setAdjustForm({ ...adjustForm, employeeId: e.target.value })} />
              <select value={adjustForm.leaveTypeId} onChange={(e) => setAdjustForm({ ...adjustForm, leaveTypeId: e.target.value })}>
                <option value="">Leave type</option>
                {types.map((type) => (
                  <option key={type.id} value={type.id}>{type.name}</option>
                ))}
              </select>
              <input placeholder="Delta" value={adjustForm.delta} onChange={(e) => setAdjustForm({ ...adjustForm, delta: e.target.value })} />
              <input placeholder="Reason" value={adjustForm.reason} onChange={(e) => setAdjustForm({ ...adjustForm, reason: e.target.value })} />
              <button type="submit">Apply</button>
            </form>
          </div>
        )}
      </div>

      {isHR && (
        <div className="card-grid">
          <div className="card">
            <h3>Leave Types</h3>
            <form className="inline-form" onSubmit={createType}>
              <input placeholder="Name" value={typeForm.name} onChange={(e) => setTypeForm({ ...typeForm, name: e.target.value })} />
              <input placeholder="Code" value={typeForm.code} onChange={(e) => setTypeForm({ ...typeForm, code: e.target.value })} />
              <select value={typeForm.isPaid ? 'paid' : 'unpaid'} onChange={(e) => setTypeForm({ ...typeForm, isPaid: e.target.value === 'paid' })}>
                <option value="paid">Paid</option>
                <option value="unpaid">Unpaid</option>
              </select>
              <select value={typeForm.requiresDoc ? 'yes' : 'no'} onChange={(e) => setTypeForm({ ...typeForm, requiresDoc: e.target.value === 'yes' })}>
                <option value="no">No doc</option>
                <option value="yes">Requires doc</option>
              </select>
              <button type="submit">Add type</button>
            </form>
            <div className="table">
              <div className="table-row header">
                <span>Name</span>
                <span>Code</span>
                <span>Paid</span>
              </div>
              {types.map((type) => (
                <div key={type.id} className="table-row">
                  <span>{type.name}</span>
                  <span>{type.code}</span>
                  <span>{type.isPaid ? 'Yes' : 'No'}</span>
                </div>
              ))}
            </div>
          </div>

          <div className="card">
            <h3>Policies</h3>
            <form className="inline-form" onSubmit={createPolicy}>
              <select value={policyForm.leaveTypeId} onChange={(e) => setPolicyForm({ ...policyForm, leaveTypeId: e.target.value })}>
                <option value="">Leave type</option>
                {types.map((type) => (
                  <option key={type.id} value={type.id}>{type.name}</option>
                ))}
              </select>
              <input placeholder="Accrual rate" value={policyForm.accrualRate} onChange={(e) => setPolicyForm({ ...policyForm, accrualRate: e.target.value })} />
              <input placeholder="Entitlement" value={policyForm.entitlement} onChange={(e) => setPolicyForm({ ...policyForm, entitlement: e.target.value })} />
              <input placeholder="Carry over limit" value={policyForm.carryOverLimit} onChange={(e) => setPolicyForm({ ...policyForm, carryOverLimit: e.target.value })} />
              <select value={policyForm.accrualPeriod} onChange={(e) => setPolicyForm({ ...policyForm, accrualPeriod: e.target.value })}>
                <option value="monthly">Monthly</option>
                <option value="yearly">Yearly</option>
              </select>
              <select value={policyForm.allowNegative ? 'yes' : 'no'} onChange={(e) => setPolicyForm({ ...policyForm, allowNegative: e.target.value === 'yes' })}>
                <option value="no">No negative</option>
                <option value="yes">Allow negative</option>
              </select>
              <label className="checkbox">
                <input
                  type="checkbox"
                  checked={policyForm.requiresHrApproval}
                  onChange={(e) => setPolicyForm({ ...policyForm, requiresHrApproval: e.target.checked })}
                />
                Requires HR approval
              </label>
              <button type="submit">Add policy</button>
            </form>
            <div className="table">
              <div className="table-row header">
                <span>Type</span>
                <span>Accrual</span>
                <span>Entitlement</span>
                <span>HR approval</span>
              </div>
              {policies.map((policy) => (
                <div key={policy.id} className="table-row">
                  <span>{typeLookup[policy.leaveTypeId] || policy.leaveTypeId}</span>
                  <span>{policy.accrualRate} / {policy.accrualPeriod}</span>
                  <span>{policy.entitlement}</span>
                  <span>{policy.requiresHrApproval ? 'Required' : 'No'}</span>
                </div>
              ))}
            </div>
          </div>
        </div>
      )}

      {isHR && (
        <div className="card-grid">
          <div className="card">
            <h3>Holidays</h3>
            <form className="inline-form" onSubmit={createHoliday}>
              <input type="date" value={holidayForm.date} onChange={(e) => setHolidayForm({ ...holidayForm, date: e.target.value })} />
              <input placeholder="Name" value={holidayForm.name} onChange={(e) => setHolidayForm({ ...holidayForm, name: e.target.value })} />
              <input placeholder="Region" value={holidayForm.region} onChange={(e) => setHolidayForm({ ...holidayForm, region: e.target.value })} />
              <button type="submit">Add holiday</button>
            </form>
            <div className="table">
              <div className="table-row header">
                <span>Date</span>
                <span>Name</span>
                <span>Region</span>
                <span>Actions</span>
              </div>
              {holidays.map((holiday) => (
                <div key={holiday.id} className="table-row">
                  <span>{holiday.date?.slice(0, 10)}</span>
                  <span>{holiday.name}</span>
                  <span>{holiday.region || '-'}</span>
                  <span>
                    <button type="button" className="ghost" onClick={() => deleteHoliday(holiday.id)}>Delete</button>
                  </span>
                </div>
              ))}
            </div>
          </div>

          <div className="card">
            <h3>Calendar</h3>
            <div className="row-actions">
              <button type="button" onClick={() => exportCalendar('csv')}>Export CSV</button>
              <button type="button" className="ghost" onClick={() => exportCalendar('ics')}>Export ICS</button>
            </div>
            <div className="table">
              <div className="table-row header">
                <span>Employee</span>
                <span>Type</span>
                <span>Dates</span>
                <span>Status</span>
              </div>
              {calendar.map((item) => (
                <div key={item.id} className="table-row">
                  <span>{item.employeeId}</span>
                  <span>{typeLookup[item.leaveTypeId] || item.leaveTypeId}</span>
                  <span>{item.start?.slice(0, 10)} → {item.end?.slice(0, 10)}</span>
                  <span>{item.status}</span>
                </div>
              ))}
            </div>
          </div>
        </div>
      )}

      {isHR && (
        <div className="card-grid">
          <div className="card">
            <h3>Balance Report</h3>
            <div className="table">
              <div className="table-row header">
                <span>Employee</span>
                <span>Type</span>
                <span>Balance</span>
                <span>Pending</span>
                <span>Used</span>
              </div>
              {balanceReport.map((row) => (
                <div key={`${row.employeeId}-${row.leaveTypeId}`} className="table-row">
                  <span>{row.employeeId}</span>
                  <span>{typeLookup[row.leaveTypeId] || row.leaveTypeId}</span>
                  <span>{row.balance}</span>
                  <span>{row.pending}</span>
                  <span>{row.used}</span>
                </div>
              ))}
            </div>
          </div>

          <div className="card">
            <h3>Usage Report</h3>
            <div className="table">
              <div className="table-row header">
                <span>Type</span>
                <span>Total days</span>
              </div>
              {usageReport.map((row) => (
                <div key={row.leaveTypeId} className="table-row">
                  <span>{typeLookup[row.leaveTypeId] || row.leaveTypeId}</span>
                  <span>{row.totalDays}</span>
                </div>
              ))}
            </div>
          </div>
        </div>
      )}
    </section>
  );
}
