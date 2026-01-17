import React, { useEffect, useMemo, useState } from 'react';
import { api } from '../../../services/apiClient.js';
import { useAuth } from '../../auth/auth.jsx';
import { ROLE_HR } from '../../../shared/constants/roles.js';
import {
  PAYROLL_PERIOD_DRAFT,
  PAYROLL_PERIOD_REVIEWED,
  PAYROLL_PERIOD_FINALIZED,
} from '../../../shared/constants/statuses.js';
import {
  PAYROLL_FREQUENCIES,
  PAYROLL_ELEMENT_TYPES,
  PAYROLL_CALC_TYPES,
  PAYROLL_INPUT_SOURCES,
} from '../../../shared/constants/payroll.js';

const downloadBlob = ({ blob, filename }) => {
  const url = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  link.download = filename;
  link.click();
  URL.revokeObjectURL(url);
};

export default function Payroll() {
  const { user, employee } = useAuth();
  const isHR = (user?.role || user?.RoleName) === ROLE_HR;

  const [schedules, setSchedules] = useState([]);
  const [groups, setGroups] = useState([]);
  const [elements, setElements] = useState([]);
  const [journalTemplates, setJournalTemplates] = useState([]);
  const [periods, setPeriods] = useState([]);
  const [inputs, setInputs] = useState([]);
  const [adjustments, setAdjustments] = useState([]);
  const [summary, setSummary] = useState(null);
  const [payslips, setPayslips] = useState([]);
  const [selectedPeriodId, setSelectedPeriodId] = useState('');
  const [error, setError] = useState('');
  const [importMessage, setImportMessage] = useState('');

  const [scheduleForm, setScheduleForm] = useState({ name: '', frequency: 'monthly', payDay: '' });
  const [groupForm, setGroupForm] = useState({ name: '', scheduleId: '', currency: 'USD' });
  const [elementForm, setElementForm] = useState({
    name: '',
    elementType: 'earning',
    calcType: 'fixed',
    amount: '',
    taxable: true,
  });
  const [periodForm, setPeriodForm] = useState({ scheduleId: '', startDate: '', endDate: '' });
  const [inputForm, setInputForm] = useState({
    employeeId: '',
    elementId: '',
    units: '',
    rate: '',
    amount: '',
    source: 'manual',
  });
  const [adjustmentForm, setAdjustmentForm] = useState({ employeeId: '', description: '', amount: '', effectiveDate: '' });
  const [journalTemplateForm, setJournalTemplateForm] = useState({
    name: '',
    expenseAccount: '',
    deductionAccount: '',
    cashAccount: '',
    headers: '',
  });
  const [journalTemplateId, setJournalTemplateId] = useState('');
  const [importFile, setImportFile] = useState(null);

  const scheduleLookup = useMemo(() => {
    return schedules.reduce((acc, s) => {
      acc[s.id] = s.name;
      return acc;
    }, {});
  }, [schedules]);

  const elementLookup = useMemo(() => {
    return elements.reduce((acc, e) => {
      acc[e.id] = e.name;
      return acc;
    }, {});
  }, [elements]);

  const loadBase = async () => {
    setError('');
    try {
      const payslipPath = employee?.id ? `/payroll/payslips?employeeId=${employee.id}` : '/payroll/payslips';
      const results = await Promise.allSettled([
        api.get('/payroll/schedules'),
        api.get('/payroll/groups'),
        api.get('/payroll/elements'),
        api.get('/payroll/journal-templates'),
        api.get('/payroll/periods'),
        api.get(payslipPath),
      ]);

      const setters = [setSchedules, setGroups, setElements, setJournalTemplates, setPeriods, setPayslips];
      results.forEach((result, idx) => {
        if (result.status === 'fulfilled') {
          setters[idx](Array.isArray(result.value) ? result.value : []);
        } else if (!error) {
          setError(result.reason?.message || 'Failed to load payroll data');
        }
      });

      const periodList = results[4]?.status === 'fulfilled' ? results[4].value : [];
      if (!selectedPeriodId && Array.isArray(periodList) && periodList.length > 0) {
        setSelectedPeriodId(periodList[0].id);
      }
    } catch (err) {
      setError(err.message);
    }
  };

  const loadPeriodDetails = async (periodId) => {
    if (!periodId) {
      setInputs([]);
      setAdjustments([]);
      setSummary(null);
      return;
    }
    setError('');
    try {
      const requests = [
        api.get(`/payroll/periods/${periodId}/inputs`),
        api.get(`/payroll/periods/${periodId}/adjustments`),
      ];
      if (isHR) {
        requests.push(api.get(`/payroll/periods/${periodId}/summary`));
      }
      const results = await Promise.allSettled(requests);
      if (results[0]?.status === 'fulfilled') {
        setInputs(Array.isArray(results[0].value) ? results[0].value : []);
      }
      if (results[1]?.status === 'fulfilled') {
        setAdjustments(Array.isArray(results[1].value) ? results[1].value : []);
      }
      if (isHR) {
        if (results[2]?.status === 'fulfilled') {
          setSummary(results[2].value);
        } else {
          setSummary(null);
        }
      }
      const failure = results.find((res) => res.status === 'rejected');
      if (failure && !error) {
        setError(failure.reason?.message || 'Failed to load period details');
      }
    } catch (err) {
      setError(err.message);
    }
  };

  useEffect(() => {
    loadBase();
  }, [employee]);

  useEffect(() => {
    loadPeriodDetails(selectedPeriodId);
  }, [selectedPeriodId, isHR]);

  const createSchedule = async (e) => {
    e.preventDefault();
    setError('');
    try {
      await api.post('/payroll/schedules', {
        name: scheduleForm.name,
        frequency: scheduleForm.frequency,
        payDay: scheduleForm.payDay ? Number(scheduleForm.payDay) : null,
      });
      setScheduleForm({ name: '', frequency: 'monthly', payDay: '' });
      await loadBase();
    } catch (err) {
      setError(err.message);
    }
  };

  const createGroup = async (e) => {
    e.preventDefault();
    setError('');
    try {
      await api.post('/payroll/groups', {
        name: groupForm.name,
        scheduleId: groupForm.scheduleId,
        currency: groupForm.currency,
      });
      setGroupForm({ name: '', scheduleId: '', currency: 'USD' });
      await loadBase();
    } catch (err) {
      setError(err.message);
    }
  };

  const createElement = async (e) => {
    e.preventDefault();
    setError('');
    try {
      await api.post('/payroll/elements', {
        name: elementForm.name,
        elementType: elementForm.elementType,
        calcType: elementForm.calcType,
        amount: Number(elementForm.amount || 0),
        taxable: elementForm.taxable,
      });
      setElementForm({ name: '', elementType: 'earning', calcType: 'fixed', amount: '', taxable: true });
      await loadBase();
    } catch (err) {
      setError(err.message);
    }
  };

  const createJournalTemplate = async (e) => {
    e.preventDefault();
    setError('');
    try {
      const headers = journalTemplateForm.headers
        ? journalTemplateForm.headers.split(',').map((h) => h.trim()).filter(Boolean)
        : [];
      await api.post('/payroll/journal-templates', {
        name: journalTemplateForm.name,
        config: {
          expenseAccount: journalTemplateForm.expenseAccount,
          deductionAccount: journalTemplateForm.deductionAccount,
          cashAccount: journalTemplateForm.cashAccount,
          headers,
        },
      });
      setJournalTemplateForm({
        name: '',
        expenseAccount: '',
        deductionAccount: '',
        cashAccount: '',
        headers: '',
      });
      await loadBase();
    } catch (err) {
      setError(err.message);
    }
  };

  const createPeriod = async (e) => {
    e.preventDefault();
    setError('');
    try {
      await api.post('/payroll/periods', periodForm);
      setPeriodForm({ scheduleId: '', startDate: '', endDate: '' });
      await loadBase();
    } catch (err) {
      setError(err.message);
    }
  };

  const runPayroll = async (id) => {
    try {
      await api.post(`/payroll/periods/${id}/run`, {});
      await loadBase();
      await loadPeriodDetails(id);
    } catch (err) {
      setError(err.message);
    }
  };

  const finalizePayroll = async (id) => {
    try {
      await api.post(`/payroll/periods/${id}/finalize`, {});
      await loadBase();
      await loadPeriodDetails(id);
    } catch (err) {
      setError(err.message);
    }
  };

  const reopenPayroll = async (id) => {
    const reason = window.prompt('Reason for reopening this payroll period?');
    if (!reason) {
      return;
    }
    try {
      await api.post(`/payroll/periods/${id}/reopen`, { reason });
      await loadBase();
      await loadPeriodDetails(id);
    } catch (err) {
      setError(err.message);
    }
  };

  const createInput = async (e) => {
    e.preventDefault();
    if (!selectedPeriodId) {
      setError('Select a payroll period first');
      return;
    }
    try {
      await api.post(`/payroll/periods/${selectedPeriodId}/inputs`, {
        employeeId: inputForm.employeeId,
        elementId: inputForm.elementId,
        units: Number(inputForm.units || 0),
        rate: Number(inputForm.rate || 0),
        amount: Number(inputForm.amount || 0),
        source: inputForm.source,
      });
      setInputForm({ employeeId: '', elementId: '', units: '', rate: '', amount: '', source: 'manual' });
      await loadPeriodDetails(selectedPeriodId);
    } catch (err) {
      setError(err.message);
    }
  };

  const importInputs = async (e) => {
    e.preventDefault();
    if (!selectedPeriodId) {
      setError('Select a payroll period first');
      return;
    }
    if (!importFile) {
      setError('Select a CSV file to import');
      return;
    }
    setImportMessage('');
    try {
      const payload = await importFile.text();
      const response = await api.postRaw(`/payroll/periods/${selectedPeriodId}/inputs/import`, payload, 'text/csv');
      setImportMessage(`Imported ${response.imported} inputs.`);
      setImportFile(null);
      await loadPeriodDetails(selectedPeriodId);
    } catch (err) {
      setError(err.message);
    }
  };

  const createAdjustment = async (e) => {
    e.preventDefault();
    if (!selectedPeriodId) {
      setError('Select a payroll period first');
      return;
    }
    try {
      await api.post(`/payroll/periods/${selectedPeriodId}/adjustments`, {
        employeeId: adjustmentForm.employeeId,
        description: adjustmentForm.description,
        amount: Number(adjustmentForm.amount || 0),
        effectiveDate: adjustmentForm.effectiveDate,
      });
      setAdjustmentForm({ employeeId: '', description: '', amount: '', effectiveDate: '' });
      await loadPeriodDetails(selectedPeriodId);
    } catch (err) {
      setError(err.message);
    }
  };

  const exportRegister = async (id) => {
    try {
      const result = await api.download(`/payroll/periods/${id}/export/register`);
      downloadBlob(result);
    } catch (err) {
      setError(err.message);
    }
  };

  const exportJournal = async (id) => {
    try {
      const templateParam = journalTemplateId ? `?templateId=${journalTemplateId}` : '';
      const result = await api.download(`/payroll/periods/${id}/export/journal${templateParam}`);
      downloadBlob(result);
    } catch (err) {
      setError(err.message);
    }
  };

  const downloadPayslip = async (id) => {
    try {
      const result = await api.download(`/payroll/payslips/${id}/download`);
      downloadBlob(result);
    } catch (err) {
      setError(err.message);
    }
  };

  const regeneratePayslip = async (id) => {
    try {
      await api.post(`/payroll/payslips/${id}/regenerate`, {});
      await loadBase();
    } catch (err) {
      setError(err.message);
    }
  };

  return (
    <section className="page">
      <header className="page-header">
        <div>
          <h2>Payroll</h2>
          <p>Configure schedules, run payroll, and manage payslips.</p>
        </div>
      </header>

      {error && <div className="error">{error}</div>}
      {importMessage && <div className="success">{importMessage}</div>}

      {isHR && (
        <div className="card-grid">
          <div className="card">
            <h3>Pay schedules</h3>
            <form className="stack" onSubmit={createSchedule}>
              <input
                placeholder="Schedule name"
                value={scheduleForm.name}
                onChange={(e) => setScheduleForm({ ...scheduleForm, name: e.target.value })}
                required
              />
              <select
                value={scheduleForm.frequency}
                onChange={(e) => setScheduleForm({ ...scheduleForm, frequency: e.target.value })}
              >
                {PAYROLL_FREQUENCIES.map((freq) => (
                  <option key={freq.value} value={freq.value}>
                    {freq.label}
                  </option>
                ))}
              </select>
              <input
                type="number"
                min="1"
                max="31"
                placeholder="Pay day (optional)"
                value={scheduleForm.payDay}
                onChange={(e) => setScheduleForm({ ...scheduleForm, payDay: e.target.value })}
              />
              <button type="submit">Add schedule</button>
            </form>
            <div className="list">
              {schedules.map((schedule) => (
                <div key={schedule.id} className="list-item">
                  <div>
                    <strong>{schedule.name}</strong>
                    <p>{schedule.frequency}</p>
                  </div>
                  <small>Pay day: {schedule.payDay ?? '—'}</small>
                </div>
              ))}
            </div>
          </div>

          <div className="card">
            <h3>Pay groups</h3>
            <form className="stack" onSubmit={createGroup}>
              <input
                placeholder="Group name"
                value={groupForm.name}
                onChange={(e) => setGroupForm({ ...groupForm, name: e.target.value })}
                required
              />
              <select
                value={groupForm.scheduleId}
                onChange={(e) => setGroupForm({ ...groupForm, scheduleId: e.target.value })}
              >
                <option value="">Select schedule</option>
                {schedules.map((schedule) => (
                  <option key={schedule.id} value={schedule.id}>
                    {schedule.name}
                  </option>
                ))}
              </select>
              <input
                placeholder="Currency"
                value={groupForm.currency}
                onChange={(e) => setGroupForm({ ...groupForm, currency: e.target.value })}
              />
              <button type="submit">Add group</button>
            </form>
            <div className="list">
              {groups.map((group) => (
                <div key={group.id} className="list-item">
                  <div>
                    <strong>{group.name}</strong>
                    <p>{scheduleLookup[group.scheduleId] || 'No schedule'}</p>
                  </div>
                  <small>{group.currency}</small>
                </div>
              ))}
            </div>
          </div>

          <div className="card">
            <h3>Pay elements</h3>
            <form className="stack" onSubmit={createElement}>
              <input
                placeholder="Element name"
                value={elementForm.name}
                onChange={(e) => setElementForm({ ...elementForm, name: e.target.value })}
                required
              />
              <select
                value={elementForm.elementType}
                onChange={(e) => setElementForm({ ...elementForm, elementType: e.target.value })}
              >
                {PAYROLL_ELEMENT_TYPES.map((type) => (
                  <option key={type.value} value={type.value}>
                    {type.label}
                  </option>
                ))}
              </select>
              <select
                value={elementForm.calcType}
                onChange={(e) => setElementForm({ ...elementForm, calcType: e.target.value })}
              >
                {PAYROLL_CALC_TYPES.map((type) => (
                  <option key={type.value} value={type.value}>
                    {type.label}
                  </option>
                ))}
              </select>
              <input
                type="number"
                step="0.01"
                placeholder="Default amount"
                value={elementForm.amount}
                onChange={(e) => setElementForm({ ...elementForm, amount: e.target.value })}
              />
              <label className="checkbox">
                <input
                  type="checkbox"
                  checked={elementForm.taxable}
                  onChange={(e) => setElementForm({ ...elementForm, taxable: e.target.checked })}
                />
                Taxable
              </label>
              <button type="submit">Add element</button>
            </form>
            <div className="list">
              {elements.map((element) => (
                <div key={element.id} className="list-item">
                  <div>
                    <strong>{element.name}</strong>
                    <p>{element.elementType} · {element.calcType}</p>
                  </div>
                  <small>{element.amount}</small>
                </div>
              ))}
            </div>
          </div>

          <div className="card">
            <h3>Journal templates</h3>
            <form className="stack" onSubmit={createJournalTemplate}>
              <input
                placeholder="Template name"
                value={journalTemplateForm.name}
                onChange={(e) => setJournalTemplateForm({ ...journalTemplateForm, name: e.target.value })}
                required
              />
              <input
                placeholder="Expense account"
                value={journalTemplateForm.expenseAccount}
                onChange={(e) => setJournalTemplateForm({ ...journalTemplateForm, expenseAccount: e.target.value })}
              />
              <input
                placeholder="Deduction account"
                value={journalTemplateForm.deductionAccount}
                onChange={(e) => setJournalTemplateForm({ ...journalTemplateForm, deductionAccount: e.target.value })}
              />
              <input
                placeholder="Cash account"
                value={journalTemplateForm.cashAccount}
                onChange={(e) => setJournalTemplateForm({ ...journalTemplateForm, cashAccount: e.target.value })}
              />
              <input
                placeholder="Headers (comma-separated)"
                value={journalTemplateForm.headers}
                onChange={(e) => setJournalTemplateForm({ ...journalTemplateForm, headers: e.target.value })}
              />
              <button type="submit">Add template</button>
            </form>
            <div className="list">
              {journalTemplates.map((template) => (
                <div key={template.id} className="list-item">
                  <div>
                    <strong>{template.name}</strong>
                    <p>{template.config?.expenseAccount || 'Default mapping'}</p>
                  </div>
                  <small>{template.createdAt?.slice(0, 10)}</small>
                </div>
              ))}
            </div>
          </div>
        </div>
      )}

      {isHR && (
        <form className="inline-form" onSubmit={createPeriod}>
          <select
            value={periodForm.scheduleId}
            onChange={(e) => setPeriodForm({ ...periodForm, scheduleId: e.target.value })}
          >
            <option value="">Select schedule</option>
            {schedules.map((schedule) => (
              <option key={schedule.id} value={schedule.id}>
                {schedule.name}
              </option>
            ))}
          </select>
          <input
            type="date"
            value={periodForm.startDate}
            onChange={(e) => setPeriodForm({ ...periodForm, startDate: e.target.value })}
            required
          />
          <input
            type="date"
            value={periodForm.endDate}
            onChange={(e) => setPeriodForm({ ...periodForm, endDate: e.target.value })}
            required
          />
          <button type="submit">Create period</button>
        </form>
      )}

      {isHR && (
        <div className="row-actions">
          <label className="inline-note">Journal template</label>
          <select value={journalTemplateId} onChange={(e) => setJournalTemplateId(e.target.value)}>
            <option value="">Default template</option>
            {journalTemplates.map((template) => (
              <option key={template.id} value={template.id}>{template.name}</option>
            ))}
          </select>
        </div>
      )}

      <div className="table">
        <div className="table-row header">
          <span>Period</span>
          <span>Status</span>
          <span>Schedule</span>
          <span>Actions</span>
        </div>
        {periods.map((period) => (
          <div key={period.id} className="table-row">
            <span>{period.startDate?.slice(0, 10)} → {period.endDate?.slice(0, 10)}</span>
            <span>{period.status}</span>
            <span>{scheduleLookup[period.scheduleId] || period.scheduleId}</span>
            <span className="row-actions">
              <button onClick={() => setSelectedPeriodId(period.id)}>Details</button>
              {isHR && (
                <>
                  <button onClick={() => runPayroll(period.id)} disabled={period.status !== PAYROLL_PERIOD_DRAFT}>Run</button>
                  <button onClick={() => finalizePayroll(period.id)} disabled={period.status !== PAYROLL_PERIOD_REVIEWED}>Finalize</button>
                  <button onClick={() => reopenPayroll(period.id)} disabled={period.status !== PAYROLL_PERIOD_FINALIZED}>Reopen</button>
                  <button onClick={() => exportRegister(period.id)}>Export register</button>
                  <button onClick={() => exportJournal(period.id)}>Export journal</button>
                </>
              )}
            </span>
          </div>
        ))}
      </div>

      {selectedPeriodId && (
        <div className="card-grid">
          <div className="card">
            <h3>Inputs</h3>
            {isHR && (
              <form className="stack" onSubmit={createInput}>
                <input
                  placeholder="Employee ID"
                  value={inputForm.employeeId}
                  onChange={(e) => setInputForm({ ...inputForm, employeeId: e.target.value })}
                  required
                />
                <select
                  value={inputForm.elementId}
                  onChange={(e) => setInputForm({ ...inputForm, elementId: e.target.value })}
                  required
                >
                  <option value="">Select element</option>
                  {elements.map((element) => (
                    <option key={element.id} value={element.id}>
                      {element.name}
                    </option>
                  ))}
                </select>
                <input
                  type="number"
                  step="0.01"
                  placeholder="Units"
                  value={inputForm.units}
                  onChange={(e) => setInputForm({ ...inputForm, units: e.target.value })}
                />
                <input
                  type="number"
                  step="0.01"
                  placeholder="Rate"
                  value={inputForm.rate}
                  onChange={(e) => setInputForm({ ...inputForm, rate: e.target.value })}
                />
                <input
                  type="number"
                  step="0.01"
                  placeholder="Amount"
                  value={inputForm.amount}
                  onChange={(e) => setInputForm({ ...inputForm, amount: e.target.value })}
                />
                <select
                  value={inputForm.source}
                  onChange={(e) => setInputForm({ ...inputForm, source: e.target.value })}
                >
                  {PAYROLL_INPUT_SOURCES.map((source) => (
                    <option key={source.value} value={source.value}>
                      {source.label}
                    </option>
                  ))}
                </select>
                <button type="submit">Add input</button>
              </form>
            )}
            <div className="table">
              <div className="table-row header">
                <span>Employee</span>
                <span>Element</span>
                <span>Units</span>
                <span>Amount</span>
              </div>
              {inputs.map((input, idx) => (
                <div key={`${input.employeeId}-${idx}`} className="table-row">
                  <span>{input.employeeId}</span>
                  <span>{elementLookup[input.elementId] || input.elementId}</span>
                  <span>{input.units}</span>
                  <span>{input.amount}</span>
                </div>
              ))}
            </div>
            {isHR && (
              <form className="stack" onSubmit={importInputs}>
                <input type="file" accept=".csv" onChange={(e) => setImportFile(e.target.files?.[0] || null)} />
                <button type="submit">Import CSV</button>
              </form>
            )}
          </div>

          <div className="card">
            <h3>Adjustments</h3>
            {isHR && (
              <form className="stack" onSubmit={createAdjustment}>
                <input
                  placeholder="Employee ID"
                  value={adjustmentForm.employeeId}
                  onChange={(e) => setAdjustmentForm({ ...adjustmentForm, employeeId: e.target.value })}
                  required
                />
                <input
                  placeholder="Description"
                  value={adjustmentForm.description}
                  onChange={(e) => setAdjustmentForm({ ...adjustmentForm, description: e.target.value })}
                  required
                />
                <input
                  type="number"
                  step="0.01"
                  placeholder="Amount"
                  value={adjustmentForm.amount}
                  onChange={(e) => setAdjustmentForm({ ...adjustmentForm, amount: e.target.value })}
                  required
                />
                <input
                  type="date"
                  value={adjustmentForm.effectiveDate}
                  onChange={(e) => setAdjustmentForm({ ...adjustmentForm, effectiveDate: e.target.value })}
                />
                <button type="submit">Add adjustment</button>
              </form>
            )}
            <div className="table">
              <div className="table-row header">
                <span>Employee</span>
                <span>Description</span>
                <span>Amount</span>
                <span>Effective</span>
              </div>
              {adjustments.map((adj) => (
                <div key={adj.id} className="table-row">
                  <span>{adj.employeeId}</span>
                  <span>{adj.description}</span>
                  <span>{adj.amount}</span>
                  <span>{adj.effectiveDate || '—'}</span>
                </div>
              ))}
            </div>
          </div>

          {isHR && (
            <div className="card">
              <h3>Summary</h3>
              {summary ? (
                <>
                  <p><strong>Total gross:</strong> {summary.totalGross}</p>
                  <p><strong>Total deductions:</strong> {summary.totalDeductions}</p>
                  <p><strong>Total net:</strong> {summary.totalNet}</p>
                  <p><strong>Employees:</strong> {summary.employeeCount}</p>
                  {summary.warnings && (
                    <div>
                      <strong>Warnings</strong>
                      <ul>
                        {Object.entries(summary.warnings).map(([key, value]) => (
                          <li key={key}>{key}: {value}</li>
                        ))}
                      </ul>
                    </div>
                  )}
                </>
              ) : (
                <p>No summary yet.</p>
              )}
            </div>
          )}
        </div>
      )}

      <h3>Payslips</h3>
      <div className="table">
        <div className="table-row header">
          <span>Period</span>
          <span>Net</span>
          <span>Currency</span>
          <span>Actions</span>
        </div>
        {payslips.map((slip) => (
          <div key={slip.id} className="table-row">
            <span>{slip.periodId}</span>
            <span>{slip.net}</span>
            <span>{slip.currency}</span>
            <span className="row-actions">
              <button onClick={() => downloadPayslip(slip.id)}>Download</button>
              {isHR && <button onClick={() => regeneratePayslip(slip.id)}>Regenerate</button>}
            </span>
          </div>
        ))}
      </div>
    </section>
  );
}
