# obstand — стенд для оценки наблюдаемости микросервисов

Минимальный экспериментальный стенд из 3 микросервисов на Go, общающихся по HTTP.
Каждый сервис экспортирует метрики в формате Prometheus и пишет структурные JSON-логи.

## Архитектура

```
client ──HTTP──▶ gateway:8080
                   ├──▶ users:8081    GET /users/{id}
                   └──▶ orders:8082   GET /orders?user_id={id}
```

Топология «звезда»: gateway вызывает users и orders последовательно,
склеивает их ответы в один JSON.

## Сервисы

| Сервис  | Порт | Эндпоинты                               |
|---------|------|-----------------------------------------|
| gateway | 8080 | `GET /api/users/{id}`, `GET /metrics`   |
| users   | 8081 | `GET /users/{id}`, `GET /metrics`       |
| orders  | 8082 | `GET /orders?user_id=X`, `GET /metrics` |

## Запуск

Требуется Docker и Docker Compose.

```bash
docker compose up --build
```

При первом запуске собираются три образа (multi-stage build на основе
`golang:1.23-alpine`, итоговый рантайм — `alpine:3.19`, ~30 МБ на сервис).

## Проверка

```bash
# Полный сценарий через gateway:
curl -s 'http://localhost:8080/api/users/42' | jq .

# Прямые вызовы отдельных сервисов:
curl -s 'http://localhost:8081/users/42'
curl -s 'http://localhost:8082/orders?user_id=42'

# Метрики:
curl -s http://localhost:8080/metrics | grep ^http_
```

## Что наблюдается

Метрики (Prometheus exposition format, `/metrics`):

- `http_requests_total{method,path,status}` — счётчик входящих запросов
- `http_request_duration_seconds_bucket{method,path,le}` — гистограмма латентности
- `http_client_requests_total{target,method,status}` — счётчик исходящих запросов gateway
- `http_client_request_duration_seconds_bucket{target,method,le}` — их латентность
- стандартные `go_*` и `process_*` метрики (goroutines, GC, RSS, CPU)

Лейбл `path` использует шаблон роута (`/users/{id}`), а не реальный URL,
чтобы избежать cardinality explosion в Prometheus.

Логи (JSON, stdout):

- `service` — имя сервиса (gateway/users/orders)
- `http_request` — событие на каждый входящий запрос: method, path, route, status, duration_ms
- `http_client_request` — событие на каждый исходящий запрос: target, url, status, duration_ms
- бизнес-события (`user_lookup`, …)

## Структура проекта

```
.
├── cmd/                    # точки входа сервисов (по одному main на каждый)
│   ├── gateway/
│   ├── users/
│   └── orders/
├── internal/
│   ├── obs/                # общий код наблюдаемости: логер и метрики
│   └── httpx/              # middleware и HTTP-клиент с метриками
├── Dockerfile              # multi-stage, параметризован ARG SERVICE
└── docker-compose.yml      # три сервиса + общая сеть
```

## Остановка

```bash
docker compose down
```

## Дальнейшие шаги

В следующих итерациях стенда планируется добавить:

- Prometheus + Grafana для сбора и визуализации метрик
- параллельные вызовы users и orders в gateway через `errgroup`
- unit-тесты и GitHub Actions (build, test, golangci-lint)
- Loki + Promtail для централизованного сбора логов
- Jaeger + OpenTelemetry для распределённого трейсинга
- toxiproxy для fault injection между сервисами
