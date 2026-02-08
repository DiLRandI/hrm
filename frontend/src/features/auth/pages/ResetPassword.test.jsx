import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import ResetPassword from './ResetPassword.jsx';

const { postMock, navigateMock } = vi.hoisted(() => ({
  postMock: vi.fn(),
  navigateMock: vi.fn(),
}));

vi.mock('../../../services/apiClient.js', () => ({
  api: {
    post: (...args) => postMock(...args),
  },
}));

vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual('react-router-dom');
  return {
    ...actual,
    useNavigate: () => navigateMock,
  };
});

function renderPage(entry = '/reset') {
  return render(
    <MemoryRouter initialEntries={[entry]}>
      <Routes>
        <Route path="/reset" element={<ResetPassword />} />
      </Routes>
    </MemoryRouter>
  );
}

describe('ResetPassword page', () => {
  beforeEach(() => {
    postMock.mockReset();
    postMock.mockResolvedValue({ status: 'password_reset' });
    navigateMock.mockReset();
  });

  it('uses token from reset link by default', async () => {
    renderPage('/reset?token=token-from-link');

    const tokenInput = screen.getByLabelText(/reset token/i);
    expect(tokenInput).toHaveValue('token-from-link');
    expect(tokenInput).toBeDisabled();

    await userEvent.type(screen.getByLabelText(/^new password$/i), 'Stronger123');
    await userEvent.type(screen.getByLabelText(/confirm password/i), 'Stronger123');
    await userEvent.click(screen.getByRole('button', { name: /update password/i }));

    expect(postMock).toHaveBeenCalledWith('/auth/reset', {
      token: 'token-from-link',
      newPassword: 'Stronger123',
    });
  });

  it('allows overriding token from reset link', async () => {
    renderPage('/reset?token=token-from-link');

    await userEvent.click(screen.getByRole('button', { name: /use different token/i }));

    const tokenInput = screen.getByLabelText(/reset token/i);
    expect(tokenInput).not.toBeDisabled();
    await userEvent.clear(tokenInput);
    await userEvent.type(tokenInput, 'manual-token');

    await userEvent.type(screen.getByLabelText(/^new password$/i), 'Stronger123');
    await userEvent.type(screen.getByLabelText(/confirm password/i), 'Stronger123');
    await userEvent.click(screen.getByRole('button', { name: /update password/i }));

    expect(postMock).toHaveBeenCalledWith('/auth/reset', {
      token: 'manual-token',
      newPassword: 'Stronger123',
    });
  });

  it('blocks submit when password does not meet strength rules', async () => {
    renderPage('/reset?token=token-from-link');

    await userEvent.type(screen.getByLabelText(/^new password$/i), 'Weak12');
    await userEvent.type(screen.getByLabelText(/confirm password/i), 'Weak12');
    await userEvent.click(screen.getByRole('button', { name: /update password/i }));

    expect(screen.getByText(/password must be at least 10 characters/i)).toBeInTheDocument();
    expect(postMock).not.toHaveBeenCalled();
  });
});
