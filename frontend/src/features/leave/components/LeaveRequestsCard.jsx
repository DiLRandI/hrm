import React from 'react';
import { LEAVE_STATUS_PENDING, LEAVE_STATUS_PENDING_HR } from '../../../shared/constants/statuses.js';

export default function LeaveRequestsCard({
  types,
  typeLookup,
  requests,
  requestForm,
  onFormChange,
  onDocumentsChange,
  onSubmit,
  isManager,
  isHR,
  onApprove,
  onReject,
  onCancel,
  onDownloadDocument,
  requiresDocument,
  disabled,
}) {
  return (
    <div className="card">
      <h3>Request Leave</h3>
      <form className="inline-form" onSubmit={onSubmit} aria-label="Request leave">
        <select
          aria-label="Leave type"
          value={requestForm.leaveTypeId}
          onChange={(e) => onFormChange('leaveTypeId', e.target.value)}
        >
          <option value="">Leave type</option>
          {types.map((type) => (
            <option key={type.id} value={type.id}>{type.name}</option>
          ))}
        </select>
        <input
          aria-label="Start date"
          type="date"
          value={requestForm.startDate}
          onChange={(e) => onFormChange('startDate', e.target.value)}
        />
        <input
          aria-label="End date"
          type="date"
          value={requestForm.endDate}
          onChange={(e) => onFormChange('endDate', e.target.value)}
        />
        <select
          aria-label="Start day duration"
          value={requestForm.startHalf ? 'half' : 'full'}
          onChange={(e) => onFormChange('startHalf', e.target.value === 'half')}
        >
          <option value="full">Start full day</option>
          <option value="half">Start half day</option>
        </select>
        <select
          aria-label="End day duration"
          value={requestForm.endHalf ? 'half' : 'full'}
          onChange={(e) => onFormChange('endHalf', e.target.value === 'half')}
        >
          <option value="full">End full day</option>
          <option value="half">End half day</option>
        </select>
        <input
          aria-label="Supporting documents"
          type="file"
          multiple
          onChange={(e) => onDocumentsChange(Array.from(e.target.files || []))}
        />
        <input
          aria-label="Reason"
          placeholder="Reason"
          value={requestForm.reason}
          onChange={(e) => onFormChange('reason', e.target.value)}
        />
        <button type="submit" disabled={disabled}>Submit request</button>
      </form>
      {requiresDocument && (
        <small className="hint">Supporting document is required for this leave type.</small>
      )}

      <div className="table">
        <div className="table-row header">
          <span>Employee</span>
          <span>Type</span>
          <span>Dates</span>
          <span>Days</span>
          <span>Documents</span>
          <span>Status</span>
          <span>Actions</span>
        </div>
        {requests.map((req) => (
          <div key={req.id} className="table-row">
            <span>{req.employeeId}</span>
            <span>{typeLookup[req.leaveTypeId] || req.leaveTypeId}</span>
            <span>
              {req.startDate?.slice(0, 10)} {req.startHalf ? '(half)' : '(full)'} → {req.endDate?.slice(0, 10)}{' '}
              {req.endHalf ? '(half)' : '(full)'}
            </span>
            <span>{req.days}</span>
            <span className="row-actions">
              {(req.documents || []).map((doc) => (
                <button key={doc.id} type="button" className="ghost" onClick={() => onDownloadDocument(req.id, doc.id)}>
                  {doc.fileName}
                </button>
              ))}
            </span>
            <span>{req.status}</span>
            <span className="row-actions">
              {(isManager || isHR) && req.status === LEAVE_STATUS_PENDING && (
                <>
                  <button type="button" onClick={() => onApprove(req.id)}>Approve</button>
                  <button type="button" className="ghost" onClick={() => onReject(req.id)}>Reject</button>
                </>
              )}
              {isHR && req.status === LEAVE_STATUS_PENDING_HR && (
                <>
                  <button type="button" onClick={() => onApprove(req.id)}>Approve</button>
                  <button type="button" className="ghost" onClick={() => onReject(req.id)}>Reject</button>
                </>
              )}
              {!isHR && req.status === LEAVE_STATUS_PENDING && (
                <button type="button" className="ghost" onClick={() => onCancel(req.id)}>Cancel</button>
              )}
            </span>
          </div>
        ))}
      </div>
    </div>
  );
}
