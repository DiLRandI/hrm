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
      if (path.startsWith('/reports/jobs')) {
        return Promise.resolve([{ id: 'job-1', jobType: 'gdpr_retention', status: 'completed' }]);
      }
      return Promise.resolve({});
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

    const filter = screen.getByRole('combobox');
    await userEvent.selectOptions(filter, 'gdpr_retention');
    await waitFor(() => {
      expect(api.get).toHaveBeenCalledWith('/reports/jobs?jobType=gdpr_retention');
    });
  });
});
