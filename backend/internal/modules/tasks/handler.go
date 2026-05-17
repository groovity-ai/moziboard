package tasks

import (
	"strconv"
	"strings"

	docsmodule "moziboard-backend/internal/modules/docs"

	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	svc                   *Service
	docsSvc               *docsmodule.Service
	requireBoardAuth      fiber.Handler
	requireBoardFromBody  fiber.Handler
	requireBoardFromQuery fiber.Handler
	requireTaskAuth       fiber.Handler
}

func NewHandler(svc *Service, docsSvc *docsmodule.Service, requireBoardAuth fiber.Handler, requireBoardFromBody fiber.Handler, requireBoardFromQuery fiber.Handler, requireTaskAuth fiber.Handler) *Handler {
	return &Handler{svc: svc, docsSvc: docsSvc, requireBoardAuth: requireBoardAuth, requireBoardFromBody: requireBoardFromBody, requireBoardFromQuery: requireBoardFromQuery, requireTaskAuth: requireTaskAuth}
}

func (h *Handler) RegisterRoutes(app fiber.Router) {
	app.Get("/api/boards/:id/tasks", h.requireBoardAuth, h.GetBoardTasks)
	app.Post("/api/tasks", h.requireBoardFromBody, h.CreateTask)
	app.Put("/api/tasks/:id", h.requireTaskAuth, h.UpdateTask)
	app.Post("/api/tasks/:id/review-action", h.requireTaskAuth, h.ApplyReviewAction)
	app.Get("/api/tasks/:id/activities", h.requireTaskAuth, h.GetTaskActivities)
	app.Get("/api/search", h.requireBoardFromQuery, h.SearchTasks)
}

func (h *Handler) GetBoardTasks(c *fiber.Ctx) error {
	items, err := h.svc.ListByBoard(c.Context(), c.Params("id"))
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	return c.JSON(items)
}

func (h *Handler) CreateTask(c *fiber.Ctx) error {
	var t Task
	if err := c.BodyParser(&t); err != nil {
		return c.Status(400).SendString(err.Error())
	}
	if err := h.svc.Create(c.Context(), &t); err != nil {
		if isBadRequestError(err) {
			return c.Status(400).SendString(err.Error())
		}
		return c.Status(500).SendString(err.Error())
	}
	return c.JSON(t)
}

func (h *Handler) UpdateTask(c *fiber.Ctx) error {
	id, _ := strconv.Atoi(c.Params("id"))
	var patch Task
	if err := c.BodyParser(&patch); err != nil {
		return c.Status(400).SendString(err.Error())
	}
	updated, err := h.svc.Update(c.Context(), id, &patch)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return c.Status(404).SendString("Task not found")
		}
		if isBadRequestError(err) {
			return c.Status(400).SendString(err.Error())
		}
		return c.Status(500).SendString(err.Error())
	}
	return c.JSON(updated)
}

func (h *Handler) ApplyReviewAction(c *fiber.Ctx) error {
	id, _ := strconv.Atoi(c.Params("id"))
	var req ReviewActionReq
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).SendString(err.Error())
	}
	updated, err := h.svc.ApplyReviewAction(c.Context(), id, req)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return c.Status(404).SendString("Task not found")
		}
		if isBadRequestError(err) {
			return c.Status(400).SendString(err.Error())
		}
		return c.Status(500).SendString(err.Error())
	}
	return c.JSON(updated)
}

func (h *Handler) GetTaskActivities(c *fiber.Ctx) error {
	taskID, _ := strconv.Atoi(c.Params("id"))
	items, err := h.svc.ListActivities(c.Context(), taskID)
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	return c.JSON(items)
}

func (h *Handler) SearchTasks(c *fiber.Ctx) error {
	boardID := c.Query("board_id")
	query := c.Query("q")

	tasks, err := h.svc.Search(c.Context(), boardID, query)
	if err != nil {
		if err.Error() == "query required" || err.Error() == "board_id required" {
			return c.Status(400).SendString(err.Error())
		}
		return c.Status(500).SendString(err.Error())
	}

	type searchResult struct {
		ID       string      `json:"id"`
		Type     string      `json:"type"`
		Title    string      `json:"title"`
		Subtitle string      `json:"subtitle,omitempty"`
		Preview  string      `json:"preview,omitempty"`
		Data     interface{} `json:"data"`
	}

	results := make([]searchResult, 0, len(tasks)+5)
	for _, task := range tasks {
		preview := strings.TrimSpace(task.Description)
		if len(preview) > 160 {
			preview = preview[:160] + "…"
		}
		results = append(results, searchResult{
			ID:       strconv.Itoa(task.ID),
			Type:     "task",
			Title:    task.Title,
			Subtitle: task.Status,
			Preview:  preview,
			Data:     task,
		})
	}

	if h.docsSvc != nil {
		docs, err := h.docsSvc.Search(c.Context(), boardID, query)
		if err != nil && err.Error() != "query required" {
			return c.Status(500).SendString(err.Error())
		}
		for _, doc := range docs {
			preview := strings.TrimSpace(doc.Content)
			if len(preview) > 160 {
				preview = preview[:160] + "…"
			}
			results = append(results, searchResult{
				ID:       strconv.Itoa(doc.ID),
				Type:     "doc",
				Title:    doc.Title,
				Subtitle: "document",
				Preview:  preview,
				Data:     doc,
			})
		}
	}

	return c.JSON(results)
}

func isBadRequestError(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	switch s {
	case "title is required", "no board found", "assignee is not a board member", "review action is required", "invalid review action", "initial task state is not allowed":
		return true
	}
	return strings.Contains(s, "invalid task transition") || strings.Contains(s, "required")
}
