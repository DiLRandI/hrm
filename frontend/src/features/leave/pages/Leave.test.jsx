import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import userEvent from '@testing-library/user-event';
import Leave from './Leave.jsx';
import { api } from '../../../services/apiClient.js';

vi.mock('../../../services/apiClient.js', () => ({
  api: {
    get: vi.fn(),
    getWithMeta: vi.fn(),
    post: vi.fn(),
    postForm: vi.fn(),
    del: vi.fn(),
    download: vi.fn(),
  },
}));

vi.mock('../../auth/auth.jsx', () => ({
  useAuth: () => ({
    user: { role: 'Employee' },
    employee: { id: 'emp-1' },
  }),
}));

describe('Leave page', () => {
  beforeEach(() => {
    api.get.mockReset();
    api.getWithMeta.mockReset();
    api.post.mockReset();
    api.postForm.mockReset();
    api.del.mockReset();
  });

  it('submits a leave request', async () => {
    api.get.mockImplementation((path) => {
      if (path === '/leave/types') {
        return Promise.resolve([{ id: 't1', name: 'Annual' }]);
      }
      return Promise.resolve([]);
    });
    api.getWithMeta.mockResolvedValue({ data: [], total: 0 });
    api.postForm.mockResolvedValue({ id: 'req-1' });

    const { container } = render(
      <MemoryRouter initialEntries={['/leave/requests']}>
        <Routes>
          <Route path="/leave/*" element={<Leave />} />
        </Routes>
      </MemoryRouter>,
    );

    const leaveTypeSelect = await screen.findByRole('combobox', { name: /leave type/i });
    await userEvent.selectOptions(leaveTypeSelect, 't1');
    const dateInputs = container.querySelectorAll('input[type="date"]');
    await userEvent.type(dateInputs[0], '2026-01-10');
    await userEvent.type(dateInputs[1], '2026-01-12');
    await userEvent.type(screen.getByPlaceholderText('Reason'), 'Family trip');

    const submit = screen.getByRole('button', { name: /submit request/i });
    await userEvent.click(submit);

    await waitFor(() => {
      expect(api.postForm).toHaveBeenCalled();
    });

    const [path, formData] = api.postForm.mock.calls[0];
    expect(path).toBe('/leave/requests');
    expect(formData).toBeInstanceOf(FormData);
    expect(formData.get('employeeId')).toBe('emp-1');
    expect(formData.get('leaveTypeId')).toBe('t1');
    expect(formData.get('startDate')).toBe('2026-01-10');
    expect(formData.get('endDate')).toBe('2026-01-12');
    expect(formData.get('reason')).toBe('Family trip');
  });

  it('requires a document when leave type requires it', async () => {
    api.get.mockImplementation((path) => {
      if (path === '/leave/types') {
        return Promise.resolve([{ id: 't1', name: 'Medical', requiresDoc: true }]);
      }
      return Promise.resolve([]);
    });
    api.getWithMeta.mockResolvedValue({ data: [], total: 0 });

    const { container } = render(
      <MemoryRouter initialEntries={['/leave/requests']}>
        <Routes>
          <Route path="/leave/*" element={<Leave />} />
        </Routes>
      </MemoryRouter>,
    );

    const leaveTypeSelect = await screen.findByRole('combobox', { name: /leave type/i });
    await userEvent.selectOptions(leaveTypeSelect, 't1');
    const dateInputs = container.querySelectorAll('input[type="date"]');
    await userEvent.type(dateInputs[0], '2026-01-10');
    await userEvent.type(dateInputs[1], '2026-01-10');

    const submit = screen.getByRole('button', { name: /submit request/i });
    await userEvent.click(submit);

    await waitFor(() => {
      expect(api.postForm).not.toHaveBeenCalled();
      expect(screen.getByRole('alert')).toHaveTextContent(/supporting document is required/i);
    });
  });
});
