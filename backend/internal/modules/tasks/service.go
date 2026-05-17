package tasks

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"moziboard-backend/internal/workflow"
)

type LogActivityFunc func(taskID int, userID, action, details string)
type BroadcastFunc func(msg string)
type Embedder func(text string) (string, error)

type AgentInfra interface {
	CreateNotification(taskID *int, targetAgentID string, sourceAgentID *string, notifType, content string)
	CreateAgentRun(taskID int, agentID string, status string, activity string) *int
	EnqueueAgentEvent(agentID string, taskID int, eventType string, payload map[string]interface{}) *int
}

type AgentStateUpdater interface {
	MarkQueued(taskID int, runID int, agentID string)
	MarkBlocked(blockedReason string, agentID string)
	MarkOnline(agentID string)
	MarkInProgress(taskID int, agentID string)
}

type AgentStateUpdaterFuncs struct {
	MarkQueuedFunc     func(taskID int, runID int, agentID string)
	MarkBlockedFunc    func(blockedReason string, agentID string)
	MarkOnlineFunc     func(agentID string)
	MarkInProgressFunc func(taskID int, agentID string)
}

func (f AgentStateUpdaterFuncs) MarkQueued(taskID int, runID int, agentID string) {
	if f.MarkQueuedFunc != nil {
		f.MarkQueuedFunc(taskID, runID, agentID)
	}
}

func (f AgentStateUpdaterFuncs) MarkBlocked(blockedReason string, agentID string) {
	if f.MarkBlockedFunc != nil {
		f.MarkBlockedFunc(blockedReason, agentID)
	}
}

func (f AgentStateUpdaterFuncs) MarkOnline(agentID string) {
	if f.MarkOnlineFunc != nil {
		f.MarkOnlineFunc(agentID)
	}
}

func (f AgentStateUpdaterFuncs) MarkInProgress(taskID int, agentID string) {
	if f.MarkInProgressFunc != nil {
		f.MarkInProgressFunc(taskID, agentID)
	}
}

type BoardStatsUpdater interface {
	RecomputeBoard(ctx context.Context, boardID string) error
}

type AuditLogFunc func(actorType, actorID, entityType, entityID, action string, oldValue, newValue, metadata interface{})

type SideEffects struct {
	AgentInfra        AgentInfra
	LogActivity       LogActivityFunc
	AuditLog          AuditLogFunc
	Broadcast         BroadcastFunc
	AgentStateUpdater AgentStateUpdater
	Embedder          Embedder
	BoardStats        BoardStatsUpdater
}

type Service struct {
	repo        *Repository
	sideEffects SideEffects
}

func NewService(repo *Repository, sideEffects SideEffects) *Service {
	return &Service{repo: repo, sideEffects: sideEffects}
}

func (s *Service) ListByBoard(ctx context.Context, boardID string) ([]Task, error) {
	return s.repo.ListByBoard(ctx, boardID)
}

func (s *Service) ListActivities(ctx context.Context, taskID int) ([]Activity, error) {
	return s.repo.ListActivities(ctx, taskID)
}

func (s *Service) Search(ctx context.Context, boardID string, query string) ([]Task, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, errors.New("query required")
	}
	boardID = strings.TrimSpace(boardID)
	if boardID == "" {
		return nil, errors.New("board_id required")
	}
	if s.sideEffects.Embedder == nil {
		return nil, errors.New("embedder not configured")
	}
	embedding, err := s.sideEffects.Embedder(query)
	if err != nil {
		return nil, err
	}
	return s.repo.Search(ctx, boardID, query, embedding)
}

func (s *Service) Create(ctx context.Context, t *Task) error {
	if t.BoardID == "" {
		defaultID, err := s.repo.GetDefaultBoardID(ctx)
		if err != nil || defaultID == "" {
			return errors.New("no board found")
		}
		t.BoardID = defaultID
	}
	t.Title = strings.TrimSpace(t.Title)
	if t.Title == "" {
		return errors.New("title is required")
	}
	if t.AssigneeID != nil && strings.TrimSpace(*t.AssigneeID) != "" {
		ok, err := s.repo.IsBoardMember(ctx, t.BoardID, strings.TrimSpace(*t.AssigneeID))
		if err != nil {
			return err
		}
		if !ok {
			return errors.New("assignee is not a board member")
		}
	}

	hasAssignee := t.AssigneeID != nil && strings.TrimSpace(*t.AssigneeID) != ""
	status := workflow.NormalizeTaskState(t.Status)
	if strings.TrimSpace(t.Status) == "" {
		if hasAssignee {
			status = workflow.StateAssigned
		} else {
			status = workflow.StateTodo
		}
	}
	if status == workflow.StateBlocked && strings.TrimSpace(t.BlockedReason) == "" {
		return errors.New("blocked_reason is required when moving task to blocked")
	}
	if status == workflow.StateReview || status == workflow.StateDone {
		return errors.New("initial task state is not allowed")
	}
	t.Status = status
	t.ListID = workflow.StatusToListID(status)
	if t.Position <= 0 {
		next, err := s.repo.NextPosition(ctx, t.BoardID, t.ListID)
		if err == nil {
			t.Position = next
		}
	}
	if err := s.repo.Create(ctx, t); err != nil {
		return err
	}
	if hasAssignee {
		s.onAssigned(t.ID, t.BoardID, t.Title, *t.AssigneeID, "system")
	}
	if s.sideEffects.AuditLog != nil {
		s.sideEffects.AuditLog("system", "system", "task", fmt.Sprintf("%d", t.ID), "task_created", nil, map[string]interface{}{"status": t.Status, "list_id": t.ListID, "assignee_id": t.AssigneeID}, map[string]interface{}{"board_id": t.BoardID})
	}
	if s.sideEffects.LogActivity != nil {
		details := fmt.Sprintf("Task created with status %s", t.Status)
		go s.sideEffects.LogActivity(t.ID, "system", "created", details)
		if hasAssignee {
			go s.sideEffects.LogActivity(t.ID, "system", "task_assigned", fmt.Sprintf("Assigned to %s", *t.AssigneeID))
		}
	}
	go s.updateSearchEmbedding(t.ID, t.Title+" "+t.Description)
	s.afterTaskMutation(t.BoardID)
	return nil
}

func (s *Service) Update(ctx context.Context, id int, patch *Task) (Task, error) {
	oldTask, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return Task{}, err
	}
	newTask := *patch
	if newTask.BoardID == "" {
		newTask.BoardID = oldTask.BoardID
	}
	if newTask.Title == "" {
		newTask.Title = oldTask.Title
	}
	if newTask.Description == "" {
		newTask.Description = oldTask.Description
	}
	if newTask.AssigneeID == nil {
		newTask.AssigneeID = oldTask.AssigneeID
	}
	if newTask.ParentID == nil {
		newTask.ParentID = oldTask.ParentID
	}
	oldAssignee, hadAssignee := normalizedAssignee(oldTask.AssigneeID)
	newAssignee, hasAssignee := normalizedAssignee(newTask.AssigneeID)
	if hasAssignee {
		ok, err := s.repo.IsBoardMember(ctx, newTask.BoardID, newAssignee)
		if err != nil {
			return Task{}, err
		}
		if !ok {
			return Task{}, errors.New("assignee is not a board member")
		}
		newTask.AssigneeID = &newAssignee
	}

	candidateStatus := strings.TrimSpace(newTask.Status)
	if candidateStatus == "" {
		candidateStatus = oldTask.Status
	}
	candidateStatus = workflow.ResolveAssignmentState(candidateStatus, hadAssignee, hasAssignee)
	candidateStatus = workflow.NormalizeTaskState(candidateStatus)
	if candidateStatus == workflow.StateBlocked && strings.TrimSpace(newTask.BlockedReason) == "" {
		newTask.BlockedReason = oldTask.BlockedReason
	}

	if _, err := workflow.ValidateTransition(workflow.TransitionRequest{
		FromState: oldTask.Status,
		ToState:   candidateStatus,
		Trigger:   workflow.TriggerManualUpdate,
		ActorType: "user",
		ActorID:   newTask.UpdatedBy,
		Reason:    newTask.BlockedReason,
	}); err != nil {
		return Task{}, err
	}

	newTask.Status = candidateStatus
	newTask.ListID = workflow.StatusToListID(candidateStatus)
	if candidateStatus != workflow.StateBlocked && oldTask.Status == workflow.StateBlocked && strings.TrimSpace(patch.BlockedReason) == "" {
		newTask.BlockedReason = ""
	}
	if newTask.Position <= 0 {
		next, err := s.repo.NextPosition(ctx, newTask.BoardID, newTask.ListID)
		if err == nil {
			newTask.Position = next
		}
	}
	if err := s.repo.Update(ctx, id, &newTask); err != nil {
		return Task{}, err
	}
	userID := "system"
	if strings.TrimSpace(newTask.UpdatedBy) != "" {
		userID = newTask.UpdatedBy
	}
	if newTask.Status != oldTask.Status && s.sideEffects.LogActivity != nil {
		result, _ := workflow.ValidateTransition(workflow.TransitionRequest{FromState: oldTask.Status, ToState: newTask.Status, Trigger: workflow.TriggerManualUpdate, ActorType: "user", ActorID: userID, Reason: newTask.BlockedReason})
		go s.sideEffects.LogActivity(id, userID, result.ActivityAction, result.ActivityDetail)
	}
	if s.sideEffects.AuditLog != nil {
		s.sideEffects.AuditLog("user", userID, "task", fmt.Sprintf("%d", id), "task_updated", map[string]interface{}{"status": oldTask.Status, "list_id": oldTask.ListID, "assignee_id": oldTask.AssigneeID, "blocked_reason": oldTask.BlockedReason}, map[string]interface{}{"status": newTask.Status, "list_id": newTask.ListID, "assignee_id": newTask.AssigneeID, "blocked_reason": newTask.BlockedReason}, map[string]interface{}{"board_id": newTask.BoardID})
	}
	if newAssignee != oldAssignee {
		if newAssignee != "" {
			if s.sideEffects.LogActivity != nil {
				go s.sideEffects.LogActivity(id, userID, "task_assigned", fmt.Sprintf("Assigned to %s", newAssignee))
			}
			s.onAssigned(id, newTask.BoardID, newTask.Title, newAssignee, userID)
		} else if s.sideEffects.LogActivity != nil {
			go s.sideEffects.LogActivity(id, userID, "task_unassigned", "Removed assignee")
			if oldAssignee != "" && s.sideEffects.AgentStateUpdater != nil {
				s.sideEffects.AgentStateUpdater.MarkOnline(oldAssignee)
			}
		}
	}
	if newTask.Status == workflow.StateReview && oldTask.Status != workflow.StateReview && newAssignee != "" && s.sideEffects.AgentInfra != nil {
		s.sideEffects.AgentInfra.CreateNotification(&id, newAssignee, nil, "review_requested", fmt.Sprintf("Task ready for review: %s", newTask.Title))
	}
	if newTask.Status == workflow.StateBlocked && oldTask.Status != workflow.StateBlocked && newAssignee != "" {
		if s.sideEffects.AgentInfra != nil {
			s.sideEffects.AgentInfra.CreateNotification(&id, newAssignee, nil, "blocked", fmt.Sprintf("Task blocked: %s", newTask.BlockedReason))
		}
		if s.sideEffects.AgentStateUpdater != nil {
			s.sideEffects.AgentStateUpdater.MarkBlocked(newTask.BlockedReason, newAssignee)
		}
	}
	if newTask.Status == workflow.StateDone && newAssignee != "" && s.sideEffects.AgentStateUpdater != nil {
		s.sideEffects.AgentStateUpdater.MarkOnline(newAssignee)
	}
	if newTask.Description != oldTask.Description && s.sideEffects.LogActivity != nil {
		go s.sideEffects.LogActivity(id, userID, "updated", "Updated task description")
	}
	go s.updateSearchEmbedding(id, newTask.Title+" "+newTask.Description)
	s.afterTaskMutation(oldTask.BoardID)
	if newTask.BoardID != oldTask.BoardID {
		s.afterTaskMutation(newTask.BoardID)
	}
	freshTask, err := s.repo.GetByID(ctx, id)
	if err != nil {
		newTask.ID = id
		return newTask, nil
	}
	freshTask.UpdatedBy = newTask.UpdatedBy
	return freshTask, nil
}

func (s *Service) ApplyReviewAction(ctx context.Context, id int, req ReviewActionReq) (Task, error) {
	oldTask, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return Task{}, err
	}
	action := strings.TrimSpace(req.Action)
	if action == "" {
		return Task{}, errors.New("review action is required")
	}
	toState := ""
	switch action {
	case workflow.ReviewApprove:
		toState = workflow.StateDone
	case workflow.ReviewRequestChange, workflow.ReviewReopen:
		toState = workflow.StateInProgress
	default:
		return Task{}, errors.New("invalid review action")
	}
	result, err := workflow.ValidateTransition(workflow.TransitionRequest{
		FromState:    oldTask.Status,
		ToState:      toState,
		Trigger:      workflow.TriggerReviewAction,
		ActorType:    "user",
		ActorID:      req.UpdatedBy,
		Note:         req.Note,
		ReviewAction: action,
	})
	if err != nil {
		return Task{}, err
	}
	patch := oldTask
	patch.Status = result.NormalizedTo
	patch.ListID = workflow.StatusToListID(result.NormalizedTo)
	patch.UpdatedBy = req.UpdatedBy
	if result.NormalizedTo != workflow.StateBlocked {
		patch.BlockedReason = ""
	}
	if err := s.repo.Update(ctx, id, &patch); err != nil {
		return Task{}, err
	}
	userID := "system"
	if strings.TrimSpace(req.UpdatedBy) != "" {
		userID = req.UpdatedBy
	}
	if s.sideEffects.LogActivity != nil {
		go s.sideEffects.LogActivity(id, userID, result.ActivityAction, result.ActivityDetail)
	}
	if s.sideEffects.AuditLog != nil {
		s.sideEffects.AuditLog("user", userID, "task", fmt.Sprintf("%d", id), result.ActivityAction, map[string]interface{}{"status": oldTask.Status}, map[string]interface{}{"status": result.NormalizedTo}, map[string]interface{}{"review_action": action, "note": req.Note})
	}
	assignee, hasAssignee := normalizedAssignee(oldTask.AssigneeID)
	if hasAssignee {
		if result.NormalizedTo == workflow.StateDone {
			_ = s.repo.MarkLatestRunDone(ctx, id, assignee, req.Note)
			if s.sideEffects.AgentStateUpdater != nil {
				s.sideEffects.AgentStateUpdater.MarkOnline(assignee)
			}
		} else if result.NormalizedTo == workflow.StateInProgress {
			_ = s.repo.MarkLatestRunInProgress(ctx, id, assignee, req.Note)
			if s.sideEffects.AgentStateUpdater != nil {
				s.sideEffects.AgentStateUpdater.MarkInProgress(id, assignee)
			}
		}
	}
	s.afterTaskMutation(oldTask.BoardID)
	freshTask, err := s.repo.GetByID(ctx, id)
	if err != nil {
		patch.ID = id
		return patch, nil
	}
	return freshTask, nil
}

func (s *Service) onAssigned(taskID int, boardID, title, assigneeID, actorID string) {
	if s.sideEffects.AgentInfra != nil {
		s.sideEffects.AgentInfra.CreateNotification(&taskID, assigneeID, nil, "task_assigned", fmt.Sprintf("Task assigned: %s", title))
		if runID := s.sideEffects.AgentInfra.CreateAgentRun(taskID, assigneeID, workflow.MapTaskStateToRunState(workflow.StateAssigned), "Awaiting pickup"); runID != nil && s.sideEffects.AgentStateUpdater != nil {
			s.sideEffects.AgentStateUpdater.MarkQueued(taskID, *runID, assigneeID)
		}
		s.sideEffects.AgentInfra.EnqueueAgentEvent(assigneeID, taskID, "task.assigned", map[string]interface{}{"task_id": taskID, "title": title, "status": workflow.StateAssigned, "board_id": boardID})
	}
}

func (s *Service) afterTaskMutation(boardID string) {
	if s.sideEffects.Broadcast != nil {
		go s.sideEffects.Broadcast("UPDATE")
	}
	if s.sideEffects.BoardStats != nil {
		go s.sideEffects.BoardStats.RecomputeBoard(context.Background(), boardID)
	}
}

func (s *Service) updateSearchEmbedding(taskID int, text string) {
	if s.sideEffects.Embedder == nil {
		return
	}
	embedding, err := s.sideEffects.Embedder(text)
	if err != nil {
		log.Printf("task embedding err: %v", err)
		return
	}
	if err := s.repo.UpdateEmbedding(context.Background(), taskID, embedding); err != nil {
		log.Printf("task embedding db err: %v", err)
	}
}

func normalizedAssignee(id *string) (string, bool) {
	if id == nil {
		return "", false
	}
	v := strings.TrimSpace(*id)
	if v == "" {
		return "", false
	}
	return v, true
}
