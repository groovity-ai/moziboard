package clawn

import (
	"strings"

	"github.com/gofiber/fiber/v2"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) RegisterRoutes(app fiber.Router) {
	app.Get("/api/agents/sync/aiagenz", h.GetAiAgenzProjects)
	app.Get("/api/integrations/clawn/projects", h.ListProjectsFacade)
	app.Post("/api/integrations/clawn/connect", h.ConnectProjectFacade)
}

func (h *Handler) GetAiAgenzProjects(c *fiber.Ctx) error {
	items, statusCode, err := h.svc.FetchProjects(c)
	if err != nil {
		return c.Status(statusCode).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"data": items})
}

func (h *Handler) ListProjectsFacade(c *fiber.Ctx) error {
	items, statusCode, err := h.svc.ListProjectSources(c, strings.TrimSpace(c.Query("board_id")))
	if err != nil {
		return c.Status(statusCode).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(items)
}

func (h *Handler) ConnectProjectFacade(c *fiber.Ctx) error {
	var req ConnectReq
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	res, statusCode, err := h.svc.ConnectProject(c, req)
	if err != nil {
		msg := err.Error()
		if strings.HasPrefix(msg, "clawn_project_not_connectable:") {
			reason := strings.TrimPrefix(msg, "clawn_project_not_connectable:")
			return c.Status(409).JSON(fiber.Map{"error": "clawn_project_not_connectable", "reason": reason})
		}
		return c.Status(statusCode).JSON(fiber.Map{"error": msg})
	}
	return c.Status(statusCode).JSON(res)
}
