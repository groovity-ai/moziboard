package authsession

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

type Session struct {
	UserID string                 `json:"user_id"`
	Raw    map[string]interface{} `json:"raw"`
}

type Middleware struct {
	upstreamURL string
	httpClient  *http.Client
}

func New(upstreamURL string) *Middleware {
	return &Middleware{
		upstreamURL: strings.TrimRight(upstreamURL, "/"),
		httpClient:  &http.Client{Timeout: 8 * time.Second},
	}
}

func (m *Middleware) RequireAuth(c *fiber.Ctx) error {
	path := c.Path()
	if path == "/api/health" || path == "/api/auth/login" || path == "/api/auth/me" || strings.HasPrefix(path, "/api/runtime/") {
		return c.Next()
	}
	if !strings.HasPrefix(path, "/api/") {
		return c.Next()
	}

	sess, statusCode, err := m.ResolveSession(c)
	if err != nil {
		if statusCode == 0 {
			statusCode = 401
		}
		return c.Status(statusCode).JSON(fiber.Map{"error": err.Error()})
	}
	c.Locals("auth.session", sess)
	c.Locals("auth.user_id", sess.UserID)
	return c.Next()
}

func (m *Middleware) ResolveSession(c *fiber.Ctx) (*Session, int, error) {
	token := extractToken(c)
	if token == "" {
		return nil, 401, fmt.Errorf("unauthorized")
	}

	req, err := http.NewRequest("GET", m.upstreamURL+"/api/projects", nil)
	if err != nil {
		return nil, 500, fmt.Errorf("failed to create auth request")
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, 502, fmt.Errorf("failed to validate session")
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
	if resp.StatusCode != 200 {
		if resp.StatusCode == 401 {
			return nil, 401, fmt.Errorf("unauthorized")
		}
		return nil, resp.StatusCode, fmt.Errorf("session validation failed")
	}

	claims, err := decodeJWTClaims(token)
	if err != nil {
		return nil, 401, fmt.Errorf("invalid token payload")
	}
	userID := firstNonEmpty(
		interfaceToString(claims["sub"]),
		interfaceToString(claims["user_id"]),
		interfaceToString(claims["userId"]),
		interfaceToString(claims["id"]),
	)
	if strings.TrimSpace(userID) == "" {
		return nil, 403, fmt.Errorf("authenticated session missing user id")
	}

	var raw map[string]interface{}
	_ = json.Unmarshal(body, &raw)
	if raw == nil {
		raw = map[string]interface{}{}
	}
	raw["token_claims"] = claims
	return &Session{UserID: userID, Raw: raw}, 200, nil
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
	cookieHeader := strings.TrimSpace(c.Get("Cookie"))
	if cookieHeader == "" {
		return ""
	}
	for _, part := range strings.Split(cookieHeader, ";") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "session_id=") {
			return strings.TrimSpace(strings.TrimPrefix(part, "session_id="))
		}
	}
	return ""
}

func decodeJWTClaims(token string) (map[string]interface{}, error) {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid jwt")
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
