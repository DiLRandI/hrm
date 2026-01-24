import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import userEvent from '@testing-library/user-event';
import GDPR from './GDPR.jsx';
import { api } from '../../../services/apiClient.js';

vi.mock('../../../services/apiClient.js', () => ({
  api: {
    get: vi.fn(),
    getWithMeta: vi.fn(),
    post: vi.fn(),
    download: vi.fn(),
  },
}));

vi.mock('../../auth/auth.jsx', () => ({
  useAuth: () => ({
    user: { role: 'Employee' },
    employee: { id: 'emp-1' },
  }),
}));

describe('GDPR page', () => {
  beforeEach(() => {
    api.get.mockReset();
    api.getWithMeta.mockReset();
    api.post.mockReset();
  });

  it('requests a DSAR export', async () => {
    api.getWithMeta.mockResolvedValue({ data: [], total: 0 });
    api.post.mockResolvedValue({ id: 'dsar-1' });

    render(
      <MemoryRouter initialEntries={['/gdpr/dsar']}>
        <Routes>
          <Route path="/gdpr/*" element={<GDPR />} />
        </Routes>
      </MemoryRouter>,
    );

    const input = await screen.findByPlaceholderText(/employee id/i);
    await userEvent.type(input, 'emp-1');
    await userEvent.click(screen.getByRole('button', { name: /request export/i }));

    await waitFor(() => {
      expect(api.post).toHaveBeenCalledWith('/gdpr/dsar', { employeeId: 'emp-1' });
    });
  });
});
