package taskruns

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	agentsmodule "moziboard-backend/internal/modules/agents"
)

type Handler struct {
	db              *pgxpool.Pool
	requireTaskAuth fiber.Handler
}

func NewHandler(db *pgxpool.Pool, requireTaskAuth fiber.Handler) *Handler {
	return &Handler{db: db, requireTaskAuth: requireTaskAuth}
}

func (h *Handler) RegisterRoutes(app fiber.Router) {
	app.Get("/api/tasks/:id/runs", h.requireTaskAuth, h.GetTaskRuns)
}

type agentRun = agentsmodule.AgentRun

var _ time.Time

func (h *Handler) GetTaskRuns(c *fiber.Ctx) error {
	taskID := c.Params("id")
	rows, err := h.db.Query(context.Background(), "SELECT id, task_id, agent_id, session_key, provider_type, status, started_at, ended_at, current_activity, error_summary, result_summary FROM agent_runs WHERE task_id=$1 ORDER BY started_at DESC", taskID)
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	defer rows.Close()
	var runs []agentRun
	for rows.Next() {
		var r agentRun
		if err := rows.Scan(&r.ID, &r.TaskID, &r.AgentID, &r.SessionKey, &r.ProviderType, &r.Status, &r.StartedAt, &r.EndedAt, &r.CurrentActivity, &r.ErrorSummary, &r.ResultSummary); err != nil {
			return c.Status(500).SendString(err.Error())
		}
		runs = append(runs, r)
	}
	if runs == nil {
		runs = []agentRun{}
	}
	return c.JSON(runs)
}
