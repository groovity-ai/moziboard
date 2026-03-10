package authproxy

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v2"
)

type Handler struct{}

func NewHandler() *Handler { return &Handler{} }

func (h *Handler) RegisterRoutes(app fiber.Router) {
	app.Post("/api/auth/login", h.ProxyAuthLogin)
	app.Get("/api/auth/me", h.ProxyAuthMe)
}

func (h *Handler) ProxyAuthLogin(c *fiber.Ctx) error {
	proxyURL := "http://172.17.0.1:4001/api/auth/login"
	req, err := http.NewRequest("POST", proxyURL, bytes.NewReader(c.Body()))
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create proxy request"})
	}
	req.Header.Set("Content-Type", c.Get("Content-Type"))
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return c.Status(502).JSON(fiber.Map{"error": "Failed to connect to AiAgenz backend: " + err.Error()})
	}
	defer resp.Body.Close()
	for _, cookie := range resp.Header["Set-Cookie"] {
		c.Append("Set-Cookie", cookie)
	}
	var data interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to parse AiAgenz response"})
	}
	return c.Status(resp.StatusCode).JSON(data)
}

func (h *Handler) ProxyAuthMe(c *fiber.Ctx) error {
	authCookie := c.Get("Cookie")
	if authCookie == "" {
		return c.Status(401).JSON(fiber.Map{"error": "Unauthorized: Missing authentication cookie"})
	}
	proxyURL := "http://172.17.0.1:4001/api/auth/me"
	req, err := http.NewRequest("GET", proxyURL, nil)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create proxy request"})
	}
	req.Header.Set("Cookie", authCookie)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return c.Status(502).JSON(fiber.Map{"error": "Failed to connect to AiAgenz backend: " + err.Error()})
	}
	defer resp.Body.Close()
	var data interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to parse AiAgenz response"})
	}
	return c.Status(resp.StatusCode).JSON(data)
}
