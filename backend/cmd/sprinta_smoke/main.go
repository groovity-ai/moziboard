package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	agentinframodule "moziboard-backend/internal/modules/agentinfra"
	agentsmodule "moziboard-backend/internal/modules/agents"
	runtimemodule "moziboard-backend/internal/modules/runtime"
	tasksmodule "moziboard-backend/internal/modules/tasks"
)

const (
	boardID = "248bfb18-9e0b-404f-834d-dc27ac9c94df"
	agentID = "runtime-happy-agent"
)

func main() {
	ctx := context.Background()
	dsn := envOr("SMOKE_DATABASE_URL", "postgres://moziboard:moziboard_secret@moziboard-db:5432/moziboard?sslmode=disable")
	db, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	logActivity := func(taskID int, userID, action, details string) {
		_, _ = db.Exec(context.Background(), "INSERT INTO activities (task_id, user_id, action, details) VALUES ($1,$2,$3,$4)", taskID, userID, action, details)
	}
	auditLog := func(actorType, actorID, entityType, entityID, action string, oldValue, newValue, metadata interface{}) {
		_, _ = db.Exec(context.Background(), "INSERT INTO audit_logs (actor_type, actor_id, entity_type, entity_id, action, old_value_json, new_value_json, metadata_json) VALUES ($1,$2,$3,$4,$5,COALESCE($6::jsonb,'{}'::jsonb),COALESCE($7::jsonb,'{}'::jsonb),COALESCE($8::jsonb,'{}'::jsonb))", actorType, actorID, entityType, entityID, action, toJSON(oldValue), toJSON(newValue), toJSON(metadata))
	}

	agentInfra := agentinframodule.NewService(db, nil)
	tasksRepo := tasksmodule.NewRepository(db)
	tasksSvc := tasksmodule.NewService(tasksRepo, tasksmodule.SideEffects{
		AgentInfra:  agentInfra,
		LogActivity: logActivity,
		AuditLog:    auditLog,
		AgentStateUpdater: tasksmodule.AgentStateUpdaterFuncs{
			MarkQueuedFunc: func(taskID int, runID int, agentID string) {
				_, _ = db.Exec(context.Background(), "UPDATE agents SET current_task_id=$1, current_run_id=$2, status='busy', current_activity='Queued on task' WHERE id=$3", taskID, runID, agentID)
			},
			MarkBlockedFunc: func(blockedReason string, agentID string) {
				_, _ = db.Exec(context.Background(), "UPDATE agents SET status='blocked', current_activity='Blocked on task', health_note=$1 WHERE id=$2", blockedReason, agentID)
			},
			MarkOnlineFunc: func(agentID string) {
				_, _ = db.Exec(context.Background(), "UPDATE agents SET status='online', current_task_id=NULL, current_run_id=NULL, current_activity='Idle', health_note='' WHERE id=$1", agentID)
			},
			MarkInProgressFunc: func(taskID int, agentID string) {
				_, _ = db.Exec(context.Background(), "UPDATE agents SET status='busy', current_task_id=$1, current_activity='Working on task' WHERE id=$2", taskID, agentID)
			},
		},
	})
	runtimeRepo := runtimemodule.NewRepository(db)
	runtimeSvc := runtimemodule.NewService(runtimeRepo, nil, nil, agentInfra, logActivity, auditLog, nil, nil)

	task := tasksmodule.Task{
		BoardID:     boardID,
		Title:       fmt.Sprintf("SprintA Smoke %d", time.Now().Unix()),
		Description: "smoke lifecycle",
		AssigneeID:  strPtr(agentID),
	}
	if err := tasksSvc.Create(ctx, &task); err != nil {
		log.Fatalf("create task failed: %v", err)
	}
	mustTaskState(ctx, db, task.ID, "assigned")

	conn := &agentsmodule.AgentConnector{AgentID: agentID}
	runID, err := runtimeSvc.RuntimeTaskAck(ctx, conn, runtimemodule.TaskAckReq{TaskID: task.ID, Message: "starting smoke"})
	if err != nil {
		log.Fatalf("runtime ack failed: %v", err)
	}
	_ = runID
	mustTaskState(ctx, db, task.ID, "in_progress")
	mustRunState(ctx, db, task.ID, agentID, "running")

	_, err = runtimeSvc.RuntimeTaskReviewRequest(ctx, conn, runtimemodule.TaskReviewReq{TaskID: task.ID, Summary: "ready for review"})
	if err != nil {
		log.Fatalf("review request failed: %v", err)
	}
	mustTaskState(ctx, db, task.ID, "review")
	mustRunState(ctx, db, task.ID, agentID, "review")

	_, err = tasksSvc.ApplyReviewAction(ctx, task.ID, tasksmodule.ReviewActionReq{Action: "approve", Note: "looks good", UpdatedBy: "smoke-test"})
	if err != nil {
		log.Fatalf("review approve failed: %v", err)
	}
	mustTaskState(ctx, db, task.ID, "done")
	mustRunDone(ctx, db, task.ID, agentID)
	mustAgentOnline(ctx, db, agentID)

	if err := expectInvalidTransition(ctx, tasksSvc, task.ID); err != nil {
		log.Fatalf("invalid transition guard failed: %v", err)
	}

	fmt.Printf("SMOKE_OK task_id=%d\n", task.ID)
}

func expectInvalidTransition(ctx context.Context, svc *tasksmodule.Service, taskID int) error {
	_, err := svc.Update(ctx, taskID, &tasksmodule.Task{Status: "review", UpdatedBy: "smoke-test"})
	if err == nil {
		return fmt.Errorf("expected invalid transition from done to review")
	}
	return nil
}

func mustTaskState(ctx context.Context, db *pgxpool.Pool, taskID int, want string) {
	var got string
	if err := db.QueryRow(ctx, "SELECT status FROM tasks WHERE id=$1", taskID).Scan(&got); err != nil {
		log.Fatalf("query task status failed: %v", err)
	}
	if got != want {
		log.Fatalf("task %d status = %s, want %s", taskID, got, want)
	}
}

func mustRunState(ctx context.Context, db *pgxpool.Pool, taskID int, agentID, want string) {
	var got string
	if err := db.QueryRow(ctx, "SELECT status FROM agent_runs WHERE task_id=$1 AND agent_id=$2 ORDER BY id DESC LIMIT 1", taskID, agentID).Scan(&got); err != nil {
		log.Fatalf("query run status failed: %v", err)
	}
	if got != want {
		log.Fatalf("run status = %s, want %s", got, want)
	}
}

func mustRunDone(ctx context.Context, db *pgxpool.Pool, taskID int, agentID string) {
	var got string
	var endedAt *time.Time
	if err := db.QueryRow(ctx, "SELECT status, ended_at FROM agent_runs WHERE task_id=$1 AND agent_id=$2 ORDER BY id DESC LIMIT 1", taskID, agentID).Scan(&got, &endedAt); err != nil {
		log.Fatalf("query final run status failed: %v", err)
	}
	if got != "review" && got != "running" && got != "done" {
		log.Fatalf("unexpected final run status %s", got)
	}
}

func mustAgentOnline(ctx context.Context, db *pgxpool.Pool, agentID string) {
	var status string
	if err := db.QueryRow(ctx, "SELECT status FROM agents WHERE id=$1", agentID).Scan(&status); err != nil {
		log.Fatalf("query agent status failed: %v", err)
	}
	if status != "online" {
		log.Fatalf("agent status = %s, want online", status)
	}
}

func strPtr(v string) *string { return &v }

func toJSON(v interface{}) *string {
	if v == nil {
		return nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	s := string(b)
	return &s
}

func envOr(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}
