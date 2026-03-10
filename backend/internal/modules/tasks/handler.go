package tasks

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) RegisterRoutes(app fiber.Router) {
	app.Get("/api/boards/:id/tasks", h.GetBoardTasks)
	app.Post("/api/tasks", h.CreateTask)
	app.Put("/api/tasks/:id", h.UpdateTask)
	app.Get("/api/tasks/:id/activities", h.GetTaskActivities)
	app.Get("/api/search", h.SearchTasks)
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
		if err.Error() == "title is required" || err.Error() == "no board found" {
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
	items, err := h.svc.Search(c.Context(), c.Query("q"))
	if err != nil {
		if err.Error() == "query required" {
			return c.Status(400).SendString(err.Error())
		}
		return c.Status(500).SendString(err.Error())
	}
	return c.JSON(items)
}
