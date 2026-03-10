package authproxy

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
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
	var data interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to parse AiAgenz response"})
	}
	return c.Status(resp.StatusCode).JSON(data)
}

func (h *Handler) ProxyAuthMe(c *fiber.Ctx) error {
	token := extractToken(c)
	if token == "" {
		return c.Status(401).JSON(fiber.Map{"error": "Unauthorized"})
	}
	claims, err := decodeJWTClaims(token)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{"error": "Invalid token"})
	}
	userID := firstNonEmpty(interfaceToString(claims["sub"]), interfaceToString(claims["id"]), interfaceToString(claims["user_id"]), interfaceToString(claims["userId"]))
	if userID == "" {
		return c.Status(401).JSON(fiber.Map{"error": "Invalid token"})
	}
	return c.JSON(fiber.Map{
		"id":    userID,
		"email": interfaceToString(claims["email"]),
		"role":  interfaceToString(claims["role"]),
	})
}

func extractToken(c *fiber.Ctx) string {
	authHeader := strings.TrimSpace(c.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		return strings.TrimSpace(authHeader[7:])
	}
	cookieToken := strings.TrimSpace(c.Cookies("session_id"))
	if cookieToken != "" {
		return cookieToken
	}
	return ""
}

func decodeJWTClaims(token string) (map[string]interface{}, error) {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return nil, fiber.ErrUnauthorized
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, err
	}
	var claims map[string]interface{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, err
	}
	return claims, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func interfaceToString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case json.Number:
		return strings.TrimSpace(t.String())
	case float64:
		return strings.TrimSpace(fmt.Sprintf("%.0f", t))
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", v))
	}
}
