package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

var (
	db  *pgxpool.Pool
	rdb *redis.Client
)

// Task model
type Task struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	ListID      string `json:"list_id"`
	Position    int    `json:"position"`
}

func initDB() {
	dbURL := fmt.Sprintf("postgres://%s:%s@%s:%s/%s",
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_NAME"),
	)
	var err error
	db, err = pgxpool.New(context.Background(), dbURL)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v\n", err)
	}

	// Auto-migrate (simple)
	query := `
	CREATE TABLE IF NOT EXISTS tasks (
		id SERIAL PRIMARY KEY,
		title TEXT NOT NULL,
		description TEXT,
		list_id TEXT NOT NULL,
		position INT DEFAULT 0
	);
	`
	_, err = db.Exec(context.Background(), query)
	if err != nil {
		log.Fatalf("Failed to migrate database: %v\n", err)
	}
	fmt.Println("✅ Database migrated!")
}

func main() {
	initDB()

	// Connect Redis
	rdb = redis.NewClient(&redis.Options{
		Addr:     os.Getenv("REDIS_ADDR"),
		Password: "moziboard_redis_secret",
		DB:       0,
	})

	// Setup Fiber
	app := fiber.New()

	// CORS config
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Origin, Content-Type, Accept",
	}))

	// Routes
	app.Get("/api/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "message": "Moziboard Backend v1"})
	})

	app.Get("/api/tasks", getTasks)
	app.Post("/api/tasks", createTask)
	app.Put("/api/tasks/:id", updateTask)

	port := ":8080"
	log.Fatal(app.Listen(port))
}

func getTasks(c *fiber.Ctx) error {
	rows, err := db.Query(context.Background(), "SELECT id, title, description, list_id, position FROM tasks ORDER BY position ASC")
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		var t Task
		if err := rows.Scan(&t.ID, &t.Title, &t.Description, &t.ListID, &t.Position); err != nil {
			return c.Status(500).SendString(err.Error())
		}
		tasks = append(tasks, t)
	}
	
	if tasks == nil {
		tasks = []Task{}
	}

	return c.JSON(tasks)
}

func createTask(c *fiber.Ctx) error {
	t := new(Task)
	if err := c.BodyParser(t); err != nil {
		return c.Status(400).SendString(err.Error())
	}

	var id int
	err := db.QueryRow(context.Background(), 
		"INSERT INTO tasks (title, description, list_id, position) VALUES ($1, $2, $3, $4) RETURNING id",
		t.Title, t.Description, t.ListID, t.Position).Scan(&id)
	
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}

	t.ID = id
	return c.JSON(t)
}

func updateTask(c *fiber.Ctx) error {
	id, _ := strconv.Atoi(c.Params("id"))
	t := new(Task)
	if err := c.BodyParser(t); err != nil {
		return c.Status(400).SendString(err.Error())
	}

	_, err := db.Exec(context.Background(), 
		"UPDATE tasks SET title=$1, description=$2, list_id=$3, position=$4 WHERE id=$5",
		t.Title, t.Description, t.ListID, t.Position, id)
	
	if err != nil {
		return c.Status(500).SendString(err.Error())
	}

	return c.JSON(t)
}
