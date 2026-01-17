package performance

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"hrm/internal/api"
	"hrm/internal/middleware"
)

type Handler struct {
	DB *pgxpool.Pool
}

func NewHandler(db *pgxpool.Pool) *Handler {
	return &Handler{DB: db}
}

type Goal struct {
	ID          string    `json:"id"`
	EmployeeID  string    `json:"employeeId"`
	ManagerID   string    `json:"managerId"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Metric      string    `json:"metric"`
	DueDate     time.Time `json:"dueDate"`
	Weight      float64   `json:"weight"`
	Status      string    `json:"status"`
	Progress    float64   `json:"progress"`
}

type ReviewCycle struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	StartDate time.Time `json:"startDate"`
	EndDate   time.Time `json:"endDate"`
	Status    string    `json:"status"`
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/performance", func(r chi.Router) {
		r.Get("/goals", h.handleListGoals)
		r.Post("/goals", h.handleCreateGoal)
		r.Post("/goals/{goalID}/comments", h.handleAddGoalComment)
		r.Get("/review-cycles", h.handleListReviewCycles)
		r.Post("/review-cycles", h.handleCreateReviewCycle)
		r.Get("/review-tasks", h.handleListReviewTasks)
		r.Post("/review-tasks/{taskID}/responses", h.handleSubmitReviewResponse)
		r.Get("/feedback", h.handleListFeedback)
		r.Post("/feedback", h.handleCreateFeedback)
		r.Get("/checkins", h.handleListCheckins)
		r.Post("/checkins", h.handleCreateCheckin)
		r.Get("/pips", h.handleListPIPs)
		r.Post("/pips", h.handleCreatePIP)
	})
}

func (h *Handler) handleListGoals(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	rows, err := h.DB.Query(r.Context(), `
    SELECT id, employee_id, manager_id, title, description, metric, due_date, weight, status, progress
    FROM goals
    WHERE tenant_id = $1
    ORDER BY created_at DESC
  `, user.TenantID)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "goal_list_failed", "failed to list goals", middleware.GetRequestID(r.Context()))
		return
	}
	defer rows.Close()

	var goals []Goal
	for rows.Next() {
		var g Goal
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

	var payload Goal
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "invalid request payload", middleware.GetRequestID(r.Context()))
		return
	}

	var id string
	err := h.DB.QueryRow(r.Context(), `
    INSERT INTO goals (tenant_id, employee_id, manager_id, title, description, metric, due_date, weight, status, progress)
    VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
    RETURNING id
  `, user.TenantID, payload.EmployeeID, payload.ManagerID, payload.Title, payload.Description, payload.Metric, payload.DueDate, payload.Weight, payload.Status, payload.Progress).Scan(&id)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "goal_create_failed", "failed to create goal", middleware.GetRequestID(r.Context()))
		return
	}
	api.Created(w, map[string]string{"id": id}, middleware.GetRequestID(r.Context()))
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

func (h *Handler) handleListReviewCycles(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}
	rows, err := h.DB.Query(r.Context(), `
    SELECT id, name, start_date, end_date, status
    FROM review_cycles
    WHERE tenant_id = $1
    ORDER BY start_date DESC
  `, user.TenantID)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "review_cycle_failed", "failed to list review cycles", middleware.GetRequestID(r.Context()))
		return
	}
	defer rows.Close()

	var cycles []ReviewCycle
	for rows.Next() {
		var c ReviewCycle
		if err := rows.Scan(&c.ID, &c.Name, &c.StartDate, &c.EndDate, &c.Status); err != nil {
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
	if user.RoleName != "HR" {
		api.Fail(w, http.StatusForbidden, "forbidden", "hr role required", middleware.GetRequestID(r.Context()))
		return
	}

	var payload ReviewCycle
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "invalid request payload", middleware.GetRequestID(r.Context()))
		return
	}

	var id string
	err := h.DB.QueryRow(r.Context(), `
    INSERT INTO review_cycles (tenant_id, name, start_date, end_date, status)
    VALUES ($1,$2,$3,$4,$5)
    RETURNING id
  `, user.TenantID, payload.Name, payload.StartDate, payload.EndDate, payload.Status).Scan(&id)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "review_cycle_create_failed", "failed to create review cycle", middleware.GetRequestID(r.Context()))
		return
	}
	api.Created(w, map[string]string{"id": id}, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleListReviewTasks(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}
	rows, err := h.DB.Query(r.Context(), `
    SELECT id, cycle_id, employee_id, manager_id, status, self_due, manager_due, hr_due
    FROM review_tasks
    WHERE tenant_id = $1
    ORDER BY created_at DESC
  `, user.TenantID)
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
	var payload map[string]any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "invalid request payload", middleware.GetRequestID(r.Context()))
		return
	}

	responses, _ := json.Marshal(payload["responses"])
	rating, _ := payload["rating"].(float64)
	role, _ := payload["role"].(string)

	_, err := h.DB.Exec(r.Context(), `
    INSERT INTO review_responses (tenant_id, task_id, respondent_id, role, responses_json, rating, submitted_at)
    VALUES ($1,$2,$3,$4,$5,$6,now())
  `, user.TenantID, taskID, user.UserID, role, responses, rating)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "review_response_failed", "failed to submit response", middleware.GetRequestID(r.Context()))
		return
	}

	api.Created(w, map[string]string{"status": "submitted"}, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleListFeedback(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	rows, err := h.DB.Query(r.Context(), `
    SELECT id, from_user_id, to_employee_id, type, message, related_goal_id, created_at
    FROM feedback
    WHERE tenant_id = $1
    ORDER BY created_at DESC
  `, user.TenantID)
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

	_, err := h.DB.Exec(r.Context(), `
    INSERT INTO feedback (tenant_id, from_user_id, to_employee_id, type, message, related_goal_id)
    VALUES ($1,$2,$3,$4,$5,$6)
  `, user.TenantID, user.UserID, payload.ToEmployeeID, payload.Type, payload.Message, nullIfEmpty(payload.RelatedGoal))
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "feedback_create_failed", "failed to create feedback", middleware.GetRequestID(r.Context()))
		return
	}

	api.Created(w, map[string]string{"status": "feedback_created"}, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleListCheckins(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	rows, err := h.DB.Query(r.Context(), `
    SELECT id, employee_id, manager_id, notes, private, created_at
    FROM checkins
    WHERE tenant_id = $1
    ORDER BY created_at DESC
  `, user.TenantID)
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

	_, err := h.DB.Exec(r.Context(), `
    INSERT INTO checkins (tenant_id, employee_id, manager_id, notes, private)
    VALUES ($1,$2,$3,$4,$5)
  `, user.TenantID, payload.EmployeeID, nullIfEmpty(payload.ManagerID), payload.Notes, payload.Private)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "checkin_create_failed", "failed to create checkin", middleware.GetRequestID(r.Context()))
		return
	}

	api.Created(w, map[string]string{"status": "checkin_created"}, middleware.GetRequestID(r.Context()))
}

func (h *Handler) handleListPIPs(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.GetUser(r.Context())
	if !ok {
		api.Fail(w, http.StatusUnauthorized, "unauthorized", "authentication required", middleware.GetRequestID(r.Context()))
		return
	}

	rows, err := h.DB.Query(r.Context(), `
    SELECT id, employee_id, manager_id, hr_owner_id, objectives_json, milestones_json, review_dates_json, status, created_at
    FROM pips
    WHERE tenant_id = $1
    ORDER BY created_at DESC
  `, user.TenantID)
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
	if user.RoleName != "HR" && user.RoleName != "Manager" {
		api.Fail(w, http.StatusForbidden, "forbidden", "manager or hr required", middleware.GetRequestID(r.Context()))
		return
	}

	var payload map[string]any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		api.Fail(w, http.StatusBadRequest, "invalid_payload", "invalid request payload", middleware.GetRequestID(r.Context()))
		return
	}

	employeeID, _ := payload["employeeId"].(string)
	managerID, _ := payload["managerId"].(string)
	hrOwnerID, _ := payload["hrOwnerId"].(string)
	objectives, _ := json.Marshal(payload["objectives"])
	milestones, _ := json.Marshal(payload["milestones"])
	reviewDates, _ := json.Marshal(payload["reviewDates"])
	status, _ := payload["status"].(string)

	var id string
	err := h.DB.QueryRow(r.Context(), `
    INSERT INTO pips (tenant_id, employee_id, manager_id, hr_owner_id, objectives_json, milestones_json, review_dates_json, status)
    VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
    RETURNING id
  `, user.TenantID, employeeID, nullIfEmpty(managerID), nullIfEmpty(hrOwnerID), objectives, milestones, reviewDates, status).Scan(&id)
	if err != nil {
		api.Fail(w, http.StatusInternalServerError, "pip_create_failed", "failed to create pip", middleware.GetRequestID(r.Context()))
		return
	}

	api.Created(w, map[string]string{"id": id}, middleware.GetRequestID(r.Context()))
}

func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}
