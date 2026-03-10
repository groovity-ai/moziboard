package tasks

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
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
}

type AgentStateUpdaterFuncs struct {
	MarkQueuedFunc  func(taskID int, runID int, agentID string)
	MarkBlockedFunc func(blockedReason string, agentID string)
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

type BoardStatsUpdater interface {
	RecomputeBoard(ctx context.Context, boardID string) error
}

type SideEffects struct {
	AgentInfra        AgentInfra
	LogActivity       LogActivityFunc
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
	if t.ListID == "" {
		t.ListID = "todo"
	}
	t.Status = NormalizeTaskStatus(t.ListID)
	t.ListID = StatusToListID(t.Status)
	if t.Position <= 0 {
		next, err := s.repo.NextPosition(ctx, t.BoardID, t.ListID)
		if err == nil {
			t.Position = next
		}
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
	if err := s.repo.Create(ctx, t); err != nil {
		return err
	}
	if t.AssigneeID != nil && *t.AssigneeID != "" {
		if s.sideEffects.AgentInfra != nil {
			s.sideEffects.AgentInfra.CreateNotification(&t.ID, *t.AssigneeID, nil, "task_assigned", fmt.Sprintf("New task assigned: %s", t.Title))
		}
		if s.sideEffects.AgentInfra != nil {
			if runID := s.sideEffects.AgentInfra.CreateAgentRun(t.ID, *t.AssigneeID, "queued", "Awaiting pickup"); runID != nil && s.sideEffects.AgentStateUpdater != nil {
				s.sideEffects.AgentStateUpdater.MarkQueued(t.ID, *runID, *t.AssigneeID)
			}
		}
		if s.sideEffects.AgentInfra != nil {
			s.sideEffects.AgentInfra.EnqueueAgentEvent(*t.AssigneeID, t.ID, "task.assigned", map[string]interface{}{"task_id": t.ID, "title": t.Title, "status": t.Status, "board_id": t.BoardID})
		}
	}
	if s.sideEffects.LogActivity != nil {
		go s.sideEffects.LogActivity(t.ID, "system", "created", fmt.Sprintf("Task created with status %s", t.Status))
	}
	go s.updateSearchEmbedding(t.ID, t.Title+" "+t.Description)
	if s.sideEffects.Broadcast != nil {
		go s.sideEffects.Broadcast("UPDATE")
	}
	if s.sideEffects.BoardStats != nil {
		go s.sideEffects.BoardStats.RecomputeBoard(context.Background(), t.BoardID)
	}
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
	if newTask.ListID == "" {
		newTask.ListID = oldTask.ListID
	}
	if newTask.AssigneeID == nil {
		newTask.AssigneeID = oldTask.AssigneeID
	}
	if newTask.AssigneeID != nil && strings.TrimSpace(*newTask.AssigneeID) != "" {
		ok, err := s.repo.IsBoardMember(ctx, newTask.BoardID, strings.TrimSpace(*newTask.AssigneeID))
		if err != nil {
			return Task{}, err
		}
		if !ok {
			return Task{}, errors.New("assignee is not a board member")
		}
	}
	if newTask.ParentID == nil {
		newTask.ParentID = oldTask.ParentID
	}
	if newTask.Status == "" {
		newTask.Status = NormalizeTaskStatus(newTask.ListID)
	}
	newTask.ListID = StatusToListID(newTask.Status)
	if newTask.Position <= 0 {
		next, err := s.repo.NextPosition(ctx, newTask.BoardID, newTask.ListID)
		if err == nil {
			newTask.Position = next
		}
	}
	if newTask.Status == "blocked" && newTask.BlockedReason == "" {
		newTask.BlockedReason = oldTask.BlockedReason
	}
	if err := s.repo.Update(ctx, id, &newTask); err != nil {
		return Task{}, err
	}
	userID := "system"
	if newTask.UpdatedBy != "" {
		userID = newTask.UpdatedBy
	}
	if newTask.ListID != oldTask.ListID || newTask.Status != oldTask.Status {
		if s.sideEffects.LogActivity != nil {
			go s.sideEffects.LogActivity(id, userID, "moved", fmt.Sprintf("Moved to %s", newTask.Status))
		}
	}
	newAssignee, oldAssignee := "", ""
	if newTask.AssigneeID != nil {
		newAssignee = *newTask.AssigneeID
	}
	if oldTask.AssigneeID != nil {
		oldAssignee = *oldTask.AssigneeID
	}
	if newAssignee != oldAssignee {
		if newAssignee != "" {
			if s.sideEffects.LogActivity != nil {
				go s.sideEffects.LogActivity(id, userID, "assigned", fmt.Sprintf("Assigned to %s", newAssignee))
			}
			if s.sideEffects.AgentInfra != nil {
				s.sideEffects.AgentInfra.CreateNotification(&id, newAssignee, nil, "task_assigned", fmt.Sprintf("Task assigned: %s", newTask.Title))
			}
			if s.sideEffects.AgentInfra != nil {
				if runID := s.sideEffects.AgentInfra.CreateAgentRun(id, newAssignee, "queued", "Awaiting pickup"); runID != nil && s.sideEffects.AgentStateUpdater != nil {
					s.sideEffects.AgentStateUpdater.MarkQueued(id, *runID, newAssignee)
				}
			}
			if s.sideEffects.AgentInfra != nil {
				s.sideEffects.AgentInfra.EnqueueAgentEvent(newAssignee, id, "task.assigned", map[string]interface{}{"task_id": id, "title": newTask.Title, "status": newTask.Status, "board_id": newTask.BoardID})
			}
		} else {
			if s.sideEffects.LogActivity != nil {
				go s.sideEffects.LogActivity(id, userID, "unassigned", "Removed assignee")
			}
		}
	}
	if newTask.Status == "review" && oldTask.Status != "review" && oldTask.AssigneeID != nil && s.sideEffects.AgentInfra != nil {
		s.sideEffects.AgentInfra.CreateNotification(&id, *oldTask.AssigneeID, nil, "review_requested", fmt.Sprintf("Task ready for review: %s", newTask.Title))
	}
	if newTask.Status == "blocked" && oldTask.Status != "blocked" && newTask.AssigneeID != nil {
		if s.sideEffects.AgentInfra != nil {
			s.sideEffects.AgentInfra.CreateNotification(&id, *newTask.AssigneeID, nil, "blocked", fmt.Sprintf("Task blocked: %s", newTask.BlockedReason))
		}
		if s.sideEffects.AgentStateUpdater != nil {
			s.sideEffects.AgentStateUpdater.MarkBlocked(newTask.BlockedReason, *newTask.AssigneeID)
		}
	}
	if newTask.Description != oldTask.Description && s.sideEffects.LogActivity != nil {
		go s.sideEffects.LogActivity(id, userID, "updated", "Updated task description")
	}
	go s.updateSearchEmbedding(id, newTask.Title+" "+newTask.Description)
	if s.sideEffects.Broadcast != nil {
		go s.sideEffects.Broadcast("UPDATE")
	}
	if s.sideEffects.BoardStats != nil {
		go s.sideEffects.BoardStats.RecomputeBoard(context.Background(), oldTask.BoardID)
		if newTask.BoardID != oldTask.BoardID {
			go s.sideEffects.BoardStats.RecomputeBoard(context.Background(), newTask.BoardID)
		}
	}
	freshTask, err := s.repo.GetByID(ctx, id)
	if err != nil {
		newTask.ID = id
		return newTask, nil
	}
	freshTask.UpdatedBy = newTask.UpdatedBy
	return freshTask, nil
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
