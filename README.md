# TaskGo

Два Go-сервиса, каждый — отдельный Go-модуль в своей папке:

- **[Ledger](ledger/README.md)** — бизнес-логика учёта трат и бюджетов.
  Данные хранятся в **PostgreSQL**, схема создаётся миграциями goose.
  Результаты отчёта и список бюджетов кешируются в **Redis** с коротким TTL.
- **[Gateway](gateway/README.md)** — HTTP API на порту `8080`. Обращается к тем
  же HTTP-эндпоинтам, что и раньше; разница в том, что под ними теперь
  PostgreSQL, а не память процесса, поэтому данные переживают перезапуск.

```
curl ──► Gateway :8080 ──► ledger/store ──► PostgreSQL (budgets, expenses)
                                   └──────► Redis (report:summary:*, budgets:all)
```

## Быстрый старт

Команды для Git Bash из корня репозитория (в PowerShell переменные задаются
через `$env:NAME = "..."`).

1. PostgreSQL и Redis в Docker:

   ```
   docker compose -f ledger/docker-compose.yml up -d --wait
   ```

2. Переменные окружения:

   ```
   export DATABASE_URL="postgres://postgres:postgres@localhost:5432/cashapp?sslmode=disable"
   export REDIS_ADDR=localhost:6379 REDIS_DB=0
   ```

3. Миграции (goose: `go install github.com/pressly/goose/v3/cmd/goose@latest`):

   ```
   goose -dir ./ledger/migrations postgres "$DATABASE_URL" up
   ```

   Откатить последнюю: `goose -dir ./ledger/migrations postgres "$DATABASE_URL" down`.

4. Только после успешного `goose up` — Ledger:

   ```
   cd ledger && go run .
   ```

5. Gateway и проверка:

   ```
   cd gateway && go run .
   curl http://localhost:8080/ping
   ```

Подробности, переменные окружения и примеры cURL (включая двойной запрос
`GET /api/reports/summary` для проверки кеша) — в README сервисов.

## Тесты и покрытие

Нужны запущенные PostgreSQL и Redis. Тесты используют отдельную базу
`cashapp_test` и Redis db `15`, рабочие данные не трогают. Модули
запускаются по очереди:

```
cd ledger && go test ./... -v
cd gateway && go test ./... -v
```

Отчёт о покрытии (в каждом модуле; для Ledger добавьте `-coverpkg=./...`,
потому что тесты лежат в пакете `main`, а код — в подпакетах):

```
go test ./... -cover -coverprofile=cover.out
go tool cover -html=cover.out -o cover.html
```

| Модуль / пакет          | Покрытие |
|-------------------------|----------|
| `gateway/internal/api`  | 90.8%    |
| `ledger` (весь модуль)  | 71.6%    |
| `ledger/store`          | 87.6%    |

Не покрыты демонстрационные `main()` и обработка сбоев самой базы.
Файлы `cover.out` и `cover.html` добавлены в `.gitignore`.
