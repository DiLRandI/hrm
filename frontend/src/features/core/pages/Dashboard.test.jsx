import React from 'react';
import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import Dashboard from './Dashboard.jsx';
import { api } from '../../../services/apiClient.js';
import { ROLE_HR } from '../../../shared/constants/roles.js';

vi.mock('../../../services/apiClient.js', () => ({
  api: { get: vi.fn() },
}));

vi.mock('../../auth/auth.jsx', () => ({
  useAuth: () => ({ user: { role: ROLE_HR }, employee: { firstName: 'Ava' } }),
}));

describe('Dashboard', () => {
  it('loads HR dashboard metrics', async () => {
    api.get.mockResolvedValue({ payrollPeriods: 2 });
    render(<Dashboard />);

    const metricTitle = await screen.findByText('payrollPeriods');
    expect(metricTitle).toBeInTheDocument();
    expect(api.get).toHaveBeenCalledWith('/reports/dashboard/hr', expect.any(Object));
  });
});
