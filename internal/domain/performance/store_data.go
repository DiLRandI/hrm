package performance

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type GoalDetails struct {
	EmployeeID  string
	ManagerID   string
	Title       string
	Description string
	Metric      string
	Weight      float64
	Status      string
	Progress    float64
	DueDate     any
}

type EmployeeRef struct {
	EmployeeID string
	ManagerID  string
	UserID     string
}

type ReviewTaskContext struct {
	EmployeeID string
	ManagerID  string
	Status     string
	TemplateID string
	HRRequired bool
}

func (s *Store) EmployeeIDByUserID(ctx context.Context, tenantID, userID string) (string, error) {
	var employeeID string
	if err := s.DB.QueryRow(ctx, "SELECT id FROM employees WHERE tenant_id = $1 AND user_id = $2", tenantID, userID).Scan(&employeeID); err != nil {
		return "", err
	}
	return employeeID, nil
}

func (s *Store) EmployeeUserID(ctx context.Context, tenantID, employeeID string) (string, error) {
	var userID string
	if err := s.DB.QueryRow(ctx, "SELECT user_id FROM employees WHERE tenant_id = $1 AND id = $2", tenantID, employeeID).Scan(&userID); err != nil {
		return "", err
	}
	return userID, nil
}

func (s *Store) ManagerIDByEmployeeID(ctx context.Context, tenantID, employeeID string) (string, error) {
	var managerID string
	if err := s.DB.QueryRow(ctx, "SELECT manager_id FROM employees WHERE tenant_id = $1 AND id = $2", tenantID, employeeID).Scan(&managerID); err != nil {
		return "", err
	}
	return managerID, nil
}

func (s *Store) IsManagerOfEmployee(ctx context.Context, tenantID, employeeID, managerID string) (bool, error) {
	var count int
	if err := s.DB.QueryRow(ctx, `
    SELECT COUNT(1)
    FROM employees
    WHERE tenant_id = $1 AND id = $2 AND manager_id = $3
  `, tenantID, employeeID, managerID).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *Store) ListGoals(ctx context.Context, tenantID, employeeID, managerID string) ([]Goal, error) {
	query := `
    SELECT id, employee_id, manager_id, title, description, metric, due_date, weight, status, progress
    FROM goals
    WHERE tenant_id = $1
  `
	args := []any{tenantID}
	if employeeID != "" {
		query += " AND employee_id = $2"
		args = append(args, employeeID)
	} else if managerID != "" {
		query += " AND (manager_id = $2 OR employee_id = $2)"
		args = append(args, managerID)
	}
	query += " ORDER BY created_at DESC"

	rows, err := s.DB.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var goals []Goal
	for rows.Next() {
		var goal Goal
		if err := rows.Scan(&goal.ID, &goal.EmployeeID, &goal.ManagerID, &goal.Title, &goal.Description, &goal.Metric, &goal.DueDate, &goal.Weight, &goal.Status, &goal.Progress); err != nil {
			return nil, err
		}
		goals = append(goals, goal)
	}
	return goals, nil
}

func (s *Store) GetGoal(ctx context.Context, tenantID, goalID string) (GoalDetails, error) {
	var details GoalDetails
	if err := s.DB.QueryRow(ctx, `
    SELECT employee_id, manager_id, title, description, metric, weight, status, progress, due_date
    FROM goals
    WHERE tenant_id = $1 AND id = $2
  `, tenantID, goalID).Scan(&details.EmployeeID, &details.ManagerID, &details.Title, &details.Description, &details.Metric, &details.Weight, &details.Status, &details.Progress, &details.DueDate); err != nil {
		return GoalDetails{}, err
	}
	return details, nil
}

func (s *Store) CreateGoal(ctx context.Context, tenantID, employeeID, managerID, title, description, metric string, dueDate any, weight float64, status string, progress float64) (string, error) {
	var id string
	if err := s.DB.QueryRow(ctx, `
    INSERT INTO goals (tenant_id, employee_id, manager_id, title, description, metric, due_date, weight, status, progress)
    VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
    RETURNING id
  `, tenantID, employeeID, managerID, title, description, metric, dueDate, weight, status, progress).Scan(&id); err != nil {
		return "", err
	}
	return id, nil
}

func (s *Store) UpdateGoal(ctx context.Context, tenantID, goalID string, details GoalDetails) error {
	_, err := s.DB.Exec(ctx, `
    UPDATE goals
    SET title = $1, description = $2, metric = $3, weight = $4, status = $5, progress = $6, due_date = $7
    WHERE tenant_id = $8 AND id = $9
  `, details.Title, details.Description, details.Metric, details.Weight, details.Status, details.Progress, details.DueDate, tenantID, goalID)
	return err
}

func (s *Store) CreateGoalComment(ctx context.Context, goalID, authorID, comment string) error {
	_, err := s.DB.Exec(ctx, `
    INSERT INTO goal_comments (goal_id, author_id, comment)
    VALUES ($1,$2,$3)
  `, goalID, authorID, comment)
	return err
}

func (s *Store) ListReviewTemplates(ctx context.Context, tenantID string) ([]ReviewTemplate, error) {
	rows, err := s.DB.Query(ctx, `
    SELECT id, name, rating_scale_json, questions_json, created_at
    FROM review_templates
    WHERE tenant_id = $1
    ORDER BY created_at DESC
  `, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var templates []ReviewTemplate
	for rows.Next() {
		var tmpl ReviewTemplate
		var ratingJSON, questionsJSON []byte
		if err := rows.Scan(&tmpl.ID, &tmpl.Name, &ratingJSON, &questionsJSON, &tmpl.CreatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(ratingJSON, &tmpl.RatingScale); err != nil {
			tmpl.RatingScale = nil
		}
		if err := json.Unmarshal(questionsJSON, &tmpl.Questions); err != nil {
			tmpl.Questions = nil
		}
		templates = append(templates, tmpl)
	}
	return templates, nil
}

func (s *Store) CreateReviewTemplate(ctx context.Context, tenantID, name string, ratingJSON, questionsJSON []byte) (string, error) {
	var id string
	if err := s.DB.QueryRow(ctx, `
    INSERT INTO review_templates (tenant_id, name, rating_scale_json, questions_json)
    VALUES ($1,$2,$3,$4)
    RETURNING id
  `, tenantID, name, ratingJSON, questionsJSON).Scan(&id); err != nil {
		return "", err
	}
	return id, nil
}

func (s *Store) ListReviewCycles(ctx context.Context, tenantID string) ([]ReviewCycle, error) {
	rows, err := s.DB.Query(ctx, `
    SELECT id, name, start_date, end_date, status, COALESCE(template_id, ''), hr_required
    FROM review_cycles
    WHERE tenant_id = $1
    ORDER BY start_date DESC
  `, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cycles []ReviewCycle
	for rows.Next() {
		var cycle ReviewCycle
		if err := rows.Scan(&cycle.ID, &cycle.Name, &cycle.StartDate, &cycle.EndDate, &cycle.Status, &cycle.TemplateID, &cycle.HRRequired); err != nil {
			return nil, err
		}
		cycles = append(cycles, cycle)
	}
	return cycles, nil
}

func (s *Store) CreateReviewCycle(ctx context.Context, tenantID, name string, startDate, endDate time.Time, status, templateID string, hrRequired bool) (string, error) {
	var id string
	if err := s.DB.QueryRow(ctx, `
    INSERT INTO review_cycles (tenant_id, name, start_date, end_date, status, template_id, hr_required)
    VALUES ($1,$2,$3,$4,$5,$6,$7)
    RETURNING id
  `, tenantID, name, startDate, endDate, status, nullIfEmpty(templateID), hrRequired).Scan(&id); err != nil {
		return "", err
	}
	return id, nil
}

func (s *Store) ListActiveEmployeesForReview(ctx context.Context, tenantID string, employeeIDs []string) ([]EmployeeRef, error) {
	query := `
    SELECT id, manager_id, user_id
    FROM employees
    WHERE tenant_id = $1 AND status = $2
  `
	args := []any{tenantID, "active"}
	if len(employeeIDs) > 0 {
		query += " AND id = ANY($3)"
		args = append(args, employeeIDs)
	}

	rows, err := s.DB.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var employees []EmployeeRef
	for rows.Next() {
		var employee EmployeeRef
		if err := rows.Scan(&employee.EmployeeID, &employee.ManagerID, &employee.UserID); err != nil {
			return nil, err
		}
		employees = append(employees, employee)
	}
	return employees, nil
}

func (s *Store) CreateReviewTask(ctx context.Context, tenantID, cycleID, employeeID, managerID, status string, selfDue, managerDue, hrDue time.Time) error {
	_, err := s.DB.Exec(ctx, `
    INSERT INTO review_tasks (tenant_id, cycle_id, employee_id, manager_id, status, self_due, manager_due, hr_due)
    VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
  `, tenantID, cycleID, employeeID, nullIfEmpty(managerID), status, selfDue, managerDue, hrDue)
	return err
}

func (s *Store) ReviewCycleStatus(ctx context.Context, tenantID, cycleID string) (string, error) {
	var status string
	if err := s.DB.QueryRow(ctx, `
    SELECT status
    FROM review_cycles
    WHERE tenant_id = $1 AND id = $2
  `, tenantID, cycleID).Scan(&status); err != nil {
		return "", err
	}
	return status, nil
}

func (s *Store) UpdateReviewCycleStatus(ctx context.Context, tenantID, cycleID, status string) error {
	_, err := s.DB.Exec(ctx, `
    UPDATE review_cycles
    SET status = $1
    WHERE tenant_id = $2 AND id = $3
  `, status, tenantID, cycleID)
	return err
}

func (s *Store) UpdateReviewTasksStatusByCycle(ctx context.Context, tenantID, cycleID, status string) error {
	_, err := s.DB.Exec(ctx, `
    UPDATE review_tasks
    SET status = $1
    WHERE tenant_id = $2 AND cycle_id = $3
  `, status, tenantID, cycleID)
	return err
}

func (s *Store) ListReviewTasks(ctx context.Context, tenantID, employeeID, managerID string) ([]ReviewTask, error) {
	query := `
    SELECT id, cycle_id, employee_id, manager_id, status, self_due, manager_due, hr_due
    FROM review_tasks
    WHERE tenant_id = $1
  `
	args := []any{tenantID}
	if employeeID != "" {
		query += " AND employee_id = $2"
		args = append(args, employeeID)
	} else if managerID != "" {
		query += " AND (manager_id = $2 OR employee_id = $2)"
		args = append(args, managerID)
	}
	query += " ORDER BY created_at DESC"

	rows, err := s.DB.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []ReviewTask
	for rows.Next() {
		var task ReviewTask
		if err := rows.Scan(&task.ID, &task.CycleID, &task.EmployeeID, &task.ManagerID, &task.Status, &task.SelfDue, &task.ManagerDue, &task.HRDue); err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}

func (s *Store) ReviewTaskContext(ctx context.Context, tenantID, taskID string) (ReviewTaskContext, error) {
	var ctxInfo ReviewTaskContext
	if err := s.DB.QueryRow(ctx, `
    SELECT rt.employee_id, rt.manager_id, rt.status, COALESCE(rc.template_id::text,''), rc.hr_required
    FROM review_tasks rt
    JOIN review_cycles rc ON rt.cycle_id = rc.id
    WHERE rt.tenant_id = $1 AND rt.id = $2
  `, tenantID, taskID).Scan(&ctxInfo.EmployeeID, &ctxInfo.ManagerID, &ctxInfo.Status, &ctxInfo.TemplateID, &ctxInfo.HRRequired); err != nil {
		return ReviewTaskContext{}, err
	}
	return ctxInfo, nil
}

func (s *Store) ReviewTemplateQuestions(ctx context.Context, tenantID, templateID string) ([]byte, error) {
	var questionsJSON []byte
	if err := s.DB.QueryRow(ctx, `
    SELECT questions_json FROM review_templates WHERE tenant_id = $1 AND id = $2
  `, tenantID, templateID).Scan(&questionsJSON); err != nil {
		return nil, err
	}
	return questionsJSON, nil
}

func (s *Store) CreateReviewResponse(ctx context.Context, tenantID, taskID, respondentID, role string, responses []byte, rating any) error {
	_, err := s.DB.Exec(ctx, `
    INSERT INTO review_responses (tenant_id, task_id, respondent_id, role, responses_json, rating, submitted_at)
    VALUES ($1,$2,$3,$4,$5,$6,now())
  `, tenantID, taskID, respondentID, role, responses, rating)
	return err
}

func (s *Store) UpdateReviewTaskStatus(ctx context.Context, tenantID, taskID, status string) error {
	_, err := s.DB.Exec(ctx, "UPDATE review_tasks SET status = $1 WHERE tenant_id = $2 AND id = $3", status, tenantID, taskID)
	return err
}

func (s *Store) ListFeedback(ctx context.Context, tenantID, employeeID, managerID, managerUserID string) ([]Feedback, error) {
	query := `
    SELECT id, from_user_id, to_employee_id, type, message, related_goal_id, created_at
    FROM feedback
    WHERE tenant_id = $1
  `
	args := []any{tenantID}
	if employeeID != "" {
		query += " AND to_employee_id = $2"
		args = append(args, employeeID)
	} else if managerID != "" {
		query += " AND (to_employee_id IN (SELECT id FROM employees WHERE tenant_id = $1 AND manager_id = $2) OR from_user_id = $3)"
		args = append(args, managerID, managerUserID)
	}
	query += " ORDER BY created_at DESC"

	rows, err := s.DB.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var feedbacks []Feedback
	for rows.Next() {
		var feedback Feedback
		if err := rows.Scan(&feedback.ID, &feedback.FromUserID, &feedback.ToEmployeeID, &feedback.Type, &feedback.Message, &feedback.RelatedGoalID, &feedback.CreatedAt); err != nil {
			return nil, err
		}
		feedbacks = append(feedbacks, feedback)
	}
	return feedbacks, nil
}

func (s *Store) CreateFeedback(ctx context.Context, tenantID, fromUserID, toEmployeeID, feedbackType, message string, relatedGoalID any) error {
	_, err := s.DB.Exec(ctx, `
    INSERT INTO feedback (tenant_id, from_user_id, to_employee_id, type, message, related_goal_id)
    VALUES ($1,$2,$3,$4,$5,$6)
  `, tenantID, fromUserID, toEmployeeID, feedbackType, message, relatedGoalID)
	return err
}

func (s *Store) ListCheckins(ctx context.Context, tenantID, employeeID, managerID string) ([]Checkin, error) {
	query := `
    SELECT id, employee_id, manager_id, notes, private, created_at
    FROM checkins
    WHERE tenant_id = $1
  `
	args := []any{tenantID}
	if employeeID != "" {
		query += " AND employee_id = $2"
		args = append(args, employeeID)
	} else if managerID != "" {
		query += " AND (manager_id = $2 OR employee_id = $2)"
		args = append(args, managerID)
	}
	query += " ORDER BY created_at DESC"

	rows, err := s.DB.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var checkins []Checkin
	for rows.Next() {
		var checkin Checkin
		if err := rows.Scan(&checkin.ID, &checkin.EmployeeID, &checkin.ManagerID, &checkin.Notes, &checkin.Private, &checkin.CreatedAt); err != nil {
			return nil, err
		}
		checkins = append(checkins, checkin)
	}
	return checkins, nil
}

func (s *Store) CreateCheckin(ctx context.Context, tenantID, employeeID, managerID, notes string, private bool) error {
	_, err := s.DB.Exec(ctx, `
    INSERT INTO checkins (tenant_id, employee_id, manager_id, notes, private)
    VALUES ($1,$2,$3,$4,$5)
  `, tenantID, employeeID, nullIfEmpty(managerID), notes, private)
	return err
}

func (s *Store) ListPIPs(ctx context.Context, tenantID, employeeID, managerID string) ([]PIP, error) {
	query := `
    SELECT id, employee_id, manager_id, hr_owner_id, objectives_json, milestones_json, review_dates_json, status, created_at
    FROM pips
    WHERE tenant_id = $1
  `
	args := []any{tenantID}
	if employeeID != "" {
		query += " AND employee_id = $2"
		args = append(args, employeeID)
	} else if managerID != "" {
		query += " AND (manager_id = $2 OR employee_id = $2)"
		args = append(args, managerID)
	}
	query += " ORDER BY created_at DESC"

	rows, err := s.DB.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pips []PIP
	for rows.Next() {
		var pip PIP
		var objectivesJSON, milestonesJSON, reviewDatesJSON []byte
		if err := rows.Scan(&pip.ID, &pip.EmployeeID, &pip.ManagerID, &pip.HROwnerID, &objectivesJSON, &milestonesJSON, &reviewDatesJSON, &pip.Status, &pip.CreatedAt); err != nil {
			return nil, err
		}
		pip.Objectives = json.RawMessage(objectivesJSON)
		pip.Milestones = json.RawMessage(milestonesJSON)
		pip.ReviewDates = json.RawMessage(reviewDatesJSON)
		pips = append(pips, pip)
	}
	return pips, nil
}

func (s *Store) CreatePIP(ctx context.Context, tenantID, employeeID, managerID, hrOwnerID string, objectives, milestones, reviewDates []byte, status string) (string, error) {
	var id string
	if err := s.DB.QueryRow(ctx, `
    INSERT INTO pips (tenant_id, employee_id, manager_id, hr_owner_id, objectives_json, milestones_json, review_dates_json, status)
    VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
    RETURNING id
  `, tenantID, employeeID, nullIfEmpty(managerID), nullIfEmpty(hrOwnerID), objectives, milestones, reviewDates, status).Scan(&id); err != nil {
		return "", err
	}
	return id, nil
}

func (s *Store) GetPIP(ctx context.Context, tenantID, pipID string) (string, string, error) {
	var employeeID, managerID string
	if err := s.DB.QueryRow(ctx, `
    SELECT employee_id, manager_id
    FROM pips
    WHERE tenant_id = $1 AND id = $2
  `, tenantID, pipID).Scan(&employeeID, &managerID); err != nil {
		return "", "", err
	}
	return employeeID, managerID, nil
}

func (s *Store) UpdatePIP(ctx context.Context, tenantID, pipID, status string, objectivesJSON, milestonesJSON, reviewDatesJSON []byte) error {
	_, err := s.DB.Exec(ctx, `
    UPDATE pips
    SET status = COALESCE(NULLIF($1,''), status),
        objectives_json = COALESCE($2, objectives_json),
        milestones_json = COALESCE($3, milestones_json),
        review_dates_json = COALESCE($4, review_dates_json),
        updated_at = now()
    WHERE tenant_id = $5 AND id = $6
  `, status, objectivesJSON, milestonesJSON, reviewDatesJSON, tenantID, pipID)
	return err
}

func (s *Store) PerformanceSummaryData(ctx context.Context, tenantID, managerID string) (int, int, int, int, []float64, error) {
	goalArgs := []any{tenantID}
	goalFilter := ""
	if managerID != "" {
		goalFilter = " AND manager_id = $2"
		goalArgs = append(goalArgs, managerID)
	}
	statusPos := len(goalArgs) + 1
	goalArgs = append(goalArgs, GoalStatusCompleted)
	goalQuery := fmt.Sprintf(`
    SELECT COUNT(1),
           COALESCE(SUM(CASE WHEN status = $%d THEN 1 ELSE 0 END),0)
    FROM goals
    WHERE tenant_id = $1%s`, statusPos, goalFilter)

	var goalsTotal, goalsCompleted int
	if err := s.DB.QueryRow(ctx, goalQuery, goalArgs...).Scan(&goalsTotal, &goalsCompleted); err != nil {
		return 0, 0, 0, 0, nil, err
	}

	taskArgs := []any{tenantID}
	taskFilter := ""
	if managerID != "" {
		taskFilter = " AND manager_id = $2"
		taskArgs = append(taskArgs, managerID)
	}
	statusStart := len(taskArgs) + 1
	taskArgs = append(taskArgs, ReviewTaskStatusCompleted)
	taskQuery := fmt.Sprintf(`
    SELECT COUNT(1),
           COALESCE(SUM(CASE WHEN status = $%d THEN 1 ELSE 0 END),0)
    FROM review_tasks
    WHERE tenant_id = $1%s`, statusStart, taskFilter)

	var tasksTotal, tasksCompleted int
	if err := s.DB.QueryRow(ctx, taskQuery, taskArgs...).Scan(&tasksTotal, &tasksCompleted); err != nil {
		return 0, 0, 0, 0, nil, err
	}

	responseFilter := ""
	responseArgs := []any{tenantID}
	if managerID != "" {
		responseFilter = " AND rt.manager_id = $2"
		responseArgs = append(responseArgs, managerID)
	}
	query := `
    SELECT rr.rating
    FROM review_responses rr
    JOIN review_tasks rt ON rr.task_id = rt.id
    WHERE rr.tenant_id = $1 AND rr.rating IS NOT NULL` + responseFilter
	rows, err := s.DB.Query(ctx, query, responseArgs...)
	if err != nil {
		return goalsTotal, goalsCompleted, tasksTotal, tasksCompleted, nil, nil
	}
	defer rows.Close()

	var ratings []float64
	for rows.Next() {
		var rating float64
		if err := rows.Scan(&rating); err != nil {
			continue
		}
		ratings = append(ratings, rating)
	}

	return goalsTotal, goalsCompleted, tasksTotal, tasksCompleted, ratings, nil
}

func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}
