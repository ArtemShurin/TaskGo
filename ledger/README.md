# Ledger

Бизнес-логика учёта трат и бюджетов. Данные хранятся в **PostgreSQL**
(таблицы `budgets` и `expenses`), а результаты отчёта и список бюджетов
кешируются в **Redis** с коротким TTL. Gateway вызывает функции пакета
`ledger/store` и работает с теми же базой и кешем.

## Структура

| Пакет / папка        | Назначение |
|----------------------|------------|
| `store`              | доменные модели, `Validate()`, работа с базой и кешем |
| `db`                 | подключение к PostgreSQL, строка подключения из env, пул соединений |
| `cache`              | клиент Redis (`go-redis/v9`), настройки из env, JSON-хелперы |
| `migrations`         | SQL-миграции goose (встроены в код через `embed` для тестов) |
| `dbtest`             | хелперы тестов: тестовая база `cashapp_test` и Redis db `15` |
| `docker-compose.yml` | PostgreSQL 17 и Redis 7 для локальной разработки |
| `main.go`            | демонстрационный запуск |

Пакеты `db` и `cache` не `internal`, потому что ими пользуется и Gateway.

## Хранение данных

| Функция                          | Что делает |
|----------------------------------|------------|
| `SetBudget(b)`                   | `INSERT ... ON CONFLICT (category) DO UPDATE SET limit_amount = EXCLUDED.limit_amount`, затем `DEL budgets:all` |
| `ListBudgets()`                  | `budgets:all` из Redis или `SELECT category, limit_amount FROM budgets ORDER BY category` |
| `AddTransaction(tx)`             | лимит категории → `COALESCE(SUM(amount), 0)` по категории → если `spent + amount > limit`, ошибка `budget exceeded` без вставки, иначе `INSERT ... RETURNING id` |
| `ListTransactions()`             | `SELECT id, amount, category, description, date FROM expenses ORDER BY date DESC, id DESC` |
| `GetReportSummary(ctx, from, to)`| кеш `report:summary:<from>:<to>` или `SUM(amount) ... GROUP BY category` за период |
| `LoadBudgets(r)`                 | проверяет все бюджеты из JSON и сохраняет их в одной SQL-транзакции |
| `Reset()`                        | `TRUNCATE expenses, budgets RESTART IDENTITY` и очистка ключей кеша — только для тестов |

Особенности:

- перед записью всегда вызывается `Validate()`: сумма или лимит от `0.01`
  и меньше `10^12` (границы `NUMERIC(14,2)`), категория не пустая, дата в
  формате `YYYY-MM-DD`;
- если у категории нет бюджета, трата добавляется без ограничений;
- поле `period` в базе не хранится, бюджеты месячные, API возвращает
  `"period": "month"`;
- суммы сравниваются в копейках, чтобы не было ошибок округления `float64`;
- проверка лимита и вставка выполняются без SQL-транзакции (допускается
  заданием): внутри процесса их защищает мьютекс, но при одновременной записи
  из двух процессов лимит теоретически может быть превышен.

## 1. Запуск PostgreSQL и Redis

```
cd ledger
docker compose up -d --wait
```

Поднимаются два контейнера:

- `ledger-postgres` — PostgreSQL на `localhost:5432`, база `cashapp`,
  пользователь и пароль `postgres`;
- `cashcraft-redis` — Redis на `localhost:6379`.

Остановить: `docker compose down`. Данные PostgreSQL сохраняются в volume
`ledger-pgdata`; чтобы удалить и их, используйте `docker compose down -v`.

Можно использовать и свой PostgreSQL. Тогда создайте базу вручную:

```
createdb -U postgres cashapp
```

## 2. Миграции goose

Установка:

```
go install github.com/pressly/goose/v3/cmd/goose@latest
```

| Файл                        | Что создаёт |
|-----------------------------|-------------|
| `00001_create_budgets.sql`  | `budgets`: `id SERIAL`, `category TEXT UNIQUE NOT NULL`, `limit_amount NUMERIC(14,2) CHECK (> 0)` |
| `00002_create_expenses.sql` | `expenses`: `id SERIAL`, `amount NUMERIC(14,2) CHECK (<> 0)`, `category`, `description`, `date DATE`; индекс `(category, date)` |

Каждый файл содержит секции `-- +goose Up` и `-- +goose Down`.

```
export DATABASE_URL="postgres://postgres:postgres@localhost:5432/cashapp?sslmode=disable"

goose -dir ./migrations postgres "$DATABASE_URL" up      # применить все
goose -dir ./migrations postgres "$DATABASE_URL" status  # что применено
goose -dir ./migrations postgres "$DATABASE_URL" down    # откатить последнюю
```

Миграции применяются **до** первого запуска Ledger и после каждого изменения
схемы.

## 3. Запуск Ledger

Переменные окружения:

| Переменная       | По умолчанию | Назначение |
|------------------|--------------|------------|
| `DATABASE_URL`   | —            | строка подключения к PostgreSQL; если задана, остальные `DB_*` не используются |
| `DB_HOST`        | `localhost`  | хост PostgreSQL |
| `DB_PORT`        | `5432`       | порт |
| `DB_USER`        | `postgres`   | пользователь |
| `DB_PASS`        | `postgres`   | пароль |
| `DB_NAME`        | `cashapp`    | база |
| `REDIS_ADDR`     | `localhost:6379` | адрес Redis |
| `REDIS_DB`       | `0`          | номер логической базы Redis |
| `REDIS_PASSWORD` | не задан     | пароль Redis |

Запуск (из папки `ledger`, чтобы `budgets.json` нашёлся по относительному пути):

```
export DATABASE_URL="postgres://postgres:postgres@localhost:5432/cashapp?sslmode=disable"
export REDIS_ADDR=localhost:6379 REDIS_DB=0
go run .
```

В PowerShell переменные задаются так: `$env:DATABASE_URL = "postgres://..."`.

Ожидаемый лог при старте (пароль скрыт):

```
Ledger service started
connected to database postgres://postgres:xxxxx@localhost:5432/cashapp?sslmode=disable
connected to redis localhost:6379 (db 0)
```

Затем программа загружает бюджеты из `budgets.json` (повторный запуск их
просто обновляет), показывает валидацию через интерфейс `Validatable` и
функцию `CheckValid` (в базу ничего не пишется), выводит бюджеты и траты из
базы и дважды строит отчёт с 2025-01-01 по сегодня:

```
cache miss: report:summary:2025-01-01:2026-11-01
total 1270.00, categories 3, took 3.9ms
cache hit: report:summary:2025-01-01:2026-11-01
total 1270.00, categories 3, took 0s
```

Если PostgreSQL недоступен, Ledger выводит причину и подсказку, какие
переменные проверить, и завершается с кодом 1. Если недоступен Redis,
Ledger пишет `cache disabled, working without Redis` и работает напрямую с
базой.

### Пул соединений

После `sql.Open` задаются параметры пула (константы в `db/db.go`):

| Параметр                | Значение |
|-------------------------|----------|
| `SetMaxOpenConns`       | 10 |
| `SetMaxIdleConns`       | 5 |
| `SetConnMaxLifetime`    | 30 минут |

## Кеш Redis

| Ключ                         | Что хранится              | TTL  | Сброс |
|------------------------------|---------------------------|------|-------|
| `report:summary:<from>:<to>` | отчёт целиком в JSON      | 30 с | только по TTL |
| `budgets:all`                | список бюджетов в JSON    | 20 с | `DEL` после `SetBudget` и `LoadBudgets` |

Пример значения отчёта:

```json
{"from":"2025-09-01","to":"2025-09-30","total":1270,
 "by_category":[{"category":"еда","total":450},{"category":"кофе","total":120},{"category":"спорт","total":700}]}
```

Отчёт сбрасывается только по TTL, поэтому новая трата появляется в уже
закешированном отчёте не позже чем через 30 секунд. Если Redis перестал
отвечать во время работы, ошибка пишется в лог, а данные берутся из базы.
Чтобы при этом запросы не зависали, у клиента Redis короткие таймауты
(200 мс на подключение, чтение и запись), одна попытка подключения и без
повторов команд; после неудачного чтения запись в кеш не выполняется. Когда
Redis снова доступен, кеш начинает работать без перезапуска сервиса.

Посмотреть ключи:

```
docker exec cashcraft-redis redis-cli --scan
docker exec cashcraft-redis redis-cli ttl "report:summary:2025-09-01:2025-09-30"
```

## Тесты

```
go test ./... -v
```

Нужны запущенные PostgreSQL и Redis. Тесты не трогают рабочие данные:

- база — `TEST_DATABASE_URL`, по умолчанию
  `postgres://postgres:postgres@localhost:5432/cashapp_test?sslmode=disable`;
  хелпер `dbtest` сам создаёт её и применяет миграции;
- Redis — `TEST_REDIS_ADDR` (по умолчанию `localhost:6379`) и логическая
  база `TEST_REDIS_DB` (по умолчанию `15`), которая очищается до и после теста.

Если PostgreSQL или Redis недоступен, зависящие от них тесты помечаются
`SKIP` с причиной. Тесты Ledger и Gateway используют одну тестовую базу,
поэтому запускайте модули по очереди.

| Файл                   | Что проверяет |
|------------------------|---------------|
| `ledger_test.go`       | табличные тесты `Validate()` и `CheckValid`, сценарий бюджета (в пределах, превышение, ровно на лимите), порядок `ListTransactions`, UPSERT, копейки, атомарность `LoadBudgets`, `Reset` |
| `report_test.go`       | агрегация отчёта и границы периода, неверный период, cache miss с TTL, cache hit, сброс `budgets:all`, работа без Redis, параметры пула |
| `db/db_test.go`        | строка подключения из env и скрытие пароля |
| `cache/cache_test.go`  | настройки Redis из env |

Покрытие (`go test ./... -coverpkg=./... -coverprofile=cover.out`): модуль
целиком ~72%, пакет `store` ~88%. Не покрыты демонстрационная `main()` и
обработка сбоев самой базы.
