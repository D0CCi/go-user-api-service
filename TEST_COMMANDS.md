# Команды для тестирования API

Здесь я собрал все команды для проверки функционала сервиса. Можно копировать и выполнять по порядку.

## Базовые сценарии

### 1. Создание команд

```bash
# Команда Alpha с несколькими пользователями
curl -X POST http://localhost:8080/team/add \
  -H "Content-Type: application/json" \
  -d '{
    "team_name": "Alpha",
    "members": [
      {"user_id": "alice", "username": "Alice", "is_active": true},
      {"user_id": "bob", "username": "Bob", "is_active": true},
      {"user_id": "charlie", "username": "Charlie", "is_active": true},
      {"user_id": "eve", "username": "Eve", "is_active": false}
    ]
  }'

# Команда Beta с двумя пользователями
curl -X POST http://localhost:8080/team/add \
  -H "Content-Type: application/json" \
  -d '{
    "team_name": "Beta",
    "members": [
      {"user_id": "frank", "username": "Frank", "is_active": true},
      {"user_id": "grace", "username": "Grace", "is_active": true}
    ]
  }'

# Команда Gamma с одним пользователем
curl -X POST http://localhost:8080/team/add \
  -H "Content-Type: application/json" \
  -d '{
    "team_name": "Gamma",
    "members": [
      {"user_id": "heidy", "username": "Heidy", "is_active": true}
    ]
  }'
```

### 2. Создание PR с автоназначением ревьюеров

```bash
# PR с двумя ревьюерами (из команды автора)
curl -X POST http://localhost:8080/pullRequest/create \
  -H "Content-Type: application/json" \
  -d '{
    "pull_request_id": "pr-1",
    "pull_request_name": "Happy Path Test",
    "author_id": "alice"
  }'

# PR с одним ревьюером (в команде только 2 человека)
curl -X POST http://localhost:8080/pullRequest/create \
  -H "Content-Type: application/json" \
  -d '{
    "pull_request_id": "pr-2",
    "pull_request_name": "One Reviewer Test",
    "author_id": "frank"
  }'

# PR без ревьюеров (автор один в команде)
curl -X POST http://localhost:8080/pullRequest/create \
  -H "Content-Type: application/json" \
  -d '{
    "pull_request_id": "pr-3",
    "pull_request_name": "No Reviewers Test",
    "author_id": "heidy"
  }'
```

### 3. Переназначение ревьюера

```bash
# Успешное переназначение (когда есть кандидаты)
curl -X POST http://localhost:8080/pullRequest/reassign \
  -H "Content-Type: application/json" \
  -d '{
    "pull_request_id": "pr-1",
    "old_user_id": "bob"
  }'

# Ошибка NO_CANDIDATE (когда нет доступных кандидатов)
curl -X POST http://localhost:8080/pullRequest/reassign \
  -H "Content-Type: application/json" \
  -d '{
    "pull_request_id": "pr-2",
    "old_user_id": "grace"
  }'
```

### 4. Мерж PR

```bash
# Мерж PR
curl -X POST http://localhost:8080/pullRequest/merge \
  -H "Content-Type: application/json" \
  -d '{
    "pull_request_id": "pr-1"
  }'

# Проверка идемпотентности (повторный мерж не должен вызывать ошибку)
curl -X POST http://localhost:8080/pullRequest/merge \
  -H "Content-Type: application/json" \
  -d '{
    "pull_request_id": "pr-1"
  }'

# Попытка переназначения на мерженном PR (должна быть ошибка)
curl -X POST http://localhost:8080/pullRequest/reassign \
  -H "Content-Type: application/json" \
  -d '{
    "pull_request_id": "pr-1",
    "old_user_id": "charlie"
  }'
```

### 5. Управление активностью пользователей

```bash
# Деактивация пользователя
curl -X POST http://localhost:8080/users/setIsActive \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "grace",
    "is_active": false
  }'

# Проверка, что неактивный пользователь не назначается
curl -X POST http://localhost:8080/pullRequest/create \
  -H "Content-Type: application/json" \
  -d '{
    "pull_request_id": "pr-4",
    "pull_request_name": "Inactive Reviewer Check",
    "author_id": "frank"
  }'
```

### 6. Получение данных

```bash
# Получить команду
curl -X GET "http://localhost:8080/team/get?team_name=Alpha"

# Получить PR пользователя
curl -X GET "http://localhost:8080/users/getReview?user_id=charlie"

# Получить статистику
curl -X GET http://localhost:8080/statistics
```

## Дополнительные функции

### 7. Массовая деактивация команды

```bash
# Создаем команду для теста
curl -X POST http://localhost:8080/team/add \
  -H "Content-Type: application/json" \
  -d '{
    "team_name": "TestTeam",
    "members": [
      {"user_id": "t1", "username": "Test1", "is_active": true},
      {"user_id": "t2", "username": "Test2", "is_active": true},
      {"user_id": "t3", "username": "Test3", "is_active": true}
    ]
  }'

# Создаем PR с ревьюерами
curl -X POST http://localhost:8080/pullRequest/create \
  -H "Content-Type: application/json" \
  -d '{
    "pull_request_id": "pr-bulk-test",
    "pull_request_name": "Bulk Test",
    "author_id": "t1"
  }'

# Деактивируем команду (должно переназначить PR, если есть активные в команде автора)
curl -X POST http://localhost:8080/team/bulkDeactivate \
  -H "Content-Type: application/json" \
  -d '{"team_name": "TestTeam"}'
```

## Тесты обработки ошибок

### 8. Проверка ошибок

```bash
# Создание PR с существующим ID (должна быть ошибка PR_EXISTS)
curl -X POST http://localhost:8080/pullRequest/create \
  -H "Content-Type: application/json" \
  -d '{
    "pull_request_id": "pr-1",
    "pull_request_name": "Duplicate",
    "author_id": "alice"
  }'

# Создание команды с существующим именем (должна быть ошибка TEAM_EXISTS)
curl -X POST http://localhost:8080/team/add \
  -H "Content-Type: application/json" \
  -d '{
    "team_name": "Alpha",
    "members": []
  }'

# Получение несуществующей команды (должна быть ошибка 404)
curl -X GET "http://localhost:8080/team/get?team_name=NonExistent"

# Получение несуществующего пользователя (должна быть ошибка 404)
curl -X GET "http://localhost:8080/users/getReview?user_id=non_existent"

# Мерж несуществующего PR (должна быть ошибка 404)
curl -X POST http://localhost:8080/pullRequest/merge \
  -H "Content-Type: application/json" \
  -d '{"pull_request_id": "non-existent"}'

# Переназначение несуществующего ревьюера (должна быть ошибка NOT_ASSIGNED)
curl -X POST http://localhost:8080/pullRequest/reassign \
  -H "Content-Type: application/json" \
  -d '{
    "pull_request_id": "pr-2",
    "old_user_id": "non_existent"
  }'

# Создание PR с несуществующим автором (должна быть ошибка 404)
curl -X POST http://localhost:8080/pullRequest/create \
  -H "Content-Type: application/json" \
  -d '{
    "pull_request_id": "pr-invalid-author",
    "pull_request_name": "Invalid Author",
    "author_id": "non_existent"
  }'
```

## Health check

```bash
curl -X GET http://localhost:8080/health
```

