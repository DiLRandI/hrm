import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import Login from './Login.jsx';

const loginMock = vi.fn().mockResolvedValue();

vi.mock('../auth.jsx', () => ({
  useAuth: () => ({ login: loginMock }),
}));

describe('Login page', () => {
  beforeEach(() => {
    loginMock.mockClear();
  });

  it('submits credentials', async () => {
    render(
      <MemoryRouter>
        <Login />
      </MemoryRouter>
    );

    const emailInput = screen.getByLabelText(/email/i);
    const passwordInput = screen.getByLabelText(/password/i);
    const button = screen.getByRole('button', { name: /sign in/i });

    await userEvent.clear(emailInput);
    await userEvent.type(emailInput, 'user@example.com');
    await userEvent.clear(passwordInput);
    await userEvent.type(passwordInput, 'secret');
    await userEvent.click(button);

    expect(loginMock).toHaveBeenCalledWith('user@example.com', 'secret', '');
  });
});
