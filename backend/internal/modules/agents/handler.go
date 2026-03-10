package agents

import "github.com/gofiber/fiber/v2"

type Handler struct {
	svc              *Service
	requireBoardAuth fiber.Handler
}

func NewHandler(svc *Service, requireBoardAuth fiber.Handler) *Handler { return &Handler{svc: svc, requireBoardAuth: requireBoardAuth} }

func (h *Handler) RegisterRoutes(app fiber.Router) {
	app.Get("/api/agents", h.GetAgents)
	app.Get("/api/agents/:id", h.GetAgentProfile)
	app.Post("/api/agents/register", h.RegisterAgent)
	app.Post("/api/agents/:id/connect-board", h.ConnectAgentToBoard)
	app.Get("/api/boards/:id/agents", h.requireBoardAuth, h.GetBoardAgents)
	app.Get("/api/agents/:id/runs", h.GetAgentRuns)
	app.Get("/api/agents/:id/notifications", h.GetAgentNotifications)
	app.Post("/api/agents/:id/heartbeat", h.HeartbeatAgent)
	app.Put("/api/agents/:id/status", h.UpdateAgentStatus)
}

func (h *Handler) GetAgents(c *fiber.Ctx) error {
	items, err := h.svc.ListAgents(c.Context())
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	return c.JSON(items)
}
func (h *Handler) GetAgentProfile(c *fiber.Ctx) error {
	item, err := h.svc.GetAgent(c.Context(), c.Params("id"))
	if err != nil {
		return c.Status(404).SendString("Agent not found")
	}
	return c.JSON(item)
}
func (h *Handler) RegisterAgent(c *fiber.Ctx) error {
	var req AgentRegisterReq
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).SendString(err.Error())
	}
	res, err := h.svc.Register(c.Context(), req)
	if err != nil {
		if err.Error() == "id and display_name are required" {
			return c.Status(400).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(res)
}
func (h *Handler) ConnectAgentToBoard(c *fiber.Ctx) error {
	var req AgentBoardConnectReq
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).SendString(err.Error())
	}
	id, err := h.svc.ConnectToBoard(c.Context(), c.Params("id"), req)
	if err != nil {
		if err.Error() == "board_id is required" {
			return c.Status(400).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true, "board_agent_id": id})
}
func (h *Handler) GetBoardAgents(c *fiber.Ctx) error {
	items, err := h.svc.ListBoardAgents(c.Context(), c.Params("id"))
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	return c.JSON(items)
}
func (h *Handler) GetAgentRuns(c *fiber.Ctx) error {
	items, err := h.svc.ListRuns(c.Context(), c.Params("id"))
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	return c.JSON(items)
}
func (h *Handler) GetAgentNotifications(c *fiber.Ctx) error {
	items, err := h.svc.ListNotifications(c.Context(), c.Params("id"))
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	return c.JSON(items)
}
func (h *Handler) HeartbeatAgent(c *fiber.Ctx) error {
	if err := h.svc.Heartbeat(c.Context(), c.Params("id")); err != nil {
		return c.Status(500).SendString(err.Error())
	}
	return c.JSON(fiber.Map{"ok": true})
}
func (h *Handler) UpdateAgentStatus(c *fiber.Ctx) error {
	var req AgentStatusUpdateReq
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).SendString(err.Error())
	}
	if err := h.svc.UpdateStatus(c.Context(), c.Params("id"), req); err != nil {
		return c.Status(500).SendString(err.Error())
	}
	return c.JSON(fiber.Map{"ok": true})
}
