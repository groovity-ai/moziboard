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

type Task struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	ListID      string `json:"list_id"`
	Position    int    `json:"position"`
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
	
	query := `
	CREATE TABLE IF NOT EXISTS tasks (
		id SERIAL PRIMARY KEY,
		title TEXT NOT NULL,
		description TEXT,
		list_id TEXT NOT NULL,
		position INT DEFAULT 0
	);
	`
	db.Exec(context.Background(), query)
	db.Exec(context.Background(), "ALTER TABLE tasks ADD COLUMN IF NOT EXISTS embedding vector(3072)")
	
	fmt.Println("✅ Database migrated!")
}

func initAI() {
	// Only init OpenAI client if key is present
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
		// Manual REST Call to Gemini v1beta (text-embedding-004)
		url := "https://generativelanguage.googleapis.com/v1beta/models/text-embedding-004:embedContent?key=" + geminiKey
		
		body := map[string]interface{}{
			"model": "models/text-embedding-004",
			"content": map[string]interface{}{
				"parts": []map[string]interface{}{
					{"text": text},
				},
			},
		}
		
		jsonBody, _ := json.Marshal(body)
		resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonBody))
		if err != nil { return nil, err }
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			// If 404, try fallback model gemini-embedding-001
			if resp.StatusCode == 404 {
				url = "https://generativelanguage.googleapis.com/v1beta/models/gemini-embedding-001:embedContent?key=" + geminiKey
				body["model"] = "models/gemini-embedding-001"
				jsonBody, _ = json.Marshal(body)
				resp, err = http.Post(url, "application/json", bytes.NewBuffer(jsonBody))
				if err != nil { return nil, err }
				defer resp.Body.Close()
			}
			
			if resp.StatusCode != 200 {
				buf := new(bytes.Buffer)
				buf.ReadFrom(resp.Body)
				return nil, fmt.Errorf("gemini api error %d: %s", resp.StatusCode, buf.String())
			}
		}

		var result GeminiEmbeddingResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil { return nil, err }
		return result.Embedding.Values, nil
	}

	// OpenAI Fallback (Commented out to avoid build errors if pkg missing)
	/*
	if openaiClient != nil {
		resp, err := openaiClient.CreateEmbeddings(context.Background(), openai.EmbeddingRequest{
			Input: []string{text},
			Model: openai.SmallEmbedding3, 
		})
		if err != nil { return nil, err }
		return resp.Data[0].Embedding, nil
	}
	*/

	return nil, fmt.Errorf("no AI provider configured")
}

func main() {
	initDB()
	initAI()

	rdb = redis.NewClient(&redis.Options{
		Addr: os.Getenv("REDIS_ADDR"), Password: "moziboard_redis_secret", DB: 0,
	})

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
	app.Get("/api/tasks", getTasks)
	app.Post("/api/tasks", createTask)
	app.Put("/api/tasks/:id", updateTask)
	app.Get("/api/search", searchTasks)

	log.Fatal(app.Listen(":8080"))
}

func getTasks(c *fiber.Ctx) error {
	rows, err := db.Query(context.Background(), "SELECT id, title, description, list_id, position FROM tasks ORDER BY position ASC")
	if err != nil { return c.Status(500).SendString(err.Error()) }
	defer rows.Close()
	var tasks []Task
	for rows.Next() {
		var t Task
		if err := rows.Scan(&t.ID, &t.Title, &t.Description, &t.ListID, &t.Position); err != nil { return c.Status(500).SendString(err.Error()) }
		tasks = append(tasks, t)
	}
	if tasks == nil { tasks = []Task{} }
	return c.JSON(tasks)
}

func createTask(c *fiber.Ctx) error {
	t := new(Task)
	if err := c.BodyParser(t); err != nil { return c.Status(400).SendString(err.Error()) }
	var id int
	err := db.QueryRow(context.Background(), 
		"INSERT INTO tasks (title, description, list_id, position) VALUES ($1, $2, $3, $4) RETURNING id",
		t.Title, t.Description, t.ListID, t.Position).Scan(&id)
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
	_, err := db.Exec(context.Background(), 
		"UPDATE tasks SET title=$1, description=$2, list_id=$3, position=$4 WHERE id=$5",
		t.Title, t.Description, t.ListID, t.Position, id)
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

func pgvector(v []float32) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func searchTasks(c *fiber.Ctx) error {
	query := c.Query("q")
	if query == "" { return c.Status(400).SendString("Query required") }
	emb, err := generateEmbedding(query)
	if err != nil { return c.Status(500).SendString(err.Error()) }
	rows, err := db.Query(context.Background(), 
		"SELECT id, title, description, list_id, position FROM tasks ORDER BY embedding <=> $1 LIMIT 5",
		pgvector(emb))
	if err != nil { return c.Status(500).SendString(err.Error()) }
	defer rows.Close()
	var tasks []Task
	for rows.Next() {
		var t Task
		if err := rows.Scan(&t.ID, &t.Title, &t.Description, &t.ListID, &t.Position); err != nil { return c.Status(500).SendString(err.Error()) }
		tasks = append(tasks, t)
	}
	if tasks == nil { tasks = []Task{} }
	return c.JSON(tasks)
}
