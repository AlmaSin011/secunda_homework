# go-task-project

REST API сервис управления задачами с командной работой, ролевой моделью,
историей изменений

Стек: Go 1.25 · MySQL 8 · Redis 7 · Gin · sqlx · go-redis · golang-jwt ·
golang-migrate · Prometheus · testcontainers.

## Быстрый старт (Docker)

```bash
cp .env.example .env             # при необходимости отредактируйте секреты
make up              
make logs                      
make down                      
make down-v                    
```

После `make up` API доступен на `http://localhost:8080`, Prometheus-метрики — на `http://localhost:8080/metrics`.

## Локальная разработка без Docker

Через `make`:

```bash
cp .env.example .env
go mod tidy
make build                      
./bin/app                       # запуск (нужен свой MySQL+Redis)
# или
make run                        # через go run
```

Только через `go` (без `make` и без бинарников):

```bash
cp .env.example .env
go mod tidy
go run ./cmd/app               

# миграции 
go run ./cmd/migrate up         # применить миграции
go run ./cmd/migrate version    # текущая версия БД

# тесты
go test ./...                                # все unit-тесты
go test -tags=integration ./test/integration/...   # e2e (нужен MYSQL_TEST_DSN)

# сборка
go build -o bin/app ./cmd/app
go build -o bin/migrate ./cmd/migrate
```

## Make-цели

```text
make build            собрать bin/app и bin/migrate
make run              запустить ./cmd/app
make test             unit-тесты с покрытием (./internal/... ./pkg/...)
make test-integration интеграционные тесты (-tags=integration), нужен MYSQL_TEST_DSN
make cover            HTML-отчёт о покрытии
make tidy             go mod tidy
make up / down        docker compose up/down
make down-v           down + удалить тома
make logs             docker compose logs -f
make migrate-create   make migrate-create name=<имя_миграции>
```

## Миграции

```bash
./bin/migrate up                  # применить миграции
./bin/migrate down                # откатить одну
./bin/migrate version             # текущая версия
./bin/migrate force 1             # выставить версию (recovery)
```

DSN берётся из `MYSQL_DSN` (или `MYSQL_TEST_DSN` для тестов).

## Тесты

```bash
make test                                   # unit
MYSQL_TEST_DSN='mysql://... ' make test-integration   # e2e через testcontainers
```

Интеграционные тесты находятся в `test/integration/` (build tag `integration`).

## Эндпоинты

Публичные (без auth): `GET /healthz`, `GET /ping`, `GET /metrics`,
`POST /api/v1/register`, `POST /api/v1/login`.

Под `/api/v1` (требуется Bearer JWT + rate limit 100 req/min на пользователя):
teams, tasks, comments, stats. Полная маршрутизация — в
[`internal/router/router.go`](./internal/router/router.go).

## Конфигурация

Приоритет: defaults → `configs/config.yaml` → ENV. Ключевые переменные в
[`.env.example`](./.env.example). В production обязателен `JWT_SECRET` ≥ 32
символов (валидируется при старте).
