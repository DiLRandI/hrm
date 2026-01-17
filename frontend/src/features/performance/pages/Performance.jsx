import React, { useEffect, useMemo, useState } from 'react';
import { api } from '../../../services/apiClient.js';
import { useAuth } from '../../auth/auth.jsx';
import {
  GOAL_STATUS_ACTIVE,
  REVIEW_TASK_ASSIGNED,
  PIP_STATUS_ACTIVE,
  PIP_STATUS_CLOSED,
  REVIEW_CYCLE_CLOSED,
} from '../../../shared/constants/statuses.js';
import { ROLE_HR, ROLE_MANAGER } from '../../../shared/constants/roles.js';
import { FEEDBACK_TYPES, REVIEW_RESPONSE_ROLES } from '../../../shared/constants/performance.js';

export default function Performance() {
  const { user, employee } = useAuth();
  const role = user?.role || user?.RoleName;
  const isHR = role === ROLE_HR;
  const isManager = role === ROLE_MANAGER;
  const canViewSummary = isHR || isManager;

  const [employees, setEmployees] = useState([]);
  const [goals, setGoals] = useState([]);
  const [templates, setTemplates] = useState([]);
  const [cycles, setCycles] = useState([]);
  const [tasks, setTasks] = useState([]);
  const [feedback, setFeedback] = useState([]);
  const [checkins, setCheckins] = useState([]);
  const [pips, setPips] = useState([]);
  const [summary, setSummary] = useState(null);
  const [error, setError] = useState('');

  const [goalForm, setGoalForm] = useState({ title: '', metric: '', dueDate: '', weight: '' });
  const [templateForm, setTemplateForm] = useState({ name: '', ratingScale: '', questions: '' });
  const [cycleForm, setCycleForm] = useState({ name: '', startDate: '', endDate: '', templateId: '' });
  const [reviewForm, setReviewForm] = useState({ taskId: '', role: 'self', rating: '', responses: '' });
  const [feedbackForm, setFeedbackForm] = useState({ toEmployeeId: '', type: 'recognition', message: '', relatedGoalId: '' });
  const [checkinForm, setCheckinForm] = useState({ employeeId: '', notes: '', private: true });
  const [pipForm, setPipForm] = useState({ employeeId: '', objectives: '[]', milestones: '[]', reviewDates: '[]' });

  const employeeLookup = useMemo(() => {
    return employees.reduce((acc, e) => {
      acc[e.id] = `${e.firstName} ${e.lastName}`.trim();
      return acc;
    }, {});
  }, [employees]);

  const load = async () => {
    setError('');
    try {
      const requests = [
        api.get('/employees'),
        api.get('/performance/goals'),
        api.get('/performance/review-templates'),
        api.get('/performance/review-cycles'),
        api.get('/performance/review-tasks'),
        api.get('/performance/feedback'),
        api.get('/performance/checkins'),
        api.get('/performance/pips'),
      ];
      if (canViewSummary) {
        requests.push(api.get('/performance/reports/summary'));
      }

      const results = await Promise.allSettled(requests);
      const setters = [setEmployees, setGoals, setTemplates, setCycles, setTasks, setFeedback, setCheckins, setPips];
      results.slice(0, setters.length).forEach((result, idx) => {
        if (result.status === 'fulfilled') {
          setters[idx](Array.isArray(result.value) ? result.value : []);
        } else if (!error) {
          setError(result.reason?.message || 'Failed to load performance data');
        }
      });
      if (canViewSummary) {
        const summaryResult = results[requests.length - 1];
        if (summaryResult?.status === 'fulfilled') {
          setSummary(summaryResult.value);
        }
      }
    } catch (err) {
      setError(err.message);
    }
  };

  useEffect(() => {
    load();
  }, [role]);

  const submitGoal = async (e) => {
    e.preventDefault();
    try {
      await api.post('/performance/goals', {
        employeeId: employee?.id,
        title: goalForm.title,
        metric: goalForm.metric,
        dueDate: goalForm.dueDate,
        weight: Number(goalForm.weight || 0),
        status: GOAL_STATUS_ACTIVE,
        progress: 0,
      });
      setGoalForm({ title: '', metric: '', dueDate: '', weight: '' });
      await load();
    } catch (err) {
      setError(err.message);
    }
  };

  const submitTemplate = async (e) => {
    e.preventDefault();
    try {
      const ratingScale = templateForm.ratingScale ? JSON.parse(templateForm.ratingScale) : null;
      const questions = templateForm.questions ? JSON.parse(templateForm.questions) : null;
      await api.post('/performance/review-templates', { name: templateForm.name, ratingScale, questions });
      setTemplateForm({ name: '', ratingScale: '', questions: '' });
      await load();
    } catch (err) {
      setError(err.message || 'Invalid JSON for template');
    }
  };

  const submitCycle = async (e) => {
    e.preventDefault();
    try {
      await api.post('/performance/review-cycles', cycleForm);
      setCycleForm({ name: '', startDate: '', endDate: '', templateId: '' });
      await load();
    } catch (err) {
      setError(err.message);
    }
  };

  const finalizeCycle = async (cycleId) => {
    try {
      await api.post(`/performance/review-cycles/${cycleId}/finalize`, {});
      await load();
    } catch (err) {
      setError(err.message);
    }
  };

  const submitReview = async (e) => {
    e.preventDefault();
    if (!reviewForm.taskId) {
      setError('Select a review task');
      return;
    }
    try {
      const responses = reviewForm.responses ? JSON.parse(reviewForm.responses) : [];
      await api.post(`/performance/review-tasks/${reviewForm.taskId}/responses`, {
        role: reviewForm.role,
        rating: reviewForm.rating ? Number(reviewForm.rating) : null,
        responses,
      });
      setReviewForm({ taskId: '', role: 'self', rating: '', responses: '' });
      await load();
    } catch (err) {
      setError(err.message || 'Invalid JSON for responses');
    }
  };

  const submitFeedback = async (e) => {
    e.preventDefault();
    try {
      await api.post('/performance/feedback', feedbackForm);
      setFeedbackForm({ toEmployeeId: '', type: 'recognition', message: '', relatedGoalId: '' });
      await load();
    } catch (err) {
      setError(err.message);
    }
  };

  const submitCheckin = async (e) => {
    e.preventDefault();
    try {
      await api.post('/performance/checkins', checkinForm);
      setCheckinForm({ employeeId: '', notes: '', private: true });
      await load();
    } catch (err) {
      setError(err.message);
    }
  };

  const submitPip = async (e) => {
    e.preventDefault();
    try {
      const objectives = pipForm.objectives ? JSON.parse(pipForm.objectives) : [];
      const milestones = pipForm.milestones ? JSON.parse(pipForm.milestones) : [];
      const reviewDates = pipForm.reviewDates ? JSON.parse(pipForm.reviewDates) : [];
      await api.post('/performance/pips', {
        employeeId: pipForm.employeeId,
        objectives,
        milestones,
        reviewDates,
        status: PIP_STATUS_ACTIVE,
      });
      setPipForm({ employeeId: '', objectives: '[]', milestones: '[]', reviewDates: '[]' });
      await load();
    } catch (err) {
      setError(err.message || 'Invalid JSON for PIP fields');
    }
  };

  const closePip = async (pipId) => {
    try {
      await api.put(`/performance/pips/${pipId}`, { status: PIP_STATUS_CLOSED });
      await load();
    } catch (err) {
      setError(err.message);
    }
  };

  return (
    <section className="page">
      <header className="page-header">
        <div>
          <h2>Performance</h2>
          <p>Goals, review cycles, feedback, and improvement plans.</p>
        </div>
      </header>

      {error && <div className="error">{error}</div>}

      <div className="card-grid">
        <div className="card">
          <h3>Goals</h3>
          <form className="stack" onSubmit={submitGoal}>
            <input placeholder="Goal title" value={goalForm.title} onChange={(e) => setGoalForm({ ...goalForm, title: e.target.value })} />
            <input placeholder="Metric" value={goalForm.metric} onChange={(e) => setGoalForm({ ...goalForm, metric: e.target.value })} />
            <input type="date" value={goalForm.dueDate} onChange={(e) => setGoalForm({ ...goalForm, dueDate: e.target.value })} />
            <input type="number" step="0.1" placeholder="Weight" value={goalForm.weight} onChange={(e) => setGoalForm({ ...goalForm, weight: e.target.value })} />
            <button type="submit">Add goal</button>
          </form>
          <div className="table">
            <div className="table-row header">
              <span>Title</span>
              <span>Due</span>
              <span>Status</span>
            </div>
            {goals.map((goal) => (
              <div key={goal.id} className="table-row">
                <span>{goal.title}</span>
                <span>{goal.dueDate?.slice(0, 10)}</span>
                <span>{goal.status}</span>
              </div>
            ))}
          </div>
        </div>

        {isHR && (
          <div className="card">
            <h3>Review templates</h3>
            <form className="stack" onSubmit={submitTemplate}>
              <input placeholder="Template name" value={templateForm.name} onChange={(e) => setTemplateForm({ ...templateForm, name: e.target.value })} />
              <textarea
                placeholder='Rating scale JSON, e.g. [{"label":"Exceeds","value":5}]'
                value={templateForm.ratingScale}
                onChange={(e) => setTemplateForm({ ...templateForm, ratingScale: e.target.value })}
              />
              <textarea
                placeholder='Questions JSON, e.g. [{"question":"What went well?"}]'
                value={templateForm.questions}
                onChange={(e) => setTemplateForm({ ...templateForm, questions: e.target.value })}
              />
              <button type="submit">Add template</button>
            </form>
            <div className="list">
              {templates.map((template) => (
                <div key={template.id} className="list-item">
                  <div>
                    <strong>{template.name}</strong>
                  </div>
                  <small>{template.createdAt?.slice(0, 10)}</small>
                </div>
              ))}
            </div>
          </div>
        )}

        {(isHR || isManager) && (
          <div className="card">
            <h3>Review cycles</h3>
            {isHR && (
              <form className="stack" onSubmit={submitCycle}>
                <input placeholder="Cycle name" value={cycleForm.name} onChange={(e) => setCycleForm({ ...cycleForm, name: e.target.value })} />
                <input type="date" value={cycleForm.startDate} onChange={(e) => setCycleForm({ ...cycleForm, startDate: e.target.value })} />
                <input type="date" value={cycleForm.endDate} onChange={(e) => setCycleForm({ ...cycleForm, endDate: e.target.value })} />
                <select value={cycleForm.templateId} onChange={(e) => setCycleForm({ ...cycleForm, templateId: e.target.value })}>
                  <option value="">Select template</option>
                  {templates.map((template) => (
                    <option key={template.id} value={template.id}>
                      {template.name}
                    </option>
                  ))}
                </select>
                <button type="submit">Create cycle</button>
              </form>
            )}
            <div className="table">
              <div className="table-row header">
                <span>Name</span>
                <span>Status</span>
                <span>Dates</span>
                <span>Actions</span>
              </div>
              {cycles.map((cycle) => (
                <div key={cycle.id} className="table-row">
                  <span>{cycle.name}</span>
                  <span>{cycle.status}</span>
                  <span>{cycle.startDate?.slice(0, 10)} → {cycle.endDate?.slice(0, 10)}</span>
                  <span className="row-actions">
                    {isHR && cycle.status !== REVIEW_CYCLE_CLOSED && (
                      <button onClick={() => finalizeCycle(cycle.id)}>Finalize</button>
                    )}
                  </span>
                </div>
              ))}
            </div>
          </div>
        )}

        <div className="card">
          <h3>Review tasks</h3>
          <form className="stack" onSubmit={submitReview}>
            <select value={reviewForm.taskId} onChange={(e) => setReviewForm({ ...reviewForm, taskId: e.target.value })}>
              <option value="">Select task</option>
              {tasks.map((task) => (
                <option key={task.id} value={task.id}>
                  {task.id} ({task.status || REVIEW_TASK_ASSIGNED})
                </option>
              ))}
            </select>
            <select value={reviewForm.role} onChange={(e) => setReviewForm({ ...reviewForm, role: e.target.value })}>
              {REVIEW_RESPONSE_ROLES.map((roleOption) => (
                <option key={roleOption.value} value={roleOption.value}>
                  {roleOption.label}
                </option>
              ))}
            </select>
            <input
              type="number"
              step="0.1"
              placeholder="Rating (optional)"
              value={reviewForm.rating}
              onChange={(e) => setReviewForm({ ...reviewForm, rating: e.target.value })}
            />
            <textarea
              placeholder='Responses JSON, e.g. [{"question":"...","answer":"..."}]'
              value={reviewForm.responses}
              onChange={(e) => setReviewForm({ ...reviewForm, responses: e.target.value })}
            />
            <button type="submit">Submit review</button>
          </form>
          <div className="table">
            <div className="table-row header">
              <span>Employee</span>
              <span>Status</span>
              <span>Due</span>
            </div>
            {tasks.map((task) => (
              <div key={task.id} className="table-row">
                <span>{employeeLookup[task.employeeId] || task.employeeId}</span>
                <span>{task.status}</span>
                <span>{task.selfDue || task.managerDue || task.hrDue || '—'}</span>
              </div>
            ))}
          </div>
        </div>

        <div className="card">
          <h3>Feedback</h3>
          <form className="stack" onSubmit={submitFeedback}>
            <input placeholder="Employee ID" value={feedbackForm.toEmployeeId} onChange={(e) => setFeedbackForm({ ...feedbackForm, toEmployeeId: e.target.value })} required />
            <select value={feedbackForm.type} onChange={(e) => setFeedbackForm({ ...feedbackForm, type: e.target.value })}>
              {FEEDBACK_TYPES.map((item) => (
                <option key={item.value} value={item.value}>{item.label}</option>
              ))}
            </select>
            <textarea placeholder="Message" value={feedbackForm.message} onChange={(e) => setFeedbackForm({ ...feedbackForm, message: e.target.value })} required />
            <input placeholder="Related goal ID (optional)" value={feedbackForm.relatedGoalId} onChange={(e) => setFeedbackForm({ ...feedbackForm, relatedGoalId: e.target.value })} />
            <button type="submit">Add feedback</button>
          </form>
          <div className="list">
            {feedback.map((item) => (
              <div key={item.id} className="list-item">
                <div>
                  <strong>{item.type}</strong>
                  <p>{item.message}</p>
                </div>
                <small>{item.createdAt?.slice(0, 10)}</small>
              </div>
            ))}
          </div>
        </div>

        <div className="card">
          <h3>Check-ins</h3>
          <form className="stack" onSubmit={submitCheckin}>
            <input placeholder="Employee ID" value={checkinForm.employeeId} onChange={(e) => setCheckinForm({ ...checkinForm, employeeId: e.target.value })} required />
            <textarea placeholder="Notes" value={checkinForm.notes} onChange={(e) => setCheckinForm({ ...checkinForm, notes: e.target.value })} required />
            <label className="checkbox">
              <input type="checkbox" checked={checkinForm.private} onChange={(e) => setCheckinForm({ ...checkinForm, private: e.target.checked })} />
              Private
            </label>
            <button type="submit">Add check-in</button>
          </form>
          <div className="list">
            {checkins.map((item) => (
              <div key={item.id} className="list-item">
                <div>
                  <strong>{employeeLookup[item.employeeId] || item.employeeId}</strong>
                  <p>{item.notes}</p>
                </div>
                <small>{item.createdAt?.slice(0, 10)}</small>
              </div>
            ))}
          </div>
        </div>

        {(isHR || isManager) && (
          <div className="card">
            <h3>PIPs</h3>
            <form className="stack" onSubmit={submitPip}>
              <input placeholder="Employee ID" value={pipForm.employeeId} onChange={(e) => setPipForm({ ...pipForm, employeeId: e.target.value })} required />
              <textarea placeholder='Objectives JSON' value={pipForm.objectives} onChange={(e) => setPipForm({ ...pipForm, objectives: e.target.value })} />
              <textarea placeholder='Milestones JSON' value={pipForm.milestones} onChange={(e) => setPipForm({ ...pipForm, milestones: e.target.value })} />
              <textarea placeholder='Review dates JSON' value={pipForm.reviewDates} onChange={(e) => setPipForm({ ...pipForm, reviewDates: e.target.value })} />
              <button type="submit">Create PIP</button>
            </form>
            <div className="table">
              <div className="table-row header">
                <span>Employee</span>
                <span>Status</span>
                <span>Actions</span>
              </div>
              {pips.map((pip) => (
                <div key={pip.id} className="table-row">
                  <span>{employeeLookup[pip.employeeId] || pip.employeeId}</span>
                  <span>{pip.status}</span>
                  <span className="row-actions">
                    {pip.status !== PIP_STATUS_CLOSED && <button onClick={() => closePip(pip.id)}>Close</button>}
                  </span>
                </div>
              ))}
            </div>
          </div>
        )}

        {canViewSummary && summary && (
          <div className="card">
            <h3>Summary</h3>
            <p><strong>Goals completed:</strong> {summary.goalsCompleted} / {summary.goalsTotal}</p>
            <p><strong>Review tasks completed:</strong> {summary.reviewTasksCompleted} / {summary.reviewTasksTotal}</p>
            <p><strong>Review completion rate:</strong> {(summary.reviewCompletionRate || 0) * 100}%</p>
          </div>
        )}
      </div>
    </section>
  );
}
