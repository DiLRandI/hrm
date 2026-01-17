package performancehandler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"hrm/internal/domain/audit"
	"hrm/internal/domain/auth"
	"hrm/internal/domain/core"
	"hrm/internal/domain/notifications"
	"hrm/internal/domain/performance"
	"hrm/internal/transport/http/api"
	"hrm/internal/transport/http/middleware"
	"hrm/internal/transport/http/shared"
)

type Handler struct {
	DB     *pgxpool.Pool
	Perms  middleware.PermissionStore
	Notify *notifications.Service
}

func NewHandler(db *pgxpool.Pool, perms middleware.PermissionStore, notify *notifications.Service) *Handler {
	return &Handler{DB: db, Perms: perms, Notify: notify}
}

type reviewTemplate struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	RatingScale any       `json:"ratingScale"`
	Questions   any       `json:"questions"`
	CreatedAt   time.Time `json:"createdAt"`
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/performance", func(r chi.Router) {
		r.With(middleware.RequirePermission(auth.PermPerformanceRead, h.Perms)).Get("/goals", h.handleListGoals)
		r.With(middleware.RequirePermission(auth.PermPerformanceWrite, h.Perms)).Post("/goals", h.handleCreateGoal)
		r.With(middleware.RequirePermission(auth.PermPerformanceWrite, h.Perms)).Put("/goals/{goalID}", h.handleUpdateGoal)
		r.With(middleware.RequirePermission(auth.PermPerformanceWrite, h.Perms)).Post("/goals/{goalID}/comments", h.handleAddGoalComment)
		r.With(middleware.RequirePermission(auth.PermPerformanceRead, h.Perms)).Get("/review-templates", h.handleListReviewTemplates)
		r.With(middleware.RequirePermission(auth.PermPerformanceWrite, h.Perms)).Post("/review-templates", h.handleCreateReviewTemplate)
		r.With(middleware.RequirePermission(auth.PermPerformanceRead, h.Perms)).Get("/review-cycles", h.handleListReviewCycles)
		r.With(middleware.RequirePermission(auth.PermPerformanceWrite, h.Perms)).Post("/review-cycles", h.handleCreateReviewCycle)
		r.With(middleware.RequirePermission(auth.PermPerformanceFinalize, h.Perms)).Post("/review-cycles/{cycleID}/finalize", h.handleFinalizeReviewCycle)
		r.With(middleware.RequirePermission(auth.PermPerformanceRead, h.Perms)).Get("/review-tasks", h.handleListReviewTasks)
		r.With(middleware.RequirePermission(auth.PermPerformanceReview, h.Perms)).Post("/review-tasks/{taskID}/responses", h.handleSubmitReviewResponse)
		r.With(middleware.RequirePermission(auth.PermPerformanceRead, h.Perms)).Get("/feedback", h.handleListFeedback)
		r.With(middleware.RequirePermission(auth.PermPerformanceWrite, h.Perms)).Post("/feedback", h.handleCreateFeedback)
		r.With(middleware.RequirePermission(auth.PermPerformanceRead, h.Perms)).Get("/checkins", h.handleListCheckins)
		r.With(middleware.RequirePermission(auth.PermPerformanceWrite, h.Perms)).Post("/checkins", h.handleCreateCheckin)
		r.With(middleware.RequirePermission(auth.PermPerformanceRead, h.Perms)).Get("/pips", h.handleListPIPs)
		r.With(middleware.RequirePermission(auth.PermPerformanceWrite, h.Perms)).Post("/pips", h.handleCreatePIP)
		r.With(middleware.RequirePermission(auth.PermPerformanceWrite, h.Perms)).Put("/pips/{pipID}", h.handleUpdatePIP)
		r.With(middleware.RequirePermission(auth.PermPerformanceRead, h.Perms)).Get("/reports/summary", h.handlePerformanceSummary)
	})
}

func (h *Handler) handleListGoals(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	query := `
    SELECT id, employee_id, manager_id, title, description, metric, due_date, weight, status, progress
    FROM goals
    WHERE tenant_id = $1
  `
	args := []any{user.TenantID}
	if user.RoleName == auth.RoleEmployee {
		var employeeID string
		if err := h.DB.QueryRow(r.Context(), "SELECT id FROM employees WHERE tenant_id = $1 AND user_id = $2", user.TenantID, user.UserID).Scan(&employeeID); err != nil {
			slog.Warn("goal list employee lookup failed", "err", err)
		}
		query += " AND employee_id = $2"
		args = append(args, employeeID)
	}
	if user.RoleName == auth.RoleManager {
		var managerEmployeeID string
		if err := h.DB.QueryRow(r.Context(), "SELECT id FROM employees WHERE tenant_id = $1 AND user_id = $2", user.TenantID, user.UserID).Scan(&managerEmployeeID); err != nil {
			slog.Warn("goal list manager lookup failed", "err", err)
		}
		query += " AND (manager_id = $2 OR employee_id = $2)"
		args = append(args, managerEmployeeID)
	}
	query += " ORDER BY created_at DESC"

	rows, err := h.DB.Query(r.Context(), query, args...)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "goal_list_failed", "failed to list goals", middleware.GetRequestID(r.Context()))
		return
	}
	defer rows.Close()

	var goals []performance.Goal
	for rows.Next() {
		var g performance.Goal
		if err := rows.Scan(&g.ID, &g.EmployeeID, &g.ManagerID, &g.Title, &g.Description, &g.Metric, &g.DueDate, &g.Weight, &g.Status, &g.Progress); err != nil {
			api.Fail(w, http.StatusInternalServerError, "goal_list_failed", "failed to list goals", middleware.GetRequestID(r.Context()))
			return
		}
		goals = append(goals, g)
	}
	api.Success(w, goals, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleCreateGoal(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	var payload struct {
		EmployeeID  string  `json:"employeeId"`
		ManagerID   string  `json:"managerId"`
		Title       string  `json:"title"`
		Description string  `json:"description"`
		Metric      string  `json:"metric"`
		Weight      float64 `json:"weight"`
		DueDate     string  `json:"dueDate"`
		Status      string  `json:"status"`
		Progress    float64 `json:"progress"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "invalid request payload", middleware.GetRequestID(r.Context()))
		return
	}

	if payload.EmployeeID == "" {
		if err := h.DB.QueryRow(r.Context(), "SELECT id FROM employees WHERE tenant_id = $1 AND user_id = $2", user.TenantID, user.UserID).Scan(&payload.EmployeeID); err != nil {
			slog.Warn("goal create employee lookup failed", "err", err)
		}
	}
	if payload.EmployeeID == "" {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "employee id required", middleware.GetRequestID(r.Context()))
		return
	}
	if payload.ManagerID == "" {
		if err := h.DB.QueryRow(r.Context(), "SELECT manager_id FROM employees WHERE tenant_id = $1 AND id = $2", user.TenantID, payload.EmployeeID).Scan(&payload.ManagerID); err != nil {
			slog.Warn("goal create manager lookup failed", "err", err)
		}
	}
	if payload.Status == "" {
		payload.Status = performance.GoalStatusActive
	}

	var dueDate any
	if payload.DueDate != "" {
		parsed, err := shared.ParseDate(payload.DueDate)
		if err != nil {
			api.Fail(w, http.StatusBadRequest, "invalid_payload", "invalid due date", middleware.GetRequestID(r.Context()))
			return
		}
		dueDate = parsed
	}

	var id string
	if err := h.DB.QueryRow(r.Context(), `
    INSERT INTO goals (tenant_id, employee_id, manager_id, title, description, metric, due_date, weight, status, progress)
    VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
    RETURNING id
  `, user.TenantID, payload.EmployeeID, payload.ManagerID, payload.Title, payload.Description, payload.Metric, dueDate, payload.Weight, payload.Status, payload.Progress).Scan(&id); err != nil {
		api.Fail(w, http.StatusInternalServerError, "goal_create_failed", "failed to create goal", middleware.GetRequestID(r.Context()))
		return
	}
	if err := audit.New(h.DB).Record(r.Context(), user.TenantID, user.UserID, "performance.goal.create", "goal", id, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, payload); err != nil {
		slog.Warn("audit performance.goal.create failed", "err", err)
	}
	if h.Notify != nil && payload.EmployeeID != "" {
		var employeeUserID string
		if err := h.DB.QueryRow(r.Context(), "SELECT user_id FROM employees WHERE tenant_id = $1 AND id = $2", user.TenantID, payload.EmployeeID).Scan(&employeeUserID); err != nil {
			slog.Warn("goal create employee user lookup failed", "err", err)
		}
		if employeeUserID != "" {
			if err := h.Notify.Create(r.Context(), user.TenantID, employeeUserID, notifications.TypeGoalCreated, "Goal created", "A new goal has been added to your plan."); err != nil {
				slog.Warn("goal created notification failed", "err", err)
			}
		}
	}
	if h.Notify != nil && payload.ManagerID != "" {
		var managerUserID string
		if err := h.DB.QueryRow(r.Context(), "SELECT user_id FROM employees WHERE tenant_id = $1 AND id = $2", user.TenantID, payload.ManagerID).Scan(&managerUserID); err != nil {
			slog.Warn("goal create manager user lookup failed", "err", err)
		}
		if managerUserID != "" {
			if err := h.Notify.Create(r.Context(), user.TenantID, managerUserID, notifications.TypeGoalCreated, "Goal created", "A goal has been created for your report."); err != nil {
				slog.Warn("goal created manager notification failed", "err", err)
			}
		}
	}
	api.Created(w, map[string]string{"id": id}, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleUpdateGoal(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	goalID := chi.URLParam(r, "goalID")
	var employeeID, managerID string
	var current struct {
		Title       string
		Description string
		Metric      string
		Weight      float64
		Status      string
		Progress    float64
		DueDate     any
	}
	if err := h.DB.QueryRow(r.Context(), `
    SELECT employee_id, manager_id, title, description, metric, weight, status, progress, due_date
    FROM goals
    WHERE tenant_id = $1 AND id = $2
  `, user.TenantID, goalID).Scan(&employeeID, &managerID, &current.Title, &current.Description, &current.Metric, &current.Weight, &current.Status, &current.Progress, &current.DueDate); err != nil {
		api.Fail(w, http.StatusNotFound, "not_found", "goal not found", middleware.GetRequestID(r.Context()))
		return
	}

	switch user.RoleName {
	case auth.RoleEmployee:
		var selfEmployeeID string
		if err := h.DB.QueryRow(r.Context(), "SELECT id FROM employees WHERE tenant_id = $1 AND user_id = $2", user.TenantID, user.UserID).Scan(&selfEmployeeID); err != nil {
			slog.Warn("goal update self employee lookup failed", "err", err)
		}
		if selfEmployeeID == "" || selfEmployeeID != employeeID {
			api.Fail(w, http.StatusForbidden, "forbidden", "not allowed", middleware.GetRequestID(r.Context()))
			return
		}
	case auth.RoleManager:
		var managerEmployeeID string
		if err := h.DB.QueryRow(r.Context(), "SELECT id FROM employees WHERE tenant_id = $1 AND user_id = $2", user.TenantID, user.UserID).Scan(&managerEmployeeID); err != nil {
			slog.Warn("goal update manager lookup failed", "err", err)
		}
		if managerEmployeeID == "" || (managerEmployeeID != managerID && managerEmployeeID != employeeID) {
			api.Fail(w, http.StatusForbidden, "forbidden", "not allowed", middleware.GetRequestID(r.Context()))
			return
		}
	}

	var payload struct {
		Title       *string  `json:"title"`
		Description *string  `json:"description"`
		Metric      *string  `json:"metric"`
		Weight      *float64 `json:"weight"`
		Status      *string  `json:"status"`
		Progress    *float64 `json:"progress"`
		DueDate     *string  `json:"dueDate"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "invalid request payload", middleware.GetRequestID(r.Context()))
		return
	}

	if payload.Progress != nil {
		current.Progress = *payload.Progress
	}
	if payload.Status != nil {
		current.Status = *payload.Status
	}
	if user.RoleName == auth.RoleHR || user.RoleName == auth.RoleManager {
		if payload.Title != nil {
			current.Title = *payload.Title
		}
		if payload.Description != nil {
			current.Description = *payload.Description
		}
		if payload.Metric != nil {
			current.Metric = *payload.Metric
		}
		if payload.Weight != nil {
			current.Weight = *payload.Weight
		}
		if payload.DueDate != nil {
			parsed, err := shared.ParseDate(*payload.DueDate)
			if err != nil {
				api.Fail(w, http.StatusBadRequest, "invalid_payload", "invalid due date", middleware.GetRequestID(r.Context()))
				return
			}
			current.DueDate = parsed
		}
	}

	if _, err := h.DB.Exec(r.Context(), `
    UPDATE goals
    SET title = $1, description = $2, metric = $3, weight = $4, status = $5, progress = $6, due_date = $7
    WHERE tenant_id = $8 AND id = $9
  `, current.Title, current.Description, current.Metric, current.Weight, current.Status, current.Progress, current.DueDate, user.TenantID, goalID); err != nil {
		api.Fail(w, http.StatusInternalServerError, "goal_update_failed", "failed to update goal", middleware.GetRequestID(r.Context()))
		return
	}

	if err := audit.New(h.DB).Record(r.Context(), user.TenantID, user.UserID, "performance.goal.update", "goal", goalID, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, payload); err != nil {
		slog.Warn("audit performance.goal.update failed", "err", err)
	}
	api.Success(w, map[string]string{"id": goalID}, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleAddGoalComment(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	goalID := chi.URLParam(r, "goalID")
	var payload struct {
		Comment string `json:"comment"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "invalid request payload", middleware.GetRequestID(r.Context()))
		return
	}

	_, err := h.DB.Exec(r.Context(), `
    INSERT INTO goal_comments (goal_id, author_id, comment)
    VALUES ($1,$2,$3)
  `, goalID, user.UserID, payload.Comment)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "goal_comment_failed", "failed to add comment", middleware.GetRequestID(r.Context()))
		return
	}

	api.Created(w, map[string]string{"status": "comment_added"}, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleListReviewTemplates(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	rows, err := h.DB.Query(r.Context(), `
    SELECT id, name, rating_scale_json, questions_json, created_at
    FROM review_templates
    WHERE tenant_id = $1
    ORDER BY created_at DESC
  `, user.TenantID)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "review_templates_failed", "failed to list review templates", middleware.GetRequestID(r.Context()))
		return
	}
	defer rows.Close()

	var templates []reviewTemplate
	for rows.Next() {
		var t reviewTemplate
		var ratingJSON, questionsJSON []byte
		if err := rows.Scan(&t.ID, &t.Name, &ratingJSON, &questionsJSON, &t.CreatedAt); err != nil {
			api.Fail(w, http.StatusInternalServerError, "review_templates_failed", "failed to list review templates", middleware.GetRequestID(r.Context()))
			return
		}
		if err := json.Unmarshal(ratingJSON, &t.RatingScale); err != nil {
			slog.Warn("review template rating unmarshal failed", "err", err)
		}
		if err := json.Unmarshal(questionsJSON, &t.Questions); err != nil {
			slog.Warn("review template questions unmarshal failed", "err", err)
		}
		templates = append(templates, t)
	}
	api.Success(w, templates, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleCreateReviewTemplate(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}
	if user.RoleName != auth.RoleHR {
		api.Fail(w, http.StatusForbidden, "forbidden", "hr role required", middleware.GetRequestID(r.Context()))
		return
	}

	var payload struct {
		Name        string `json:"name"`
		RatingScale any    `json:"ratingScale"`
		Questions   any    `json:"questions"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "invalid request payload", middleware.GetRequestID(r.Context()))
		return
	}
	if payload.Name == "" {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "template name required", middleware.GetRequestID(r.Context()))
		return
	}

	ratingJSON, err := json.Marshal(payload.RatingScale)
	if err != nil {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "invalid rating scale", middleware.GetRequestID(r.Context()))
		return
	}
	questionsJSON, err := json.Marshal(payload.Questions)
	if err != nil {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "invalid questions", middleware.GetRequestID(r.Context()))
		return
	}

	var id string
	if err := h.DB.QueryRow(r.Context(), `
    INSERT INTO review_templates (tenant_id, name, rating_scale_json, questions_json)
    VALUES ($1,$2,$3,$4)
    RETURNING id
  `, user.TenantID, payload.Name, ratingJSON, questionsJSON).Scan(&id); err != nil {
		api.Fail(w, http.StatusInternalServerError, "review_template_create_failed", "failed to create review template", middleware.GetRequestID(r.Context()))
		return
	}

	if err := audit.New(h.DB).Record(r.Context(), user.TenantID, user.UserID, "performance.review_template.create", "review_template", id, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, payload); err != nil {
		slog.Warn("audit performance.review_template.create failed", "err", err)
	}
	api.Created(w, map[string]string{"id": id}, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleListReviewCycles(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}
	rows, err := h.DB.Query(r.Context(), `
    SELECT id, name, start_date, end_date, status, COALESCE(template_id, ''), hr_required
    FROM review_cycles
    WHERE tenant_id = $1
    ORDER BY start_date DESC
  `, user.TenantID)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "review_cycle_failed", "failed to list review cycles", middleware.GetRequestID(r.Context()))
		return
	}
	defer rows.Close()

	var cycles []performance.ReviewCycle
	for rows.Next() {
		var c performance.ReviewCycle
		if err := rows.Scan(&c.ID, &c.Name, &c.StartDate, &c.EndDate, &c.Status, &c.TemplateID, &c.HRRequired); err != nil {
			api.Fail(w, http.StatusInternalServerError, "review_cycle_failed", "failed to list review cycles", middleware.GetRequestID(r.Context()))
			return
		}
		cycles = append(cycles, c)
	}
	api.Success(w, cycles, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleCreateReviewCycle(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}
	if user.RoleName != auth.RoleHR {
		api.Fail(w, http.StatusForbidden, "forbidden", "hr role required", middleware.GetRequestID(r.Context()))
		return
	}

	var payload struct {
		Name        string   `json:"name"`
		StartDate   string   `json:"startDate"`
		EndDate     string   `json:"endDate"`
		Status      string   `json:"status"`
		TemplateID  string   `json:"templateId"`
		EmployeeIDs []string `json:"employeeIds"`
		HRRequired  bool     `json:"hrRequired"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "invalid request payload", middleware.GetRequestID(r.Context()))
		return
	}

	startDate, err := shared.ParseDate(payload.StartDate)
	if err != nil || startDate.IsZero() {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "invalid start date", middleware.GetRequestID(r.Context()))
		return
	}
	endDate, err := shared.ParseDate(payload.EndDate)
	if err != nil || endDate.IsZero() {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "invalid end date", middleware.GetRequestID(r.Context()))
		return
	}
	if payload.Status == "" {
		payload.Status = performance.ReviewCycleStatusDraft
	}

	var id string
	if err := h.DB.QueryRow(r.Context(), `
    INSERT INTO review_cycles (tenant_id, name, start_date, end_date, status, template_id, hr_required)
    VALUES ($1,$2,$3,$4,$5,$6,$7)
    RETURNING id
  `, user.TenantID, payload.Name, startDate, endDate, payload.Status, nullIfEmpty(payload.TemplateID), payload.HRRequired).Scan(&id); err != nil {
		api.Fail(w, http.StatusInternalServerError, "review_cycle_create_failed", "failed to create review cycle", middleware.GetRequestID(r.Context()))
		return
	}

	midpoint := startDate.Add(endDate.Sub(startDate) / 2)
	employeesQuery := `
    SELECT id, manager_id, user_id
    FROM employees
    WHERE tenant_id = $1 AND status = $2
  `
	args := []any{user.TenantID, core.EmployeeStatusActive}
	if len(payload.EmployeeIDs) > 0 {
		employeesQuery += " AND id = ANY($2)"
		args = append(args, payload.EmployeeIDs)
	}

	rows, err := h.DB.Query(r.Context(), employeesQuery, args...)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var employeeID, managerID, employeeUserID string
			if err := rows.Scan(&employeeID, &managerID, &employeeUserID); err != nil {
				slog.Warn("review cycle employee scan failed", "err", err)
				continue
			}
			if _, err := h.DB.Exec(r.Context(), `
        INSERT INTO review_tasks (tenant_id, cycle_id, employee_id, manager_id, status, self_due, manager_due, hr_due)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
      `, user.TenantID, id, employeeID, nullIfEmpty(managerID), performance.ReviewTaskStatusSelfPending, midpoint, endDate, endDate); err != nil {
				slog.Warn("review task insert failed", "err", err)
			}

			if h.Notify != nil && employeeUserID != "" {
				if err := h.Notify.Create(r.Context(), user.TenantID, employeeUserID, notifications.TypeReviewAssigned, "Review assigned", "Your self-review is ready to complete."); err != nil {
					slog.Warn("review assigned notification failed", "err", err)
				}
			}
			if managerID != "" {
				var managerUserID string
				if err := h.DB.QueryRow(r.Context(), "SELECT user_id FROM employees WHERE tenant_id = $1 AND id = $2", user.TenantID, managerID).Scan(&managerUserID); err != nil {
					slog.Warn("review cycle manager user lookup failed", "err", err)
				}
				if h.Notify != nil && managerUserID != "" {
					if err := h.Notify.Create(r.Context(), user.TenantID, managerUserID, notifications.TypeReviewAssigned, "Manager review assigned", "A manager review has been assigned to you."); err != nil {
						slog.Warn("manager review assigned notification failed", "err", err)
					}
				}
			}
		}
	}

	if err := audit.New(h.DB).Record(r.Context(), user.TenantID, user.UserID, "performance.review_cycle.create", "review_cycle", id, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, payload); err != nil {
		slog.Warn("audit performance.review_cycle.create failed", "err", err)
	}
	api.Created(w, map[string]string{"id": id}, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleFinalizeReviewCycle(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}
	if user.RoleName != auth.RoleHR {
		api.Fail(w, http.StatusForbidden, "forbidden", "hr role required", middleware.GetRequestID(r.Context()))
		return
	}

	cycleID := chi.URLParam(r, "cycleID")
	var status string
	if err := h.DB.QueryRow(r.Context(), `
    SELECT status
    FROM review_cycles
    WHERE tenant_id = $1 AND id = $2
  `, user.TenantID, cycleID).Scan(&status); err != nil {
		api.Fail(w, http.StatusNotFound, "not_found", "review cycle not found", middleware.GetRequestID(r.Context()))
		return
	}
	if status == performance.ReviewCycleStatusClosed {
		api.Fail(w, http.StatusBadRequest, "invalid_state", "review cycle already closed", middleware.GetRequestID(r.Context()))
		return
	}

	if _, err := h.DB.Exec(r.Context(), `
    UPDATE review_cycles
    SET status = $1
    WHERE tenant_id = $2 AND id = $3
  `, performance.ReviewCycleStatusClosed, user.TenantID, cycleID); err != nil {
		api.Fail(w, http.StatusInternalServerError, "review_cycle_finalize_failed", "failed to finalize review cycle", middleware.GetRequestID(r.Context()))
		return
	}

	if _, err := h.DB.Exec(r.Context(), `
    UPDATE review_tasks
    SET status = $1
    WHERE tenant_id = $2 AND cycle_id = $3
  `, performance.ReviewTaskStatusCompleted, user.TenantID, cycleID); err != nil {
		slog.Warn("review tasks finalize failed", "err", err)
	}

	if err := audit.New(h.DB).Record(r.Context(), user.TenantID, user.UserID, "performance.review.finalize", "review_cycle", cycleID, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, map[string]any{"status": performance.ReviewCycleStatusClosed}); err != nil {
		slog.Warn("audit performance.review.finalize failed", "err", err)
	}

	api.Success(w, map[string]string{"status": performance.ReviewCycleStatusClosed}, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleListReviewTasks(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}
	query := `
    SELECT id, cycle_id, employee_id, manager_id, status, self_due, manager_due, hr_due
    FROM review_tasks
    WHERE tenant_id = $1
  `
	args := []any{user.TenantID}
	if user.RoleName == auth.RoleEmployee {
		var employeeID string
		if err := h.DB.QueryRow(r.Context(), "SELECT id FROM employees WHERE tenant_id = $1 AND user_id = $2", user.TenantID, user.UserID).Scan(&employeeID); err != nil {
			slog.Warn("review tasks employee lookup failed", "err", err)
		}
		query += " AND employee_id = $2"
		args = append(args, employeeID)
	}
	if user.RoleName == auth.RoleManager {
		var managerEmployeeID string
		if err := h.DB.QueryRow(r.Context(), "SELECT id FROM employees WHERE tenant_id = $1 AND user_id = $2", user.TenantID, user.UserID).Scan(&managerEmployeeID); err != nil {
			slog.Warn("review tasks manager lookup failed", "err", err)
		}
		query += " AND (manager_id = $2 OR employee_id = $2)"
		args = append(args, managerEmployeeID)
	}
	query += " ORDER BY created_at DESC"

	rows, err := h.DB.Query(r.Context(), query, args...)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "review_tasks_failed", "failed to list review tasks", middleware.GetRequestID(r.Context()))
		return
	}
	defer rows.Close()

	var tasks []map[string]any
	for rows.Next() {
		var id, cycleID, employeeID, managerID, status string
		var selfDue, managerDue, hrDue any
		if err := rows.Scan(&id, &cycleID, &employeeID, &managerID, &status, &selfDue, &managerDue, &hrDue); err != nil {
			api.Fail(w, http.StatusInternalServerError, "review_tasks_failed", "failed to list review tasks", middleware.GetRequestID(r.Context()))
			return
		}
		tasks = append(tasks, map[string]any{
			"id":         id,
			"cycleId":    cycleID,
			"employeeId": employeeID,
			"managerId":  managerID,
			"status":     status,
			"selfDue":    selfDue,
			"managerDue": managerDue,
			"hrDue":      hrDue,
		})
	}
	api.Success(w, tasks, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleSubmitReviewResponse(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	taskID := chi.URLParam(r, "taskID")
	var taskEmployeeID, taskManagerID, taskStatus, templateID string
	var hrRequired bool
	if err := h.DB.QueryRow(r.Context(), `
    SELECT rt.employee_id, rt.manager_id, rt.status, COALESCE(rc.template_id::text,''), rc.hr_required
    FROM review_tasks rt
    JOIN review_cycles rc ON rt.cycle_id = rc.id
    WHERE rt.tenant_id = $1 AND rt.id = $2
  `, user.TenantID, taskID).Scan(&taskEmployeeID, &taskManagerID, &taskStatus, &templateID, &hrRequired); err != nil {
		api.Fail(w, http.StatusNotFound, "not_found", "review task not found", middleware.GetRequestID(r.Context()))
		return
	}

	if user.RoleName == auth.RoleEmployee {
		var selfEmployeeID string
		if err := h.DB.QueryRow(r.Context(), "SELECT id FROM employees WHERE tenant_id = $1 AND user_id = $2", user.TenantID, user.UserID).Scan(&selfEmployeeID); err != nil {
			slog.Warn("review response self employee lookup failed", "err", err)
		}
		if selfEmployeeID == "" || selfEmployeeID != taskEmployeeID {
			api.Fail(w, http.StatusForbidden, "forbidden", "not allowed", middleware.GetRequestID(r.Context()))
			return
		}
		if taskStatus != performance.ReviewTaskStatusSelfPending && taskStatus != performance.ReviewTaskStatusAssigned {
			api.Fail(w, http.StatusBadRequest, "invalid_state", "self review not available", middleware.GetRequestID(r.Context()))
			return
		}
	}
	if user.RoleName == auth.RoleManager {
		var managerEmployeeID string
		if err := h.DB.QueryRow(r.Context(), "SELECT id FROM employees WHERE tenant_id = $1 AND user_id = $2", user.TenantID, user.UserID).Scan(&managerEmployeeID); err != nil {
			slog.Warn("review response manager lookup failed", "err", err)
		}
		if managerEmployeeID == "" || managerEmployeeID != taskManagerID {
			api.Fail(w, http.StatusForbidden, "forbidden", "not allowed", middleware.GetRequestID(r.Context()))
			return
		}
		if taskStatus != performance.ReviewTaskStatusManagerPending {
			api.Fail(w, http.StatusBadRequest, "invalid_state", "manager review not available", middleware.GetRequestID(r.Context()))
			return
		}
	}
	if user.RoleName == auth.RoleHR {
		if taskStatus != performance.ReviewTaskStatusHRPending {
			api.Fail(w, http.StatusBadRequest, "invalid_state", "hr review not available", middleware.GetRequestID(r.Context()))
			return
		}
	}

	var payload map[string]any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "invalid request payload", middleware.GetRequestID(r.Context()))
		return
	}

	responses, err := json.Marshal(payload["responses"])
	if err != nil {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "invalid responses payload", middleware.GetRequestID(r.Context()))
		return
	}

	if templateID != "" {
		var questionsJSON []byte
		if err := h.DB.QueryRow(r.Context(), `
      SELECT questions_json FROM review_templates WHERE tenant_id = $1 AND id = $2
    `, user.TenantID, templateID).Scan(&questionsJSON); err == nil {
			var questions []any
			if err := json.Unmarshal(questionsJSON, &questions); err == nil && len(questions) > 0 {
				var responsesArr []any
				if err := json.Unmarshal(responses, &responsesArr); err != nil || len(responsesArr) < len(questions) {
					api.Fail(w, http.StatusBadRequest, "invalid_payload", "responses do not match template", middleware.GetRequestID(r.Context()))
					return
				}
			}
		}
	}
	rating, ok := payload["rating"].(float64)
	if !ok && payload["rating"] != nil {
		slog.Warn("review response rating type invalid")
	}
	role, ok := payload["role"].(string)
	if !ok && payload["role"] != nil {
		slog.Warn("review response role type invalid")
	}
	if role == "" {
		switch user.RoleName {
		case auth.RoleEmployee:
			role = "self"
		case auth.RoleManager:
			role = "manager"
		default:
			role = "hr"
		}
	}

	if _, err := h.DB.Exec(r.Context(), `
    INSERT INTO review_responses (tenant_id, task_id, respondent_id, role, responses_json, rating, submitted_at)
    VALUES ($1,$2,$3,$4,$5,$6,now())
  `, user.TenantID, taskID, user.UserID, role, responses, rating); err != nil {
		api.Fail(w, http.StatusInternalServerError, "review_response_failed", "failed to submit response", middleware.GetRequestID(r.Context()))
		return
	}

	status := performance.ReviewTaskStatusCompleted
	switch role {
	case "self":
		if hrRequired || taskManagerID != "" {
			status = performance.ReviewTaskStatusManagerPending
		}
	case "manager":
		if hrRequired {
			status = performance.ReviewTaskStatusHRPending
		}
	case "hr":
		status = performance.ReviewTaskStatusCompleted
	}
	if _, err := h.DB.Exec(r.Context(), "UPDATE review_tasks SET status = $1 WHERE tenant_id = $2 AND id = $3", status, user.TenantID, taskID); err != nil {
		slog.Warn("review task status update failed", "err", err)
	}

	if err := audit.New(h.DB).Record(r.Context(), user.TenantID, user.UserID, "performance.review.submit", "review_task", taskID, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, map[string]any{"role": role}); err != nil {
		slog.Warn("audit performance.review.submit failed", "err", err)
	}
	api.Created(w, map[string]string{"status": status}, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleListFeedback(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	query := `
    SELECT id, from_user_id, to_employee_id, type, message, related_goal_id, created_at
    FROM feedback
    WHERE tenant_id = $1
  `
	args := []any{user.TenantID}
	if user.RoleName == auth.RoleEmployee {
		var employeeID string
		if err := h.DB.QueryRow(r.Context(), "SELECT id FROM employees WHERE tenant_id = $1 AND user_id = $2", user.TenantID, user.UserID).Scan(&employeeID); err != nil {
			slog.Warn("feedback list employee lookup failed", "err", err)
		}
		query += " AND to_employee_id = $2"
		args = append(args, employeeID)
	}
	if user.RoleName == auth.RoleManager {
		var managerEmployeeID string
		if err := h.DB.QueryRow(r.Context(), "SELECT id FROM employees WHERE tenant_id = $1 AND user_id = $2", user.TenantID, user.UserID).Scan(&managerEmployeeID); err != nil {
			slog.Warn("feedback list manager lookup failed", "err", err)
		}
		query += " AND (to_employee_id IN (SELECT id FROM employees WHERE tenant_id = $1 AND manager_id = $2) OR from_user_id = $3)"
		args = append(args, managerEmployeeID, user.UserID)
	}
	query += " ORDER BY created_at DESC"

	rows, err := h.DB.Query(r.Context(), query, args...)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "feedback_list_failed", "failed to list feedback", middleware.GetRequestID(r.Context()))
		return
	}
	defer rows.Close()

	var feedbacks []map[string]any
	for rows.Next() {
		var id, fromUser, toEmployee, ftype, message string
		var relatedGoal any
		var created time.Time
		if err := rows.Scan(&id, &fromUser, &toEmployee, &ftype, &message, &relatedGoal, &created); err != nil {
			api.Fail(w, http.StatusInternalServerError, "feedback_list_failed", "failed to list feedback", middleware.GetRequestID(r.Context()))
			return
		}
		feedbacks = append(feedbacks, map[string]any{
			"id":            id,
			"fromUserId":    fromUser,
			"toEmployeeId":  toEmployee,
			"type":          ftype,
			"message":       message,
			"relatedGoalId": relatedGoal,
			"createdAt":     created,
		})
	}
	api.Success(w, feedbacks, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleCreateFeedback(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	var payload struct {
		ToEmployeeID string `json:"toEmployeeId"`
		Type         string `json:"type"`
		Message      string `json:"message"`
		RelatedGoal  string `json:"relatedGoalId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "invalid request payload", middleware.GetRequestID(r.Context()))
		return
	}
	if payload.ToEmployeeID == "" {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "employee id required", middleware.GetRequestID(r.Context()))
		return
	}
	if user.RoleName == auth.RoleEmployee {
		var selfEmployeeID string
		if err := h.DB.QueryRow(r.Context(), "SELECT id FROM employees WHERE tenant_id = $1 AND user_id = $2", user.TenantID, user.UserID).Scan(&selfEmployeeID); err != nil {
			slog.Warn("feedback create self employee lookup failed", "err", err)
		}
		if selfEmployeeID == "" || payload.ToEmployeeID != selfEmployeeID {
			api.Fail(w, http.StatusForbidden, "forbidden", "not allowed", middleware.GetRequestID(r.Context()))
			return
		}
	}
	if user.RoleName == auth.RoleManager {
		var managerEmployeeID string
		if err := h.DB.QueryRow(r.Context(), "SELECT id FROM employees WHERE tenant_id = $1 AND user_id = $2", user.TenantID, user.UserID).Scan(&managerEmployeeID); err != nil {
			slog.Warn("feedback create manager lookup failed", "err", err)
		}
		var allowed int
		if err := h.DB.QueryRow(r.Context(), `
      SELECT COUNT(1)
      FROM employees
      WHERE tenant_id = $1 AND id = $2 AND manager_id = $3
    `, user.TenantID, payload.ToEmployeeID, managerEmployeeID).Scan(&allowed); err != nil {
			slog.Warn("feedback create manager scope failed", "err", err)
		}
		if allowed == 0 {
			api.Fail(w, http.StatusForbidden, "forbidden", "not allowed", middleware.GetRequestID(r.Context()))
			return
		}
	}

	_, err := h.DB.Exec(r.Context(), `
    INSERT INTO feedback (tenant_id, from_user_id, to_employee_id, type, message, related_goal_id)
    VALUES ($1,$2,$3,$4,$5,$6)
  `, user.TenantID, user.UserID, payload.ToEmployeeID, payload.Type, payload.Message, nullIfEmpty(payload.RelatedGoal))
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "feedback_create_failed", "failed to create feedback", middleware.GetRequestID(r.Context()))
		return
	}

	var toUserID string
	if err := h.DB.QueryRow(r.Context(), "SELECT user_id FROM employees WHERE tenant_id = $1 AND id = $2", user.TenantID, payload.ToEmployeeID).Scan(&toUserID); err != nil {
		slog.Warn("feedback recipient user lookup failed", "err", err)
	}
	if toUserID != "" {
		if h.Notify != nil {
			if err := h.Notify.Create(r.Context(), user.TenantID, toUserID, notifications.TypeFeedbackReceived, "New feedback", "You have received new feedback."); err != nil {
				slog.Warn("feedback notification failed", "err", err)
			}
		}
	}
	if err := audit.New(h.DB).Record(r.Context(), user.TenantID, user.UserID, "performance.feedback.create", "feedback", "", middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, payload); err != nil {
		slog.Warn("audit performance.feedback.create failed", "err", err)
	}
	api.Created(w, map[string]string{"status": "feedback_created"}, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleListCheckins(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	query := `
    SELECT id, employee_id, manager_id, notes, private, created_at
    FROM checkins
    WHERE tenant_id = $1
  `
	args := []any{user.TenantID}
	if user.RoleName == auth.RoleEmployee {
		var employeeID string
		if err := h.DB.QueryRow(r.Context(), "SELECT id FROM employees WHERE tenant_id = $1 AND user_id = $2", user.TenantID, user.UserID).Scan(&employeeID); err != nil {
			slog.Warn("checkin list employee lookup failed", "err", err)
		}
		query += " AND employee_id = $2"
		args = append(args, employeeID)
	}
	if user.RoleName == auth.RoleManager {
		var managerEmployeeID string
		if err := h.DB.QueryRow(r.Context(), "SELECT id FROM employees WHERE tenant_id = $1 AND user_id = $2", user.TenantID, user.UserID).Scan(&managerEmployeeID); err != nil {
			slog.Warn("checkin list manager lookup failed", "err", err)
		}
		query += " AND (manager_id = $2 OR employee_id = $2)"
		args = append(args, managerEmployeeID)
	}
	query += " ORDER BY created_at DESC"

	rows, err := h.DB.Query(r.Context(), query, args...)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "checkin_list_failed", "failed to list checkins", middleware.GetRequestID(r.Context()))
		return
	}
	defer rows.Close()

	var checkins []map[string]any
	for rows.Next() {
		var id, employeeID, managerID, notes string
		var private bool
		var created time.Time
		if err := rows.Scan(&id, &employeeID, &managerID, &notes, &private, &created); err != nil {
			api.Fail(w, http.StatusInternalServerError, "checkin_list_failed", "failed to list checkins", middleware.GetRequestID(r.Context()))
			return
		}
		checkins = append(checkins, map[string]any{
			"id":         id,
			"employeeId": employeeID,
			"managerId":  managerID,
			"notes":      notes,
			"private":    private,
			"createdAt":  created,
		})
	}
	api.Success(w, checkins, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleCreateCheckin(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	var payload struct {
		EmployeeID string `json:"employeeId"`
		ManagerID  string `json:"managerId"`
		Notes      string `json:"notes"`
		Private    bool   `json:"private"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "invalid request payload", middleware.GetRequestID(r.Context()))
		return
	}
	if payload.EmployeeID == "" {
		if err := h.DB.QueryRow(r.Context(), "SELECT id FROM employees WHERE tenant_id = $1 AND user_id = $2", user.TenantID, user.UserID).Scan(&payload.EmployeeID); err != nil {
			slog.Warn("checkin create employee lookup failed", "err", err)
		}
	}
	if payload.EmployeeID == "" {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "employee id required", middleware.GetRequestID(r.Context()))
		return
	}
	if user.RoleName == auth.RoleEmployee {
		var selfEmployeeID string
		if err := h.DB.QueryRow(r.Context(), "SELECT id FROM employees WHERE tenant_id = $1 AND user_id = $2", user.TenantID, user.UserID).Scan(&selfEmployeeID); err != nil {
			slog.Warn("checkin create self employee lookup failed", "err", err)
		}
		if selfEmployeeID == "" || payload.EmployeeID != selfEmployeeID {
			api.Fail(w, http.StatusForbidden, "forbidden", "not allowed", middleware.GetRequestID(r.Context()))
			return
		}
	}
	if user.RoleName == auth.RoleManager {
		var managerEmployeeID string
		if err := h.DB.QueryRow(r.Context(), "SELECT id FROM employees WHERE tenant_id = $1 AND user_id = $2", user.TenantID, user.UserID).Scan(&managerEmployeeID); err != nil {
			slog.Warn("checkin create manager lookup failed", "err", err)
		}
		var allowed int
		if err := h.DB.QueryRow(r.Context(), `
      SELECT COUNT(1)
      FROM employees
      WHERE tenant_id = $1 AND id = $2 AND manager_id = $3
    `, user.TenantID, payload.EmployeeID, managerEmployeeID).Scan(&allowed); err != nil {
			slog.Warn("checkin create manager scope failed", "err", err)
		}
		if allowed == 0 {
			api.Fail(w, http.StatusForbidden, "forbidden", "not allowed", middleware.GetRequestID(r.Context()))
			return
		}
	}

	_, err := h.DB.Exec(r.Context(), `
    INSERT INTO checkins (tenant_id, employee_id, manager_id, notes, private)
    VALUES ($1,$2,$3,$4,$5)
  `, user.TenantID, payload.EmployeeID, nullIfEmpty(payload.ManagerID), payload.Notes, payload.Private)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "checkin_create_failed", "failed to create checkin", middleware.GetRequestID(r.Context()))
		return
	}

	if err := audit.New(h.DB).Record(r.Context(), user.TenantID, user.UserID, "performance.checkin.create", "checkin", "", middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, payload); err != nil {
		slog.Warn("audit performance.checkin.create failed", "err", err)
	}
	api.Created(w, map[string]string{"status": "checkin_created"}, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleListPIPs(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	query := `
    SELECT id, employee_id, manager_id, hr_owner_id, objectives_json, milestones_json, review_dates_json, status, created_at
    FROM pips
    WHERE tenant_id = $1
  `
	args := []any{user.TenantID}
	if user.RoleName == auth.RoleEmployee {
		var employeeID string
		if err := h.DB.QueryRow(r.Context(), "SELECT id FROM employees WHERE tenant_id = $1 AND user_id = $2", user.TenantID, user.UserID).Scan(&employeeID); err != nil {
			slog.Warn("pip list employee lookup failed", "err", err)
		}
		query += " AND employee_id = $2"
		args = append(args, employeeID)
	}
	if user.RoleName == auth.RoleManager {
		var managerEmployeeID string
		if err := h.DB.QueryRow(r.Context(), "SELECT id FROM employees WHERE tenant_id = $1 AND user_id = $2", user.TenantID, user.UserID).Scan(&managerEmployeeID); err != nil {
			slog.Warn("pip list manager lookup failed", "err", err)
		}
		query += " AND (manager_id = $2 OR employee_id = $2)"
		args = append(args, managerEmployeeID)
	}
	query += " ORDER BY created_at DESC"

	rows, err := h.DB.Query(r.Context(), query, args...)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "pip_list_failed", "failed to list pips", middleware.GetRequestID(r.Context()))
		return
	}
	defer rows.Close()

	var pips []map[string]any
	for rows.Next() {
		var id, employeeID, managerID, hrOwnerID, status string
		var objectives, milestones, reviewDates []byte
		var created time.Time
		if err := rows.Scan(&id, &employeeID, &managerID, &hrOwnerID, &objectives, &milestones, &reviewDates, &status, &created); err != nil {
			api.Fail(w, http.StatusInternalServerError, "pip_list_failed", "failed to list pips", middleware.GetRequestID(r.Context()))
			return
		}
		pips = append(pips, map[string]any{
			"id":          id,
			"employeeId":  employeeID,
			"managerId":   managerID,
			"hrOwnerId":   hrOwnerID,
			"objectives":  json.RawMessage(objectives),
			"milestones":  json.RawMessage(milestones),
			"reviewDates": json.RawMessage(reviewDates),
			"status":      status,
			"createdAt":   created,
		})
	}
	api.Success(w, pips, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleCreatePIP(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}
	if user.RoleName != auth.RoleHR && user.RoleName != auth.RoleManager {
		api.Fail(w, http.StatusForbidden, "forbidden", "manager or hr required", middleware.GetRequestID(r.Context()))
		return
	}

	var payload map[string]any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "invalid request payload", middleware.GetRequestID(r.Context()))
		return
	}

	employeeID, ok := payload["employeeId"].(string)
	if !ok && payload["employeeId"] != nil {
		slog.Warn("pip create employeeId type invalid")
	}
	managerID, ok := payload["managerId"].(string)
	if !ok && payload["managerId"] != nil {
		slog.Warn("pip create managerId type invalid")
	}
	hrOwnerID, ok := payload["hrOwnerId"].(string)
	if !ok && payload["hrOwnerId"] != nil {
		slog.Warn("pip create hrOwnerId type invalid")
	}
	objectives, err := json.Marshal(payload["objectives"])
	if err != nil {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "invalid objectives", middleware.GetRequestID(r.Context()))
		return
	}
	milestones, err := json.Marshal(payload["milestones"])
	if err != nil {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "invalid milestones", middleware.GetRequestID(r.Context()))
		return
	}
	reviewDates, err := json.Marshal(payload["reviewDates"])
	if err != nil {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "invalid review dates", middleware.GetRequestID(r.Context()))
		return
	}
	status, ok := payload["status"].(string)
	if !ok && payload["status"] != nil {
		slog.Warn("pip create status type invalid")
	}
	if status == "" {
		status = performance.PIPStatusActive
	}

	if employeeID == "" {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "employee id required", middleware.GetRequestID(r.Context()))
		return
	}
	if user.RoleName == auth.RoleManager {
		var managerEmployeeID string
		if err := h.DB.QueryRow(r.Context(), "SELECT id FROM employees WHERE tenant_id = $1 AND user_id = $2", user.TenantID, user.UserID).Scan(&managerEmployeeID); err != nil {
			slog.Warn("pip create manager lookup failed", "err", err)
		}
		var allowed int
		if err := h.DB.QueryRow(r.Context(), `
      SELECT COUNT(1)
      FROM employees
      WHERE tenant_id = $1 AND id = $2 AND manager_id = $3
    `, user.TenantID, employeeID, managerEmployeeID).Scan(&allowed); err != nil {
			slog.Warn("pip create manager scope failed", "err", err)
		}
		if allowed == 0 {
			api.Fail(w, http.StatusForbidden, "forbidden", "not allowed", middleware.GetRequestID(r.Context()))
			return
		}
		managerID = managerEmployeeID
	}

	var id string
	if err := h.DB.QueryRow(r.Context(), `
    INSERT INTO pips (tenant_id, employee_id, manager_id, hr_owner_id, objectives_json, milestones_json, review_dates_json, status)
    VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
    RETURNING id
  `, user.TenantID, employeeID, nullIfEmpty(managerID), nullIfEmpty(hrOwnerID), objectives, milestones, reviewDates, status).Scan(&id); err != nil {
		api.Fail(w, http.StatusInternalServerError, "pip_create_failed", "failed to create pip", middleware.GetRequestID(r.Context()))
		return
	}

	if err := audit.New(h.DB).Record(r.Context(), user.TenantID, user.UserID, "performance.pip.create", "pip", id, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, payload); err != nil {
		slog.Warn("audit performance.pip.create failed", "err", err)
	}
	api.Created(w, map[string]string{"id": id}, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleUpdatePIP(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	pipID := chi.URLParam(r, "pipID")
	var employeeID, managerID string
	err := h.DB.QueryRow(r.Context(), `
    SELECT employee_id, manager_id
    FROM pips
    WHERE tenant_id = $1 AND id = $2
  `, user.TenantID, pipID).Scan(&employeeID, &managerID)
	if err != nil {
		api.Fail(w, http.StatusNotFound, "not_found", "pip not found", middleware.GetRequestID(r.Context()))
		return
	}

	if user.RoleName == auth.RoleManager {
		var managerEmployeeID string
		if err := h.DB.QueryRow(r.Context(), "SELECT id FROM employees WHERE tenant_id = $1 AND user_id = $2", user.TenantID, user.UserID).Scan(&managerEmployeeID); err != nil {
			slog.Warn("pip update manager lookup failed", "err", err)
		}
		if managerEmployeeID == "" || managerEmployeeID != managerID {
			api.Fail(w, http.StatusForbidden, "forbidden", "not allowed", middleware.GetRequestID(r.Context()))
			return
		}
	}
	if user.RoleName == auth.RoleEmployee {
		var selfEmployeeID string
		if err := h.DB.QueryRow(r.Context(), "SELECT id FROM employees WHERE tenant_id = $1 AND user_id = $2", user.TenantID, user.UserID).Scan(&selfEmployeeID); err != nil {
			slog.Warn("pip update self employee lookup failed", "err", err)
		}
		if selfEmployeeID == "" || selfEmployeeID != employeeID {
			api.Fail(w, http.StatusForbidden, "forbidden", "not allowed", middleware.GetRequestID(r.Context()))
			return
		}
	}

	var payload map[string]any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "invalid request payload", middleware.GetRequestID(r.Context()))
		return
	}

	status, ok := payload["status"].(string)
	if !ok && payload["status"] != nil {
		slog.Warn("pip update status type invalid")
	}
	var objectivesJSON, milestonesJSON, reviewDatesJSON []byte
	if payload["objectives"] != nil {
		encoded, err := json.Marshal(payload["objectives"])
		if err != nil {
			api.Fail(w, http.StatusBadRequest, "invalid_payload", "invalid objectives", middleware.GetRequestID(r.Context()))
			return
		}
		objectivesJSON = encoded
	}
	if payload["milestones"] != nil {
		encoded, err := json.Marshal(payload["milestones"])
		if err != nil {
			api.Fail(w, http.StatusBadRequest, "invalid_payload", "invalid milestones", middleware.GetRequestID(r.Context()))
			return
		}
		milestonesJSON = encoded
	}
	if payload["reviewDates"] != nil {
		encoded, err := json.Marshal(payload["reviewDates"])
		if err != nil {
			api.Fail(w, http.StatusBadRequest, "invalid_payload", "invalid review dates", middleware.GetRequestID(r.Context()))
			return
		}
		reviewDatesJSON = encoded
	}

	if _, err := h.DB.Exec(r.Context(), `
    UPDATE pips
    SET status = COALESCE(NULLIF($1,''), status),
        objectives_json = COALESCE($2, objectives_json),
        milestones_json = COALESCE($3, milestones_json),
        review_dates_json = COALESCE($4, review_dates_json),
        updated_at = now()
    WHERE tenant_id = $5 AND id = $6
  `, status, objectivesJSON, milestonesJSON, reviewDatesJSON, user.TenantID, pipID); err != nil {
		api.Fail(w, http.StatusInternalServerError, "pip_update_failed", "failed to update pip", middleware.GetRequestID(r.Context()))
		return
	}

	if err := audit.New(h.DB).Record(r.Context(), user.TenantID, user.UserID, "performance.pip.update", "pip", pipID, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, payload); err != nil {
		slog.Warn("audit performance.pip.update failed", "err", err)
	}
	api.Success(w, map[string]string{"id": pipID}, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handlePerformanceSummary(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}
	if user.RoleName != auth.RoleHR && user.RoleName != auth.RoleManager {
		api.Fail(w, http.StatusForbidden, "forbidden", "manager or hr required", middleware.GetRequestID(r.Context()))
		return
	}

	managerEmployeeID := ""
	if user.RoleName == auth.RoleManager {
		if err := h.DB.QueryRow(r.Context(), "SELECT id FROM employees WHERE tenant_id = $1 AND user_id = $2", user.TenantID, user.UserID).Scan(&managerEmployeeID); err != nil {
			slog.Warn("performance summary manager lookup failed", "err", err)
		}
	}

	goalArgs := []any{user.TenantID}
	goalFilter := ""
	if managerEmployeeID != "" {
		goalFilter = " AND manager_id = $2"
		goalArgs = append(goalArgs, managerEmployeeID)
	}
	statusPos := len(goalArgs) + 1
	goalArgs = append(goalArgs, performance.GoalStatusCompleted)
	goalQuery := fmt.Sprintf(`
    SELECT COUNT(1),
           COALESCE(SUM(CASE WHEN status = $%d THEN 1 ELSE 0 END),0)
    FROM goals
    WHERE tenant_id = $1%s`, statusPos, goalFilter)

	var goalsTotal, goalsCompleted int
	if err := h.DB.QueryRow(r.Context(), goalQuery, goalArgs...).Scan(&goalsTotal, &goalsCompleted); err != nil {
		slog.Warn("performance summary goals query failed", "err", err)
	}

	taskArgs := []any{user.TenantID}
	taskFilter := ""
	if managerEmployeeID != "" {
		taskFilter = " AND manager_id = $2"
		taskArgs = append(taskArgs, managerEmployeeID)
	}
	statusStart := len(taskArgs) + 1
	taskArgs = append(taskArgs, performance.ReviewTaskStatusCompleted)
	taskQuery := fmt.Sprintf(`
    SELECT COUNT(1),
           COALESCE(SUM(CASE WHEN status = $%d THEN 1 ELSE 0 END),0)
    FROM review_tasks
    WHERE tenant_id = $1%s`, statusStart, taskFilter)

	var tasksTotal, tasksCompleted int
	if err := h.DB.QueryRow(r.Context(), taskQuery, taskArgs...).Scan(&tasksTotal, &tasksCompleted); err != nil {
		slog.Warn("performance summary tasks query failed", "err", err)
	}

	ratingCounts := map[string]int{}
	responseFilter := ""
	responseArgs := []any{user.TenantID}
	if managerEmployeeID != "" {
		responseFilter = " AND rt.manager_id = $2"
		responseArgs = append(responseArgs, managerEmployeeID)
	}
	query := `
    SELECT rr.rating
    FROM review_responses rr
    JOIN review_tasks rt ON rr.task_id = rt.id
    WHERE rr.tenant_id = $1 AND rr.rating IS NOT NULL` + responseFilter
	rows, err := h.DB.Query(r.Context(), query, responseArgs...)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var rating float64
			if err := rows.Scan(&rating); err != nil {
				continue
			}
			key := fmt.Sprintf("%d", int(rating+0.5))
			ratingCounts[key]++
		}
	}

	completionRate := 0.0
	if tasksTotal > 0 {
		completionRate = float64(tasksCompleted) / float64(tasksTotal)
	}

	api.Success(w, map[string]any{
		"goalsTotal":           goalsTotal,
		"goalsCompleted":       goalsCompleted,
		"reviewTasksTotal":     tasksTotal,
		"reviewTasksCompleted": tasksCompleted,
		"reviewCompletionRate": completionRate,
		"ratingDistribution":   ratingCounts,
	}, middleware.GetRequestID(r.Context()))
}

func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}
