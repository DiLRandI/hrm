import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import Payroll from './Payroll.jsx';
import { api } from '../../../services/apiClient.js';

vi.mock('../../../services/apiClient.js', () => ({
  api: {
    get: vi.fn(),
    post: vi.fn(),
    postRaw: vi.fn(),
    download: vi.fn(),
  },
}));

vi.mock('../../auth/auth.jsx', () => ({
  useAuth: () => ({
    user: { role: 'HR' },
    employee: { id: 'emp-1' },
  }),
}));

describe('Payroll page', () => {
  beforeEach(() => {
    api.get.mockReset();
    api.post.mockReset();
  });

  it('creates a pay schedule', async () => {
    api.get.mockResolvedValue([]);
    api.post.mockResolvedValue({ id: 'schedule-1' });

    render(<Payroll />);

    const nameInput = await screen.findByPlaceholderText('Schedule name');
    await userEvent.type(nameInput, 'Monthly');
    await userEvent.click(screen.getByRole('button', { name: /add schedule/i }));

    await waitFor(() => {
      expect(api.post).toHaveBeenCalledWith('/payroll/schedules', expect.objectContaining({
        name: 'Monthly',
        frequency: 'monthly',
      }));
    });
  });
});
