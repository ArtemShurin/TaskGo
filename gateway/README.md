# Gateway

HTTP API на порту `8080`. Эндпоинты и форматы ответов те же, что и раньше,
но данные теперь хранятся не в памяти, а в **PostgreSQL**: Gateway вызывает
функции `ledger/store`, которые читают и пишут таблицы `budgets` и
`expenses`. Отчёт по тратам кешируется в **Redis** на 30 секунд.

Gateway импортирует Ledger из соседней папки через
`replace ledger => ../ledger` в `go.mod`.

## Запуск

Перед запуском нужны PostgreSQL и Redis с применёнными миграциями
(см. [ledger/README.md](../ledger/README.md)):

```
docker compose -f ../ledger/docker-compose.yml up -d --wait
export DATABASE_URL="postgres://postgres:postgres@localhost:5432/cashapp?sslmode=disable"
goose -dir ../ledger/migrations postgres "$DATABASE_URL" up
```

Запуск Gateway:

```
cd gateway
export DATABASE_URL="postgres://postgres:postgres@localhost:5432/cashapp?sslmode=disable"
export REDIS_ADDR=localhost:6379 REDIS_DB=0
go run .
```

Переменные окружения те же, что у Ledger: `DATABASE_URL` (или `DB_HOST`,
`DB_PORT`, `DB_USER`, `DB_PASS`, `DB_NAME`), `REDIS_ADDR`, `REDIS_DB`,
`REDIS_PASSWORD`. Лог при старте:

```
connected to database postgres://postgres:xxxxx@localhost:5432/cashapp?sslmode=disable
connected to redis localhost:6379 (db 0)
Gateway service started on :8080
```

Без PostgreSQL Gateway завершается с понятной ошибкой. Без Redis он
работает, но отчёт каждый раз считается в базе.

## Эндпоинты

| Метод | Путь                   | Описание                    | Успех            |
|-------|------------------------|-----------------------------|------------------|
| GET   | `/ping`                | проверка доступности        | `200` `"pong"`   |
| POST  | `/api/transactions`    | создать трату               | `201` трата      |
| GET   | `/api/transactions`    | список трат                 | `200` массив     |
| POST  | `/api/budgets`         | создать или обновить бюджет | `201` бюджет     |
| GET   | `/api/budgets`         | список бюджетов             | `200` массив     |
| GET   | `/api/reports/summary?from=YYYY-MM-DD&to=YYYY-MM-DD` | сумма трат по категориям за период (обе даты включительно) | `200` отчёт |

Транзакции читаются из PostgreSQL, бюджеты — из PostgreSQL или из кеша
`budgets:all` (TTL 20 секунд, сбрасывается сразу после `POST /api/budgets`).
Если данных нет — пустой массив `[]`. Транзакции отсортированы по дате и `id`
от новых к старым, бюджеты — по категории.

Все ответы имеют заголовок `Content-Type: application/json; charset=utf-8`,
ошибки — формат `{"error": "описание"}`:

| Статус | Когда |
|--------|-------|
| `400`  | невалидный JSON, ошибка `Validate()`, нет или неверные `from`/`to` |
| `404`  | неизвестный путь |
| `405`  | неподдерживаемый метод |
| `409`  | трата превышает бюджет категории (`budget exceeded`), запись не добавляется |
| `500`  | ошибка базы или другая внутренняя ошибка |

Каждый запрос логируется: метод, путь, статус и длительность.

## Примеры cURL

Создать бюджет — `201`, строка появляется в `budgets`:

```
curl -X POST http://localhost:8080/api/budgets \
  -H "Content-Type: application/json" \
  -d '{"category":"спорт","limit":1000}'
```

Трата в пределах лимита — `201`, строка появляется в `expenses`:

```
curl -X POST http://localhost:8080/api/transactions \
  -H "Content-Type: application/json" \
  -d '{"amount":700,"category":"спорт","description":"абонемент","date":"2025-09-15"}'
```

Трата сверх лимита (700 + 400 > 1000) — `409`, в `expenses` ничего не
добавляется:

```
curl -X POST http://localhost:8080/api/transactions \
  -H "Content-Type: application/json" \
  -d '{"amount":400,"category":"спорт","description":"кроссовки","date":"2025-09-16"}'
```

Ошибка валидации — `400`:

```
curl -X POST http://localhost:8080/api/transactions \
  -H "Content-Type: application/json" \
  -d '{"amount":-5,"category":"еда","date":"2025-09-10"}'
```

Списки из базы:

```
curl http://localhost:8080/api/transactions
curl http://localhost:8080/api/budgets
```

### Эффект кеша

Выполните отчёт дважды подряд:

```
curl -w "\n%{time_total}s\n" "http://localhost:8080/api/reports/summary?from=2025-09-01&to=2025-09-30"
curl -w "\n%{time_total}s\n" "http://localhost:8080/api/reports/summary?from=2025-09-01&to=2025-09-30"
```

Ответы одинаковые:

```json
{"from":"2025-09-01","to":"2025-09-30","total":1270,"by_category":[{"category":"еда","total":450},{"category":"кофе","total":120},{"category":"спорт","total":700}]}
```

а в логе Gateway видно, что первый запрос посчитан в базе, а второй взят из
Redis:

```
cache miss: report:summary:2025-09-01:2025-09-30
GET /api/reports/summary 200 3.0015ms
cache hit: report:summary:2025-09-01:2025-09-30
GET /api/reports/summary 200 1.0002ms
```

Если в течение 30 секунд добавить трату, отчёт за тот же период её ещё не
покажет — он берётся из кеша до истечения TTL.

На Windows в Git Bash curl может отправить кириллицу не в UTF-8. Тогда
передавайте тело из файла: `--data-binary @body.json`.

## Проверка сохранности данных

После запросов выше перезапустите Ledger (`go run .` в папке `ledger`) и
Gateway и снова выполните `GET /api/transactions` и `GET /api/budgets` —
данные на месте, потому что хранятся в PostgreSQL. Проверить таблицы
напрямую:

```
docker exec ledger-postgres psql -U postgres -d cashapp -c "SELECT * FROM budgets" -c "SELECT * FROM expenses"
```

## Структура

`internal/api`:

- `dto.go` — DTO запросов и ответов и функции преобразования в доменные
  модели `store`;
- `handlers.go` — обработчики и запись JSON-ответов;
- `middleware.go` — логирование запросов и заголовок `Content-Type`;
- `router.go` — маршруты и подключение middleware.

## Тесты

```
go test ./... -v
```

`internal/api/handlers_test.go` — интеграционные тесты через `httptest` на
настоящих PostgreSQL (`cashapp_test`) и Redis (db `15`): все эндпоинты,
статусы 201/200/400/404/405/409, формат ошибок, `Content-Type`, отчёт и его
кеширование. Покрытие пакета `internal/api` ~91%.
