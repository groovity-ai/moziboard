package comments

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	svc             *Service
	requireTaskAuth fiber.Handler
}

func NewHandler(svc *Service, requireTaskAuth fiber.Handler) *Handler {
	return &Handler{svc: svc, requireTaskAuth: requireTaskAuth}
}

func (h *Handler) RegisterRoutes(app fiber.Router) {
	app.Get("/api/tasks/:id/comments", h.requireTaskAuth, h.GetTaskComments)
	app.Post("/api/tasks/:id/comments", h.requireTaskAuth, h.CreateComment)
}

func (h *Handler) GetTaskComments(c *fiber.Ctx) error {
	taskID, _ := strconv.Atoi(c.Params("id"))
	comments, err := h.svc.ListByTask(c.Context(), taskID)
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	return c.JSON(comments)
}

func (h *Handler) CreateComment(c *fiber.Ctx) error {
	taskID, _ := strconv.Atoi(c.Params("id"))
	var cm Comment
	if err := c.BodyParser(&cm); err != nil {
		return c.Status(400).SendString(err.Error())
	}
	if err := h.svc.Create(c.Context(), taskID, &cm); err != nil {
		return c.Status(400).SendString(err.Error())
	}
	return c.JSON(cm)
}
