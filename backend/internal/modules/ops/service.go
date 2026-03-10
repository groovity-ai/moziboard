package ops

import (
	"context"
	"errors"

	agentsmodule "moziboard-backend/internal/modules/agents"
)

type DispatcherKick func()

type Service struct {
	repo           *Repository
	dispatcherKick DispatcherKick
}

func NewService(repo *Repository, dispatcherKick DispatcherKick) *Service {
	return &Service{repo: repo, dispatcherKick: dispatcherKick}
}

func (s *Service) ListAgentEvents(ctx context.Context, status, agentID, eventType, boardID string) ([]AgentEvent, error) {
	return s.repo.ListAgentEvents(ctx, status, agentID, eventType, boardID)
}

func (s *Service) ListConnectors(ctx context.Context, status, agentID, connectorType, transportMode string) ([]agentsmodule.AgentConnector, error) {
	return s.repo.ListConnectors(ctx, status, agentID, connectorType, transportMode)
}

func (s *Service) RetryEvent(ctx context.Context, eventID string) error {
	ok, err := s.repo.RetryEvent(ctx, eventID)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("event_not_retryable")
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
	return nil
}
