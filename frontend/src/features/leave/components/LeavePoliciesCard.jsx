import React from 'react';

export default function LeavePoliciesCard({ types, policies, typeLookup, form, onChange, onSubmit, disabled }) {
  return (
    <div className="card">
      <h3>Policies</h3>
      <form className="inline-form" onSubmit={onSubmit} aria-label="Add leave policy">
        <select
          aria-label="Leave type"
          value={form.leaveTypeId}
          onChange={(e) => onChange('leaveTypeId', e.target.value)}
        >
          <option value="">Leave type</option>
          {types.map((type) => (
            <option key={type.id} value={type.id}>{type.name}</option>
          ))}
        </select>
        <input
          aria-label="Accrual rate"
          placeholder="Accrual rate"
          value={form.accrualRate}
          onChange={(e) => onChange('accrualRate', e.target.value)}
        />
        <input
          aria-label="Entitlement"
          placeholder="Entitlement"
          value={form.entitlement}
          onChange={(e) => onChange('entitlement', e.target.value)}
        />
        <input
          aria-label="Carry over limit"
          placeholder="Carry over limit"
          value={form.carryOverLimit}
          onChange={(e) => onChange('carryOverLimit', e.target.value)}
        />
        <select
          aria-label="Accrual period"
          value={form.accrualPeriod}
          onChange={(e) => onChange('accrualPeriod', e.target.value)}
        >
          <option value="monthly">Monthly</option>
          <option value="yearly">Yearly</option>
        </select>
        <select
          aria-label="Allow negative"
          value={form.allowNegative ? 'yes' : 'no'}
          onChange={(e) => onChange('allowNegative', e.target.value === 'yes')}
        >
          <option value="no">No negative</option>
          <option value="yes">Allow negative</option>
        </select>
        <label className="checkbox">
          <input
            type="checkbox"
            checked={form.requiresHrApproval}
            onChange={(e) => onChange('requiresHrApproval', e.target.checked)}
          />
          Requires HR approval
        </label>
        <button type="submit" disabled={disabled}>Add policy</button>
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
  );
}
