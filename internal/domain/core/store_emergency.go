package core

import (
	"context"
)

func (s *Store) ListEmergencyContacts(ctx context.Context, tenantID, employeeID string) ([]EmergencyContact, error) {
	rows, err := s.DB.Query(ctx, `
    SELECT id,
           employee_id,
           full_name,
           relationship,
           COALESCE(phone, ''),
           COALESCE(email, ''),
           COALESCE(address, ''),
           is_primary,
           created_at,
           updated_at
    FROM employee_emergency_contacts
    WHERE tenant_id = $1 AND employee_id = $2
    ORDER BY is_primary DESC, created_at ASC
  `, tenantID, employeeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []EmergencyContact
	for rows.Next() {
		var contact EmergencyContact
		if err := rows.Scan(
			&contact.ID,
			&contact.EmployeeID,
			&contact.FullName,
			&contact.Relationship,
			&contact.Phone,
			&contact.Email,
			&contact.Address,
			&contact.IsPrimary,
			&contact.CreatedAt,
			&contact.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, contact)
	}
	return out, rows.Err()
}

func (s *Store) ReplaceEmergencyContacts(ctx context.Context, tenantID, employeeID string, contacts []EmergencyContact) error {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
    DELETE FROM employee_emergency_contacts
    WHERE tenant_id = $1 AND employee_id = $2
  `, tenantID, employeeID); err != nil {
		return err
	}

	for _, contact := range contacts {
		if contact.FullName == "" || contact.Relationship == "" {
			continue
		}
		if _, err := tx.Exec(ctx, `
      INSERT INTO employee_emergency_contacts
        (tenant_id, employee_id, full_name, relationship, phone, email, address, is_primary)
      VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
    `, tenantID, employeeID, contact.FullName, contact.Relationship, nullIfEmpty(contact.Phone),
			nullIfEmpty(contact.Email), nullIfEmpty(contact.Address), contact.IsPrimary); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}
