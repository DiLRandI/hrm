import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import Leave from './Leave.jsx';
import { api } from '../../../services/apiClient.js';

vi.mock('../../../services/apiClient.js', () => ({
  api: {
    get: vi.fn(),
    getWithMeta: vi.fn(),
    post: vi.fn(),
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
    api.post.mockResolvedValue({ id: 'req-1' });

    const { container } = render(<Leave />);

    const leaveTypeSelect = await screen.findByRole('combobox');
    await userEvent.selectOptions(leaveTypeSelect, 't1');
    const dateInputs = container.querySelectorAll('input[type="date"]');
    await userEvent.type(dateInputs[0], '2026-01-10');
    await userEvent.type(dateInputs[1], '2026-01-12');
    await userEvent.type(screen.getByPlaceholderText('Reason'), 'Family trip');

    const submit = screen.getByRole('button', { name: /submit request/i });
    await userEvent.click(submit);

    await waitFor(() => {
      expect(api.post).toHaveBeenCalledWith('/leave/requests', expect.objectContaining({
        employeeId: 'emp-1',
        leaveTypeId: 't1',
        reason: 'Family trip',
      }));
    });
  });
});
