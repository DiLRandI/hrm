import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import Notifications from './Notifications.jsx';
import { api } from '../../../services/apiClient.js';

vi.mock('../../../services/apiClient.js', () => ({
  api: {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
  },
}));

vi.mock('../../auth/auth.jsx', () => ({
  useAuth: () => ({
    user: { role: 'HR' },
  }),
}));

describe('Notifications page', () => {
  beforeEach(() => {
    api.get.mockReset();
    api.post.mockReset();
    api.put.mockReset();
  });

  it('marks notifications as read and updates settings', async () => {
    api.get.mockImplementation((path) => {
      if (path === '/notifications') {
        return Promise.resolve([{ id: 'n1', title: 'Leave approved', body: 'Approved', readAt: null, createdAt: '2026-01-01' }]);
      }
      if (path === '/notifications/settings') {
        return Promise.resolve({ emailEnabled: false, emailFrom: 'no-reply@test.local' });
      }
      return Promise.resolve([]);
    });
    api.post.mockResolvedValue({});
    api.put.mockResolvedValue({});

    render(<Notifications />);

    const markRead = await screen.findByRole('button', { name: /mark read/i });
    await userEvent.click(markRead);
    await waitFor(() => {
      expect(api.post).toHaveBeenCalledWith('/notifications/n1/read', {});
    });

    const checkbox = screen.getByRole('checkbox');
    await userEvent.click(checkbox);
    const fromInput = screen.getByPlaceholderText('From address');
    await userEvent.clear(fromInput);
    await userEvent.type(fromInput, 'alerts@test.local');
    await userEvent.click(screen.getByRole('button', { name: /save settings/i }));

    await waitFor(() => {
      expect(api.put).toHaveBeenCalledWith('/notifications/settings', expect.objectContaining({
        emailEnabled: true,
        emailFrom: 'alerts@test.local',
      }));
    });
  });
});
