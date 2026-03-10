package deliverables

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) RegisterRoutes(app fiber.Router) {
	app.Get("/api/tasks/:id/deliverables", h.GetTaskDeliverables)
	app.Post("/api/tasks/:id/deliverables", h.CreateDeliverable)
}

func (h *Handler) GetTaskDeliverables(c *fiber.Ctx) error {
	taskID, _ := strconv.Atoi(c.Params("id"))
	deliverables, err := h.svc.ListByTask(c.Context(), taskID)
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	return c.JSON(deliverables)
}

func (h *Handler) CreateDeliverable(c *fiber.Ctx) error {
	taskID, _ := strconv.Atoi(c.Params("id"))
	var d Deliverable
	if err := c.BodyParser(&d); err != nil {
		return c.Status(400).SendString(err.Error())
	}
	if err := h.svc.Create(c.Context(), taskID, &d); err != nil {
		return c.Status(400).SendString(err.Error())
	}
	return c.JSON(d)
}
