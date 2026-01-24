import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, within } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import userEvent from '@testing-library/user-event';
import Performance from './Performance.jsx';
import { api } from '../../../services/apiClient.js';

vi.mock('../../../services/apiClient.js', () => ({
  api: {
    get: vi.fn(),
    post: vi.fn(),
  },
}));

vi.mock('../../auth/auth.jsx', () => ({
  useAuth: () => ({
    user: { role: 'Employee' },
    employee: { id: 'emp-1' },
  }),
}));

const loadMockData = () => {
  api.get.mockImplementation((path) => {
    if (path === '/employees') {
      return Promise.resolve([{ id: 'emp-1', firstName: 'Employee', lastName: 'One' }]);
    }
    if (path === '/performance/review-templates') {
      return Promise.resolve([
        { id: 'tmpl-1', name: 'Template', ratingScale: [{ value: 1, label: '1' }], questions: [{ question: 'Q1' }] },
      ]);
    }
    if (path === '/performance/review-cycles') {
      return Promise.resolve([{ id: 'cycle-1', name: 'Cycle', templateId: 'tmpl-1' }]);
    }
    if (path === '/performance/review-tasks') {
      return Promise.resolve([{ id: 'task-1', cycleId: 'cycle-1', status: 'self_pending' }]);
    }
    return Promise.resolve([]);
  });
};

describe('Performance page', () => {
  beforeEach(() => {
    api.get.mockReset();
    api.post.mockReset();
  });

  it('submits a goal for the current employee', async () => {
    loadMockData();
    api.post.mockResolvedValue({ id: 'goal-1' });

    const { container } = render(
      <MemoryRouter initialEntries={['/performance/goals']}>
        <Routes>
          <Route path="/performance/*" element={<Performance />} />
        </Routes>
      </MemoryRouter>,
    );

    await userEvent.type(await screen.findByPlaceholderText('Goal title'), 'Ship Q1');
    await userEvent.type(screen.getByPlaceholderText('Metric'), 'Launches');
    const dateInput = container.querySelector('input[type="date"]');
    if (!dateInput) {
      throw new Error('date input not found');
    }
    await userEvent.type(dateInput, '2026-02-01');
    await userEvent.type(screen.getByPlaceholderText('Weight'), '1');

    await userEvent.click(screen.getByRole('button', { name: /add goal/i }));

    await waitFor(() => {
      expect(api.post).toHaveBeenCalledWith('/performance/goals', expect.objectContaining({
        employeeId: 'emp-1',
        title: 'Ship Q1',
        metric: 'Launches',
      }));
    });
  });

  it('submits a review response', async () => {
    loadMockData();
    api.post.mockResolvedValue({ status: 'manager_pending' });

    render(
      <MemoryRouter initialEntries={['/performance/reviews']}>
        <Routes>
          <Route path="/performance/*" element={<Performance />} />
        </Routes>
      </MemoryRouter>,
    );

    const reviewForm = screen.getByRole('button', { name: /submit review/i }).closest('form');
    if (!reviewForm) {
      throw new Error('review form not found');
    }
    await within(reviewForm).findByRole('option', { name: /task-1/i });
    const taskSelect = within(reviewForm).getAllByRole('combobox')[0];
    await userEvent.selectOptions(taskSelect, 'task-1');
    await waitFor(() => {
      expect(within(reviewForm).getAllByRole('combobox')).toHaveLength(2);
    });
    const ratingSelect = within(reviewForm).getAllByRole('combobox')[1];
    await userEvent.selectOptions(ratingSelect, '1');
    const answer = within(reviewForm).getByRole('textbox');
    await userEvent.type(answer, 'Solid work');

    await userEvent.click(screen.getByRole('button', { name: /submit review/i }));

    await waitFor(() => {
      expect(api.post).toHaveBeenCalledWith('/performance/review-tasks/task-1/responses', expect.objectContaining({
        role: 'self',
        responses: [{ question: 'Q1', answer: 'Solid work' }],
      }));
    });
  });
});
