package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/sashabaranov/go-openai"
	activityfeedmodule "moziboard-backend/internal/modules/activityfeed"
	agentinframodule "moziboard-backend/internal/modules/agentinfra"
	agentsmodule "moziboard-backend/internal/modules/agents"
	attentionmodule "moziboard-backend/internal/modules/attention"
	authproxymodule "moziboard-backend/internal/modules/authproxy"
	boardhealthmodule "moziboard-backend/internal/modules/boardhealth"
	boardmodule "moziboard-backend/internal/modules/boards"
	boardstatsmodule "moziboard-backend/internal/modules/boardstats"
	clawnmodule "moziboard-backend/internal/modules/clawn"
	commentsmodule "moziboard-backend/internal/modules/comments"
	deliverablesmodule "moziboard-backend/internal/modules/deliverables"
	deliverymodule "moziboard-backend/internal/modules/delivery"
	dispatchmodule "moziboard-backend/internal/modules/dispatch"
	docsmodule "moziboard-backend/internal/modules/docs"
	homemodule "moziboard-backend/internal/modules/home"
	opsmodule "moziboard-backend/internal/modules/ops"
	runtimemodule "moziboard-backend/internal/modules/runtime"
	taskrunsmodule "moziboard-backend/internal/modules/taskruns"
	tasksmodule "moziboard-backend/internal/modules/tasks"
	platformai "moziboard-backend/internal/platform/ai"
	"moziboard-backend/internal/platform/authsession"
	"moziboard-backend/internal/platform/authz"
	"moziboard-backend/internal/platform/cache"
	"moziboard-backend/internal/platform/config"
	platformdb "moziboard-backend/internal/platform/db"
	"moziboard-backend/internal/platform/gatewayutil"
	"moziboard-backend/internal/platform/httpapp"
	"moziboard-backend/internal/realtime"
)

var (
	db                *pgxpool.Pool
	rdb               *redis.Client
	openaiClient      *openai.Client
	clients           = make(map[*websocket.Conn]bool)
	clientsMu         sync.Mutex
	wsHub             *realtime.Hub
	deliverySemaphore = make(chan struct{}, 4)
	latencyStats      = struct {
		sync.Mutex
		ByPath map[string]fiber.Map
	}{ByPath: map[string]fiber.Map{}}
)

type Board struct {
	ID          string  `json:"id"`
	UserID      *string `json:"user_id"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
}

type Task = tasksmodule.Task

type Agent = agentsmodule.Agent

type AgentConnector = agentsmodule.AgentConnector

type BoardAgent = agentsmodule.BoardAgent

type AgentRun = agentsmodule.AgentRun

type Notification = agentsmodule.Notification

type AgentEvent = opsmodule.AgentEvent

type Deliverable struct {
	ID           int       `json:"id"`
	TaskID       int       `json:"task_id"`
	Title        string    `json:"title"`
	Description  string    `json:"description"`
	ArtifactType string    `json:"artifact_type"`
	Content      string    `json:"content"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Member struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Role   string `json:"role"`
	Avatar string `json:"avatar"`
}

type Activity = tasksmodule.Activity

type BoardMemberReq struct {
	MemberID string `json:"member_id"`
	Role     string `json:"role"`
}

type AgentRegisterReq = agentsmodule.AgentRegisterReq

type AgentConnectorInput = agentsmodule.AgentConnectorInput

type AgentBoardConnectReq = agentsmodule.AgentBoardConnectReq

type Document struct {
	ID        int       `json:"id"`
	BoardID   string    `json:"board_id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Comment struct {
	ID        int       `json:"id"`
	TaskID    int       `json:"task_id"`
	UserID    string    `json:"user_id"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

type ClawnProjectSource = clawnmodule.ProjectSource

type ClawnConnectReq = clawnmodule.ConnectReq

type GeminiEmbeddingResponse struct {
	Embedding struct {
		Values []float32 `json:"values"`
	} `json:"embedding"`
}

func broadcastUpdate(msg string) {
	if wsHub != nil {
		wsHub.Broadcast(msg)
		return
	}
	clientsMu.Lock()
	defer clientsMu.Unlock()
	for client := range clients {
		if err := client.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
			log.Println("WS write error:", err)
			client.Close()
			delete(clients, client)
		}
	}
}

func initDB() {
	db.Exec(context.Background(), "CREATE EXTENSION IF NOT EXISTS vector")
	db.Exec(context.Background(), "CREATE EXTENSION IF NOT EXISTS pgcrypto")

	db.Exec(context.Background(), `
	CREATE TABLE IF NOT EXISTS boards (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		user_id TEXT,
		title TEXT NOT NULL,
		description TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);`)
	db.Exec(context.Background(), "ALTER TABLE boards ADD COLUMN IF NOT EXISTS user_id TEXT")

	var defaultBoardID string
	err := db.QueryRow(context.Background(), "SELECT id::text FROM boards WHERE title='Main Project' LIMIT 1").Scan(&defaultBoardID)
	if err != nil {
		db.QueryRow(context.Background(), "INSERT INTO boards (title, description) VALUES ('Main Project', 'Default board') RETURNING id::text").Scan(&defaultBoardID)
	}

	db.Exec(context.Background(), `
	CREATE TABLE IF NOT EXISTS members (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		role TEXT NOT NULL,
		avatar TEXT
	);`)

	db.Exec(context.Background(), `
	CREATE TABLE IF NOT EXISTS board_members (
		board_id UUID NOT NULL,
		member_id TEXT NOT NULL,
		role TEXT DEFAULT 'editor',
		joined_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (board_id, member_id),
		CONSTRAINT fk_bm_board FOREIGN KEY(board_id) REFERENCES boards(id) ON DELETE CASCADE,
		CONSTRAINT fk_bm_member FOREIGN KEY(member_id) REFERENCES members(id) ON DELETE CASCADE
	);`)

	db.Exec(context.Background(), `
	CREATE TABLE IF NOT EXISTS agents (
		id TEXT PRIMARY KEY REFERENCES members(id) ON DELETE CASCADE,
		soul TEXT DEFAULT '',
		memory TEXT DEFAULT '',
		rules TEXT DEFAULT '',
		cron_schedule TEXT DEFAULT '*/10 * * * *',
		active BOOLEAN DEFAULT true,
		status TEXT DEFAULT 'offline',
		last_heartbeat_at TIMESTAMP NULL,
		last_seen_at TIMESTAMP NULL,
		current_task_id INT NULL,
		current_run_id INT NULL,
		current_activity TEXT DEFAULT '',
		health_note TEXT DEFAULT '',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);`)
	db.Exec(context.Background(), "ALTER TABLE agents ADD COLUMN IF NOT EXISTS status TEXT DEFAULT 'offline'")
	db.Exec(context.Background(), "ALTER TABLE agents ADD COLUMN IF NOT EXISTS last_heartbeat_at TIMESTAMP NULL")
	db.Exec(context.Background(), "ALTER TABLE agents ADD COLUMN IF NOT EXISTS last_seen_at TIMESTAMP NULL")
	db.Exec(context.Background(), "ALTER TABLE agents ADD COLUMN IF NOT EXISTS current_task_id INT NULL")
	db.Exec(context.Background(), "ALTER TABLE agents ADD COLUMN IF NOT EXISTS current_run_id INT NULL")
	db.Exec(context.Background(), "ALTER TABLE agents ADD COLUMN IF NOT EXISTS current_activity TEXT DEFAULT ''")
	db.Exec(context.Background(), "ALTER TABLE agents ADD COLUMN IF NOT EXISTS health_note TEXT DEFAULT ''")
	db.Exec(context.Background(), "ALTER TABLE agents ADD COLUMN IF NOT EXISTS display_name TEXT DEFAULT ''")
	db.Exec(context.Background(), "ALTER TABLE agents ADD COLUMN IF NOT EXISTS role_name TEXT DEFAULT ''")
	db.Exec(context.Background(), "ALTER TABLE agents ADD COLUMN IF NOT EXISTS avatar TEXT DEFAULT ''")
	db.Exec(context.Background(), "ALTER TABLE agents ADD COLUMN IF NOT EXISTS provider TEXT DEFAULT 'external'")
	db.Exec(context.Background(), "ALTER TABLE agents ADD COLUMN IF NOT EXISTS engine TEXT DEFAULT 'custom'")
	db.Exec(context.Background(), "ALTER TABLE agents ADD COLUMN IF NOT EXISTS description TEXT DEFAULT ''")
	db.Exec(context.Background(), "ALTER TABLE agents ADD COLUMN IF NOT EXISTS is_native_clawn BOOLEAN DEFAULT false")

	db.Exec(context.Background(), `
	CREATE TABLE IF NOT EXISTS tasks (
		id SERIAL PRIMARY KEY,
		board_id UUID NOT NULL,
		title TEXT NOT NULL,
		description TEXT,
		list_id TEXT NOT NULL,
		position INT DEFAULT 0,
		assignee_id TEXT REFERENCES members(id),
		parent_id INT REFERENCES tasks(id) ON DELETE CASCADE,
		CONSTRAINT fk_board FOREIGN KEY(board_id) REFERENCES boards(id) ON DELETE CASCADE
	);`)
	db.Exec(context.Background(), "ALTER TABLE tasks ADD COLUMN IF NOT EXISTS parent_id INT REFERENCES tasks(id) ON DELETE CASCADE")
	db.Exec(context.Background(), "ALTER TABLE tasks ADD COLUMN IF NOT EXISTS status TEXT DEFAULT 'todo'")
	db.Exec(context.Background(), "ALTER TABLE tasks ADD COLUMN IF NOT EXISTS blocked_reason TEXT DEFAULT ''")
	db.Exec(context.Background(), "UPDATE tasks SET status = COALESCE(NULLIF(status, ''), lower(list_id), 'todo')")
	db.Exec(context.Background(), "UPDATE tasks SET status = 'in_progress' WHERE status = 'doing'")
	db.Exec(context.Background(), "UPDATE tasks SET status = 'review' WHERE status = 'qa'")
	db.Exec(context.Background(), "UPDATE tasks SET status = lower(list_id) WHERE status NOT IN ('backlog','todo','in_progress','review','done','blocked')")

	db.Exec(context.Background(), `
	CREATE TABLE IF NOT EXISTS deliverables (
		id SERIAL PRIMARY KEY,
		task_id INT NOT NULL,
		title TEXT NOT NULL,
		description TEXT,
		artifact_type TEXT,
		content TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		CONSTRAINT fk_dev_task FOREIGN KEY(task_id) REFERENCES tasks(id) ON DELETE CASCADE
	);`)

	db.Exec(context.Background(), `
	CREATE TABLE IF NOT EXISTS activities (
		id SERIAL PRIMARY KEY,
		task_id INT NOT NULL,
		user_id TEXT NOT NULL,
		action TEXT NOT NULL,
		details TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		CONSTRAINT fk_act_task FOREIGN KEY(task_id) REFERENCES tasks(id) ON DELETE CASCADE
	);`)

	db.Exec(context.Background(), "ALTER TABLE tasks ADD COLUMN IF NOT EXISTS embedding vector(3072)")

	db.Exec(context.Background(), `
	CREATE TABLE IF NOT EXISTS documents (
		id SERIAL PRIMARY KEY,
		board_id UUID NOT NULL,
		title TEXT NOT NULL,
		content TEXT DEFAULT '',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		CONSTRAINT fk_doc_board FOREIGN KEY(board_id) REFERENCES boards(id) ON DELETE CASCADE
	);`)

	db.Exec(context.Background(), "ALTER TABLE documents ADD COLUMN IF NOT EXISTS embedding vector(3072)")

	db.Exec(context.Background(), `
	CREATE TABLE IF NOT EXISTS comments (
		id SERIAL PRIMARY KEY,
		task_id INT NOT NULL,
		user_id TEXT NOT NULL,
		content TEXT NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		CONSTRAINT fk_comment_task FOREIGN KEY(task_id) REFERENCES tasks(id) ON DELETE CASCADE
	);`)

	db.Exec(context.Background(), `
	CREATE TABLE IF NOT EXISTS agent_runs (
		id SERIAL PRIMARY KEY,
		task_id INT NOT NULL,
		agent_id TEXT NOT NULL REFERENCES members(id) ON DELETE CASCADE,
		session_key TEXT DEFAULT '',
		provider_type TEXT DEFAULT 'openclaw',
		status TEXT DEFAULT 'queued',
		started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		ended_at TIMESTAMP NULL,
		current_activity TEXT DEFAULT '',
		error_summary TEXT DEFAULT '',
		result_summary TEXT DEFAULT '',
		CONSTRAINT fk_agent_run_task FOREIGN KEY(task_id) REFERENCES tasks(id) ON DELETE CASCADE
	);`)

	db.Exec(context.Background(), `
	CREATE TABLE IF NOT EXISTS notifications (
		id SERIAL PRIMARY KEY,
		task_id INT NULL REFERENCES tasks(id) ON DELETE CASCADE,
		target_agent_id TEXT NOT NULL REFERENCES members(id) ON DELETE CASCADE,
		source_agent_id TEXT NULL REFERENCES members(id) ON DELETE SET NULL,
		type TEXT NOT NULL,
		content TEXT NOT NULL,
		delivered BOOLEAN DEFAULT false,
		delivered_at TIMESTAMP NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);`)

	db.Exec(context.Background(), `
	CREATE TABLE IF NOT EXISTS agent_connectors (
		id SERIAL PRIMARY KEY,
		agent_id TEXT NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
		connector_type TEXT NOT NULL,
		transport_mode TEXT DEFAULT 'internal',
		auth_type TEXT NOT NULL,
		machine_token_hash TEXT DEFAULT '',
		endpoint_url TEXT DEFAULT '',
		base_url TEXT DEFAULT '',
		agent_ref TEXT DEFAULT '',
		session_key TEXT DEFAULT '',
		shared_secret_hash TEXT DEFAULT '',
		status TEXT DEFAULT 'pending',
		metadata_json JSONB DEFAULT '{}'::jsonb,
		last_success_at TIMESTAMP NULL,
		last_error TEXT DEFAULT '',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);`)
	db.Exec(context.Background(), "ALTER TABLE agent_connectors ADD COLUMN IF NOT EXISTS transport_mode TEXT DEFAULT 'internal'")
	db.Exec(context.Background(), "ALTER TABLE agent_connectors ADD COLUMN IF NOT EXISTS base_url TEXT DEFAULT ''")
	db.Exec(context.Background(), "ALTER TABLE agent_connectors ADD COLUMN IF NOT EXISTS agent_ref TEXT DEFAULT ''")
	db.Exec(context.Background(), "ALTER TABLE agent_connectors ADD COLUMN IF NOT EXISTS is_primary BOOLEAN DEFAULT false")
	db.Exec(context.Background(), "ALTER TABLE agent_events ADD COLUMN IF NOT EXISTS next_attempt_at TIMESTAMP NULL")

	db.Exec(context.Background(), `
	CREATE TABLE IF NOT EXISTS board_agents (
		id SERIAL PRIMARY KEY,
		board_id UUID NOT NULL REFERENCES boards(id) ON DELETE CASCADE,
		agent_id TEXT NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
		board_role TEXT DEFAULT 'worker',
		active BOOLEAN DEFAULT true,
		auto_accept_tasks BOOLEAN DEFAULT true,
		can_comment BOOLEAN DEFAULT true,
		can_update_status BOOLEAN DEFAULT true,
		can_access_docs BOOLEAN DEFAULT true,
		can_create_deliverables BOOLEAN DEFAULT true,
		capabilities_json JSONB DEFAULT '{}'::jsonb,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		UNIQUE (board_id, agent_id)
	);`)

	db.Exec(context.Background(), `
	CREATE TABLE IF NOT EXISTS agent_events (
		id SERIAL PRIMARY KEY,
		agent_id TEXT NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
		board_id UUID NULL REFERENCES boards(id) ON DELETE CASCADE,
		task_id INT NULL REFERENCES tasks(id) ON DELETE CASCADE,
		event_type TEXT NOT NULL,
		payload_json JSONB NOT NULL,
		delivery_status TEXT DEFAULT 'pending',
		delivery_attempts INT DEFAULT 0,
		next_attempt_at TIMESTAMP NULL,
		last_delivery_at TIMESTAMP NULL,
		response_status TEXT DEFAULT '',
		response_body TEXT DEFAULT '',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		processed_at TIMESTAMP NULL
	);`)

	db.Exec(context.Background(), `
	CREATE TABLE IF NOT EXISTS board_stats (
		board_id UUID PRIMARY KEY REFERENCES boards(id) ON DELETE CASCADE,
		open_tasks INT DEFAULT 0,
		review_tasks INT DEFAULT 0,
		blocked_tasks INT DEFAULT 0,
		docs_count INT DEFAULT 0,
		connected_agents INT DEFAULT 0,
		agent_issues INT DEFAULT 0,
		last_task_event_at TIMESTAMP NULL,
		last_doc_event_at TIMESTAMP NULL,
		last_agent_event_at TIMESTAMP NULL,
		last_event_at TIMESTAMP NULL,
		computed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);`)
	db.Exec(context.Background(), `
	CREATE TABLE IF NOT EXISTS audit_logs (
		id SERIAL PRIMARY KEY,
		actor_type TEXT NOT NULL,
		actor_id TEXT NOT NULL,
		entity_type TEXT NOT NULL,
		entity_id TEXT NOT NULL,
		action TEXT NOT NULL,
		old_value_json JSONB DEFAULT '{}'::jsonb,
		new_value_json JSONB DEFAULT '{}'::jsonb,
		metadata_json JSONB DEFAULT '{}'::jsonb,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);`)

	db.Exec(context.Background(), "CREATE INDEX IF NOT EXISTS idx_board_stats_last_event_at ON board_stats(last_event_at DESC)")
	db.Exec(context.Background(), "CREATE INDEX IF NOT EXISTS idx_tasks_board_status ON tasks(board_id, status)")
	db.Exec(context.Background(), "CREATE INDEX IF NOT EXISTS idx_audit_logs_entity ON audit_logs(entity_type, entity_id, created_at DESC)")
	db.Exec(context.Background(), "CREATE INDEX IF NOT EXISTS idx_audit_logs_action ON audit_logs(action, created_at DESC)")
	db.Exec(context.Background(), "CREATE INDEX IF NOT EXISTS idx_documents_board_updated_at ON documents(board_id, updated_at DESC)")
	db.Exec(context.Background(), "CREATE INDEX IF NOT EXISTS idx_activities_task_created_at ON activities(task_id, created_at DESC)")
	db.Exec(context.Background(), "CREATE INDEX IF NOT EXISTS idx_board_agents_board_active ON board_agents(board_id, active)")
	db.Exec(context.Background(), "CREATE INDEX IF NOT EXISTS idx_agent_events_board_created_at ON agent_events(board_id, created_at DESC)")
	db.Exec(context.Background(), "CREATE INDEX IF NOT EXISTS idx_agent_events_board_delivery_at ON agent_events(board_id, last_delivery_at DESC)")

	seedMembers()
	seedBoardMembers()

	fmt.Println("✅ Database migrated!")
}

func seedMembers() {
	members := []Member{
		{ID: "mirza", Name: "Mirza", Role: "human", Avatar: "👤"},
		{ID: "devo", Name: "Devo", Role: "agent", Avatar: "🛡️"},
		{ID: "kodinger", Name: "Kodinger", Role: "agent", Avatar: "👨‍💻"},
		{ID: "mimin", Name: "Mimin", Role: "agent", Avatar: "📢"},
		{ID: "antigravity", Name: "Antigravity", Role: "agent", Avatar: "🌌"},
	}
	for _, m := range members {
		db.Exec(context.Background(),
			"INSERT INTO members (id, name, role, avatar) VALUES ($1, $2, $3, $4) ON CONFLICT (id) DO UPDATE SET name=$2, role=$3, avatar=$4",
			m.ID, m.Name, m.Role, m.Avatar)

		if m.Role == "agent" {
			db.Exec(context.Background(),
				"INSERT INTO agents (id, display_name, role_name, avatar, provider, engine, description, is_native_clawn, soul, memory, rules, cron_schedule, status) VALUES ($1, $2, $3, $4, 'clawn', 'openclaw', '', true, $5, $6, $7, $8, 'offline') ON CONFLICT (id) DO UPDATE SET display_name=EXCLUDED.display_name, role_name=EXCLUDED.role_name, avatar=EXCLUDED.avatar, provider=EXCLUDED.provider, engine=EXCLUDED.engine, is_native_clawn=EXCLUDED.is_native_clawn",
				m.ID, m.Name, m.Role, m.Avatar, "You are a helpful AI assistant in a software squad.", "No memories yet.", "Be concise.", "*/1 * * * *")
		}
	}
}

func seedBoardMembers() {
	rows, err := db.Query(context.Background(), "SELECT id FROM boards")
	if err != nil {
		fmt.Println("seedBoardMembers: failed to query boards:", err)
		return
	}
	defer rows.Close()
	var boardIDs []string
	for rows.Next() {
		var id string
		rows.Scan(&id)
		boardIDs = append(boardIDs, id)
	}

	mRows, err := db.Query(context.Background(), "SELECT id FROM members")
	if err != nil {
		fmt.Println("seedBoardMembers: failed to query members:", err)
		return
	}
	defer mRows.Close()
	var memberIDs []string
	for mRows.Next() {
		var id string
		mRows.Scan(&id)
		memberIDs = append(memberIDs, id)
	}

	for _, bid := range boardIDs {
		for _, mid := range memberIDs {
			db.Exec(context.Background(),
				"INSERT INTO board_members (board_id, member_id) VALUES ($1, $2) ON CONFLICT DO NOTHING",
				bid, mid)
		}
	}
}

func initAI() {
	// OpenAI client reserved for future use (e.g., chat completions).
	// Currently only Gemini is used for embeddings.
	apiKey := os.Getenv("OPENAI_API_KEY")
	baseURL := os.Getenv("OPENAI_BASE_URL")
	if apiKey != "" {
		config := openai.DefaultConfig(apiKey)
		if baseURL != "" {
			config.BaseURL = baseURL
		}
		openaiClient = openai.NewClientWithConfig(config)
	}
}

func generateEmbedding(text string) ([]float32, error) {
	geminiKey := os.Getenv("GEMINI_API_KEY")
	if geminiKey != "" {
		url := "https://generativelanguage.googleapis.com/v1beta/models/text-embedding-004:embedContent?key=" + geminiKey
		body := map[string]interface{}{
			"model":   "models/text-embedding-004",
			"content": map[string]interface{}{"parts": []map[string]interface{}{{"text": text}}},
		}
		jsonBody, _ := json.Marshal(body)
		resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonBody))
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			if resp.StatusCode == 404 {
				url = "https://generativelanguage.googleapis.com/v1beta/models/gemini-embedding-001:embedContent?key=" + geminiKey
				body["model"] = "models/gemini-embedding-001"
				jsonBody, _ = json.Marshal(body)
				resp, err = http.Post(url, "application/json", bytes.NewBuffer(jsonBody))
				if err != nil {
					return nil, err
				}
				defer resp.Body.Close()
			}
			if resp.StatusCode != 200 {
				buf := new(bytes.Buffer)
				buf.ReadFrom(resp.Body)
				return nil, fmt.Errorf("gemini api error %d: %s", resp.StatusCode, buf.String())
			}
		}
		var result GeminiEmbeddingResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, err
		}
		return result.Embedding.Values, nil
	}
	return nil, fmt.Errorf("no AI provider configured")
}

func main() {
	cfg := config.Load()
	ctx := context.Background()

	var err error
	db, err = platformdb.New(ctx, cfg)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v\n", err)
	}
	initDB()
	openaiClient = platformai.NewOpenAIClient(cfg)
	initAI()
	rdb = cache.NewRedis(cfg)
	wsHub = realtime.NewHub()

	deliverySvc := deliverymodule.NewService(db, deliverySemaphore, signWebhookPayload, gatewayutil.TruncateForLog, gatewayutil.SendAgentMessageViaGateway)
	agentInfraSvc := agentinframodule.NewService(db, deliverySvc.DeliverPendingAgentEvents)
	dispatchRepo := dispatchmodule.NewRepository(db)
	dispatchSvc := dispatchmodule.NewService(dispatchRepo, agentInfraSvc.CreateAgentRun, logActivity, broadcastUpdate, gatewayutil.TruncateForLog)
	go dispatchSvc.StartDispatcher(context.Background())

	app := httpapp.New()
	app.Use("/ws", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			c.Locals("allowed", true)
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})
	app.Get("/ws", websocket.New(wsHub.HandleConnection))
	app.Get("/api/health", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"status": "ok"}) })
	app.Get("/api/readiness", func(c *fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		latencyStats.Lock()
		latencySnapshot := fiber.Map{}
		for k, v := range latencyStats.ByPath {
			latencySnapshot[k] = v
		}
		latencyStats.Unlock()
		status := fiber.Map{
			"status":  "ready",
			"time":    time.Now().UTC().Format(time.RFC3339),
			"checks":  fiber.Map{},
			"latency": latencySnapshot,
		}
		checks := status["checks"].(fiber.Map)
		ready := true
		if err := db.Ping(ctx); err != nil {
			ready = false
			checks["db"] = fiber.Map{"ok": false, "error": err.Error()}
		} else {
			checks["db"] = fiber.Map{"ok": true}
		}
		if err := rdb.Ping(ctx).Err(); err != nil {
			ready = false
			checks["redis"] = fiber.Map{"ok": false, "error": err.Error()}
		} else {
			checks["redis"] = fiber.Map{"ok": true}
		}
		checks["build"] = fiber.Map{"ok": true, "version": "moziboard-backend", "go_env": os.Getenv("GO_ENV")}
		if !ready {
			status["status"] = "degraded"
			return c.Status(fiber.StatusServiceUnavailable).JSON(status)
		}
		return c.JSON(status)
	})

	app.Use(func(c *fiber.Ctx) error {
		if strings.HasPrefix(c.Path(), "/api/") {
			requestID := c.Get("X-Request-ID")
			if strings.TrimSpace(requestID) == "" {
				requestID = uuid.NewString()
			}
			path := string(append([]byte(nil), c.Path()...))
			method := c.Method()
			c.Set("X-Request-ID", requestID)
			c.Locals("request_id", requestID)
			start := time.Now()
			err := c.Next()
			durationMs := time.Since(start).Milliseconds()
			log.Printf("api_request request_id=%s method=%s path=%s status=%d duration_ms=%d", requestID, method, path, c.Response().StatusCode(), durationMs)
			if strings.HasPrefix(path, "/api/ops/") || strings.HasPrefix(path, "/api/runtime/") || path == "/api/readiness" || path == "/api/home/overview" {
				latencyStats.Lock()
				prev, ok := latencyStats.ByPath[path]
				count := int64(1)
				avg := durationMs
				max := durationMs
				if ok {
					if v, vok := prev["count"].(int64); vok {
						count = v + 1
					}
					if v, vok := prev["avg_ms"].(int64); vok {
						avg = ((v * (count - 1)) + durationMs) / count
					}
					if v, vok := prev["max_ms"].(int64); vok && v > max {
						max = v
					}
				}
				latencyStats.ByPath[path] = fiber.Map{"last_ms": durationMs, "avg_ms": avg, "max_ms": max, "count": count, "last_status": c.Response().StatusCode(), "updated_at": time.Now().UTC().Format(time.RFC3339)}
				latencyStats.Unlock()
				log.Printf("api_latency request_id=%s method=%s path=%s status=%d duration_ms=%d", requestID, method, path, c.Response().StatusCode(), durationMs)
			}
			return err
		}
		return c.Next()
	})

	authSession := authsession.New("http://172.17.0.1:4001")
	app.Use(func(c *fiber.Ctx) error {
		if c.Path() == "/api/health" || c.Path() == "/api/readiness" {
			return c.Next()
		}
		return authSession.RequireAuth(c)
	})

	boardAuthz := authz.NewBoardAuthorizer(db)
	requireBoardAuth := boardAuthz.RequireBoardAccess("id")
	requireTaskAuth := boardAuthz.RequireTaskAccess("id")
	requireDocAuth := boardAuthz.RequireDocumentAccess("id")
	requireBoardFromBody := boardAuthz.RequireBoardAccessFromBody("board_id")
	requireBoardFromQuery := boardAuthz.RequireBoardAccessFromQuery("board_id")

	boardRepo := boardmodule.NewRepository(db)
	boardSvc := boardmodule.NewService(boardRepo)
	boardHandler := boardmodule.NewHandler(boardSvc, requireBoardAuth)
	boardHandler.RegisterRoutes(app)

	boardStatsRepo := boardstatsmodule.NewRepository(db)
	boardStatsSvc := boardstatsmodule.NewService(boardStatsRepo, rdb)
	if err := boardStatsSvc.RecomputeAll(context.Background()); err != nil {
		log.Printf("board_stats recompute failed on startup: %v", err)
	}
	boardStatsHandler := boardstatsmodule.NewHandler(boardStatsSvc, requireBoardAuth)
	boardStatsHandler.RegisterRoutes(app)

	activityfeedRepo := activityfeedmodule.NewRepository(db)
	activityfeedSvc := activityfeedmodule.NewService(activityfeedRepo)
	activityfeedHandler := activityfeedmodule.NewHandler(activityfeedSvc, requireBoardAuth)
	activityfeedHandler.RegisterRoutes(app)
	attentionRepo := attentionmodule.NewRepository(db)
	attentionSvc := attentionmodule.NewService(attentionRepo)
	attentionHandler := attentionmodule.NewHandler(attentionSvc, requireBoardAuth)
	attentionHandler.RegisterRoutes(app)
	boardhealthRepo := boardhealthmodule.NewRepository(db)
	boardhealthSvc := boardhealthmodule.NewService(boardhealthRepo)
	boardhealthHandler := boardhealthmodule.NewHandler(boardhealthSvc, requireBoardAuth)
	boardhealthHandler.RegisterRoutes(app)
	homeSvc := homemodule.NewService(activityfeedSvc, attentionSvc, boardhealthSvc, boardStatsSvc, rdb)
	homeHandler := homemodule.NewHandler(homeSvc)
	homeHandler.RegisterRoutes(app)

	docsRepo := docsmodule.NewRepository(db)
	docsSvc := docsmodule.NewService(docsRepo, updateDocEmbedding, func(text string) (string, error) {
		emb, err := generateEmbedding(text)
		if err != nil {
			return "", err
		}
		return pgvector(emb), nil
	}, boardStatsSvc)
	docsHandler := docsmodule.NewHandler(docsSvc, requireBoardAuth, requireBoardFromQuery, requireDocAuth)
	docsHandler.RegisterRoutes(app)

	commentsRepo := commentsmodule.NewRepository(db)
	commentsSvc := commentsmodule.NewService(commentsRepo)
	commentsHandler := commentsmodule.NewHandler(commentsSvc, requireTaskAuth)
	commentsHandler.RegisterRoutes(app)

	deliverablesRepo := deliverablesmodule.NewRepository(db)
	deliverablesSvc := deliverablesmodule.NewService(deliverablesRepo, logActivity)
	deliverablesHandler := deliverablesmodule.NewHandler(deliverablesSvc, requireTaskAuth)
	deliverablesHandler.RegisterRoutes(app)

	tasksRepo := tasksmodule.NewRepository(db)
	tasksSvc := tasksmodule.NewService(tasksRepo, tasksmodule.SideEffects{
		AgentInfra:  agentInfraSvc,
		LogActivity: logActivity,
		AuditLog:    logAudit,
		Broadcast:   broadcastUpdate,
		AgentStateUpdater: tasksmodule.AgentStateUpdaterFuncs{
			MarkQueuedFunc: func(taskID int, runID int, agentID string) {
				db.Exec(context.Background(), "UPDATE agents SET current_task_id=$1, current_run_id=$2, status='busy', current_activity=$3, last_seen_at=CURRENT_TIMESTAMP, updated_at=CURRENT_TIMESTAMP WHERE id=$4", taskID, runID, "Queued on task", agentID)
			},
			MarkBlockedFunc: func(blockedReason string, agentID string) {
				db.Exec(context.Background(), "UPDATE agents SET status='blocked', current_activity=$1, health_note=$2, updated_at=CURRENT_TIMESTAMP WHERE id=$3", "Blocked on task", blockedReason, agentID)
			},
			MarkOnlineFunc: func(agentID string) {
				db.Exec(context.Background(), "UPDATE agents SET status='online', current_task_id=NULL, current_run_id=NULL, current_activity='Idle', health_note='', updated_at=CURRENT_TIMESTAMP WHERE id=$1", agentID)
			},
			MarkInProgressFunc: func(taskID int, agentID string) {
				db.Exec(context.Background(), "UPDATE agents SET status='busy', current_task_id=$1, current_activity='Working on task', updated_at=CURRENT_TIMESTAMP WHERE id=$2", taskID, agentID)
			},
		},
		Embedder: func(text string) (string, error) {
			emb, err := generateEmbedding(text)
			if err != nil {
				return "", err
			}
			return pgvector(emb), nil
		},
		BoardStats: boardStatsSvc,
	})
	tasksHandler := tasksmodule.NewHandler(tasksSvc, docsSvc, requireBoardAuth, requireBoardFromBody, requireBoardFromQuery, requireTaskAuth)
	tasksHandler.RegisterRoutes(app)

	agentsRepo := agentsmodule.NewRepository(db)
	agentsSvc := agentsmodule.NewService(agentsRepo, generateMachineToken, hashToken, boardStatsSvc)
	agentsHandler := agentsmodule.NewHandler(agentsSvc, requireBoardAuth)
	agentsHandler.RegisterRoutes(app)

	runtimeRepo := runtimemodule.NewRepository(db)
	runtimeSvc := runtimemodule.NewService(runtimeRepo, hashToken, signWebhookPayload, agentInfraSvc, logActivity, logAudit, broadcastUpdate, boardStatsSvc)
	runtimeHandler := runtimemodule.NewHandler(runtimeSvc)
	runtimeHandler.RegisterRoutes(app)

	opsRepo := opsmodule.NewRepository(db)
	opsSvc := opsmodule.NewService(opsRepo, deliverySvc.DeliverPendingAgentEvents, logAudit)
	if every := strings.TrimSpace(os.Getenv("OPS_MAINTENANCE_EVERY")); every != "" {
		if dur, err := time.ParseDuration(every); err != nil {
			log.Printf("ops_maintenance_scheduler invalid_duration=%q err=%v", every, err)
		} else if dur > 0 {
			opsSvc.ConfigureMaintenanceScheduler(dur)
			opsSvc.StartMaintenanceScheduler(context.Background())
		}
	}
	opsHandler := opsmodule.NewHandler(opsSvc)
	opsHandler.RegisterRoutes(app)

	clawnRepo := clawnmodule.NewRepository(db)
	clawnSvc := clawnmodule.NewService(clawnRepo, generateMachineToken)
	clawnHandler := clawnmodule.NewHandler(clawnSvc)
	clawnHandler.RegisterRoutes(app)

	taskRunsHandler := taskrunsmodule.NewHandler(db, requireTaskAuth)
	taskRunsHandler.RegisterRoutes(app)

	authProxyHandler := authproxymodule.NewHandler()
	authProxyHandler.RegisterRoutes(app)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Fatal(app.Listen(":" + port))
}

// --- Agent Dispatcher ---

func generateMachineToken() (string, string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", "", err
	}
	plain := "mb_rt_" + hex.EncodeToString(buf)
	h := sha256.Sum256([]byte(plain))
	return plain, hex.EncodeToString(h[:]), nil
}

func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

func signWebhookPayload(secret string, timestamp string, body string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write([]byte(body))
	return hex.EncodeToString(mac.Sum(nil))
}

func extractBearerToken(c *fiber.Ctx) string {
	auth := c.Get("Authorization")
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return strings.TrimSpace(auth[7:])
	}
	return ""
}

func getBoards(c *fiber.Ctx) error {
	rows, err := db.Query(context.Background(), "SELECT id::text, user_id, title, description FROM boards ORDER BY created_at ASC")
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	defer rows.Close()
	var boards []Board
	for rows.Next() {
		var b Board
		if err := rows.Scan(&b.ID, &b.UserID, &b.Title, &b.Description); err != nil {
			return c.Status(500).SendString(err.Error())
		}
		boards = append(boards, b)
	}
	if boards == nil {
		boards = []Board{}
	}
	return c.JSON(boards)
}

func createBoard(c *fiber.Ctx) error {
	b := new(Board)
	if err := c.BodyParser(b); err != nil {
		return c.Status(400).SendString(err.Error())
	}
	err := db.QueryRow(context.Background(), "INSERT INTO boards (user_id, title, description) VALUES ($1, $2, $3) RETURNING id::text", b.UserID, b.Title, b.Description).Scan(&b.ID)
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	if b.UserID != nil && *b.UserID != "" {
		db.Exec(context.Background(), "INSERT INTO board_members (board_id, member_id, role) VALUES ($1, $2, 'owner')", b.ID, *b.UserID)
	} else {
		// Fallback for anonymous creations
		db.Exec(context.Background(), "INSERT INTO board_members (board_id, member_id, role) VALUES ($1, 'system', 'owner')", b.ID)
	}
	return c.JSON(b)
}

func getMembers(c *fiber.Ctx) error {
	rows, err := db.Query(context.Background(), "SELECT id, name, role, avatar FROM members")
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	defer rows.Close()
	var members []Member
	for rows.Next() {
		var m Member
		if err := rows.Scan(&m.ID, &m.Name, &m.Role, &m.Avatar); err != nil {
			return c.Status(500).SendString(err.Error())
		}
		members = append(members, m)
	}
	if members == nil {
		members = []Member{}
	}
	return c.JSON(members)
}

func buildInternalBridgeMessage(ev AgentEvent, conn *AgentConnector, envelope string) string {
	return fmt.Sprintf("MoziBoard internal bridge event. Connector: clawn_native/internal. AgentRef: %s. SessionKey: %s. Event JSON:\n%s", conn.AgentRef, conn.SessionKey, envelope)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func getBoardMembers(c *fiber.Ctx) error {
	boardID := c.Params("id")
	query := `
		SELECT m.id, m.name, m.role, m.avatar 
		FROM members m 
		JOIN board_members bm ON m.id = bm.member_id 
		WHERE bm.board_id = $1
	`
	rows, err := db.Query(context.Background(), query, boardID)
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	defer rows.Close()
	var members []Member
	for rows.Next() {
		var m Member
		if err := rows.Scan(&m.ID, &m.Name, &m.Role, &m.Avatar); err != nil {
			return c.Status(500).SendString(err.Error())
		}
		members = append(members, m)
	}
	if members == nil {
		members = []Member{}
	}
	return c.JSON(members)
}

func addBoardMember(c *fiber.Ctx) error {
	boardID := c.Params("id")
	req := new(BoardMemberReq)
	if err := c.BodyParser(req); err != nil {
		return c.Status(400).SendString(err.Error())
	}
	if req.Role == "" {
		req.Role = "editor"
	}
	_, err := db.Exec(context.Background(),
		"INSERT INTO board_members (board_id, member_id, role) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING",
		boardID, req.MemberID, req.Role)
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	return c.SendStatus(200)
}

func removeBoardMember(c *fiber.Ctx) error {
	boardID := c.Params("id")
	memberID := c.Params("mid")
	_, err := db.Exec(context.Background(),
		"DELETE FROM board_members WHERE board_id=$1 AND member_id=$2",
		boardID, memberID)
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	return c.SendStatus(200)
}

func logActivity(taskID int, userID, action, details string) {
	db.Exec(context.Background(),
		"INSERT INTO activities (task_id, user_id, action, details) VALUES ($1, $2, $3, $4)",
		taskID, userID, action, details)
}

func logAudit(actorType, actorID, entityType, entityID, action string, oldValue, newValue, metadata interface{}) {
	oldJSON, _ := json.Marshal(defaultJSONMap(oldValue))
	newJSON, _ := json.Marshal(defaultJSONMap(newValue))
	metaJSON, _ := json.Marshal(defaultJSONMap(metadata))
	db.Exec(context.Background(),
		"INSERT INTO audit_logs (actor_type, actor_id, entity_type, entity_id, action, old_value_json, new_value_json, metadata_json) VALUES ($1,$2,$3,$4,$5,$6::jsonb,$7::jsonb,$8::jsonb)",
		actorType, actorID, entityType, entityID, action, string(oldJSON), string(newJSON), string(metaJSON))
}

func defaultJSONMap(v interface{}) interface{} {
	if v == nil {
		return map[string]interface{}{}
	}
	return v
}

func pgvector(v []float32) string { b, _ := json.Marshal(v); return string(b) }

// --- Knowledge Base / Documents ---

func getBoardDocs(c *fiber.Ctx) error {
	boardID := c.Params("id")
	rows, err := db.Query(context.Background(),
		"SELECT id, board_id::text, title, content, created_at, updated_at FROM documents WHERE board_id=$1 ORDER BY updated_at DESC",
		boardID)
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	defer rows.Close()
	var docs []Document
	for rows.Next() {
		var d Document
		if err := rows.Scan(&d.ID, &d.BoardID, &d.Title, &d.Content, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return c.Status(500).SendString(err.Error())
		}
		docs = append(docs, d)
	}
	if docs == nil {
		docs = []Document{}
	}
	return c.JSON(docs)
}

func createDoc(c *fiber.Ctx) error {
	boardID := c.Params("id")
	d := new(Document)
	if err := c.BodyParser(d); err != nil {
		return c.Status(400).SendString(err.Error())
	}
	if d.Title == "" {
		return c.Status(400).SendString("Title is required")
	}
	var id int
	var createdAt, updatedAt time.Time
	err := db.QueryRow(context.Background(),
		"INSERT INTO documents (board_id, title, content) VALUES ($1, $2, $3) RETURNING id, created_at, updated_at",
		boardID, d.Title, d.Content).Scan(&id, &createdAt, &updatedAt)
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	d.ID = id
	d.BoardID = boardID
	d.CreatedAt = createdAt
	d.UpdatedAt = updatedAt
	go updateDocEmbedding(id, d.Title+" "+d.Content)
	return c.JSON(d)
}

func updateDoc(c *fiber.Ctx) error {
	id, _ := strconv.Atoi(c.Params("id"))
	d := new(Document)
	if err := c.BodyParser(d); err != nil {
		return c.Status(400).SendString(err.Error())
	}

	var existing Document
	err := db.QueryRow(context.Background(),
		"SELECT id, board_id::text, title, content FROM documents WHERE id=$1", id).Scan(
		&existing.ID, &existing.BoardID, &existing.Title, &existing.Content)
	if err != nil {
		return c.Status(404).SendString("Document not found")
	}

	if d.Title != "" {
		existing.Title = d.Title
	}
	if d.Content != "" {
		existing.Content = d.Content
	}

	_, err = db.Exec(context.Background(),
		"UPDATE documents SET title=$1, content=$2, updated_at=CURRENT_TIMESTAMP WHERE id=$3",
		existing.Title, existing.Content, id)
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}

	err = db.QueryRow(context.Background(),
		"SELECT id, board_id::text, title, content, created_at, updated_at FROM documents WHERE id=$1", id).Scan(
		&existing.ID, &existing.BoardID, &existing.Title, &existing.Content, &existing.CreatedAt, &existing.UpdatedAt)
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	go updateDocEmbedding(id, existing.Title+" "+existing.Content)
	return c.JSON(existing)
}

func deleteDoc(c *fiber.Ctx) error {
	id, _ := strconv.Atoi(c.Params("id"))
	result, err := db.Exec(context.Background(), "DELETE FROM documents WHERE id=$1", id)
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	if result.RowsAffected() == 0 {
		return c.Status(404).SendString("Document not found")
	}
	return c.SendStatus(200)
}

func updateDocEmbedding(id int, text string) {
	emb, err := generateEmbedding(text)
	if err != nil {
		log.Printf("Doc emb err: %v", err)
		return
	}
	_, err = db.Exec(context.Background(), "UPDATE documents SET embedding = $1 WHERE id = $2", pgvector(emb), id)
	if err != nil {
		log.Printf("Doc db emb err: %v", err)
	}
}

func searchDocs(c *fiber.Ctx) error {
	query := c.Query("q")
	boardID := c.Query("board_id")
	if query == "" {
		return c.Status(400).SendString("Query required")
	}
	emb, err := generateEmbedding(query)
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}

	var sqlQuery string
	var args []interface{}
	if boardID != "" {
		sqlQuery = "SELECT id, board_id::text, title, content, created_at, updated_at FROM documents WHERE board_id=$1 AND embedding IS NOT NULL ORDER BY embedding <=> $2 LIMIT 5"
		args = []interface{}{boardID, pgvector(emb)}
	} else {
		sqlQuery = "SELECT id, board_id::text, title, content, created_at, updated_at FROM documents WHERE embedding IS NOT NULL ORDER BY embedding <=> $1 LIMIT 5"
		args = []interface{}{pgvector(emb)}
	}

	rows, err := db.Query(context.Background(), sqlQuery, args...)
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	defer rows.Close()
	var docs []Document
	for rows.Next() {
		var d Document
		if err := rows.Scan(&d.ID, &d.BoardID, &d.Title, &d.Content, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return c.Status(500).SendString(err.Error())
		}
		docs = append(docs, d)
	}
	if docs == nil {
		docs = []Document{}
	}
	return c.JSON(docs)
}

func getTaskComments(c *fiber.Ctx) error {
	taskID, _ := strconv.Atoi(c.Params("id"))
	rows, err := db.Query(context.Background(),
		"SELECT c.id, c.task_id, c.user_id, c.content, c.created_at FROM comments c WHERE c.task_id=$1 ORDER BY c.created_at ASC", taskID)
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	defer rows.Close()
	var comments []Comment
	for rows.Next() {
		var cm Comment
		if err := rows.Scan(&cm.ID, &cm.TaskID, &cm.UserID, &cm.Content, &cm.CreatedAt); err != nil {
			return c.Status(500).SendString(err.Error())
		}
		comments = append(comments, cm)
	}
	if comments == nil {
		comments = []Comment{}
	}
	return c.JSON(comments)
}

func createComment(c *fiber.Ctx) error {
	taskID, _ := strconv.Atoi(c.Params("id"))
	cm := new(Comment)
	if err := c.BodyParser(cm); err != nil {
		return c.Status(400).SendString(err.Error())
	}
	if cm.UserID == "" || cm.Content == "" {
		return c.Status(400).SendString("user_id and content are required")
	}
	var id int
	var createdAt time.Time
	err := db.QueryRow(context.Background(),
		"INSERT INTO comments (task_id, user_id, content) VALUES ($1, $2, $3) RETURNING id, created_at",
		taskID, cm.UserID, cm.Content).Scan(&id, &createdAt)
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	cm.ID = id
	cm.TaskID = taskID
	cm.CreatedAt = createdAt
	return c.JSON(cm)
}

func getTaskDeliverables(c *fiber.Ctx) error {
	taskID := c.Params("id")
	rows, err := db.Query(context.Background(), "SELECT id, task_id, title, description, artifact_type, content, created_at, updated_at FROM deliverables WHERE task_id=$1 ORDER BY created_at DESC", taskID)
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	defer rows.Close()
	var deliverables []Deliverable
	for rows.Next() {
		var d Deliverable
		if err := rows.Scan(&d.ID, &d.TaskID, &d.Title, &d.Description, &d.ArtifactType, &d.Content, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return c.Status(500).SendString(err.Error())
		}
		deliverables = append(deliverables, d)
	}
	if deliverables == nil {
		deliverables = []Deliverable{}
	}
	return c.JSON(deliverables)
}

func createDeliverable(c *fiber.Ctx) error {
	taskID, _ := strconv.Atoi(c.Params("id"))
	d := new(Deliverable)
	if err := c.BodyParser(d); err != nil {
		return c.Status(400).SendString(err.Error())
	}
	d.TaskID = taskID

	err := db.QueryRow(context.Background(),
		"INSERT INTO deliverables (task_id, title, description, artifact_type, content) VALUES ($1, $2, $3, $4, $5) RETURNING id, created_at, updated_at",
		d.TaskID, d.Title, d.Description, d.ArtifactType, d.Content).Scan(&d.ID, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}

	go logActivity(taskID, "system", "created_deliverable", fmt.Sprintf("Generated deliverable: %s", d.Title))
	return c.JSON(d)
}
