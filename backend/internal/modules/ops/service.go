package ops

import (
	"context"
	"errors"
	"log"
	"strings"
	"sync/atomic"
	"time"

	agentsmodule "moziboard-backend/internal/modules/agents"
)

type DispatcherKick func()
type AuditLogFunc func(actorType, actorID, entityType, entityID, action string, oldValue, newValue, metadata interface{})

type Service struct {
	repo             *Repository
	dispatcherKick   DispatcherKick
	auditLog         AuditLogFunc
	maintenanceBusy  atomic.Bool
	maintenanceEvery time.Duration
}

func NewService(repo *Repository, dispatcherKick DispatcherKick, auditLog AuditLogFunc) *Service {
	return &Service{repo: repo, dispatcherKick: dispatcherKick, auditLog: auditLog}
}

func (s *Service) ConfigureMaintenanceScheduler(every time.Duration) {
	s.maintenanceEvery = every
}

func (s *Service) StartMaintenanceScheduler(ctx context.Context) {
	if s == nil || s.maintenanceEvery <= 0 {
		return
	}
	go func() {
		ticker := time.NewTicker(s.maintenanceEvery)
		defer ticker.Stop()
		log.Printf("ops_maintenance_scheduler started interval=%s", s.maintenanceEvery)
		for {
			select {
			case <-ctx.Done():
				log.Printf("ops_maintenance_scheduler stopped")
				return
			case <-ticker.C:
				if _, err := s.RunScheduledMaintenanceSweep(ctx); err != nil {
					log.Printf("ops_maintenance_scheduler error=%v", err)
				}
			}
		}
	}()
}

func (s *Service) ListAgentEvents(ctx context.Context, status, agentID, eventType, boardID string) ([]AgentEvent, error) {
	return s.repo.ListAgentEvents(ctx, status, agentID, eventType, boardID)
}

func (s *Service) ListConnectors(ctx context.Context, status, agentID, connectorType, transportMode string) ([]agentsmodule.AgentConnector, error) {
	return s.repo.ListConnectors(ctx, status, agentID, connectorType, transportMode)
}

func (s *Service) ListAuditLogs(ctx context.Context, entityType, entityID, action, boardID string) ([]AuditLog, error) {
	return s.repo.ListAuditLogs(ctx, entityType, entityID, action, boardID)
}

func (s *Service) GetSummary(ctx context.Context, boardID string) (OpsSummary, error) {
	return s.repo.GetOpsSummary(ctx, boardID)
}

func (s *Service) RetryEvent(ctx context.Context, eventID string) error {
	ok, err := s.repo.RetryEvent(ctx, eventID)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("event_not_retryable")
	}
	if s.auditLog != nil {
		metadata, metaErr := s.repo.GetEventBoardContext(ctx, eventID)
		if metaErr != nil || metadata == nil {
			metadata = map[string]interface{}{}
		}
		newValue := map[string]interface{}{"delivery_status": "pending"}
		for k, v := range metadata {
			newValue[k] = v
		}
		s.auditLog("user", "ops", "agent_event", eventID, "retry_requested", nil, newValue, metadata)
	}
	if s.dispatcherKick != nil {
		go s.dispatcherKick()
	}
	return nil
}

func (s *Service) RequeueEvent(ctx context.Context, eventID string) error {
	ok, err := s.repo.RequeueEvent(ctx, eventID)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("event_not_requeueable")
	}
	if s.auditLog != nil {
		metadata, metaErr := s.repo.GetEventBoardContext(ctx, eventID)
		if metaErr != nil || metadata == nil {
			metadata = map[string]interface{}{}
		}
		newValue := map[string]interface{}{"delivery_status": "pending", "delivery_attempts": 0}
		for k, v := range metadata {
			newValue[k] = v
		}
		s.auditLog("user", "ops", "agent_event", eventID, "requeue_requested", nil, newValue, metadata)
	}
	if s.dispatcherKick != nil {
		go s.dispatcherKick()
	}
	return nil
}

func (s *Service) EnableConnector(ctx context.Context, connectorID string) error {
	ok, err := s.repo.SetConnectorStatus(ctx, connectorID, "connected")
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("connector_not_found")
	}
	if s.auditLog != nil {
		metadata, metaErr := s.repo.GetConnectorBoardContext(ctx, connectorID)
		if metaErr != nil || metadata == nil {
			metadata = map[string]interface{}{}
		}
		newValue := map[string]interface{}{"status": "connected"}
		for k, v := range metadata {
			newValue[k] = v
		}
		s.auditLog("user", "ops", "connector", connectorID, "connector_enabled", nil, newValue, metadata)
	}
	return nil
}

func (s *Service) DisableConnector(ctx context.Context, connectorID string) error {
	ok, err := s.repo.SetConnectorStatus(ctx, connectorID, "disabled")
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("connector_not_found")
	}
	if s.auditLog != nil {
		metadata, metaErr := s.repo.GetConnectorBoardContext(ctx, connectorID)
		if metaErr != nil || metadata == nil {
			metadata = map[string]interface{}{}
		}
		newValue := map[string]interface{}{"status": "disabled"}
		for k, v := range metadata {
			newValue[k] = v
		}
		s.auditLog("user", "ops", "connector", connectorID, "connector_disabled", nil, newValue, metadata)
	}
	return nil
}

func (s *Service) RunMaintenanceSweep(ctx context.Context) (MaintenanceReport, error) {
	return s.runMaintenanceSweep(ctx, "manual")
}

func (s *Service) RunScheduledMaintenanceSweep(ctx context.Context) (MaintenanceReport, error) {
	if !s.maintenanceBusy.CompareAndSwap(false, true) {
		return MaintenanceReport{}, errors.New("maintenance_sweep_already_running")
	}
	defer s.maintenanceBusy.Store(false)
	return s.runMaintenanceSweep(ctx, "scheduled")
}

func (s *Service) runMaintenanceSweep(ctx context.Context, trigger string) (MaintenanceReport, error) {
	report := MaintenanceReport{}
	count, err := s.repo.MarkStaleAgentsOffline(ctx)
	if err != nil {
		return report, err
	}
	report.StaleAgentsMarked = count
	count, err = s.repo.CloseStaleRuns(ctx)
	if err != nil {
		return report, err
	}
	report.StaleRunsClosed = count
	count, err = s.repo.RepairTaskRunDrift(ctx)
	if err != nil {
		return report, err
	}
	report.DriftedTasksRepaired = count
	metadata := map[string]interface{}{"trigger": trigger}
	if s.auditLog != nil {
		s.auditLog("system", "ops_maintenance", "maintenance", "sweep", "maintenance_sweep_ran", nil, report, metadata)
	}
	if trigger == "scheduled" || strings.TrimSpace(trigger) == "scheduler" {
		log.Printf("ops_maintenance_sweep trigger=%s stale_agents=%d stale_runs=%d repaired=%d", trigger, report.StaleAgentsMarked, report.StaleRunsClosed, report.DriftedTasksRepaired)
	}
	return report, nil
}
