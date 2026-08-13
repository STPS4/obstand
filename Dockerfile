# ---- стадия 1: сборка ----
# Используем официальный alpine-образ Go — он лёгкий и в нём есть всё для go build.
FROM golang:1.23-alpine AS builder

WORKDIR /src

# Сначала копируем только go.mod/go.sum и качаем зависимости.
# Этот слой кэшируется: пока эти файлы не изменились, Docker не будет
# перекачивать prometheus/client_golang при каждой пересборке.
COPY go.mod go.sum ./
RUN go mod download

# Теперь копируем весь исходник.
COPY . .

# SERVICE — имя бинаря, передаётся через docker-compose build args.
# Один Dockerfile собирает все три сервиса, отличается только этот аргумент.
ARG SERVICE
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/app ./cmd/${SERVICE}

# ---- стадия 2: рантайм ----
# Alpine вместо scratch, чтобы в контейнере была sh для отладки.
# Можно ужать до scratch (~10 МБ итоговый образ), но для стенда это перебор.
FROM alpine:3.19

# ca-certificates нужны, если бы мы ходили в HTTPS наружу;
# для intra-cluster HTTP не критично, но безвредно и стандартно.
RUN apk add --no-cache ca-certificates

COPY --from=builder /out/app /app

ENTRYPOINT ["/app"]
