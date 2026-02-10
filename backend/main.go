package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/contrib/websocket"
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
	ID          int    `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

type Task struct {
	ID          int     `json:"id"`
	BoardID     int     `json:"board_id"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	ListID      string  `json:"list_id"`
	Position    int     `json:"position"`
	AssigneeID  *string `json:"assignee_id"`
}

type Member struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Role   string `json:"role"`
	Avatar string `json:"avatar"`
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
	if err != nil { log.Fatalf("Unable to connect to database: %v\n", err) }

	db.Exec(context.Background(), "CREATE EXTENSION IF NOT EXISTS vector")
	
	// Boards Table
	db.Exec(context.Background(), `
	CREATE TABLE IF NOT EXISTS boards (
		id SERIAL PRIMARY KEY,
		title TEXT NOT NULL,
		description TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);`)

	// Seed Default Board
	var defaultBoardID int
	// Check if board exists first
	err = db.QueryRow(context.Background(), "SELECT id FROM boards WHERE title='Main Project' LIMIT 1").Scan(&defaultBoardID)
	if err != nil {
		// Create if not exists
		db.QueryRow(context.Background(), "INSERT INTO boards (title, description) VALUES ('Main Project', 'Default board') RETURNING id").Scan(&defaultBoardID)
	}

	// Tasks Table
	db.Exec(context.Background(), `
	CREATE TABLE IF NOT EXISTS tasks (
		id SERIAL PRIMARY KEY,
		board_id INT NOT NULL DEFAULT 1,
		title TEXT NOT NULL,
		description TEXT,
		list_id TEXT NOT NULL,
		position INT DEFAULT 0
	);`)
	db.Exec(context.Background(), "ALTER TABLE tasks ADD COLUMN IF NOT EXISTS embedding vector(3072)")
	
	// Add board_id if not exists (migration)
	db.Exec(context.Background(), "ALTER TABLE tasks ADD COLUMN IF NOT EXISTS board_id INT NOT NULL DEFAULT 1")
	
	// Members Table
	db.Exec(context.Background(), `
	CREATE TABLE IF NOT EXISTS members (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		role TEXT NOT NULL,
		avatar TEXT
	);`)

	// Assignee Column
	db.Exec(context.Background(), "ALTER TABLE tasks ADD COLUMN IF NOT EXISTS assignee_id TEXT REFERENCES members(id)")

	seedMembers()
	
	fmt.Println("✅ Database migrated!")
}

func seedMembers() {
	members := []Member{
		{ID: "mirza", Name: "Mirza", Role: "human", Avatar: "👤"},
		{ID: "devo", Name: "Devo", Role: "agent", Avatar: "🛡️"},
		{ID: "kodinger", Name: "Kodinger", Role: "agent", Avatar: "👨‍💻"},
		{ID: "mimin", Name: "Mimin", Role: "agent", Avatar: "📢"},
	}
	for _, m := range members {
		_, err := db.Exec(context.Background(), 
			"INSERT INTO members (id, name, role, avatar) VALUES ($1, $2, $3, $4) ON CONFLICT (id) DO UPDATE SET name=$2, role=$3, avatar=$4",
			m.ID, m.Name, m.Role, m.Avatar)
		if err != nil { log.Printf("Seed error: %v", err) }
	}
}

func initAI() {
	apiKey := os.Getenv("OPENAI_API_KEY")
	baseURL := os.Getenv("OPENAI_BASE_URL")
	if apiKey != "" {
		config := openai.DefaultConfig(apiKey)
		if baseURL != "" { config.BaseURL = baseURL }
		openaiClient = openai.NewClientWithConfig(config)
	}
}

func generateEmbedding(text string) ([]float32, error) {
	geminiKey := os.Getenv("GEMINI_API_KEY")
	if geminiKey != "" {
		url := "https://generativelanguage.googleapis.com/v1beta/models/text-embedding-004:embedContent?key=" + geminiKey
		body := map[string]interface{}{
			"model": "models/text-embedding-004",
			"content": map[string]interface{}{"parts": []map[string]interface{}{{"text": text}}},
		}
		jsonBody, _ := json.Marshal(body)
		resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonBody))
		if err != nil { return nil, err }
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			if resp.StatusCode == 404 {
				url = "https://generativelanguage.googleapis.com/v1beta/models/gemini-embedding-001:embedContent?key=" + geminiKey
				body["model"] = "models/gemini-embedding-001"
				jsonBody, _ = json.Marshal(body)
				resp, err = http.Post(url, "application/json", bytes.NewBuffer(jsonBody))
				if err != nil { return nil, err }
				defer resp.Body.Close()
			}
			if resp.StatusCode != 200 {
				buf := new(bytes.Buffer); buf.ReadFrom(resp.Body)
				return nil, fmt.Errorf("gemini api error %d: %s", resp.StatusCode, buf.String())
			}
		}
		var result GeminiEmbeddingResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil { return nil, err }
		return result.Embedding.Values, nil
	}
	return nil, fmt.Errorf("no AI provider configured")
}

func main() {
	initDB()
	initAI()

	rdb = redis.NewClient(&redis.Options{Addr: os.Getenv("REDIS_ADDR"), Password: "moziboard_redis_secret", DB: 0})

	app := fiber.New()
	app.Use(cors.New(cors.Config{ AllowOrigins: "*", AllowHeaders: "Origin, Content-Type, Accept" }))

	app.Use("/ws", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) { c.Locals("allowed", true); return c.Next() }
		return fiber.ErrUpgradeRequired
	})

	app.Get("/ws", websocket.New(func(c *websocket.Conn) {
		clientsMu.Lock(); clients[c] = true; clientsMu.Unlock()
		defer func() { clientsMu.Lock(); delete(clients, c); clientsMu.Unlock(); c.Close() }()
		for { if _, _, err := c.ReadMessage(); err != nil { break } }
	}))

	app.Get("/api/health", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"status": "ok"}) })
	
	app.Get("/api/boards", getBoards)
	app.Post("/api/boards", createBoard)
	app.Get("/api/boards/:id/tasks", getBoardTasks)
	app.Get("/api/tasks", getTasks) // Keep for backward compat (returns all or default board)
	
	app.Post("/api/tasks", createTask)
	app.Put("/api/tasks/:id", updateTask)
	app.Get("/api/search", searchTasks)
	app.Get("/api/members", getMembers)

	log.Fatal(app.Listen(":8080"))
}

func getBoards(c *fiber.Ctx) error {
	rows, err := db.Query(context.Background(), "SELECT id, title, description FROM boards ORDER BY id ASC")
	if err != nil { return c.Status(500).SendString(err.Error()) }
	defer rows.Close()
	var boards []Board
	for rows.Next() {
		var b Board
		if err := rows.Scan(&b.ID, &b.Title, &b.Description); err != nil { return c.Status(500).SendString(err.Error()) }
		boards = append(boards, b)
	}
	if boards == nil { boards = []Board{} }
	return c.JSON(boards)
}

func createBoard(c *fiber.Ctx) error {
	b := new(Board)
	if err := c.BodyParser(b); err != nil { return c.Status(400).SendString(err.Error()) }
	err := db.QueryRow(context.Background(), "INSERT INTO boards (title, description) VALUES ($1, $2) RETURNING id", b.Title, b.Description).Scan(&b.ID)
	if err != nil { return c.Status(500).SendString(err.Error()) }
	return c.JSON(b)
}

func getBoardTasks(c *fiber.Ctx) error {
	boardID := c.Params("id")
	rows, err := db.Query(context.Background(), "SELECT id, board_id, title, description, list_id, position, assignee_id FROM tasks WHERE board_id=$1 ORDER BY position ASC", boardID)
	if err != nil { return c.Status(500).SendString(err.Error()) }
	defer rows.Close()
	var tasks []Task
	for rows.Next() {
		var t Task
		if err := rows.Scan(&t.ID, &t.BoardID, &t.Title, &t.Description, &t.ListID, &t.Position, &t.AssigneeID); err != nil { return c.Status(500).SendString(err.Error()) }
		tasks = append(tasks, t)
	}
	if tasks == nil { tasks = []Task{} }
	return c.JSON(tasks)
}

func getMembers(c *fiber.Ctx) error {
	rows, err := db.Query(context.Background(), "SELECT id, name, role, avatar FROM members")
	if err != nil { return c.Status(500).SendString(err.Error()) }
	defer rows.Close()
	var members []Member
	for rows.Next() {
		var m Member
		if err := rows.Scan(&m.ID, &m.Name, &m.Role, &m.Avatar); err != nil { return c.Status(500).SendString(err.Error()) }
		members = append(members, m)
	}
	if members == nil { members = []Member{} }
	return c.JSON(members)
}

func getTasks(c *fiber.Ctx) error {
	// Default to Board 1 if no board specified
	rows, err := db.Query(context.Background(), "SELECT id, board_id, title, description, list_id, position, assignee_id FROM tasks WHERE board_id=1 ORDER BY position ASC")
	if err != nil { return c.Status(500).SendString(err.Error()) }
	defer rows.Close()
	var tasks []Task
	for rows.Next() {
		var t Task
		if err := rows.Scan(&t.ID, &t.BoardID, &t.Title, &t.Description, &t.ListID, &t.Position, &t.AssigneeID); err != nil { return c.Status(500).SendString(err.Error()) }
		tasks = append(tasks, t)
	}
	if tasks == nil { tasks = []Task{} }
	return c.JSON(tasks)
}

func createTask(c *fiber.Ctx) error {
	t := new(Task)
	if err := c.BodyParser(t); err != nil { return c.Status(400).SendString(err.Error()) }
	if t.BoardID == 0 { t.BoardID = 1 } // Default board

	var id int
	err := db.QueryRow(context.Background(), 
		"INSERT INTO tasks (board_id, title, description, list_id, position, assignee_id) VALUES ($1, $2, $3, $4, $5, $6) RETURNING id",
		t.BoardID, t.Title, t.Description, t.ListID, t.Position, t.AssigneeID).Scan(&id)
	if err != nil { return c.Status(500).SendString(err.Error()) }
	t.ID = id
	go updateEmbedding(id, t.Title + " " + t.Description)
	go broadcastUpdate("UPDATE")
	return c.JSON(t)
}

func updateTask(c *fiber.Ctx) error {
	id, _ := strconv.Atoi(c.Params("id"))
	t := new(Task)
	if err := c.BodyParser(t); err != nil { return c.Status(400).SendString(err.Error()) }
	
	// Keep board_id if not sent? Or trust client.
	// We'll update board_id too to allow moving tasks between boards!
	_, err := db.Exec(context.Background(), 
		"UPDATE tasks SET title=$1, description=$2, list_id=$3, position=$4, assignee_id=$5, board_id=$6 WHERE id=$7",
		t.Title, t.Description, t.ListID, t.Position, t.AssigneeID, t.BoardID, id)
	if err != nil { return c.Status(500).SendString(err.Error()) }
	
	go updateEmbedding(id, t.Title + " " + t.Description)
	go broadcastUpdate("UPDATE")
	return c.JSON(t)
}

func updateEmbedding(id int, text string) {
	emb, err := generateEmbedding(text)
	if err != nil { log.Printf("Emb err: %v", err); return }
	_, err = db.Exec(context.Background(), "UPDATE tasks SET embedding = $1 WHERE id = $2", pgvector(emb), id)
	if err != nil { log.Printf("Db emb err: %v", err) }
}

func pgvector(v []float32) string { b, _ := json.Marshal(v); return string(b) }

func searchTasks(c *fiber.Ctx) error {
	query := c.Query("q")
	if query == "" { return c.Status(400).SendString("Query required") }
	emb, err := generateEmbedding(query)
	if err != nil { return c.Status(500).SendString(err.Error()) }
	
	// Search across ALL boards? Or specific?
	// For now global search
	rows, err := db.Query(context.Background(), 
		"SELECT id, board_id, title, description, list_id, position, assignee_id FROM tasks ORDER BY embedding <=> $1 LIMIT 5",
		pgvector(emb))
	if err != nil { return c.Status(500).SendString(err.Error()) }
	defer rows.Close()
	var tasks []Task
	for rows.Next() {
		var t Task
		if err := rows.Scan(&t.ID, &t.BoardID, &t.Title, &t.Description, &t.ListID, &t.Position, &t.AssigneeID); err != nil { return c.Status(500).SendString(err.Error()) }
		tasks = append(tasks, t)
	}
	if tasks == nil { tasks = []Task{} }
	return c.JSON(tasks)
}
