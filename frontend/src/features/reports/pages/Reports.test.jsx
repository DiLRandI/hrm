import React from 'react';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import userEvent from '@testing-library/user-event';
import Reports from './Reports.jsx';
import { api } from '../../../services/apiClient.js';

vi.mock('../../../services/apiClient.js', () => ({
  api: {
    get: vi.fn(),
    getWithMeta: vi.fn(),
    download: vi.fn(),
  },
}));

vi.mock('../../auth/auth.jsx', () => ({
  useAuth: () => ({
    user: { role: 'HR' },
  }),
}));

describe('Reports page', () => {
  beforeEach(() => {
    api.get.mockReset();
    api.getWithMeta.mockReset();
    api.download.mockReset();
    globalThis.URL.createObjectURL = vi.fn(() => 'blob:test');
    globalThis.URL.revokeObjectURL = vi.fn();
    vi.spyOn(window.HTMLAnchorElement.prototype, 'click').mockImplementation(() => {});
  });

  afterEach(() => {
    delete globalThis.URL.createObjectURL;
    delete globalThis.URL.revokeObjectURL;
    window.HTMLAnchorElement.prototype.click.mockRestore();
  });

  it('exports the HR dashboard and filters job runs', async () => {
    api.get.mockImplementation((path) => {
      if (path.startsWith('/reports/dashboard/hr')) {
        return Promise.resolve({ payrollStatus: 1 });
      }
      if (path.startsWith('/reports/jobs/job-1')) {
        return Promise.resolve({ id: 'job-1', details: { error: 'failed' } });
      }
      return Promise.resolve({});
    });
    api.getWithMeta.mockResolvedValue({
      data: [{ id: 'job-1', jobType: 'gdpr_retention', status: 'failed', details: { error: 'failed' } }],
      total: 1,
    });
    api.download.mockResolvedValue({ blob: new Blob(['a']), filename: 'report.csv' });

    render(
      <MemoryRouter initialEntries={['/reports/jobs']}>
        <Routes>
          <Route path="/reports/*" element={<Reports />} />
        </Routes>
      </MemoryRouter>,
    );

    const exportBtn = await screen.findByRole('button', { name: /export csv/i });
    await userEvent.click(exportBtn);
    await waitFor(() => {
      expect(api.download).toHaveBeenCalledWith('/reports/dashboard/hr/export');
    });

    const [filter, statusFilter] = screen.getAllByRole('combobox');
    await userEvent.selectOptions(filter, 'gdpr_retention');
    await userEvent.selectOptions(statusFilter, 'failed');
    await waitFor(() => {
      expect(api.getWithMeta).toHaveBeenCalledWith('/reports/jobs?jobType=gdpr_retention&status=failed');
    });

    const viewButton = await screen.findByRole('button', { name: /view/i });
    await userEvent.click(viewButton);
    await waitFor(() => {
      expect(api.get).toHaveBeenCalledWith('/reports/jobs/job-1');
    });
  });
});
