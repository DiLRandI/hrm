import React from 'react';
import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import Dashboard from './Dashboard.jsx';

const apiGetMock = vi.fn().mockResolvedValue({ payrollPeriods: 2 });

vi.mock('../api.js', () => ({
  api: { get: apiGetMock },
}));

vi.mock('../auth.jsx', () => ({
  useAuth: () => ({ user: { role: 'HR' }, employee: { firstName: 'Ava' } }),
}));

describe('Dashboard', () => {
  it('loads HR dashboard metrics', async () => {
    render(<Dashboard />);

    const metricTitle = await screen.findByText('payrollPeriods');
    expect(metricTitle).toBeInTheDocument();
    expect(apiGetMock).toHaveBeenCalledWith('/reports/dashboard/hr');
  });
});
