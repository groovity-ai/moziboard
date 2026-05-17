package ops

import "github.com/gofiber/fiber/v2"

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) RegisterRoutes(app fiber.Router) {
	app.Get("/api/ops/summary", h.GetSummary)
	app.Get("/api/ops/agent-events", h.GetAgentEvents)
	app.Post("/api/ops/agent-events/:id/retry", h.RetryAgentEvent)
	app.Post("/api/ops/agent-events/:id/requeue", h.RequeueAgentEvent)
	app.Get("/api/ops/connectors", h.GetConnectors)
	app.Post("/api/ops/connectors/:id/enable", h.EnableConnector)
	app.Post("/api/ops/connectors/:id/disable", h.DisableConnector)
	app.Get("/api/ops/audit-logs", h.GetAuditLogs)
	app.Post("/api/ops/maintenance/run", h.RunMaintenanceSweep)
}

func (h *Handler) GetSummary(c *fiber.Ctx) error {
	summary, err := h.svc.GetSummary(c.Context(), c.Query("board_id"))
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	return c.JSON(summary)
}

func (h *Handler) GetAgentEvents(c *fiber.Ctx) error {
	items, err := h.svc.ListAgentEvents(c.Context(), c.Query("status"), c.Query("agent_id"), c.Query("event_type"), c.Query("board_id"))
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	return c.JSON(items)
}

func (h *Handler) GetConnectors(c *fiber.Ctx) error {
	items, err := h.svc.ListConnectors(c.Context(), c.Query("status"), c.Query("agent_id"), c.Query("connector_type"), c.Query("transport_mode"))
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	return c.JSON(items)
}

func (h *Handler) GetAuditLogs(c *fiber.Ctx) error {
	items, err := h.svc.ListAuditLogs(c.Context(), c.Query("entity_type"), c.Query("entity_id"), c.Query("action"), c.Query("board_id"))
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	return c.JSON(items)
}

func (h *Handler) RetryAgentEvent(c *fiber.Ctx) error {
	err := h.svc.RetryEvent(c.Context(), c.Params("id"))
	if err != nil {
		if err.Error() == "event_not_retryable" {
			return c.Status(404).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(500).SendString(err.Error())
	}
	return c.JSON(fiber.Map{"ok": true, "action": "retry", "event_id": c.Params("id")})
}

func (h *Handler) RequeueAgentEvent(c *fiber.Ctx) error {
	err := h.svc.RequeueEvent(c.Context(), c.Params("id"))
	if err != nil {
		if err.Error() == "event_not_requeueable" {
			return c.Status(404).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(500).SendString(err.Error())
	}
	return c.JSON(fiber.Map{"ok": true, "action": "requeue", "event_id": c.Params("id")})
}

func (h *Handler) EnableConnector(c *fiber.Ctx) error {
	err := h.svc.EnableConnector(c.Context(), c.Params("id"))
	if err != nil {
		if err.Error() == "connector_not_found" {
			return c.Status(404).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(500).SendString(err.Error())
	}
	return c.JSON(fiber.Map{"ok": true, "action": "enable", "connector_id": c.Params("id")})
}

func (h *Handler) DisableConnector(c *fiber.Ctx) error {
	err := h.svc.DisableConnector(c.Context(), c.Params("id"))
	if err != nil {
		if err.Error() == "connector_not_found" {
			return c.Status(404).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(500).SendString(err.Error())
	}
	return c.JSON(fiber.Map{"ok": true, "action": "disable", "connector_id": c.Params("id")})
}

func (h *Handler) RunMaintenanceSweep(c *fiber.Ctx) error {
	report, err := h.svc.RunMaintenanceSweep(c.Context())
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	return c.JSON(fiber.Map{"ok": true, "report": report})
}
