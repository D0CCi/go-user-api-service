.PHONY: build run test clean docker-build docker-up docker-down migrate-up migrate-down

# Переменные
APP_NAME=pr-reviewer-service
DOCKER_COMPOSE=docker-compose

# Сборка приложения
build:
	go build -o bin/$(APP_NAME) ./cmd/server

# Запуск приложения локально (требует запущенной БД)
run:
	go run ./cmd/server

# Запуск тестов
test:
	go test -v ./...

# Очистка
clean:
	rm -rf bin/
	go clean

# Docker команды
docker-build:
	$(DOCKER_COMPOSE) build

docker-up:
	$(DOCKER_COMPOSE) up -d

docker-down:
	$(DOCKER_COMPOSE) down

docker-logs:
	$(DOCKER_COMPOSE) logs -f app

# Полный запуск через Docker
start: docker-build docker-up
	@echo "Сервис запущен. Доступен на http://localhost:8080"

# Остановка
stop: docker-down

# Миграции (для локальной разработки)
migrate-up:
	migrate -path migrations -database "postgres://pr_reviewer:pr_reviewer_pass@localhost:5432/pr_reviewer_db?sslmode=disable" up

migrate-down:
	migrate -path migrations -database "postgres://pr_reviewer:pr_reviewer_pass@localhost:5432/pr_reviewer_db?sslmode=disable" down

# Установка зависимостей
deps:
	go mod download
	go mod tidy


