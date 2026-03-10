package runtime

import (
	"github.com/gofiber/fiber/v2"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) RegisterRoutes(app fiber.Router) {
	app.Post("/api/runtime/heartbeat", h.RuntimeHeartbeat)
	app.Post("/api/runtime/task-ack", h.RuntimeTaskAck)
	app.Post("/api/runtime/task-update", h.RuntimeTaskUpdate)
	app.Post("/api/runtime/task-comment", h.RuntimeTaskComment)
	app.Post("/api/runtime/task-deliverable", h.RuntimeTaskDeliverable)
	app.Post("/api/runtime/task-review-request", h.RuntimeTaskReviewRequest)
}

func (h *Handler) RuntimeHeartbeat(c *fiber.Ctx) error {
	conn, err := h.svc.ResolveRuntimeAgent(c.Context(), c)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{"error": err.Error()})
	}
	var req HeartbeatReq
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).SendString(err.Error())
	}
	if err := h.svc.RuntimeHeartbeat(c.Context(), conn, req); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true, "agent_id": conn.AgentID})
}

func (h *Handler) RuntimeTaskAck(c *fiber.Ctx) error {
	conn, err := h.svc.ResolveRuntimeAgent(c.Context(), c)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{"error": err.Error()})
	}
	var req TaskAckReq
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).SendString(err.Error())
	}
	runID, err := h.svc.RuntimeTaskAck(c.Context(), conn, req)
	if err != nil {
		code := 500
		if err.Error() == "agent not connected to task board" {
			code = 403
		}
		return c.Status(code).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true, "run_id": runID})
}

func (h *Handler) RuntimeTaskUpdate(c *fiber.Ctx) error {
	conn, err := h.svc.ResolveRuntimeAgent(c.Context(), c)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{"error": err.Error()})
	}
	var req TaskUpdateReq
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).SendString(err.Error())
	}
	runID, err := h.svc.RuntimeTaskUpdate(c.Context(), conn, req)
	if err != nil {
		code := 500
		if err.Error() == "agent not connected to task board" {
			code = 403
		}
		return c.Status(code).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true, "run_id": runID})
}

func (h *Handler) RuntimeTaskComment(c *fiber.Ctx) error {
	conn, err := h.svc.ResolveRuntimeAgent(c.Context(), c)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{"error": err.Error()})
	}
	var req TaskCommentReq
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).SendString(err.Error())
	}
	if err := h.svc.RuntimeTaskComment(c.Context(), conn, req); err != nil {
		code := 500
		if err.Error() == "agent not connected to task board" {
			code = 403
		}
		return c.Status(code).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *Handler) RuntimeTaskDeliverable(c *fiber.Ctx) error {
	conn, err := h.svc.ResolveRuntimeAgent(c.Context(), c)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{"error": err.Error()})
	}
	var req TaskDeliverableReq
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).SendString(err.Error())
	}
	runID, err := h.svc.RuntimeTaskDeliverable(c.Context(), conn, req)
	if err != nil {
		code := 500
		if err.Error() == "agent not connected to task board" {
			code = 403
		}
		return c.Status(code).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true, "run_id": runID})
}

func (h *Handler) RuntimeTaskReviewRequest(c *fiber.Ctx) error {
	conn, err := h.svc.ResolveRuntimeAgent(c.Context(), c)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{"error": err.Error()})
	}
	var req TaskReviewReq
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).SendString(err.Error())
	}
	runID, err := h.svc.RuntimeTaskReviewRequest(c.Context(), conn, req)
	if err != nil {
		code := 500
		if err.Error() == "agent not connected to task board" {
			code = 403
		}
		return c.Status(code).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true, "run_id": runID})
}
