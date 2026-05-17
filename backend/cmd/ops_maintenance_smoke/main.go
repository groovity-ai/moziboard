package main

import (
  "context"
  "fmt"
  "log"
  "os"

  "github.com/jackc/pgx/v5/pgxpool"
  opsmodule "moziboard-backend/internal/modules/ops"
)

func main() {
  ctx := context.Background()
  dsn := os.Getenv("SMOKE_DATABASE_URL")
  if dsn == "" {
    dsn = "postgres://moziboard:moziboard_secret@moziboard-db:5432/moziboard?sslmode=disable"
  }
  db, err := pgxpool.New(ctx, dsn)
  if err != nil { log.Fatal(err) }
  defer db.Close()

  svc := opsmodule.NewService(opsmodule.NewRepository(db), nil, nil)
  report, err := svc.RunMaintenanceSweep(ctx)
  if err != nil { log.Fatal(err) }
  fmt.Printf("MAINTENANCE_OK stale_agents=%d stale_runs=%d repaired=%d\n", report.StaleAgentsMarked, report.StaleRunsClosed, report.DriftedTasksRepaired)
}
