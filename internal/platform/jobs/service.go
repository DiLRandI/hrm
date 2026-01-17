package jobs

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"hrm/internal/domain/gdpr"
	"hrm/internal/domain/leave"
	"hrm/internal/platform/config"
)

const (
	JobLeaveAccrual = "leave_accrual"
	JobRetention    = "gdpr_retention"
)

type Service struct {
	DB    *pgxpool.Pool
	Cfg   config.Config
	queue chan job
}

type job struct {
	Type     string
	TenantID string
	Run      func(context.Context) (any, error)
}

func New(db *pgxpool.Pool, cfg config.Config) *Service {
	return &Service{
		DB:    db,
		Cfg:   cfg,
		queue: make(chan job, 128),
	}
}

func (s *Service) Start(ctx context.Context) {
	go s.worker(ctx)
	if s.Cfg.LeaveAccrualInterval > 0 {
		go s.scheduleAccruals(ctx, s.Cfg.LeaveAccrualInterval)
	}
	if s.Cfg.RetentionInterval > 0 {
		go s.scheduleRetention(ctx, s.Cfg.RetentionInterval)
	}
}

func (s *Service) Enqueue(jobType, tenantID string, run func(context.Context) (any, error)) {
	select {
	case s.queue <- job{Type: jobType, TenantID: tenantID, Run: run}:
	default:
		slog.Warn("job queue full", "jobType", jobType, "tenantId", tenantID)
	}
}

func (s *Service) RunNow(ctx context.Context, jobType, tenantID string, run func(context.Context) (any, error)) (any, error) {
	return s.runJob(ctx, job{Type: jobType, TenantID: tenantID, Run: run})
}

func (s *Service) worker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case j := <-s.queue:
			if _, err := s.runJob(ctx, j); err != nil {
				slog.Warn("job run failed", "jobType", j.Type, "tenantId", j.TenantID, "err", err)
			}
		}
	}
}

func (s *Service) runJob(ctx context.Context, j job) (any, error) {
	runID := ""
	if err := s.DB.QueryRow(ctx, `
    INSERT INTO job_runs (tenant_id, job_type, status)
    VALUES ($1,$2,$3)
    RETURNING id
  `, j.TenantID, j.Type, "running").Scan(&runID); err != nil {
		slog.Warn("job run insert failed", "err", err)
	}

	details, err := j.Run(ctx)
	status := "completed"
	if err != nil {
		status = "failed"
	}
	detailsJSON, marshalErr := json.Marshal(details)
	if marshalErr != nil {
		slog.Warn("job details marshal failed", "err", marshalErr)
		detailsJSON = []byte("{}")
	}
	if runID != "" {
		if _, updErr := s.DB.Exec(ctx, `
      UPDATE job_runs
      SET status = $1, details_json = $2, completed_at = now()
      WHERE id = $3
    `, status, detailsJSON, runID); updErr != nil {
			slog.Warn("job run update failed", "err", updErr)
		}
	}
	return details, err
}

func (s *Service) scheduleAccruals(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			tenants, err := s.listTenants(ctx)
			if err != nil {
				slog.Warn("accrual scheduler tenant lookup failed", "err", err)
				continue
			}
			for _, tenantID := range tenants {
				tenant := tenantID
				s.Enqueue(JobLeaveAccrual, tenant, func(ctx context.Context) (any, error) {
					return leave.ApplyAccruals(ctx, s.DB, tenant, time.Now())
				})
			}
		}
	}
}

func (s *Service) scheduleRetention(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			tenants, err := s.listTenants(ctx)
			if err != nil {
				slog.Warn("retention scheduler tenant lookup failed", "err", err)
				continue
			}
			for _, tenantID := range tenants {
				tenant := tenantID
				policies, err := s.listRetentionPolicies(ctx, tenant)
				if err != nil {
					slog.Warn("retention policies lookup failed", "tenantId", tenant, "err", err)
					continue
				}
				for _, policy := range policies {
					p := policy
					if p.RetentionDays <= 0 {
						continue
					}
					cutoff := time.Now().AddDate(0, 0, -p.RetentionDays)
					s.Enqueue(JobRetention, tenant, func(ctx context.Context) (any, error) {
						deleted, err := gdpr.ApplyRetention(ctx, s.DB, tenant, p.DataCategory, cutoff)
						return map[string]any{
							"dataCategory": p.DataCategory,
							"cutoffDate":   cutoff,
							"deleted":      deleted,
						}, err
					})
				}
			}
		}
	}
}

type retentionPolicy struct {
	DataCategory string
	RetentionDays int
}

func (s *Service) listRetentionPolicies(ctx context.Context, tenantID string) ([]retentionPolicy, error) {
	rows, err := s.DB.Query(ctx, `
    SELECT data_category, retention_days
    FROM retention_policies
    WHERE tenant_id = $1
  `, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []retentionPolicy
	for rows.Next() {
		var p retentionPolicy
		if err := rows.Scan(&p.DataCategory, &p.RetentionDays); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

func (s *Service) listTenants(ctx context.Context) ([]string, error) {
	rows, err := s.DB.Query(ctx, `SELECT id FROM tenants`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}
