package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"pr-reviewer-service/internal/handlers"

	"pr-reviewer-service/internal/repository"
	"pr-reviewer-service/internal/service"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
)

func main() {
	// Первым делом я запускаю миграции.
	// Это нужно, чтобы убедиться, что структура БД правильная перед тем, как сервис начнет работать.
	if err := runMigrations(); err != nil {
		log.Fatalf("Не получилось выполнить миграции: %v", err)
	}

	// Теперь можно подключаться к базе данных
	db, err := connectDB()
	if err != nil {
		log.Fatalf("Не получилось подключиться к базе: %v", err)
	}
	defer db.Close()

	// Тут я "собираю" все части сервиса вместе.
	// Repository - это для общения с базой данных.
	// Service - тут вся основная логика.
	// Handlers - это для обработки HTTP-запросов.
	repo := repository.NewRepository(db)
	svc := service.NewService(repo)
	h := handlers.NewHandlers(svc)

	// Настраиваю все эндпоинты (ручки API)
	router := setupRouter(h)

	// Запускаю сервер. По умолчанию на порту 8080.
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Сервер запускается на порту %s", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Не удалось запустить сервер: %v", err)
	}
}

func connectDB() (*sql.DB, error) {
	host := getEnv("DB_HOST", "localhost")
	port := getEnv("DB_PORT", "5432")
	user := getEnv("DB_USER", "pr_reviewer")
	password := getEnv("DB_PASSWORD", "pr_reviewer_pass")
	dbname := getEnv("DB_NAME", "pr_reviewer_db")
	sslmode := getEnv("DB_SSLMODE", "disable")

	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		host, port, user, password, dbname, sslmode)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}

func runMigrations() error {
	host := getEnv("DB_HOST", "localhost")
	port := getEnv("DB_PORT", "5432")
	user := getEnv("DB_USER", "pr_reviewer")
	password := getEnv("DB_PASSWORD", "pr_reviewer_pass")
	dbname := getEnv("DB_NAME", "pr_reviewer_db")
	sslmode := getEnv("DB_SSLMODE", "disable")

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		user, password, host, port, dbname, sslmode)

	// Ждём готовности БД
	maxRetries := 10
	for i := 0; i < maxRetries; i++ {
		db, err := sql.Open("postgres", dsn)
		if err == nil {
			if err := db.Ping(); err == nil {
				db.Close()
				break
			}
			db.Close()
		}
		if i < maxRetries-1 {
			log.Printf("Waiting for database... (attempt %d/%d)", i+1, maxRetries)
			time.Sleep(2 * time.Second)
		} else {
			return fmt.Errorf("database is not available after %d attempts", maxRetries)
		}
	}

	migrationsPath := getEnv("MIGRATIONS_PATH", "file://migrations")

	m, err := migrate.New(migrationsPath, dsn)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	log.Println("Migrations applied successfully")
	return nil
}

func setupRouter(h *handlers.Handlers) *gin.Engine {
	router := gin.Default()

	// Health check (без аутентификации)
	router.GET("/health", h.HealthCheck)

	// Teams
	router.POST("/team/add", h.CreateTeam)
	router.GET("/team/get", h.GetTeam)

	// Users
	router.POST("/users/setIsActive", h.SetUserActive)
	router.GET("/users/getReview", h.GetReview)

	// Pull Requests
	router.POST("/pullRequest/create", h.CreatePullRequest)
	router.POST("/pullRequest/merge", h.MergePullRequest)
	router.POST("/pullRequest/reassign", h.ReassignReviewer)

	return router
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
