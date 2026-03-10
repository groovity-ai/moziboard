package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/sashabaranov/go-openai"
)

var (
	db           *pgxpool.Pool
	rdb          *redis.Client
	openaiClient *openai.Client
	clients      = make(map[*websocket.Conn]bool)
	clientsMu    sync.Mutex
	deliverySemaphore = make(chan struct{}, 4)
)

type Board struct {
	ID          string  `json:"id"`
	UserID      *string `json:"user_id"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
}

type Task struct {
	ID          int     `json:"id"`
	BoardID     string  `json:"board_id"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	ListID      string  `json:"list_id"`
	Position    int     `json:"position"`
	AssigneeID  *string `json:"assignee_id"`
	ParentID    *int    `json:"parent_id,omitempty"`
	UpdatedBy   string  `json:"updated_by,omitempty"`
	Status      string  `json:"status,omitempty"`
	BlockedReason string `json:"blocked_reason,omitempty"`
}

type Agent struct {
	ID              string     `json:"id"`
	DisplayName     string     `json:"display_name,omitempty"`
	RoleName        string     `json:"role_name,omitempty"`
	Avatar          string     `json:"avatar,omitempty"`
	Provider        string     `json:"provider,omitempty"`
	Engine          string     `json:"engine,omitempty"`
	Description     string     `json:"description,omitempty"`
	IsNativeClawn   bool       `json:"is_native_clawn"`
	Soul            string     `json:"soul"`
	Memory          string     `json:"memory"`
	Rules           string     `json:"rules"`
	CronSchedule    string     `json:"cron_schedule"`
	Active          bool       `json:"active"`
	Status          string     `json:"status"`
	LastHeartbeatAt *time.Time `json:"last_heartbeat_at,omitempty"`
	LastSeenAt      *time.Time `json:"last_seen_at,omitempty"`
	CurrentTaskID   *int       `json:"current_task_id,omitempty"`
	CurrentRunID    *int       `json:"current_run_id,omitempty"`
	CurrentActivity string     `json:"current_activity,omitempty"`
	HealthNote      string     `json:"health_note,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type AgentConnector struct {
	ID            int        `json:"id"`
	AgentID       string     `json:"agent_id"`
	ConnectorType string     `json:"connector_type"`
	TransportMode string     `json:"transport_mode,omitempty"`
	AuthType      string     `json:"auth_type"`
	EndpointURL   string     `json:"endpoint_url,omitempty"`
	BaseURL       string     `json:"base_url,omitempty"`
	AgentRef      string     `json:"agent_ref,omitempty"`
	SessionKey    string     `json:"session_key,omitempty"`
	Status        string     `json:"status"`
	MetadataJSON  string     `json:"metadata_json,omitempty"`
	LastSuccessAt *time.Time `json:"last_success_at,omitempty"`
	LastError     string     `json:"last_error,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type BoardAgent struct {
	ID                   int       `json:"id"`
	BoardID              string    `json:"board_id"`
	AgentID              string    `json:"agent_id"`
	BoardRole            string    `json:"board_role"`
	Active               bool      `json:"active"`
	AutoAcceptTasks      bool      `json:"auto_accept_tasks"`
	CanComment           bool      `json:"can_comment"`
	CanUpdateStatus      bool      `json:"can_update_status"`
	CanAccessDocs        bool      `json:"can_access_docs"`
	CanCreateDeliverables bool     `json:"can_create_deliverables"`
	CapabilitiesJSON     string    `json:"capabilities_json,omitempty"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
	Agent                *Agent    `json:"agent,omitempty"`
}

type AgentRun struct {
	ID             int        `json:"id"`
	TaskID         int        `json:"task_id"`
	AgentID        string     `json:"agent_id"`
	SessionKey     string     `json:"session_key"`
	ProviderType   string     `json:"provider_type"`
	Status         string     `json:"status"`
	StartedAt      time.Time  `json:"started_at"`
	EndedAt        *time.Time `json:"ended_at,omitempty"`
	CurrentActivity string    `json:"current_activity,omitempty"`
	ErrorSummary   string     `json:"error_summary,omitempty"`
	ResultSummary  string     `json:"result_summary,omitempty"`
}

type Notification struct {
	ID            int        `json:"id"`
	TaskID        *int       `json:"task_id,omitempty"`
	TargetAgentID string     `json:"target_agent_id"`
	SourceAgentID *string    `json:"source_agent_id,omitempty"`
	Type          string     `json:"type"`
	Content       string     `json:"content"`
	Delivered     bool       `json:"delivered"`
	DeliveredAt   *time.Time `json:"delivered_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

type AgentEvent struct {
	ID               int        `json:"id"`
	AgentID          string     `json:"agent_id"`
	BoardID          *string    `json:"board_id,omitempty"`
	TaskID           *int       `json:"task_id,omitempty"`
	EventType        string     `json:"event_type"`
	PayloadJSON      string     `json:"payload_json"`
	DeliveryStatus   string     `json:"delivery_status"`
	DeliveryAttempts int        `json:"delivery_attempts"`
	NextAttemptAt    *time.Time `json:"next_attempt_at,omitempty"`
	LastDeliveryAt   *time.Time `json:"last_delivery_at,omitempty"`
	ResponseStatus   string     `json:"response_status,omitempty"`
	ResponseBody     string     `json:"response_body,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	ProcessedAt      *time.Time `json:"processed_at,omitempty"`
}

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

type Activity struct {
	ID        int       `json:"id"`
	TaskID    int       `json:"task_id"`
	UserID    string    `json:"user_id"`
	Action    string    `json:"action"`
	Details   string    `json:"details"`
	CreatedAt time.Time `json:"created_at"`
}

type BoardMemberReq struct {
	MemberID string `json:"member_id"`
	Role     string `json:"role"`
}

type AgentRegisterReq struct {
	ID           string                 `json:"id"`
	DisplayName  string                 `json:"display_name"`
	RoleName     string                 `json:"role_name"`
	Avatar       string                 `json:"avatar"`
	Provider     string                 `json:"provider"`
	Engine       string                 `json:"engine"`
	Description  string                 `json:"description"`
	IsNativeClawn bool                  `json:"is_native_clawn"`
	Connector    AgentConnectorInput    `json:"connector"`
	Capabilities map[string]interface{} `json:"capabilities"`
}

type AgentConnectorInput struct {
	ConnectorType string                 `json:"connector_type"`
	TransportMode string                 `json:"transport_mode"`
	AuthType      string                 `json:"auth_type"`
	EndpointURL   string                 `json:"endpoint_url"`
	BaseURL       string                 `json:"base_url"`
	AgentRef      string                 `json:"agent_ref"`
	SessionKey    string                 `json:"session_key"`
	SharedSecret  string                 `json:"shared_secret"`
	Metadata      map[string]interface{} `json:"metadata"`
}

type AgentBoardConnectReq struct {
	BoardID         string                 `json:"board_id"`
	BoardRole       string                 `json:"board_role"`
	AutoAcceptTasks *bool                  `json:"auto_accept_tasks"`
	Permissions     map[string]interface{} `json:"permissions"`
}

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

type ClawnProjectSource struct {
	ProjectID        string   `json:"project_id"`
	DisplayName      string   `json:"display_name"`
	OwnerUserID      string   `json:"owner_user_id,omitempty"`
	Engine           string   `json:"engine,omitempty"`
	Plan             string   `json:"plan,omitempty"`
	Status           string   `json:"status,omitempty"`
	ContainerID      string   `json:"container_id,omitempty"`
	ContainerName    string   `json:"container_name,omitempty"`
	Capabilities     []string `json:"capabilities,omitempty"`
	IsConnectable    bool     `json:"is_connectable"`
	ConnectReason    string   `json:"connect_reason,omitempty"`
	AlreadyConnected bool     `json:"already_connected"`
	ExistingAgentID  *string  `json:"existing_agent_id,omitempty"`
}

type GeminiEmbeddingResponse struct {
	Embedding struct {
		Values []float32 `json:"values"`
	} `json:"embedding"`
}

func broadcastUpdate(msg string) {
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
	dbURL := fmt.Sprintf("postgres://%s:%s@%s:%s/%s",
		os.Getenv("DB_USER"), os.Getenv("DB_PASSWORD"), os.Getenv("DB_HOST"), os.Getenv("DB_PORT"), os.Getenv("DB_NAME"),
	)
	var err error
	db, err = pgxpool.New(context.Background(), dbURL)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v\n", err)
	}

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
	err = db.QueryRow(context.Background(), "SELECT id::text FROM boards WHERE title='Main Project' LIMIT 1").Scan(&defaultBoardID)
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
	initDB()
	initAI()

	rdb = redis.NewClient(&redis.Options{Addr: os.Getenv("REDIS_ADDR"), Password: os.Getenv("REDIS_PASSWORD"), DB: 0})

	go startDispatcher()

	app := fiber.New()
	app.Use(cors.New(cors.Config{AllowOrigins: "*", AllowHeaders: "Origin, Content-Type, Accept"}))

	app.Use("/ws", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			c.Locals("allowed", true)
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})

	app.Get("/ws", websocket.New(func(c *websocket.Conn) {
		clientsMu.Lock()
		clients[c] = true
		clientsMu.Unlock()
		defer func() { clientsMu.Lock(); delete(clients, c); clientsMu.Unlock(); c.Close() }()
		for {
			if _, _, err := c.ReadMessage(); err != nil {
				break
			}
		}
	}))

	app.Get("/api/health", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"status": "ok"}) })

	app.Get("/api/boards", getBoards)
	app.Post("/api/boards", createBoard)
	app.Get("/api/boards/:id/tasks", getBoardTasks)
	app.Get("/api/boards/:id/members", getBoardMembers)
	app.Post("/api/boards/:id/members", addBoardMember)
	app.Delete("/api/boards/:id/members/:mid", removeBoardMember)

	app.Post("/api/tasks", createTask)
	app.Put("/api/tasks/:id", updateTask)
	app.Get("/api/tasks/:id/activities", getTaskActivities)
	app.Get("/api/tasks/:id/runs", getTaskRuns)
	app.Get("/api/tasks/:id/deliverables", getTaskDeliverables)
	app.Post("/api/tasks/:id/deliverables", createDeliverable)
	app.Get("/api/search", searchTasks)
	app.Get("/api/members", getMembers)
	app.Get("/api/agents/:id", getAgentProfile)
	app.Get("/api/agents", getAgents)
	app.Post("/api/agents/register", registerAgent)
	app.Post("/api/agents/:id/connect-board", connectAgentToBoard)
	app.Get("/api/boards/:id/agents", getBoardAgents)
	app.Post("/api/runtime/heartbeat", runtimeHeartbeat)
	app.Post("/api/runtime/task-ack", runtimeTaskAck)
	app.Post("/api/runtime/task-update", runtimeTaskUpdate)
	app.Post("/api/runtime/task-comment", runtimeTaskComment)
	app.Post("/api/runtime/task-deliverable", runtimeTaskDeliverable)
	app.Post("/api/runtime/task-review-request", runtimeTaskReviewRequest)
	app.Get("/api/agents/:id/runs", getAgentRunsForAgent)
	app.Get("/api/agents/:id/notifications", getAgentNotifications)
	app.Get("/api/ops/agent-events", getAgentEventsOps)
	app.Post("/api/ops/agent-events/:id/retry", retryAgentEventOps)
	app.Post("/api/ops/agent-events/:id/requeue", requeueAgentEventOps)
	app.Get("/api/ops/connectors", getAgentConnectorsOps)
	app.Post("/api/ops/connectors/:id/enable", enableAgentConnectorOps)
	app.Post("/api/ops/connectors/:id/disable", disableAgentConnectorOps)
	app.Post("/api/agents/:id/heartbeat", heartbeatAgent)
	app.Put("/api/agents/:id/status", updateAgentStatus)
	app.Get("/api/agents/sync/aiagenz", getAiAgenzProjects)
	app.Get("/api/integrations/clawn/projects", listClawnProjectsFacade)

	// Auth Proxies
	app.Post("/api/auth/login", proxyAuthLogin)
	app.Get("/api/auth/me", proxyAuthMe)

	// Knowledge Base / Documents
	app.Get("/api/boards/:id/docs", getBoardDocs)
	app.Post("/api/boards/:id/docs", createDoc)
	app.Put("/api/docs/:id", updateDoc)
	app.Delete("/api/docs/:id", deleteDoc)
	app.Get("/api/docs/search", searchDocs)

	// Comments
	app.Get("/api/tasks/:id/comments", getTaskComments)
	app.Post("/api/tasks/:id/comments", createComment)

	log.Fatal(app.Listen(":8080"))
}

// --- Agent Dispatcher ---

func startDispatcher() {
	log.Println("Starting Go native agent dispatcher...")

	go func() {
		deliverPendingAgentEvents()
		pollAgentTasks()

		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			deliverPendingAgentEvents()
			pollAgentTasks()
		}
	}()
}

func pollAgentTasks() {
	query := `
		SELECT t.id, t.board_id::text, t.title, t.description, t.assignee_id
		FROM tasks t
		JOIN members m ON t.assignee_id = m.id
		JOIN board_agents ba ON ba.board_id = t.board_id AND ba.agent_id = t.assignee_id AND ba.active = true
		WHERE lower(t.list_id) = 'todo'
		  AND m.role = 'agent'
		  AND ba.auto_accept_tasks = true
	`
	rows, err := db.Query(context.Background(), query)
	if err != nil {
		log.Printf("Dispatcher check error: %v", err)
		return
	}
	defer rows.Close()

	var pendingTasks []Task
	for rows.Next() {
		var t Task
		if err := rows.Scan(&t.ID, &t.BoardID, &t.Title, &t.Description, &t.AssigneeID); err != nil {
			log.Printf("Dispatcher scan error: %v", err)
			continue
		}
		pendingTasks = append(pendingTasks, t)
	}

	if len(pendingTasks) > 0 {
		log.Printf("Found %d pending agent tasks to process.", len(pendingTasks))
		for _, t := range pendingTasks {
			go processAgentTask(t)
		}
	}
}

func processAgentTask(t Task) {
	if t.AssigneeID == nil {
		return
	}
	agentName := *t.AssigneeID
	log.Printf("Spawning session for %s to work on task %d...", agentName, t.ID)

	_, err := db.Exec(context.Background(),
		"UPDATE tasks SET list_id=$1 WHERE id=$2",
		"doing", t.ID)
	if err != nil {
		log.Printf("Dispatcher failed to mark task %d doing: %v", t.ID, err)
		return
	}

	go logActivity(t.ID, agentName, "moved", "Moved to list doing (Auto-dispatched)")
	go broadcastUpdate("UPDATE")

	message := fmt.Sprintf("MoziBoard task assignment. Task ID: %d\nTitle: %s\nDescription: %s\nPlease acknowledge, work the task, and report progress back through the connected runtime if available.", t.ID, t.Title, t.Description)
	respBody, err := sendAgentMessageViaGateway(agentName, "", message, fmt.Sprintf("moziboard:auto-dispatch:%s", agentName))
	if err != nil {
		log.Printf("OpenClaw gateway error for agent %s on task %d: %v", agentName, t.ID, err)
		return
	}
	log.Printf("OpenClaw gateway dispatched for agent %s on task %d. Response: %s", agentName, t.ID, truncateForLog(respBody, 600))
}

func normalizeTaskStatus(listID string) string {
	switch strings.ToLower(listID) {
	case "doing", "in_progress":
		return "in_progress"
	case "qa", "review":
		return "review"
	case "blocked":
		return "blocked"
	case "backlog":
		return "backlog"
	case "done":
		return "done"
	default:
		return "todo"
	}
}

func normalizeRunStatus(taskStatus string) string {
	switch strings.ToLower(strings.TrimSpace(taskStatus)) {
	case "todo", "assigned":
		return "queued"
	case "in_progress", "doing":
		return "running"
	case "review", "qa":
		return "review"
	case "blocked":
		return "blocked"
	case "done":
		return "done"
	default:
		return "queued"
	}
}

func syncTaskAndRunState(taskID int, agentID string, taskStatus string, currentActivity string, blockedReason string, resultSummary string) {
	listID := statusToListID(taskStatus)
	runStatus := normalizeRunStatus(taskStatus)
	_, _ = db.Exec(context.Background(), `UPDATE tasks SET status=$1, list_id=$2, blocked_reason=$3 WHERE id=$4`, taskStatus, listID, blockedReason, taskID)
	_, _ = db.Exec(context.Background(), `UPDATE agent_runs SET status=$1, current_activity=COALESCE(NULLIF($2,''), current_activity), error_summary=CASE WHEN $1='blocked' THEN $3 ELSE error_summary END, result_summary=CASE WHEN $4<>'' THEN $4 ELSE result_summary END WHERE task_id=$5 AND agent_id=$6 AND status IN ('queued','running','in_progress','blocked','review')`, runStatus, currentActivity, blockedReason, resultSummary, taskID, agentID)
}

func statusToListID(status string) string {
	switch strings.ToLower(status) {
	case "in_progress":
		return "doing"
	case "review":
		return "qa"
	default:
		return strings.ToLower(status)
	}
}

func createNotification(taskID *int, targetAgentID string, sourceAgentID *string, notifType, content string) {
	if targetAgentID == "" {
		return
	}
	db.Exec(context.Background(),
		"INSERT INTO notifications (task_id, target_agent_id, source_agent_id, type, content) VALUES ($1, $2, $3, $4, $5)",
		taskID, targetAgentID, sourceAgentID, notifType, content)
}

func getConnectorForAgent(agentID string) (*AgentConnector, error) {
	var conn AgentConnector
	err := db.QueryRow(context.Background(), `
		SELECT id, agent_id, connector_type, transport_mode, auth_type, endpoint_url, base_url, agent_ref, session_key, status, metadata_json::text, last_success_at, last_error, created_at, updated_at
		FROM agent_connectors
		WHERE agent_id=$1 AND status='connected'
		ORDER BY is_primary DESC, updated_at DESC, id DESC
		LIMIT 1`, agentID).
		Scan(&conn.ID, &conn.AgentID, &conn.ConnectorType, &conn.TransportMode, &conn.AuthType, &conn.EndpointURL, &conn.BaseURL, &conn.AgentRef, &conn.SessionKey, &conn.Status, &conn.MetadataJSON, &conn.LastSuccessAt, &conn.LastError, &conn.CreatedAt, &conn.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &conn, nil
}

func createAgentRun(taskID int, agentID string, status string, activity string) *int {
	conn, _ := getConnectorForAgent(agentID)
	providerType := "internal"
	sessionKey := fmt.Sprintf("mozi:%s:%d", agentID, taskID)
	if conn != nil {
		if conn.ConnectorType != "" { providerType = conn.ConnectorType }
		if conn.SessionKey != "" { sessionKey = conn.SessionKey }
	}
	var runID int
	err := db.QueryRow(context.Background(),
		"INSERT INTO agent_runs (task_id, agent_id, session_key, provider_type, status, current_activity) VALUES ($1, $2, $3, $4, $5, $6) RETURNING id",
		taskID, agentID, sessionKey, providerType, status, activity).Scan(&runID)
	if err != nil {
		log.Printf("failed to create agent run: %v", err)
		return nil
	}
	return &runID
}

func enqueueAgentEvent(agentID string, taskID int, eventType string, payload map[string]interface{}) *int {
	payloadBytes, _ := json.Marshal(payload)
	var eventID int
	var boardID string
	_ = db.QueryRow(context.Background(), `SELECT board_id::text FROM tasks WHERE id=$1`, taskID).Scan(&boardID)
	err := db.QueryRow(context.Background(), `INSERT INTO agent_events (agent_id, board_id, task_id, event_type, payload_json) VALUES ($1,$2,$3,$4,$5::jsonb) RETURNING id`, agentID, boardID, taskID, eventType, string(payloadBytes)).Scan(&eventID)
	if err != nil {
		log.Printf("failed to enqueue agent event: %v", err)
		return nil
	}
	go deliverPendingAgentEvents()
	return &eventID
}

func closeOtherRuns(taskID int, agentID string, keepRunID int) {
	_, _ = db.Exec(context.Background(), `UPDATE agent_runs SET status='done', ended_at=CURRENT_TIMESTAMP WHERE task_id=$1 AND agent_id=$2 AND id<>$3 AND status IN ('queued','running','in_progress','blocked','review')`, taskID, agentID, keepRunID)
}

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

func resolveRuntimeAgent(c *fiber.Ctx) (*AgentConnector, error) {
	token := extractBearerToken(c)
	if token == "" {
		return nil, fmt.Errorf("missing bearer token")
	}
	hash := hashToken(token)
	var conn AgentConnector
	err := db.QueryRow(context.Background(), `SELECT id, agent_id, connector_type, transport_mode, auth_type, endpoint_url, base_url, agent_ref, session_key, status, metadata_json::text, last_success_at, last_error, created_at, updated_at FROM agent_connectors WHERE machine_token_hash=$1`, hash).
		Scan(&conn.ID, &conn.AgentID, &conn.ConnectorType, &conn.TransportMode, &conn.AuthType, &conn.EndpointURL, &conn.BaseURL, &conn.AgentRef, &conn.SessionKey, &conn.Status, &conn.MetadataJSON, &conn.LastSuccessAt, &conn.LastError, &conn.CreatedAt, &conn.UpdatedAt)
	if err == nil {
		return &conn, nil
	}

	agentRef := strings.TrimSpace(c.Get("X-Mozi-Agent-Ref"))
	sharedSecret := strings.TrimSpace(c.Get("X-Mozi-Shared-Secret"))
	timestamp := strings.TrimSpace(c.Get("X-Mozi-Timestamp"))
	signature := strings.TrimSpace(c.Get("X-Mozi-Signature"))
	if agentRef == "" || sharedSecret == "" {
		return nil, err
	}
	var sharedSecretHash string
	err = db.QueryRow(context.Background(), `SELECT id, agent_id, connector_type, transport_mode, auth_type, endpoint_url, base_url, agent_ref, session_key, status, metadata_json::text, last_success_at, last_error, created_at, updated_at, shared_secret_hash FROM agent_connectors WHERE agent_ref=$1 AND auth_type='shared_secret' AND status='connected' ORDER BY updated_at DESC, id DESC LIMIT 1`, agentRef).
		Scan(&conn.ID, &conn.AgentID, &conn.ConnectorType, &conn.TransportMode, &conn.AuthType, &conn.EndpointURL, &conn.BaseURL, &conn.AgentRef, &conn.SessionKey, &conn.Status, &conn.MetadataJSON, &conn.LastSuccessAt, &conn.LastError, &conn.CreatedAt, &conn.UpdatedAt, &sharedSecretHash)
	if err != nil {
		return nil, err
	}
	if hashToken(sharedSecret) != sharedSecretHash {
		return nil, fmt.Errorf("invalid shared secret")
	}
	if timestamp != "" || signature != "" {
		if timestamp == "" || signature == "" {
			return nil, fmt.Errorf("incomplete signature headers")
		}
		parsedTs, parseErr := time.Parse(time.RFC3339, timestamp)
		if parseErr != nil {
			return nil, fmt.Errorf("invalid signature timestamp")
		}
		if time.Since(parsedTs) > 5*time.Minute || time.Until(parsedTs) > 1*time.Minute {
			return nil, fmt.Errorf("signature timestamp out of range")
		}
		body := string(c.Body())
		expected := signWebhookPayload(sharedSecret, timestamp, body)
		if !hmac.Equal([]byte(expected), []byte(signature)) {
			return nil, fmt.Errorf("invalid shared secret signature")
		}
	}
	return &conn, nil
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

func getBoardTasks(c *fiber.Ctx) error {
	boardID := c.Params("id")
	rows, err := db.Query(context.Background(), "SELECT id, board_id::text, title, description, list_id, position, assignee_id, parent_id, status, blocked_reason FROM tasks WHERE board_id=$1 ORDER BY position ASC", boardID)
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	defer rows.Close()
	var tasks []Task
	for rows.Next() {
		var t Task
		if err := rows.Scan(&t.ID, &t.BoardID, &t.Title, &t.Description, &t.ListID, &t.Position, &t.AssigneeID, &t.ParentID, &t.Status, &t.BlockedReason); err != nil {
			return c.Status(500).SendString(err.Error())
		}
		tasks = append(tasks, t)
	}
	if tasks == nil {
		tasks = []Task{}
	}
	return c.JSON(tasks)
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

func getAgents(c *fiber.Ctx) error {
	rows, err := db.Query(context.Background(), "SELECT id, display_name, role_name, avatar, provider, engine, description, is_native_clawn, soul, memory, rules, cron_schedule, active, status, last_heartbeat_at, last_seen_at, current_task_id, current_run_id, current_activity, health_note, created_at, updated_at FROM agents")
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	defer rows.Close()
	var agents []Agent
	for rows.Next() {
		var a Agent
		if err := rows.Scan(&a.ID, &a.DisplayName, &a.RoleName, &a.Avatar, &a.Provider, &a.Engine, &a.Description, &a.IsNativeClawn, &a.Soul, &a.Memory, &a.Rules, &a.CronSchedule, &a.Active, &a.Status, &a.LastHeartbeatAt, &a.LastSeenAt, &a.CurrentTaskID, &a.CurrentRunID, &a.CurrentActivity, &a.HealthNote, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return c.Status(500).SendString(err.Error())
		}
		agents = append(agents, a)
	}
	if agents == nil {
		agents = []Agent{}
	}
	return c.JSON(agents)
}

func getAgentProfile(c *fiber.Ctx) error {
	agentID := c.Params("id")
	var a Agent
	err := db.QueryRow(context.Background(),
		"SELECT id, display_name, role_name, avatar, provider, engine, description, is_native_clawn, soul, memory, rules, cron_schedule, active, status, last_heartbeat_at, last_seen_at, current_task_id, current_run_id, current_activity, health_note, created_at, updated_at FROM agents WHERE id=$1",
		agentID).Scan(&a.ID, &a.DisplayName, &a.RoleName, &a.Avatar, &a.Provider, &a.Engine, &a.Description, &a.IsNativeClawn, &a.Soul, &a.Memory, &a.Rules, &a.CronSchedule, &a.Active, &a.Status, &a.LastHeartbeatAt, &a.LastSeenAt, &a.CurrentTaskID, &a.CurrentRunID, &a.CurrentActivity, &a.HealthNote, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return c.Status(404).SendString("Agent not found")
	}
	return c.JSON(a)
}

func registerAgent(c *fiber.Ctx) error {
	req := new(AgentRegisterReq)
	if err := c.BodyParser(req); err != nil {
		return c.Status(400).SendString(err.Error())
	}
	if req.ID == "" || req.DisplayName == "" {
		return c.Status(400).JSON(fiber.Map{"error": "id and display_name are required"})
	}
	if req.RoleName == "" { req.RoleName = "Agent" }
	if req.Avatar == "" { req.Avatar = "🤖" }
	if req.Provider == "" { req.Provider = "external" }
	if req.Engine == "" { req.Engine = "custom" }
	if req.Connector.ConnectorType == "" { req.Connector.ConnectorType = "custom" }
	if req.Connector.AuthType == "" { req.Connector.AuthType = "machine_token" }
	if req.Connector.TransportMode == "" {
		if req.Connector.ConnectorType == "clawn_native" {
			req.Connector.TransportMode = "internal"
		} else {
			req.Connector.TransportMode = "push"
		}
	}
	if req.Capabilities == nil { req.Capabilities = map[string]interface{}{} }
	metaBytes, _ := json.Marshal(req.Connector.Metadata)
	capBytes, _ := json.Marshal(req.Capabilities)
	plainToken, tokenHash, err := generateMachineToken()
	if err != nil { return c.Status(500).JSON(fiber.Map{"error": err.Error()}) }
	sharedSecretHash := ""
	if strings.TrimSpace(req.Connector.SharedSecret) != "" {
		sharedSecretHash = hashToken(strings.TrimSpace(req.Connector.SharedSecret))
	}
	_, err = db.Exec(context.Background(), `INSERT INTO members (id, name, role, avatar) VALUES ($1,$2,'agent',$3) ON CONFLICT (id) DO UPDATE SET name=EXCLUDED.name, role='agent', avatar=EXCLUDED.avatar`, req.ID, req.DisplayName, req.Avatar)
	if err != nil { return c.Status(500).JSON(fiber.Map{"error": err.Error()}) }
	_, err = db.Exec(context.Background(), `INSERT INTO agents (id, display_name, role_name, avatar, provider, engine, description, is_native_clawn, soul, memory, rules, cron_schedule, active, status) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,'','', '', '*/10 * * * *', true, 'offline') ON CONFLICT (id) DO UPDATE SET display_name=EXCLUDED.display_name, role_name=EXCLUDED.role_name, avatar=EXCLUDED.avatar, provider=EXCLUDED.provider, engine=EXCLUDED.engine, description=EXCLUDED.description, is_native_clawn=EXCLUDED.is_native_clawn, updated_at=CURRENT_TIMESTAMP`, req.ID, req.DisplayName, req.RoleName, req.Avatar, req.Provider, req.Engine, req.Description, req.IsNativeClawn)
	if err != nil { return c.Status(500).JSON(fiber.Map{"error": err.Error()}) }
	var connectorID int
	err = db.QueryRow(context.Background(), `INSERT INTO agent_connectors (agent_id, connector_type, transport_mode, auth_type, machine_token_hash, endpoint_url, base_url, agent_ref, session_key, shared_secret_hash, status, metadata_json) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'connected',$11::jsonb) RETURNING id`, req.ID, req.Connector.ConnectorType, req.Connector.TransportMode, req.Connector.AuthType, tokenHash, req.Connector.EndpointURL, req.Connector.BaseURL, req.Connector.AgentRef, req.Connector.SessionKey, sharedSecretHash, string(metaBytes)).Scan(&connectorID)
	if err != nil { return c.Status(500).JSON(fiber.Map{"error": err.Error()}) }
	_ = capBytes
	return c.JSON(fiber.Map{"agent": fiber.Map{"id": req.ID, "display_name": req.DisplayName, "provider": req.Provider, "engine": req.Engine}, "connector": fiber.Map{"id": connectorID, "connector_type": req.Connector.ConnectorType, "transport_mode": req.Connector.TransportMode, "base_url": req.Connector.BaseURL, "agent_ref": req.Connector.AgentRef, "status": "connected"}, "machine_token": plainToken})
}

func connectAgentToBoard(c *fiber.Ctx) error {
	agentID := c.Params("id")
	req := new(AgentBoardConnectReq)
	if err := c.BodyParser(req); err != nil { return c.Status(400).SendString(err.Error()) }
	if req.BoardID == "" { return c.Status(400).JSON(fiber.Map{"error": "board_id is required"}) }
	if req.BoardRole == "" { req.BoardRole = "worker" }
	autoAccept := true
	if req.AutoAcceptTasks != nil { autoAccept = *req.AutoAcceptTasks }
	canComment, canUpdate, canDocs, canDeliv := true, true, true, true
	if req.Permissions != nil {
		if v, ok := req.Permissions["can_comment"].(bool); ok { canComment = v }
		if v, ok := req.Permissions["can_update_status"].(bool); ok { canUpdate = v }
		if v, ok := req.Permissions["can_access_docs"].(bool); ok { canDocs = v }
		if v, ok := req.Permissions["can_create_deliverables"].(bool); ok { canDeliv = v }
	}
	capBytes, _ := json.Marshal(req.Permissions)
	var id int
	err := db.QueryRow(context.Background(), `INSERT INTO board_agents (board_id, agent_id, board_role, active, auto_accept_tasks, can_comment, can_update_status, can_access_docs, can_create_deliverables, capabilities_json) VALUES ($1,$2,$3,true,$4,$5,$6,$7,$8,$9::jsonb) ON CONFLICT (board_id, agent_id) DO UPDATE SET board_role=EXCLUDED.board_role, active=true, auto_accept_tasks=EXCLUDED.auto_accept_tasks, can_comment=EXCLUDED.can_comment, can_update_status=EXCLUDED.can_update_status, can_access_docs=EXCLUDED.can_access_docs, can_create_deliverables=EXCLUDED.can_create_deliverables, capabilities_json=EXCLUDED.capabilities_json, updated_at=CURRENT_TIMESTAMP RETURNING id`, req.BoardID, agentID, req.BoardRole, autoAccept, canComment, canUpdate, canDocs, canDeliv, string(capBytes)).Scan(&id)
	if err != nil { return c.Status(500).JSON(fiber.Map{"error": err.Error()}) }
	_, _ = db.Exec(context.Background(), `INSERT INTO board_members (board_id, member_id, role) VALUES ($1,$2,'agent') ON CONFLICT DO NOTHING`, req.BoardID, agentID)
	return c.JSON(fiber.Map{"ok": true, "board_agent_id": id})
}

func getBoardAgents(c *fiber.Ctx) error {
	boardID := c.Params("id")
	rows, err := db.Query(context.Background(), `SELECT ba.id, ba.board_id::text, ba.agent_id, ba.board_role, ba.active, ba.auto_accept_tasks, ba.can_comment, ba.can_update_status, ba.can_access_docs, ba.can_create_deliverables, ba.capabilities_json::text, ba.created_at, ba.updated_at, a.display_name, a.role_name, a.avatar, a.provider, a.engine, a.description, a.is_native_clawn, a.status, a.last_heartbeat_at, a.last_seen_at, a.current_task_id, a.current_run_id, a.current_activity, a.health_note, a.soul, a.memory, a.rules, a.cron_schedule, a.active, a.created_at, a.updated_at FROM board_agents ba JOIN agents a ON a.id = ba.agent_id WHERE ba.board_id=$1 ORDER BY ba.created_at ASC`, boardID)
	if err != nil { return c.Status(500).SendString(err.Error()) }
	defer rows.Close()
	var result []BoardAgent
	for rows.Next() {
		var ba BoardAgent
		var ag Agent
		if err := rows.Scan(&ba.ID, &ba.BoardID, &ba.AgentID, &ba.BoardRole, &ba.Active, &ba.AutoAcceptTasks, &ba.CanComment, &ba.CanUpdateStatus, &ba.CanAccessDocs, &ba.CanCreateDeliverables, &ba.CapabilitiesJSON, &ba.CreatedAt, &ba.UpdatedAt, &ag.DisplayName, &ag.RoleName, &ag.Avatar, &ag.Provider, &ag.Engine, &ag.Description, &ag.IsNativeClawn, &ag.Status, &ag.LastHeartbeatAt, &ag.LastSeenAt, &ag.CurrentTaskID, &ag.CurrentRunID, &ag.CurrentActivity, &ag.HealthNote, &ag.Soul, &ag.Memory, &ag.Rules, &ag.CronSchedule, &ag.Active, &ag.CreatedAt, &ag.UpdatedAt); err != nil {
			return c.Status(500).SendString(err.Error())
		}
		ag.ID = ba.AgentID
		ba.Agent = &ag
		result = append(result, ba)
	}
	if result == nil { result = []BoardAgent{} }
	return c.JSON(result)
}

type RuntimeTaskRef struct {
	TaskID int `json:"task_id"`
}

type RuntimeHeartbeatReq struct {
	Status string `json:"status"`
	CurrentActivity string `json:"current_activity"`
	CurrentTaskID *int `json:"current_task_id"`
}

type RuntimeTaskAckReq struct {
	TaskID int `json:"task_id"`
	Message string `json:"message"`
}

type RuntimeTaskUpdateReq struct {
	TaskID int `json:"task_id"`
	Status string `json:"status"`
	ProgressMessage string `json:"progress_message"`
	CurrentActivity string `json:"current_activity"`
	BlockedReason string `json:"blocked_reason"`
}

type RuntimeTaskCommentReq struct {
	TaskID int `json:"task_id"`
	Content string `json:"content"`
	MessageType string `json:"message_type"`
}

type RuntimeTaskDeliverableReq struct {
	TaskID int `json:"task_id"`
	Title string `json:"title"`
	ArtifactType string `json:"artifact_type"`
	Content string `json:"content"`
	Summary string `json:"summary"`
}

type RuntimeTaskReviewReq struct {
	TaskID int `json:"task_id"`
	Summary string `json:"summary"`
}

func ensureAgentBoardAccess(agentID string, taskID int) error {
	var ok bool
	err := db.QueryRow(context.Background(), `SELECT EXISTS(SELECT 1 FROM tasks t JOIN board_agents ba ON ba.board_id = t.board_id AND ba.agent_id=$1 AND ba.active=true WHERE t.id=$2)`, agentID, taskID).Scan(&ok)
	if err != nil { return err }
	if !ok { return fmt.Errorf("agent not connected to task board") }
	return nil
}

func ensureActiveRun(taskID int, agentID string) (int, error) {
	rows, err := db.Query(context.Background(), `SELECT id FROM agent_runs WHERE task_id=$1 AND agent_id=$2 AND status IN ('queued','running','in_progress','blocked','review') ORDER BY started_at DESC`, taskID, agentID)
	if err != nil { return 0, err }
	defer rows.Close()
	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err == nil { ids = append(ids, id) }
	}
	if len(ids) > 0 {
		keep := ids[0]
		closeOtherRuns(taskID, agentID, keep)
		return keep, nil
	}
	created := createAgentRun(taskID, agentID, "running", "Started work")
	if created == nil { return 0, fmt.Errorf("failed to create run") }
	return *created, nil
}

func runtimeHeartbeat(c *fiber.Ctx) error {
	conn, err := resolveRuntimeAgent(c)
	if err != nil { return c.Status(401).JSON(fiber.Map{"error": err.Error()}) }
	var req RuntimeHeartbeatReq
	if err := c.BodyParser(&req); err != nil { return c.Status(400).SendString(err.Error()) }
	if req.Status == "" { req.Status = "online" }
	_, err = db.Exec(context.Background(), `UPDATE agents SET status=$1, current_activity=$2, current_task_id=COALESCE($3, current_task_id), last_heartbeat_at=CURRENT_TIMESTAMP, last_seen_at=CURRENT_TIMESTAMP, updated_at=CURRENT_TIMESTAMP WHERE id=$4`, req.Status, req.CurrentActivity, req.CurrentTaskID, conn.AgentID)
	if err != nil { return c.Status(500).JSON(fiber.Map{"error": err.Error()}) }
	_, _ = db.Exec(context.Background(), `UPDATE agent_connectors SET last_success_at=CURRENT_TIMESTAMP, status='connected', updated_at=CURRENT_TIMESTAMP WHERE id=$1`, conn.ID)
	return c.JSON(fiber.Map{"ok": true, "agent_id": conn.AgentID})
}

func runtimeTaskAck(c *fiber.Ctx) error {
	conn, err := resolveRuntimeAgent(c)
	if err != nil { return c.Status(401).JSON(fiber.Map{"error": err.Error()}) }
	var req RuntimeTaskAckReq
	if err := c.BodyParser(&req); err != nil { return c.Status(400).SendString(err.Error()) }
	if err := ensureAgentBoardAccess(conn.AgentID, req.TaskID); err != nil { return c.Status(403).JSON(fiber.Map{"error": err.Error()}) }
	runID, err := ensureActiveRun(req.TaskID, conn.AgentID)
	if err != nil { return c.Status(500).JSON(fiber.Map{"error": err.Error()}) }
	msg := req.Message
	if msg == "" { msg = "Task accepted, starting work." }
	_, _ = db.Exec(context.Background(), `INSERT INTO comments (task_id, user_id, content) VALUES ($1,$2,$3)`, req.TaskID, conn.AgentID, msg)
	syncTaskAndRunState(req.TaskID, conn.AgentID, "in_progress", "Working on task", "", "")
	_, _ = db.Exec(context.Background(), `UPDATE agents SET status='busy', current_task_id=$1, current_run_id=$2, current_activity='Working on task', last_seen_at=CURRENT_TIMESTAMP, updated_at=CURRENT_TIMESTAMP WHERE id=$3`, req.TaskID, runID, conn.AgentID)
	go logActivity(req.TaskID, conn.AgentID, "acknowledged", msg)
	go broadcastUpdate("UPDATE")
	return c.JSON(fiber.Map{"ok": true, "run_id": runID})
}

func runtimeTaskUpdate(c *fiber.Ctx) error {
	conn, err := resolveRuntimeAgent(c)
	if err != nil { return c.Status(401).JSON(fiber.Map{"error": err.Error()}) }
	var req RuntimeTaskUpdateReq
	if err := c.BodyParser(&req); err != nil { return c.Status(400).SendString(err.Error()) }
	if err := ensureAgentBoardAccess(conn.AgentID, req.TaskID); err != nil { return c.Status(403).JSON(fiber.Map{"error": err.Error()}) }
	if req.Status == "" { req.Status = "in_progress" }
	listID := statusToListID(req.Status)
	runID, err := ensureActiveRun(req.TaskID, conn.AgentID)
	if err != nil { return c.Status(500).JSON(fiber.Map{"error": err.Error()}) }
	_, err = db.Exec(context.Background(), `UPDATE tasks SET status=$1, list_id=$2, blocked_reason=$3 WHERE id=$4`, req.Status, listID, req.BlockedReason, req.TaskID)
	if err != nil { return c.Status(500).JSON(fiber.Map{"error": err.Error()}) }
	if req.ProgressMessage != "" { _, _ = db.Exec(context.Background(), `INSERT INTO comments (task_id, user_id, content) VALUES ($1,$2,$3)`, req.TaskID, conn.AgentID, req.ProgressMessage) }
	syncTaskAndRunState(req.TaskID, conn.AgentID, req.Status, req.CurrentActivity, req.BlockedReason, "")
	agentStatus := "busy"
	if req.Status == "blocked" { agentStatus = "blocked" }
	_, _ = db.Exec(context.Background(), `UPDATE agents SET status=$1, current_task_id=$2, current_run_id=$3, current_activity=$4, health_note=$5, last_seen_at=CURRENT_TIMESTAMP, updated_at=CURRENT_TIMESTAMP WHERE id=$6`, agentStatus, req.TaskID, runID, req.CurrentActivity, req.BlockedReason, conn.AgentID)
	if req.Status == "blocked" { createNotification(&req.TaskID, conn.AgentID, nil, "blocked", req.BlockedReason) }
	go logActivity(req.TaskID, conn.AgentID, "runtime_update", fmt.Sprintf("Status updated to %s", req.Status))
	go broadcastUpdate("UPDATE")
	return c.JSON(fiber.Map{"ok": true, "run_id": runID})
}

func runtimeTaskComment(c *fiber.Ctx) error {
	conn, err := resolveRuntimeAgent(c)
	if err != nil { return c.Status(401).JSON(fiber.Map{"error": err.Error()}) }
	var req RuntimeTaskCommentReq
	if err := c.BodyParser(&req); err != nil { return c.Status(400).SendString(err.Error()) }
	if err := ensureAgentBoardAccess(conn.AgentID, req.TaskID); err != nil { return c.Status(403).JSON(fiber.Map{"error": err.Error()}) }
	_, err = db.Exec(context.Background(), `INSERT INTO comments (task_id, user_id, content) VALUES ($1,$2,$3)`, req.TaskID, conn.AgentID, req.Content)
	if err != nil { return c.Status(500).JSON(fiber.Map{"error": err.Error()}) }
	go logActivity(req.TaskID, conn.AgentID, "commented", req.Content)
	go broadcastUpdate("UPDATE")
	return c.JSON(fiber.Map{"ok": true})
}

func runtimeTaskDeliverable(c *fiber.Ctx) error {
	conn, err := resolveRuntimeAgent(c)
	if err != nil { return c.Status(401).JSON(fiber.Map{"error": err.Error()}) }
	var req RuntimeTaskDeliverableReq
	if err := c.BodyParser(&req); err != nil { return c.Status(400).SendString(err.Error()) }
	if err := ensureAgentBoardAccess(conn.AgentID, req.TaskID); err != nil { return c.Status(403).JSON(fiber.Map{"error": err.Error()}) }
	_, err = db.Exec(context.Background(), `INSERT INTO deliverables (task_id, title, description, artifact_type, content) VALUES ($1,$2,$3,$4,$5)`, req.TaskID, req.Title, req.Summary, req.ArtifactType, req.Content)
	if err != nil { return c.Status(500).JSON(fiber.Map{"error": err.Error()}) }
	runID, _ := ensureActiveRun(req.TaskID, conn.AgentID)
	_, _ = db.Exec(context.Background(), `UPDATE agent_runs SET result_summary=$1 WHERE id=$2`, req.Summary, runID)
	go logActivity(req.TaskID, conn.AgentID, "deliverable", fmt.Sprintf("Submitted deliverable: %s", req.Title))
	go broadcastUpdate("UPDATE")
	return c.JSON(fiber.Map{"ok": true, "run_id": runID})
}

func runtimeTaskReviewRequest(c *fiber.Ctx) error {
	conn, err := resolveRuntimeAgent(c)
	if err != nil { return c.Status(401).JSON(fiber.Map{"error": err.Error()}) }
	var req RuntimeTaskReviewReq
	if err := c.BodyParser(&req); err != nil { return c.Status(400).SendString(err.Error()) }
	if err := ensureAgentBoardAccess(conn.AgentID, req.TaskID); err != nil { return c.Status(403).JSON(fiber.Map{"error": err.Error()}) }
	runID, _ := ensureActiveRun(req.TaskID, conn.AgentID)
	syncTaskAndRunState(req.TaskID, conn.AgentID, "review", "Waiting for review", "", req.Summary)
	_, _ = db.Exec(context.Background(), `UPDATE agents SET status='online', current_activity='Waiting for review', last_seen_at=CURRENT_TIMESTAMP, updated_at=CURRENT_TIMESTAMP WHERE id=$1`, conn.AgentID)
	if req.Summary != "" { _, _ = db.Exec(context.Background(), `INSERT INTO comments (task_id, user_id, content) VALUES ($1,$2,$3)`, req.TaskID, conn.AgentID, req.Summary) }
	go logActivity(req.TaskID, conn.AgentID, "review_requested", req.Summary)
	go broadcastUpdate("UPDATE")
	return c.JSON(fiber.Map{"ok": true, "run_id": runID})
}

func deliverPendingAgentEvents() {
	_, _ = db.Exec(context.Background(), `
		UPDATE agent_events
		SET delivery_status='pending', response_status='processing_timeout_requeued', next_attempt_at=CURRENT_TIMESTAMP + INTERVAL '30 seconds'
		WHERE delivery_status='processing'
		  AND processed_at IS NULL
		  AND last_delivery_at < CURRENT_TIMESTAMP - INTERVAL '60 seconds'`)

	rows, err := db.Query(context.Background(), `
		WITH claimed AS (
			SELECT id
			FROM agent_events
			WHERE delivery_status IN ('pending','failed')
			  AND (next_attempt_at IS NULL OR next_attempt_at <= CURRENT_TIMESTAMP)
			  AND delivery_attempts < 8
			ORDER BY created_at ASC
			LIMIT 20
			FOR UPDATE SKIP LOCKED
		)
		UPDATE agent_events ae
		SET delivery_status='processing', last_delivery_at=CURRENT_TIMESTAMP
		FROM claimed
		WHERE ae.id = claimed.id
		RETURNING ae.id, ae.agent_id, ae.board_id::text, ae.task_id, ae.event_type, ae.payload_json::text, ae.delivery_status, ae.delivery_attempts, ae.last_delivery_at, ae.response_status, ae.response_body, ae.created_at, ae.processed_at`)
	if err == nil {
		_, _ = db.Exec(context.Background(), `
			UPDATE agent_events
			SET delivery_status='dead', response_status='retry_exhausted', processed_at=CURRENT_TIMESTAMP
			WHERE delivery_status IN ('pending','failed')
			  AND delivery_attempts >= 8
			  AND processed_at IS NULL`)
	}
	if err != nil {
		log.Printf("event query failed: %v", err)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var ev AgentEvent
		if err := rows.Scan(&ev.ID, &ev.AgentID, &ev.BoardID, &ev.TaskID, &ev.EventType, &ev.PayloadJSON, &ev.DeliveryStatus, &ev.DeliveryAttempts, &ev.LastDeliveryAt, &ev.ResponseStatus, &ev.ResponseBody, &ev.CreatedAt, &ev.ProcessedAt); err != nil {
			log.Printf("event scan failed: %v", err)
			continue
		}

		select {
		case deliverySemaphore <- struct{}{}:
			go func(ev AgentEvent) {
				defer func() { <-deliverySemaphore }()
				defer func() {
					if r := recover(); r != nil {
						markEventFailed(ev, fmt.Sprintf("panic:%v", r), "")
					}
				}()
				processAgentEventDelivery(ev)
			}(ev)
		default:
			markEventFailed(ev, "dispatcher_backpressure", "")
		}
	}
}

func processAgentEventDelivery(ev AgentEvent) {
	conn, err := getConnectorForAgent(ev.AgentID)
	if err != nil || conn == nil {
		markEventFailed(ev, "connector_missing", "")
		return
	}

	if conn.ConnectorType == "webhook" {
		if conn.EndpointURL == "" {
			markEventFailed(ev, "webhook_missing_endpoint", "")
			return
		}
		deliverEventHTTPRequest(ev, conn, conn.EndpointURL, ev.PayloadJSON, map[string]string{"Content-Type": "application/json"}, "webhook")
		return
	}

	if conn.ConnectorType == "clawn_native" {
		envelopeBytes, _ := json.Marshal(fiber.Map{
			"event_id": ev.ID,
			"event_type": ev.EventType,
			"agent_id": ev.AgentID,
			"board_id": ev.BoardID,
			"task_id": ev.TaskID,
			"transport_mode": conn.TransportMode,
			"agent_ref": conn.AgentRef,
			"session_key": conn.SessionKey,
			"payload": json.RawMessage(ev.PayloadJSON),
		})

		switch conn.TransportMode {
		case "internal":
			if strings.TrimSpace(conn.SessionKey) == "" {
				markEventFailed(ev, "internal_missing_session_key", "")
				return
			}
			if err := deliverInternalClawnEvent(ev, conn, string(envelopeBytes)); err != nil {
				markEventFailed(ev, err.Error(), fmt.Sprintf(`{"strategy":"internal","agent_ref":%q,"session_key":%q}`, conn.AgentRef, conn.SessionKey))
				return
			}
			return
		case "remote_http":
			if conn.BaseURL == "" {
				markEventFailed(ev, "remote_http_missing_base_url", "")
				return
			}
			targetURL := strings.TrimRight(conn.BaseURL, "/") + "/api/runtime/events"
			headers := map[string]string{"Content-Type": "application/json", "X-Mozi-Agent-Ref": conn.AgentRef, "X-Mozi-Session-Key": conn.SessionKey}
			deliverEventHTTPRequest(ev, conn, targetURL, string(envelopeBytes), headers, "clawn_remote_http")
			return
		case "pull":
			responseBody := fmt.Sprintf(`{"strategy":"pull","agent_ref":%q}`, conn.AgentRef)
			_, _ = db.Exec(context.Background(), `UPDATE agent_events SET delivery_status='routed', delivery_attempts=delivery_attempts+1, response_status='pull_runtime_pending', response_body=$2, processed_at=CURRENT_TIMESTAMP, last_delivery_at=CURRENT_TIMESTAMP WHERE id=$1`, ev.ID, responseBody)
			return
		default:
			markEventFailed(ev, "unsupported_transport_mode:"+conn.TransportMode, "")
			return
		}
	}

	_, _ = db.Exec(context.Background(), `UPDATE agent_events SET delivery_status='ignored', response_status=$2, processed_at=CURRENT_TIMESTAMP, last_delivery_at=CURRENT_TIMESTAMP WHERE id=$1`, ev.ID, conn.ConnectorType)
}

func nextRetryDelay(attempts int) time.Duration {
	switch {
	case attempts <= 1:
		return 10 * time.Second
	case attempts == 2:
		return 30 * time.Second
	case attempts == 3:
		return 60 * time.Second
	case attempts <= 5:
		return 5 * time.Minute
	default:
		return 15 * time.Minute
	}
}

func markEventFailed(ev AgentEvent, responseStatus string, responseBody string) {
	nextAttempt := time.Now().Add(nextRetryDelay(ev.DeliveryAttempts + 1))
	status := "failed"
	processedAt := "NULL"
	if ev.DeliveryAttempts+1 >= 8 {
		status = "dead"
		processedAt = "CURRENT_TIMESTAMP"
	}
	query := fmt.Sprintf(`UPDATE agent_events SET delivery_status=$2, delivery_attempts=delivery_attempts+1, response_status=$3, response_body=$4, next_attempt_at=$5, last_delivery_at=CURRENT_TIMESTAMP, processed_at=%s WHERE id=$1`, processedAt)
	_, _ = db.Exec(context.Background(), query, ev.ID, status, responseStatus, responseBody, nextAttempt)
}

func deliverEventHTTPRequest(ev AgentEvent, conn *AgentConnector, targetURL string, body string, headers map[string]string, strategy string) {
	timestamp := time.Now().UTC().Format(time.RFC3339)
	req, err := http.NewRequest("POST", targetURL, strings.NewReader(body))
	if err != nil {
		markEventFailed(ev, err.Error(), "")
		return
	}
	for k, v := range headers {
		if strings.TrimSpace(v) != "" {
			req.Header.Set(k, v)
		}
	}
	req.Header.Set("X-Mozi-Timestamp", timestamp)
	if strings.TrimSpace(conn.AgentRef) != "" {
		req.Header.Set("X-Mozi-Agent-Ref", strings.TrimSpace(conn.AgentRef))
	}
	if strings.TrimSpace(conn.AuthType) == "shared_secret" && strings.TrimSpace(conn.MetadataJSON) != "" {
		var meta map[string]interface{}
		if json.Unmarshal([]byte(conn.MetadataJSON), &meta) == nil {
			if plainSecret, ok := meta["shared_secret"].(string); ok && strings.TrimSpace(plainSecret) != "" {
				req.Header.Set("X-Mozi-Signature", signWebhookPayload(plainSecret, timestamp, body))
			}
		}
	}
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		markEventFailed(ev, err.Error(), fmt.Sprintf(`{"strategy":%q,"target_url":%q}`, strategy, targetURL))
		return
	}
	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1000))
	resp.Body.Close()
	status := "failed"
	processedAt := "NULL"
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		status = "sent"
		processedAt = "CURRENT_TIMESTAMP"
	}
	responseBody := fmt.Sprintf(`{"strategy":%q,"target_url":%q,"response_body":%q}`, strategy, targetURL, string(bodyBytes))
	query := fmt.Sprintf(`UPDATE agent_events SET delivery_status=$2, delivery_attempts=delivery_attempts+1, last_delivery_at=CURRENT_TIMESTAMP, response_status=$3, response_body=$4, processed_at=%s WHERE id=$1`, processedAt)
	_, _ = db.Exec(context.Background(), query, ev.ID, status, resp.Status, responseBody)
}

func deliverInternalClawnEvent(ev AgentEvent, conn *AgentConnector, envelope string) error {
	message := buildInternalBridgeMessage(ev, conn, envelope)
	respBody, err := sendAgentMessageViaGateway(conn.AgentRef, conn.SessionKey, message, conn.SessionKey)
	if err != nil {
		return err
	}
	_, _ = db.Exec(context.Background(), `UPDATE agent_events SET delivery_status='sent', delivery_attempts=delivery_attempts+1, response_status='internal_runtime_sent', response_body=$2, processed_at=CURRENT_TIMESTAMP, last_delivery_at=CURRENT_TIMESTAMP WHERE id=$1`, ev.ID, fmt.Sprintf(`{"strategy":"internal_gateway","agent_ref":%q,"session_key":%q,"response":%q}`, conn.AgentRef, conn.SessionKey, truncateForLog(respBody, 1200)))
	return nil
}

func buildInternalBridgeMessage(ev AgentEvent, conn *AgentConnector, envelope string) string {
	return fmt.Sprintf("MoziBoard internal bridge event. Connector: clawn_native/internal. AgentRef: %s. SessionKey: %s. Event JSON:\n%s", conn.AgentRef, conn.SessionKey, envelope)
}

func sendAgentMessageViaGateway(agentID string, sessionKey string, message string, userKey string) (string, error) {
	gatewayURL := strings.TrimSpace(os.Getenv("OPENCLAW_GATEWAY_URL"))
	if gatewayURL == "" {
		gatewayURL = "http://host.docker.internal:18789/v1/chat/completions"
	}
	gatewayToken := strings.TrimSpace(os.Getenv("OPENCLAW_GATEWAY_TOKEN"))
	if gatewayToken == "" {
		return "", fmt.Errorf("gateway_missing_token")
	}
	if strings.TrimSpace(agentID) == "" {
		return "", fmt.Errorf("gateway_missing_agent_id")
	}
	payload := map[string]interface{}{
		"model":    fmt.Sprintf("openclaw:%s", agentID),
		"messages": []map[string]string{{"role": "user", "content": message}},
		"stream":   false,
		"user":     firstNonEmpty(strings.TrimSpace(userKey), fmt.Sprintf("moziboard:%s", agentID)),
	}
	body, _ := json.Marshal(payload)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "POST", gatewayURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("gateway_request_build_failed: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+gatewayToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-openclaw-agent-id", agentID)
	if strings.TrimSpace(sessionKey) != "" {
		req.Header.Set("x-openclaw-session-key", sessionKey)
	}
	resp, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		return "", fmt.Errorf("gateway_request_failed: %w", err)
	}
	defer resp.Body.Close()
	respBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("gateway_http_%d: %s", resp.StatusCode, strings.TrimSpace(string(respBytes)))
	}
	return strings.TrimSpace(string(respBytes)), nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func truncateForLog(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func interfaceToString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case fmt.Stringer:
		return strings.TrimSpace(t.String())
	case float64:
		return strings.TrimSpace(strconv.FormatFloat(t, 'f', -1, 64))
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	case json.Number:
		return t.String()
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", v))
	}
}

func interfaceToStringSlice(v interface{}) []string {
	switch t := v.(type) {
	case []string:
		return t
	case []interface{}:
		out := make([]string, 0, len(t))
		for _, item := range t {
			s := interfaceToString(item)
			if s != "" && s != "<nil>" {
				out = append(out, s)
			}
		}
		return out
	default:
		return []string{}
	}
}

func buildAiAgenzAuthHeaders(c *fiber.Ctx, req *http.Request) bool {
	authHeader := strings.TrimSpace(c.Get("Authorization"))
	authCookie := strings.TrimSpace(c.Get("Cookie"))
	if authHeader == "" && authCookie == "" {
		return false
	}
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	if authCookie != "" {
		req.Header.Set("Cookie", authCookie)
	}
	return true
}

func fetchAiAgenzProjects(c *fiber.Ctx) ([]map[string]interface{}, int, error) {
	proxyURL := "http://172.17.0.1:4001/api/projects"
	req, err := http.NewRequest("GET", proxyURL, nil)
	if err != nil {
		return nil, 500, fmt.Errorf("failed to create proxy request")
	}
	if !buildAiAgenzAuthHeaders(c, req) {
		return nil, 401, fmt.Errorf("unauthorized: missing authentication header or cookie")
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 502, fmt.Errorf("failed to connect to AiAgenz backend: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2000))
		return nil, resp.StatusCode, fmt.Errorf("AiAgenz API returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var payload struct {
		Data []map[string]interface{} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, 500, fmt.Errorf("failed to parse AiAgenz response")
	}
	if payload.Data == nil {
		payload.Data = []map[string]interface{}{}
	}
	return payload.Data, 200, nil
}

func normalizeClawnProjectSource(item map[string]interface{}) ClawnProjectSource {
	projectID := firstNonEmpty(interfaceToString(item["project_id"]), interfaceToString(item["id"]))
	displayName := firstNonEmpty(interfaceToString(item["display_name"]), interfaceToString(item["name"]), projectID)
	status := firstNonEmpty(interfaceToString(item["status"]), "unknown")
	engine := firstNonEmpty(interfaceToString(item["engine"]), "openclaw")
	caps := interfaceToStringSlice(item["capabilities"])
	isConnectable := projectID != ""
	connectReason := "project_runtime_available"
	if projectID == "" {
		isConnectable = false
		connectReason = "missing_project_id"
	} else if status == "failed" || status == "deleted" {
		isConnectable = false
		connectReason = "project_not_available"
	}

	return ClawnProjectSource{
		ProjectID:     projectID,
		DisplayName:   displayName,
		OwnerUserID:   interfaceToString(item["userId"]),
		Engine:        engine,
		Plan:          interfaceToString(item["plan"]),
		Status:        status,
		ContainerID:   interfaceToString(item["containerId"]),
		ContainerName: interfaceToString(item["containerName"]),
		Capabilities:  caps,
		IsConnectable: isConnectable,
		ConnectReason: connectReason,
	}
}

func listClawnProjectsFacade(c *fiber.Ctx) error {
	boardID := strings.TrimSpace(c.Query("board_id"))
	rawProjects, statusCode, err := fetchAiAgenzProjects(c)
	if err != nil {
		return c.Status(statusCode).JSON(fiber.Map{"error": err.Error()})
	}

	existingByProjectID := map[string]string{}
	if boardID != "" {
		rows, err := db.Query(context.Background(), `
			SELECT ba.agent_id, ac.metadata_json::text
			FROM board_agents ba
			JOIN agent_connectors ac ON ac.agent_id = ba.agent_id
			WHERE ba.board_id = $1
			  AND ac.connector_type = 'clawn_native'`, boardID)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var agentID, metaText string
				if err := rows.Scan(&agentID, &metaText); err != nil {
					continue
				}
				if strings.TrimSpace(metaText) == "" {
					continue
				}
				var meta map[string]interface{}
				if json.Unmarshal([]byte(metaText), &meta) != nil {
					continue
				}
				projectID := interfaceToString(meta["clawn_project_id"])
				if projectID != "" {
					existingByProjectID[projectID] = agentID
				}
			}
		}
	}

	items := make([]ClawnProjectSource, 0, len(rawProjects))
	for _, raw := range rawProjects {
		item := normalizeClawnProjectSource(raw)
		if existingAgentID, ok := existingByProjectID[item.ProjectID]; ok {
			item.AlreadyConnected = true
			item.ExistingAgentID = &existingAgentID
		}
		items = append(items, item)
	}
	return c.JSON(items)
}

// getAiAgenzProjects proxies a request to the main AiAgenz backend to fetch the user's projects (agents).
func getAiAgenzProjects(c *fiber.Ctx) error {
	items, statusCode, err := fetchAiAgenzProjects(c)
	if err != nil {
		return c.Status(statusCode).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"data": items})
}

// proxyAuthLogin forwards login requests to AiAgenz
func proxyAuthLogin(c *fiber.Ctx) error {
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

	// Forward the Set-Cookie header so the frontend gets the session
	for _, cookie := range resp.Header["Set-Cookie"] {
		c.Append("Set-Cookie", cookie)
	}

	var data interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to parse AiAgenz response"})
	}

	return c.Status(resp.StatusCode).JSON(data)
}

// proxyAuthMe forwards me asks to AiAgenz to check session validity
func proxyAuthMe(c *fiber.Ctx) error {
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

func createTask(c *fiber.Ctx) error {
	t := new(Task)
	if err := c.BodyParser(t); err != nil {
		return c.Status(400).SendString(err.Error())
	}
	if t.BoardID == "" {
		var defaultID string
		db.QueryRow(context.Background(), "SELECT id::text FROM boards LIMIT 1").Scan(&defaultID)
		if defaultID == "" {
			return c.Status(500).SendString("No board found")
		}
		t.BoardID = defaultID
	}
	if t.Title == "" {
		return c.Status(400).SendString("Title is required")
	}
	if t.ListID == "" {
		t.ListID = "todo"
	}
	t.Status = normalizeTaskStatus(t.ListID)
	t.ListID = statusToListID(t.Status)

	var id int
	err := db.QueryRow(context.Background(),
		"INSERT INTO tasks (board_id, title, description, list_id, position, assignee_id, parent_id, status, blocked_reason) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING id",
		t.BoardID, t.Title, t.Description, t.ListID, t.Position, t.AssigneeID, t.ParentID, t.Status, t.BlockedReason).Scan(&id)
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	t.ID = id

	if t.AssigneeID != nil && *t.AssigneeID != "" {
		createNotification(&id, *t.AssigneeID, nil, "task_assigned", fmt.Sprintf("New task assigned: %s", t.Title))
		if runID := createAgentRun(id, *t.AssigneeID, "queued", "Awaiting pickup"); runID != nil {
			db.Exec(context.Background(), "UPDATE agents SET current_task_id=$1, current_run_id=$2, status='busy', current_activity=$3, last_seen_at=CURRENT_TIMESTAMP, updated_at=CURRENT_TIMESTAMP WHERE id=$4", id, *runID, "Queued on task", *t.AssigneeID)
		}
		enqueueAgentEvent(*t.AssigneeID, id, "task.assigned", map[string]interface{}{"task_id": id, "title": t.Title, "status": t.Status, "board_id": t.BoardID})
	}

	go logActivity(id, "system", "created", fmt.Sprintf("Task created with status %s", t.Status))
	go updateEmbedding(id, t.Title+" "+t.Description)
	go broadcastUpdate("UPDATE")
	return c.JSON(t)
}

func updateTask(c *fiber.Ctx) error {
	id, _ := strconv.Atoi(c.Params("id"))

	var oldTask Task
	err := db.QueryRow(context.Background(),
		"SELECT id, board_id::text, title, description, list_id, position, assignee_id, parent_id, status, blocked_reason FROM tasks WHERE id=$1",
		id).Scan(&oldTask.ID, &oldTask.BoardID, &oldTask.Title, &oldTask.Description, &oldTask.ListID, &oldTask.Position, &oldTask.AssigneeID, &oldTask.ParentID, &oldTask.Status, &oldTask.BlockedReason)
	if err != nil {
		return c.Status(404).SendString("Task not found")
	}

	newTask := new(Task)
	if err := c.BodyParser(newTask); err != nil {
		return c.Status(400).SendString(err.Error())
	}
	if newTask.BoardID == "" { newTask.BoardID = oldTask.BoardID }
	if newTask.Title == "" { newTask.Title = oldTask.Title }
	if newTask.Description == "" { newTask.Description = oldTask.Description }
	if newTask.ListID == "" { newTask.ListID = oldTask.ListID }
	if newTask.AssigneeID == nil { newTask.AssigneeID = oldTask.AssigneeID }
	if newTask.ParentID == nil { newTask.ParentID = oldTask.ParentID }
	if newTask.Status == "" { newTask.Status = normalizeTaskStatus(newTask.ListID) }
	newTask.ListID = statusToListID(newTask.Status)
	if newTask.Status == "blocked" && newTask.BlockedReason == "" { newTask.BlockedReason = oldTask.BlockedReason }

	_, err = db.Exec(context.Background(),
		"UPDATE tasks SET title=$1, description=$2, list_id=$3, position=$4, assignee_id=$5, board_id=$6, parent_id=$7, status=$8, blocked_reason=$9 WHERE id=$10",
		newTask.Title, newTask.Description, newTask.ListID, newTask.Position, newTask.AssigneeID, newTask.BoardID, newTask.ParentID, newTask.Status, newTask.BlockedReason, id)
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}

	userID := "system"
	if newTask.UpdatedBy != "" { userID = newTask.UpdatedBy }
	if newTask.ListID != oldTask.ListID || newTask.Status != oldTask.Status {
		go logActivity(id, userID, "moved", fmt.Sprintf("Moved to %s", newTask.Status))
	}

	newAssignee, oldAssignee := "", ""
	if newTask.AssigneeID != nil { newAssignee = *newTask.AssigneeID }
	if oldTask.AssigneeID != nil { oldAssignee = *oldTask.AssigneeID }
	if newAssignee != oldAssignee {
		if newAssignee != "" {
			go logActivity(id, userID, "assigned", fmt.Sprintf("Assigned to %s", newAssignee))
			createNotification(&id, newAssignee, nil, "task_assigned", fmt.Sprintf("Task assigned: %s", newTask.Title))
			if runID := createAgentRun(id, newAssignee, "queued", "Awaiting pickup"); runID != nil {
				db.Exec(context.Background(), "UPDATE agents SET current_task_id=$1, current_run_id=$2, status='busy', current_activity=$3, last_seen_at=CURRENT_TIMESTAMP, updated_at=CURRENT_TIMESTAMP WHERE id=$4", id, *runID, "Queued on task", newAssignee)
			}
			enqueueAgentEvent(newAssignee, id, "task.assigned", map[string]interface{}{"task_id": id, "title": newTask.Title, "status": newTask.Status, "board_id": newTask.BoardID})
		} else {
			go logActivity(id, userID, "unassigned", "Removed assignee")
		}
	}
	if newTask.Status == "review" && oldTask.Status != "review" && oldTask.AssigneeID != nil {
		createNotification(&id, *oldTask.AssigneeID, nil, "review_requested", fmt.Sprintf("Task ready for review: %s", newTask.Title))
	}
	if newTask.Status == "blocked" && oldTask.Status != "blocked" && newTask.AssigneeID != nil {
		createNotification(&id, *newTask.AssigneeID, nil, "blocked", fmt.Sprintf("Task blocked: %s", newTask.BlockedReason))
		db.Exec(context.Background(), "UPDATE agents SET status='blocked', current_activity=$1, health_note=$2, updated_at=CURRENT_TIMESTAMP WHERE id=$3", "Blocked on task", newTask.BlockedReason, *newTask.AssigneeID)
	}
	if newTask.Description != oldTask.Description {
		go logActivity(id, userID, "updated", "Updated task description")
	}
	go updateEmbedding(id, newTask.Title+" "+newTask.Description)
	go broadcastUpdate("UPDATE")
	return c.JSON(newTask)
}

func logActivity(taskID int, userID, action, details string) {
	db.Exec(context.Background(),
		"INSERT INTO activities (task_id, user_id, action, details) VALUES ($1, $2, $3, $4)",
		taskID, userID, action, details)
}

func getTaskActivities(c *fiber.Ctx) error {
	taskID := c.Params("id")
	rows, err := db.Query(context.Background(), "SELECT id, task_id, user_id, action, details, created_at FROM activities WHERE task_id=$1 ORDER BY created_at DESC", taskID)
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	defer rows.Close()
	var activities []Activity
	for rows.Next() {
		var a Activity
		if err := rows.Scan(&a.ID, &a.TaskID, &a.UserID, &a.Action, &a.Details, &a.CreatedAt); err != nil {
			return c.Status(500).SendString(err.Error())
		}
		activities = append(activities, a)
	}
	if activities == nil {
		activities = []Activity{}
	}
	return c.JSON(activities)
}

func updateEmbedding(id int, text string) {
	emb, err := generateEmbedding(text)
	if err != nil {
		log.Printf("Emb err: %v", err)
		return
	}
	_, err = db.Exec(context.Background(), "UPDATE tasks SET embedding = $1 WHERE id = $2", pgvector(emb), id)
	if err != nil {
		log.Printf("Db emb err: %v", err)
	}
}

func pgvector(v []float32) string { b, _ := json.Marshal(v); return string(b) }

func searchTasks(c *fiber.Ctx) error {
	query := c.Query("q")
	if query == "" {
		return c.Status(400).SendString("Query required")
	}
	emb, err := generateEmbedding(query)
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	rows, err := db.Query(context.Background(),
		"SELECT id, board_id::text, title, description, list_id, position, assignee_id, parent_id, status, blocked_reason FROM tasks ORDER BY embedding <=> $1 LIMIT 5",
		pgvector(emb))
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	defer rows.Close()
	var tasks []Task
	for rows.Next() {
		var t Task
		if err := rows.Scan(&t.ID, &t.BoardID, &t.Title, &t.Description, &t.ListID, &t.Position, &t.AssigneeID, &t.ParentID, &t.Status, &t.BlockedReason); err != nil {
			return c.Status(500).SendString(err.Error())
		}
		tasks = append(tasks, t)
	}
	if tasks == nil {
		tasks = []Task{}
	}
	return c.JSON(tasks)
}

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

func getTaskRuns(c *fiber.Ctx) error {
	taskID := c.Params("id")
	rows, err := db.Query(context.Background(), "SELECT id, task_id, agent_id, session_key, provider_type, status, started_at, ended_at, current_activity, error_summary, result_summary FROM agent_runs WHERE task_id=$1 ORDER BY started_at DESC", taskID)
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	defer rows.Close()
	var runs []AgentRun
	for rows.Next() {
		var r AgentRun
		if err := rows.Scan(&r.ID, &r.TaskID, &r.AgentID, &r.SessionKey, &r.ProviderType, &r.Status, &r.StartedAt, &r.EndedAt, &r.CurrentActivity, &r.ErrorSummary, &r.ResultSummary); err != nil {
			return c.Status(500).SendString(err.Error())
		}
		runs = append(runs, r)
	}
	if runs == nil { runs = []AgentRun{} }
	return c.JSON(runs)
}

func getAgentRunsForAgent(c *fiber.Ctx) error {
	agentID := c.Params("id")
	rows, err := db.Query(context.Background(), "SELECT id, task_id, agent_id, session_key, provider_type, status, started_at, ended_at, current_activity, error_summary, result_summary FROM agent_runs WHERE agent_id=$1 ORDER BY started_at DESC LIMIT 20", agentID)
	if err != nil { return c.Status(500).SendString(err.Error()) }
	defer rows.Close()
	var runs []AgentRun
	for rows.Next() {
		var r AgentRun
		if err := rows.Scan(&r.ID, &r.TaskID, &r.AgentID, &r.SessionKey, &r.ProviderType, &r.Status, &r.StartedAt, &r.EndedAt, &r.CurrentActivity, &r.ErrorSummary, &r.ResultSummary); err != nil {
			return c.Status(500).SendString(err.Error())
		}
		runs = append(runs, r)
	}
	if runs == nil { runs = []AgentRun{} }
	return c.JSON(runs)
}

func getAgentNotifications(c *fiber.Ctx) error {
	agentID := c.Params("id")
	rows, err := db.Query(context.Background(), "SELECT id, task_id, target_agent_id, source_agent_id, type, content, delivered, delivered_at, created_at FROM notifications WHERE target_agent_id=$1 ORDER BY created_at DESC LIMIT 30", agentID)
	if err != nil { return c.Status(500).SendString(err.Error()) }
	defer rows.Close()
	var items []Notification
	for rows.Next() {
		var n Notification
		if err := rows.Scan(&n.ID, &n.TaskID, &n.TargetAgentID, &n.SourceAgentID, &n.Type, &n.Content, &n.Delivered, &n.DeliveredAt, &n.CreatedAt); err != nil {
			return c.Status(500).SendString(err.Error())
		}
		items = append(items, n)
	}
	if items == nil { items = []Notification{} }
	return c.JSON(items)
}

func getAgentEventsOps(c *fiber.Ctx) error {
	status := strings.TrimSpace(c.Query("status"))
	agentID := strings.TrimSpace(c.Query("agent_id"))
	eventType := strings.TrimSpace(c.Query("event_type"))
	boardID := strings.TrimSpace(c.Query("board_id"))
	query := `SELECT id, agent_id, board_id::text, task_id, event_type, payload_json::text, delivery_status, delivery_attempts, next_attempt_at, last_delivery_at, response_status, response_body, created_at, processed_at FROM agent_events WHERE 1=1`
	args := []interface{}{}
	idx := 1
	if status != "" {
		query += fmt.Sprintf(" AND delivery_status=$%d", idx)
		args = append(args, status)
		idx++
	}
	if agentID != "" {
		query += fmt.Sprintf(" AND agent_id=$%d", idx)
		args = append(args, agentID)
		idx++
	}
	if eventType != "" {
		query += fmt.Sprintf(" AND event_type=$%d", idx)
		args = append(args, eventType)
		idx++
	}
	if boardID != "" {
		query += fmt.Sprintf(" AND board_id::text=$%d", idx)
		args = append(args, boardID)
		idx++
	}
	query += " ORDER BY created_at DESC LIMIT 100"
	rows, err := db.Query(context.Background(), query, args...)
	if err != nil { return c.Status(500).SendString(err.Error()) }
	defer rows.Close()
	var items []AgentEvent
	for rows.Next() {
		var ev AgentEvent
		if err := rows.Scan(&ev.ID, &ev.AgentID, &ev.BoardID, &ev.TaskID, &ev.EventType, &ev.PayloadJSON, &ev.DeliveryStatus, &ev.DeliveryAttempts, &ev.NextAttemptAt, &ev.LastDeliveryAt, &ev.ResponseStatus, &ev.ResponseBody, &ev.CreatedAt, &ev.ProcessedAt); err != nil {
			return c.Status(500).SendString(err.Error())
		}
		items = append(items, ev)
	}
	if items == nil { items = []AgentEvent{} }
	return c.JSON(items)
}

func getAgentConnectorsOps(c *fiber.Ctx) error {
	status := strings.TrimSpace(c.Query("status"))
	agentID := strings.TrimSpace(c.Query("agent_id"))
	connectorType := strings.TrimSpace(c.Query("connector_type"))
	transportMode := strings.TrimSpace(c.Query("transport_mode"))
	query := `SELECT id, agent_id, connector_type, transport_mode, auth_type, endpoint_url, base_url, agent_ref, session_key, status, metadata_json::text, last_success_at, last_error, created_at, updated_at FROM agent_connectors WHERE 1=1`
	args := []interface{}{}
	idx := 1
	if status != "" {
		query += fmt.Sprintf(" AND status=$%d", idx)
		args = append(args, status)
		idx++
	}
	if agentID != "" {
		query += fmt.Sprintf(" AND agent_id=$%d", idx)
		args = append(args, agentID)
		idx++
	}
	if connectorType != "" {
		query += fmt.Sprintf(" AND connector_type=$%d", idx)
		args = append(args, connectorType)
		idx++
	}
	if transportMode != "" {
		query += fmt.Sprintf(" AND transport_mode=$%d", idx)
		args = append(args, transportMode)
		idx++
	}
	query += " ORDER BY updated_at DESC, id DESC LIMIT 100"
	rows, err := db.Query(context.Background(), query, args...)
	if err != nil { return c.Status(500).SendString(err.Error()) }
	defer rows.Close()
	var items []AgentConnector
	for rows.Next() {
		var conn AgentConnector
		if err := rows.Scan(&conn.ID, &conn.AgentID, &conn.ConnectorType, &conn.TransportMode, &conn.AuthType, &conn.EndpointURL, &conn.BaseURL, &conn.AgentRef, &conn.SessionKey, &conn.Status, &conn.MetadataJSON, &conn.LastSuccessAt, &conn.LastError, &conn.CreatedAt, &conn.UpdatedAt); err != nil {
			return c.Status(500).SendString(err.Error())
		}
		items = append(items, conn)
	}
	if items == nil { items = []AgentConnector{} }
	return c.JSON(items)
}

func retryAgentEventOps(c *fiber.Ctx) error {
	eventID := c.Params("id")
	cmd, err := db.Exec(context.Background(), `
		UPDATE agent_events
		SET delivery_status='pending', next_attempt_at=CURRENT_TIMESTAMP, processed_at=NULL, response_status='manual_retry_requested', response_body=''
		WHERE id=$1 AND delivery_status IN ('failed','processing')`, eventID)
	if err != nil { return c.Status(500).SendString(err.Error()) }
	if cmd.RowsAffected() == 0 {
		return c.Status(404).JSON(fiber.Map{"error": "event_not_retryable"})
	}
	go deliverPendingAgentEvents()
	return c.JSON(fiber.Map{"ok": true, "action": "retry", "event_id": eventID})
}

func requeueAgentEventOps(c *fiber.Ctx) error {
	eventID := c.Params("id")
	cmd, err := db.Exec(context.Background(), `
		UPDATE agent_events
		SET delivery_status='pending', delivery_attempts=0, next_attempt_at=CURRENT_TIMESTAMP, processed_at=NULL, response_status='manual_requeue_requested', response_body=''
		WHERE id=$1 AND delivery_status IN ('dead','failed','ignored','routed')`, eventID)
	if err != nil { return c.Status(500).SendString(err.Error()) }
	if cmd.RowsAffected() == 0 {
		return c.Status(404).JSON(fiber.Map{"error": "event_not_requeueable"})
	}
	go deliverPendingAgentEvents()
	return c.JSON(fiber.Map{"ok": true, "action": "requeue", "event_id": eventID})
}

func enableAgentConnectorOps(c *fiber.Ctx) error {
	connectorID := c.Params("id")
	cmd, err := db.Exec(context.Background(), `UPDATE agent_connectors SET status='connected', updated_at=CURRENT_TIMESTAMP WHERE id=$1`, connectorID)
	if err != nil { return c.Status(500).SendString(err.Error()) }
	if cmd.RowsAffected() == 0 {
		return c.Status(404).JSON(fiber.Map{"error": "connector_not_found"})
	}
	return c.JSON(fiber.Map{"ok": true, "action": "enable", "connector_id": connectorID})
}

func disableAgentConnectorOps(c *fiber.Ctx) error {
	connectorID := c.Params("id")
	cmd, err := db.Exec(context.Background(), `UPDATE agent_connectors SET status='disabled', updated_at=CURRENT_TIMESTAMP WHERE id=$1`, connectorID)
	if err != nil { return c.Status(500).SendString(err.Error()) }
	if cmd.RowsAffected() == 0 {
		return c.Status(404).JSON(fiber.Map{"error": "connector_not_found"})
	}
	return c.JSON(fiber.Map{"ok": true, "action": "disable", "connector_id": connectorID})
}

func heartbeatAgent(c *fiber.Ctx) error {
	agentID := c.Params("id")
	_, err := db.Exec(context.Background(), "UPDATE agents SET status='online', last_heartbeat_at=CURRENT_TIMESTAMP, last_seen_at=CURRENT_TIMESTAMP, updated_at=CURRENT_TIMESTAMP WHERE id=$1", agentID)
	if err != nil { return c.Status(500).SendString(err.Error()) }
	return c.JSON(fiber.Map{"ok": true})
}

func updateAgentStatus(c *fiber.Ctx) error {
	agentID := c.Params("id")
	var payload struct {
		Status string `json:"status"`
		CurrentActivity string `json:"current_activity"`
		HealthNote string `json:"health_note"`
	}
	if err := c.BodyParser(&payload); err != nil { return c.Status(400).SendString(err.Error()) }
	if payload.Status == "" { payload.Status = "online" }
	_, err := db.Exec(context.Background(), "UPDATE agents SET status=$1, current_activity=$2, health_note=$3, last_seen_at=CURRENT_TIMESTAMP, updated_at=CURRENT_TIMESTAMP WHERE id=$4", payload.Status, payload.CurrentActivity, payload.HealthNote, agentID)
	if err != nil { return c.Status(500).SendString(err.Error()) }
	return c.JSON(fiber.Map{"ok": true})
}
