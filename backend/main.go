package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
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
				"INSERT INTO agents (id, soul, memory, rules, cron_schedule, status) VALUES ($1, $2, $3, $4, $5, 'offline') ON CONFLICT (id) DO NOTHING",
				m.ID, "You are a helpful AI assistant in a software squad.", "No memories yet.", "Be concise.", "*/1 * * * *")
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
	app.Get("/api/agents/:id/runs", getAgentRunsForAgent)
	app.Get("/api/agents/:id/notifications", getAgentNotifications)
	app.Post("/api/agents/:id/heartbeat", heartbeatAgent)
	app.Put("/api/agents/:id/status", updateAgentStatus)
	app.Get("/api/agents/sync/aiagenz", getAiAgenzProjects)

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

	// Run polling as a background goroutine
	go func() {
		// Check immediately at startup
		pollAgentTasks()

		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			pollAgentTasks()
		}
	}()
}

func pollAgentTasks() {
	query := `
		SELECT t.id, t.board_id::text, t.title, t.description, t.assignee_id 
		FROM tasks t
		JOIN members m ON t.assignee_id = m.id
		WHERE lower(t.list_id) = 'todo' AND m.role = 'agent'
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

	// Mark as doing
	_, err := db.Exec(context.Background(),
		"UPDATE tasks SET list_id=$1, updated_by=$2 WHERE id=$3",
		"doing", agentName, t.ID)
	if err != nil {
		log.Printf("Dispatcher failed to mark task %d doing: %v", t.ID, err)
		return
	}

	go logActivity(t.ID, agentName, "moved", "Moved to list doing (Auto-dispatched)")
	go broadcastUpdate("UPDATE")

	taskContent := fmt.Sprintf("MoziBoard Task %d: %s\n%s", t.ID, t.Title, t.Description)

	var cmd *exec.Cmd
	if agentName == "kodinger" || agentName == "mochi" {
		cmd = exec.Command("openclaw", "agent", "--profile", agentName, "--agent", agentName, "--message", taskContent)
	} else {
		cmd = exec.Command("openclaw", "agent", "--agent", agentName, "--message", taskContent)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("OpenClaw error for agent %s on task %d: %v\nOutput: %s", agentName, t.ID, err, string(output))
	} else {
		log.Printf("OpenClaw finished for agent %s on task %d.\nOutput: %s", agentName, t.ID, string(output))
	}
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

func createAgentRun(taskID int, agentID string, status string, activity string) *int {
	var runID int
	err := db.QueryRow(context.Background(),
		"INSERT INTO agent_runs (task_id, agent_id, session_key, provider_type, status, current_activity) VALUES ($1, $2, $3, 'openclaw', $4, $5) RETURNING id",
		taskID, agentID, fmt.Sprintf("mozi:%s:%d", agentID, taskID), status, activity).Scan(&runID)
	if err != nil {
		log.Printf("failed to create agent run: %v", err)
		return nil
	}
	return &runID
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
	rows, err := db.Query(context.Background(), "SELECT id, soul, memory, rules, cron_schedule, active, status, last_heartbeat_at, last_seen_at, current_task_id, current_run_id, current_activity, health_note, created_at, updated_at FROM agents")
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	defer rows.Close()
	var agents []Agent
	for rows.Next() {
		var a Agent
		if err := rows.Scan(&a.ID, &a.Soul, &a.Memory, &a.Rules, &a.CronSchedule, &a.Active, &a.Status, &a.LastHeartbeatAt, &a.LastSeenAt, &a.CurrentTaskID, &a.CurrentRunID, &a.CurrentActivity, &a.HealthNote, &a.CreatedAt, &a.UpdatedAt); err != nil {
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
		"SELECT id, soul, memory, rules, cron_schedule, active, status, last_heartbeat_at, last_seen_at, current_task_id, current_run_id, current_activity, health_note, created_at, updated_at FROM agents WHERE id=$1",
		agentID).Scan(&a.ID, &a.Soul, &a.Memory, &a.Rules, &a.CronSchedule, &a.Active, &a.Status, &a.LastHeartbeatAt, &a.LastSeenAt, &a.CurrentTaskID, &a.CurrentRunID, &a.CurrentActivity, &a.HealthNote, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return c.Status(404).SendString("Agent not found")
	}
	return c.JSON(a)
}

// getAiAgenzProjects proxies a request to the main AiAgenz backend to fetch the user's projects (agents).
func getAiAgenzProjects(c *fiber.Ctx) error {
	// The user needs to pass their session cookie or bearer token from the frontend.
	// We'll forward the entire Cookie header to the AiAgenz API.
	authCookie := c.Get("Cookie")
	if authCookie == "" {
		return c.Status(401).JSON(fiber.Map{"error": "Unauthorized: Missing authentication cookie"})
	}

	// Assuming the AiAgenz backend is running on 172.17.0.1:4001
	proxyURL := "http://172.17.0.1:4001/api/projects"

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

	if resp.StatusCode != 200 {
		return c.Status(resp.StatusCode).JSON(fiber.Map{"error": "AiAgenz API returned an error status: " + resp.Status})
	}

	var data interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to parse AiAgenz response"})
	}

	return c.JSON(data)
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
