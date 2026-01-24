import React from 'react';

export default function RolePermissions({
  roles,
  permissions,
  selectedRole,
  selectedPerms,
  onSelectRole,
  onTogglePerm,
  onSave,
  disabled,
}) {
  return (
    <div className="card">
      <h3>Roles & permissions</h3>
      <div className="inline-form">
        <select
          aria-label="Role"
          value={selectedRole}
          onChange={(e) => onSelectRole(e.target.value)}
        >
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
                onChange={() => onTogglePerm(perm.key)}
              />
              {perm.key}
            </label>
          ))}
        </div>
      )}
      <button type="button" onClick={onSave} disabled={disabled || !selectedRole}>
        Save role permissions
      </button>
    </div>
  );
}
