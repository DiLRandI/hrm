package performancehandler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"hrm/internal/domain/audit"
	"hrm/internal/domain/auth"
	"hrm/internal/domain/notifications"
	"hrm/internal/domain/performance"
	"hrm/internal/transport/http/api"
	"hrm/internal/transport/http/middleware"
	"hrm/internal/transport/http/shared"
)

type Handler struct {
	Service *performance.Service
	Perms   middleware.PermissionStore
	Notify  *notifications.Service
	Audit   *audit.Service
}

func NewHandler(service *performance.Service, perms middleware.PermissionStore, notify *notifications.Service, auditSvc *audit.Service) *Handler {
	return &Handler{Service: service, Perms: perms, Notify: notify, Audit: auditSvc}
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

	employeeID := ""
	managerEmployeeID := ""
	if user.RoleName == auth.RoleEmployee {
		id, err := h.Service.EmployeeIDByUserID(r.Context(), user.TenantID, user.UserID)
		if err != nil {
			slog.Warn("goal list employee lookup failed", "err", err)
		} else {
			employeeID = id
		}
	}
	if user.RoleName == auth.RoleManager {
		id, err := h.Service.EmployeeIDByUserID(r.Context(), user.TenantID, user.UserID)
		if err != nil {
			slog.Warn("goal list manager lookup failed", "err", err)
		} else {
			managerEmployeeID = id
		}
	}

	goals, err := h.Service.ListGoals(r.Context(), user.TenantID, employeeID, managerEmployeeID)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "goal_list_failed", "failed to list goals", middleware.GetRequestID(r.Context()))
		return
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
		if id, err := h.Service.EmployeeIDByUserID(r.Context(), user.TenantID, user.UserID); err != nil {
			slog.Warn("goal create employee lookup failed", "err", err)
		} else {
			payload.EmployeeID = id
		}
	}
	if payload.EmployeeID == "" {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "employee id required", middleware.GetRequestID(r.Context()))
		return
	}
	if payload.ManagerID == "" {
		if id, err := h.Service.ManagerIDByEmployeeID(r.Context(), user.TenantID, payload.EmployeeID); err != nil {
			slog.Warn("goal create manager lookup failed", "err", err)
		} else {
			payload.ManagerID = id
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

	id, err := h.Service.CreateGoal(r.Context(), user.TenantID, payload.EmployeeID, payload.ManagerID, payload.Title, payload.Description, payload.Metric, dueDate, payload.Weight, payload.Status, payload.Progress)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "goal_create_failed", "failed to create goal", middleware.GetRequestID(r.Context()))
		return
	}
	if err := h.Audit.Record(r.Context(), user.TenantID, user.UserID, "performance.goal.create", "goal", id, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, payload); err != nil {
		slog.Warn("audit performance.goal.create failed", "err", err)
	}
	if h.Notify != nil && payload.EmployeeID != "" {
		employeeUserID, err := h.Service.EmployeeUserID(r.Context(), user.TenantID, payload.EmployeeID)
		if err != nil {
			slog.Warn("goal create employee user lookup failed", "err", err)
		}
		if employeeUserID != "" {
			if err := h.Notify.Create(r.Context(), user.TenantID, employeeUserID, notifications.TypeGoalCreated, "Goal created", "A new goal has been added to your plan."); err != nil {
				slog.Warn("goal created notification failed", "err", err)
			}
		}
	}
	if h.Notify != nil && payload.ManagerID != "" {
		managerUserID, err := h.Service.EmployeeUserID(r.Context(), user.TenantID, payload.ManagerID)
		if err != nil {
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
	current, err := h.Service.GetGoal(r.Context(), user.TenantID, goalID)
	if err != nil {
		api.Fail(w, http.StatusNotFound, "not_found", "goal not found", middleware.GetRequestID(r.Context()))
		return
	}

	switch user.RoleName {
	case auth.RoleEmployee:
		selfEmployeeID, err := h.Service.EmployeeIDByUserID(r.Context(), user.TenantID, user.UserID)
		if err != nil {
			slog.Warn("goal update self employee lookup failed", "err", err)
		}
		if selfEmployeeID == "" || selfEmployeeID != current.EmployeeID {
			api.Fail(w, http.StatusForbidden, "forbidden", "not allowed", middleware.GetRequestID(r.Context()))
			return
		}
	case auth.RoleManager:
		managerEmployeeID, err := h.Service.EmployeeIDByUserID(r.Context(), user.TenantID, user.UserID)
		if err != nil {
			slog.Warn("goal update manager lookup failed", "err", err)
		}
		if managerEmployeeID == "" || (managerEmployeeID != current.ManagerID && managerEmployeeID != current.EmployeeID) {
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

	if err := h.Service.UpdateGoal(r.Context(), user.TenantID, goalID, current); err != nil {
		api.Fail(w, http.StatusInternalServerError, "goal_update_failed", "failed to update goal", middleware.GetRequestID(r.Context()))
		return
	}

	if err := h.Audit.Record(r.Context(), user.TenantID, user.UserID, "performance.goal.update", "goal", goalID, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, payload); err != nil {
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

	if err := h.Service.CreateGoalComment(r.Context(), goalID, user.UserID, payload.Comment); err != nil {
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

	templates, err := h.Service.ListReviewTemplates(r.Context(), user.TenantID)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "review_templates_failed", "failed to list review templates", middleware.GetRequestID(r.Context()))
		return
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

	id, err := h.Service.CreateReviewTemplate(r.Context(), user.TenantID, payload.Name, ratingJSON, questionsJSON)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "review_template_create_failed", "failed to create review template", middleware.GetRequestID(r.Context()))
		return
	}

	if err := h.Audit.Record(r.Context(), user.TenantID, user.UserID, "performance.review_template.create", "review_template", id, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, payload); err != nil {
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
	cycles, err := h.Service.ListReviewCycles(r.Context(), user.TenantID)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "review_cycle_failed", "failed to list review cycles", middleware.GetRequestID(r.Context()))
		return
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

	id, err := h.Service.CreateReviewCycle(r.Context(), user.TenantID, payload.Name, startDate, endDate, payload.Status, payload.TemplateID, payload.HRRequired)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "review_cycle_create_failed", "failed to create review cycle", middleware.GetRequestID(r.Context()))
		return
	}

	midpoint := startDate.Add(endDate.Sub(startDate) / 2)
	employees, err := h.Service.ListActiveEmployeesForReview(r.Context(), user.TenantID, payload.EmployeeIDs)
	if err == nil {
		for _, employee := range employees {
			if err := h.Service.CreateReviewTask(r.Context(), user.TenantID, id, employee.EmployeeID, employee.ManagerID, performance.ReviewTaskStatusSelfPending, midpoint, endDate, endDate); err != nil {
				slog.Warn("review task insert failed", "err", err)
			}

			if h.Notify != nil && employee.UserID != "" {
				if err := h.Notify.Create(r.Context(), user.TenantID, employee.UserID, notifications.TypeReviewAssigned, "Review assigned", "Your self-review is ready to complete."); err != nil {
					slog.Warn("review assigned notification failed", "err", err)
				}
			}
			if employee.ManagerID != "" {
				managerUserID, err := h.Service.EmployeeUserID(r.Context(), user.TenantID, employee.ManagerID)
				if err != nil {
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

	if err := h.Audit.Record(r.Context(), user.TenantID, user.UserID, "performance.review_cycle.create", "review_cycle", id, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, payload); err != nil {
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
	status, err := h.Service.ReviewCycleStatus(r.Context(), user.TenantID, cycleID)
	if err != nil {
		api.Fail(w, http.StatusNotFound, "not_found", "review cycle not found", middleware.GetRequestID(r.Context()))
		return
	}
	if status == performance.ReviewCycleStatusClosed {
		api.Fail(w, http.StatusBadRequest, "invalid_state", "review cycle already closed", middleware.GetRequestID(r.Context()))
		return
	}

	if err := h.Service.UpdateReviewCycleStatus(r.Context(), user.TenantID, cycleID, performance.ReviewCycleStatusClosed); err != nil {
		api.Fail(w, http.StatusInternalServerError, "review_cycle_finalize_failed", "failed to finalize review cycle", middleware.GetRequestID(r.Context()))
		return
	}

	if err := h.Service.UpdateReviewTasksStatusByCycle(r.Context(), user.TenantID, cycleID, performance.ReviewTaskStatusCompleted); err != nil {
		slog.Warn("review tasks finalize failed", "err", err)
	}

	if err := h.Audit.Record(r.Context(), user.TenantID, user.UserID, "performance.review.finalize", "review_cycle", cycleID, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, map[string]any{"status": performance.ReviewCycleStatusClosed}); err != nil {
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
	employeeID := ""
	managerEmployeeID := ""
	if user.RoleName == auth.RoleEmployee {
		id, err := h.Service.EmployeeIDByUserID(r.Context(), user.TenantID, user.UserID)
		if err != nil {
			slog.Warn("review tasks employee lookup failed", "err", err)
		} else {
			employeeID = id
		}
	}
	if user.RoleName == auth.RoleManager {
		id, err := h.Service.EmployeeIDByUserID(r.Context(), user.TenantID, user.UserID)
		if err != nil {
			slog.Warn("review tasks manager lookup failed", "err", err)
		} else {
			managerEmployeeID = id
		}
	}

	tasks, err := h.Service.ListReviewTasks(r.Context(), user.TenantID, employeeID, managerEmployeeID)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "review_tasks_failed", "failed to list review tasks", middleware.GetRequestID(r.Context()))
		return
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
	ctxInfo, err := h.Service.ReviewTaskContext(r.Context(), user.TenantID, taskID)
	if err != nil {
		api.Fail(w, http.StatusNotFound, "not_found", "review task not found", middleware.GetRequestID(r.Context()))
		return
	}

	if user.RoleName == auth.RoleEmployee {
		selfEmployeeID, err := h.Service.EmployeeIDByUserID(r.Context(), user.TenantID, user.UserID)
		if err != nil {
			slog.Warn("review response self employee lookup failed", "err", err)
		}
		if selfEmployeeID == "" || selfEmployeeID != ctxInfo.EmployeeID {
			api.Fail(w, http.StatusForbidden, "forbidden", "not allowed", middleware.GetRequestID(r.Context()))
			return
		}
		if ctxInfo.Status != performance.ReviewTaskStatusSelfPending && ctxInfo.Status != performance.ReviewTaskStatusAssigned {
			api.Fail(w, http.StatusBadRequest, "invalid_state", "self review not available", middleware.GetRequestID(r.Context()))
			return
		}
	}
	if user.RoleName == auth.RoleManager {
		managerEmployeeID, err := h.Service.EmployeeIDByUserID(r.Context(), user.TenantID, user.UserID)
		if err != nil {
			slog.Warn("review response manager lookup failed", "err", err)
		}
		if managerEmployeeID == "" || managerEmployeeID != ctxInfo.ManagerID {
			api.Fail(w, http.StatusForbidden, "forbidden", "not allowed", middleware.GetRequestID(r.Context()))
			return
		}
		if ctxInfo.Status != performance.ReviewTaskStatusManagerPending {
			api.Fail(w, http.StatusBadRequest, "invalid_state", "manager review not available", middleware.GetRequestID(r.Context()))
			return
		}
	}
	if user.RoleName == auth.RoleHR {
		if ctxInfo.Status != performance.ReviewTaskStatusHRPending {
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

	if ctxInfo.TemplateID != "" {
		questionsJSON, err := h.Service.ReviewTemplateQuestions(r.Context(), user.TenantID, ctxInfo.TemplateID)
		if err == nil {
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

	if err := h.Service.CreateReviewResponse(r.Context(), user.TenantID, taskID, user.UserID, role, responses, rating); err != nil {
		api.Fail(w, http.StatusInternalServerError, "review_response_failed", "failed to submit response", middleware.GetRequestID(r.Context()))
		return
	}

	status := performance.ReviewTaskStatusCompleted
	switch role {
	case "self":
		if ctxInfo.HRRequired || ctxInfo.ManagerID != "" {
			status = performance.ReviewTaskStatusManagerPending
		}
	case "manager":
		if ctxInfo.HRRequired {
			status = performance.ReviewTaskStatusHRPending
		}
	case "hr":
		status = performance.ReviewTaskStatusCompleted
	}
	if err := h.Service.UpdateReviewTaskStatus(r.Context(), user.TenantID, taskID, status); err != nil {
		slog.Warn("review task status update failed", "err", err)
	}

	if err := h.Audit.Record(r.Context(), user.TenantID, user.UserID, "performance.review.submit", "review_task", taskID, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, map[string]any{"role": role}); err != nil {
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

	employeeID := ""
	managerEmployeeID := ""
	if user.RoleName == auth.RoleEmployee {
		id, err := h.Service.EmployeeIDByUserID(r.Context(), user.TenantID, user.UserID)
		if err != nil {
			slog.Warn("feedback list employee lookup failed", "err", err)
		} else {
			employeeID = id
		}
	}
	if user.RoleName == auth.RoleManager {
		id, err := h.Service.EmployeeIDByUserID(r.Context(), user.TenantID, user.UserID)
		if err != nil {
			slog.Warn("feedback list manager lookup failed", "err", err)
		} else {
			managerEmployeeID = id
		}
	}

	feedbacks, err := h.Service.ListFeedback(r.Context(), user.TenantID, employeeID, managerEmployeeID, user.UserID)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "feedback_list_failed", "failed to list feedback", middleware.GetRequestID(r.Context()))
		return
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
		selfEmployeeID, err := h.Service.EmployeeIDByUserID(r.Context(), user.TenantID, user.UserID)
		if err != nil {
			slog.Warn("feedback create self employee lookup failed", "err", err)
		}
		if selfEmployeeID == "" || payload.ToEmployeeID != selfEmployeeID {
			api.Fail(w, http.StatusForbidden, "forbidden", "not allowed", middleware.GetRequestID(r.Context()))
			return
		}
	}
	if user.RoleName == auth.RoleManager {
		managerEmployeeID, err := h.Service.EmployeeIDByUserID(r.Context(), user.TenantID, user.UserID)
		if err != nil {
			slog.Warn("feedback create manager lookup failed", "err", err)
		}
		allowed, err := h.Service.IsManagerOfEmployee(r.Context(), user.TenantID, payload.ToEmployeeID, managerEmployeeID)
		if err != nil {
			slog.Warn("feedback create manager scope failed", "err", err)
		}
		if !allowed {
			api.Fail(w, http.StatusForbidden, "forbidden", "not allowed", middleware.GetRequestID(r.Context()))
			return
		}
	}

	if err := h.Service.CreateFeedback(r.Context(), user.TenantID, user.UserID, payload.ToEmployeeID, payload.Type, payload.Message, nullIfEmpty(payload.RelatedGoal)); err != nil {
		api.Fail(w, http.StatusInternalServerError, "feedback_create_failed", "failed to create feedback", middleware.GetRequestID(r.Context()))
		return
	}

	toUserID, err := h.Service.EmployeeUserID(r.Context(), user.TenantID, payload.ToEmployeeID)
	if err != nil {
		slog.Warn("feedback recipient user lookup failed", "err", err)
	}
	if toUserID != "" {
		if h.Notify != nil {
			if err := h.Notify.Create(r.Context(), user.TenantID, toUserID, notifications.TypeFeedbackReceived, "New feedback", "You have received new feedback."); err != nil {
				slog.Warn("feedback notification failed", "err", err)
			}
		}
	}
	if err := h.Audit.Record(r.Context(), user.TenantID, user.UserID, "performance.feedback.create", "feedback", "", middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, payload); err != nil {
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

	employeeID := ""
	managerEmployeeID := ""
	if user.RoleName == auth.RoleEmployee {
		id, err := h.Service.EmployeeIDByUserID(r.Context(), user.TenantID, user.UserID)
		if err != nil {
			slog.Warn("checkin list employee lookup failed", "err", err)
		} else {
			employeeID = id
		}
	}
	if user.RoleName == auth.RoleManager {
		id, err := h.Service.EmployeeIDByUserID(r.Context(), user.TenantID, user.UserID)
		if err != nil {
			slog.Warn("checkin list manager lookup failed", "err", err)
		} else {
			managerEmployeeID = id
		}
	}

	checkins, err := h.Service.ListCheckins(r.Context(), user.TenantID, employeeID, managerEmployeeID)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "checkin_list_failed", "failed to list checkins", middleware.GetRequestID(r.Context()))
		return
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
		if id, err := h.Service.EmployeeIDByUserID(r.Context(), user.TenantID, user.UserID); err != nil {
			slog.Warn("checkin create employee lookup failed", "err", err)
		} else {
			payload.EmployeeID = id
		}
	}
	if payload.EmployeeID == "" {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "employee id required", middleware.GetRequestID(r.Context()))
		return
	}
	if user.RoleName == auth.RoleEmployee {
		selfEmployeeID, err := h.Service.EmployeeIDByUserID(r.Context(), user.TenantID, user.UserID)
		if err != nil {
			slog.Warn("checkin create self employee lookup failed", "err", err)
		}
		if selfEmployeeID == "" || payload.EmployeeID != selfEmployeeID {
			api.Fail(w, http.StatusForbidden, "forbidden", "not allowed", middleware.GetRequestID(r.Context()))
			return
		}
	}
	if user.RoleName == auth.RoleManager {
		managerEmployeeID, err := h.Service.EmployeeIDByUserID(r.Context(), user.TenantID, user.UserID)
		if err != nil {
			slog.Warn("checkin create manager lookup failed", "err", err)
		}
		allowed, err := h.Service.IsManagerOfEmployee(r.Context(), user.TenantID, payload.EmployeeID, managerEmployeeID)
		if err != nil {
			slog.Warn("checkin create manager scope failed", "err", err)
		}
		if !allowed {
			api.Fail(w, http.StatusForbidden, "forbidden", "not allowed", middleware.GetRequestID(r.Context()))
			return
		}
	}

	if err := h.Service.CreateCheckin(r.Context(), user.TenantID, payload.EmployeeID, payload.ManagerID, payload.Notes, payload.Private); err != nil {
		api.Fail(w, http.StatusInternalServerError, "checkin_create_failed", "failed to create checkin", middleware.GetRequestID(r.Context()))
		return
	}

	if err := h.Audit.Record(r.Context(), user.TenantID, user.UserID, "performance.checkin.create", "checkin", "", middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, payload); err != nil {
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

	employeeID := ""
	managerEmployeeID := ""
	if user.RoleName == auth.RoleEmployee {
		id, err := h.Service.EmployeeIDByUserID(r.Context(), user.TenantID, user.UserID)
		if err != nil {
			slog.Warn("pip list employee lookup failed", "err", err)
		} else {
			employeeID = id
		}
	}
	if user.RoleName == auth.RoleManager {
		id, err := h.Service.EmployeeIDByUserID(r.Context(), user.TenantID, user.UserID)
		if err != nil {
			slog.Warn("pip list manager lookup failed", "err", err)
		} else {
			managerEmployeeID = id
		}
	}

	pips, err := h.Service.ListPIPs(r.Context(), user.TenantID, employeeID, managerEmployeeID)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "pip_list_failed", "failed to list pips", middleware.GetRequestID(r.Context()))
		return
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
		managerEmployeeID, err := h.Service.EmployeeIDByUserID(r.Context(), user.TenantID, user.UserID)
		if err != nil {
			slog.Warn("pip create manager lookup failed", "err", err)
		}
		allowed, err := h.Service.IsManagerOfEmployee(r.Context(), user.TenantID, employeeID, managerEmployeeID)
		if err != nil {
			slog.Warn("pip create manager scope failed", "err", err)
		}
		if !allowed {
			api.Fail(w, http.StatusForbidden, "forbidden", "not allowed", middleware.GetRequestID(r.Context()))
			return
		}
		managerID = managerEmployeeID
	}

	id, err := h.Service.CreatePIP(r.Context(), user.TenantID, employeeID, managerID, hrOwnerID, objectives, milestones, reviewDates, status)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "pip_create_failed", "failed to create pip", middleware.GetRequestID(r.Context()))
		return
	}

	if err := h.Audit.Record(r.Context(), user.TenantID, user.UserID, "performance.pip.create", "pip", id, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, payload); err != nil {
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
	employeeID, managerID, err := h.Service.GetPIP(r.Context(), user.TenantID, pipID)
	if err != nil {
		api.Fail(w, http.StatusNotFound, "not_found", "pip not found", middleware.GetRequestID(r.Context()))
		return
	}

	if user.RoleName == auth.RoleManager {
		managerEmployeeID, err := h.Service.EmployeeIDByUserID(r.Context(), user.TenantID, user.UserID)
		if err != nil {
			slog.Warn("pip update manager lookup failed", "err", err)
		}
		if managerEmployeeID == "" || managerEmployeeID != managerID {
			api.Fail(w, http.StatusForbidden, "forbidden", "not allowed", middleware.GetRequestID(r.Context()))
			return
		}
	}
	if user.RoleName == auth.RoleEmployee {
		selfEmployeeID, err := h.Service.EmployeeIDByUserID(r.Context(), user.TenantID, user.UserID)
		if err != nil {
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

	if err := h.Service.UpdatePIP(r.Context(), user.TenantID, pipID, status, objectivesJSON, milestonesJSON, reviewDatesJSON); err != nil {
		api.Fail(w, http.StatusInternalServerError, "pip_update_failed", "failed to update pip", middleware.GetRequestID(r.Context()))
		return
	}

	if err := h.Audit.Record(r.Context(), user.TenantID, user.UserID, "performance.pip.update", "pip", pipID, middleware.GetRequestID(r.Context()), shared.ClientIP(r), nil, payload); err != nil {
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
		id, err := h.Service.EmployeeIDByUserID(r.Context(), user.TenantID, user.UserID)
		if err != nil {
			slog.Warn("performance summary manager lookup failed", "err", err)
		} else {
			managerEmployeeID = id
		}
	}

	summary, err := h.Service.PerformanceSummary(r.Context(), user.TenantID, managerEmployeeID)
	if err != nil {
		slog.Warn("performance summary query failed", "err", err)
	}
	api.Success(w, summary, middleware.GetRequestID(r.Context()))
}

func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}
