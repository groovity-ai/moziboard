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
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

type Task struct {
	ID          int     `json:"id"`
	BoardID     string  `json:"board_id"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	ListID      string  `json:"list_id"`
	Position    int     `json:"position"`
	AssigneeID  *string `json:"assignee_id"`
	UpdatedBy   string  `json:"updated_by,omitempty"`
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
		title TEXT NOT NULL,
		description TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);`)

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
	CREATE TABLE IF NOT EXISTS tasks (
		id SERIAL PRIMARY KEY,
		board_id UUID NOT NULL,
		title TEXT NOT NULL,
		description TEXT,
		list_id TEXT NOT NULL,
		position INT DEFAULT 0,
		assignee_id TEXT REFERENCES members(id),
		CONSTRAINT fk_board FOREIGN KEY(board_id) REFERENCES boards(id) ON DELETE CASCADE
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
	app.Get("/api/search", searchTasks)
	app.Get("/api/members", getMembers)

	// Knowledge Base / Documents
	app.Get("/api/boards/:id/docs", getBoardDocs)
	app.Post("/api/boards/:id/docs", createDoc)
	app.Put("/api/docs/:id", updateDoc)
	app.Delete("/api/docs/:id", deleteDoc)
	app.Get("/api/docs/search", searchDocs)

	log.Fatal(app.Listen(":8080"))
}

func getBoards(c *fiber.Ctx) error {
	rows, err := db.Query(context.Background(), "SELECT id::text, title, description FROM boards ORDER BY created_at ASC")
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	defer rows.Close()
	var boards []Board
	for rows.Next() {
		var b Board
		if err := rows.Scan(&b.ID, &b.Title, &b.Description); err != nil {
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
	err := db.QueryRow(context.Background(), "INSERT INTO boards (title, description) VALUES ($1, $2) RETURNING id::text", b.Title, b.Description).Scan(&b.ID)
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	db.Exec(context.Background(), "INSERT INTO board_members (board_id, member_id, role) VALUES ($1, 'mirza', 'owner')", b.ID)
	return c.JSON(b)
}

func getBoardTasks(c *fiber.Ctx) error {
	boardID := c.Params("id")
	rows, err := db.Query(context.Background(), "SELECT id, board_id::text, title, description, list_id, position, assignee_id FROM tasks WHERE board_id=$1 ORDER BY position ASC", boardID)
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	defer rows.Close()
	var tasks []Task
	for rows.Next() {
		var t Task
		if err := rows.Scan(&t.ID, &t.BoardID, &t.Title, &t.Description, &t.ListID, &t.Position, &t.AssigneeID); err != nil {
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
	if t.BoardID == "" {
		return c.Status(400).SendString("Board ID is required")
	}
	if t.ListID == "" {
		t.ListID = "todo"
	}

	var id int
	err := db.QueryRow(context.Background(),
		"INSERT INTO tasks (board_id, title, description, list_id, position, assignee_id) VALUES ($1, $2, $3, $4, $5, $6) RETURNING id",
		t.BoardID, t.Title, t.Description, t.ListID, t.Position, t.AssigneeID).Scan(&id)
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	t.ID = id
	go updateEmbedding(id, t.Title+" "+t.Description)
	go broadcastUpdate("UPDATE")
	return c.JSON(t)
}

func updateTask(c *fiber.Ctx) error {
	id, _ := strconv.Atoi(c.Params("id"))

	var oldTask Task
	err := db.QueryRow(context.Background(),
		"SELECT id, board_id::text, title, description, list_id, position, assignee_id FROM tasks WHERE id=$1",
		id).Scan(&oldTask.ID, &oldTask.BoardID, &oldTask.Title, &oldTask.Description, &oldTask.ListID, &oldTask.Position, &oldTask.AssigneeID)
	if err != nil {
		return c.Status(404).SendString("Task not found")
	}

	newTask := new(Task)
	if err := c.BodyParser(newTask); err != nil {
		return c.Status(400).SendString(err.Error())
	}

	// Preserve existing values if fields are empty/missing
	if newTask.BoardID == "" {
		newTask.BoardID = oldTask.BoardID
	}
	if newTask.Title == "" {
		newTask.Title = oldTask.Title
	}
	if newTask.Description == "" {
		newTask.Description = oldTask.Description
	}
	if newTask.ListID == "" {
		newTask.ListID = oldTask.ListID
	}
	if newTask.AssigneeID == nil {
		newTask.AssigneeID = oldTask.AssigneeID
	}
	// We keep position as is if it's 0 (might be intentional move to top),
	// but generally frontend sends it. Let's assume if it's 0 and not explicitly set, keep old?
	// No, 0 is valid. Let's trust frontend or keep logic simple.
	// Actually, Go structs default to 0/empty.
	// To truly distinguish "unset" vs "empty", we'd need pointer fields.
	// For MVP, if Title is empty, assume we keep old one.

	_, err = db.Exec(context.Background(),
		"UPDATE tasks SET title=$1, description=$2, list_id=$3, position=$4, assignee_id=$5, board_id=$6 WHERE id=$7",
		newTask.Title, newTask.Description, newTask.ListID, newTask.Position, newTask.AssigneeID, newTask.BoardID, id)
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}

	userID := "mirza" // Default
	if newTask.UpdatedBy != "" {
		userID = newTask.UpdatedBy
	}

	if newTask.ListID != oldTask.ListID {
		go logActivity(id, userID, "moved", fmt.Sprintf("Moved to list %s", newTask.ListID))
	}

	newAssignee, oldAssignee := "", ""
	if newTask.AssigneeID != nil {
		newAssignee = *newTask.AssigneeID
	}
	if oldTask.AssigneeID != nil {
		oldAssignee = *oldTask.AssigneeID
	}

	if newAssignee != oldAssignee {
		if newAssignee != "" {
			go logActivity(id, userID, "assigned", fmt.Sprintf("Assigned to %s", newAssignee))
		} else {
			go logActivity(id, userID, "unassigned", "Removed assignee")
		}
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
		"SELECT id, board_id::text, title, description, list_id, position, assignee_id FROM tasks ORDER BY embedding <=> $1 LIMIT 5",
		pgvector(emb))
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	defer rows.Close()
	var tasks []Task
	for rows.Next() {
		var t Task
		if err := rows.Scan(&t.ID, &t.BoardID, &t.Title, &t.Description, &t.ListID, &t.Position, &t.AssigneeID); err != nil {
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
