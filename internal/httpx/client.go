package httpx

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"obstand/internal/obs"
)

// Client — обёртка над http.Client с метриками исходящих запросов.
// Имя target используется как лейбл в метриках: это логическое имя
// зависимости ("users", "orders"), а не URL.
type Client struct {
	target string
	base   string // базовый URL зависимости, например "http://localhost:8081"
	http   *http.Client
	log    *slog.Logger
	m      *obs.Metrics
}

func NewClient(target, baseURL string, log *slog.Logger, m *obs.Metrics) *Client {
	return &Client{
		target: target,
		base:   baseURL,
		http: &http.Client{
			// Жёсткий потолок на весь запрос, включая TCP-handshake, TLS,
			// отправку, ожидание и чтение ответа. Без этого goroutines
			// могут копиться вечно, если бэкенд "залип".
			Timeout: 3 * time.Second,
		},
		log: log,
		m:   m,
	}
}

// GetJSON делает GET по relativePath, парсит JSON в out.
// Контекст ctx пробрасывается из вызывающего handler'а: если клиент
// gateway оборвал соединение, ctx отменится, и этот запрос тоже завершится.
func (c *Client) GetJSON(ctx context.Context, relativePath string, out any) error {
	url := c.base + relativePath
	method := http.MethodGet

	start := time.Now()
	status := "error" // дефолт на случай сетевой ошибки — статус-кода ещё нет

	// defer выполнится даже при early return, поэтому метрики попадают на
	// /metrics независимо от исхода — нет "потерянных" запросов.
	defer func() {
		dur := time.Since(start)
		c.m.ClientRequestsTotal.
			WithLabelValues(c.target, method, status).
			Inc()
		c.m.ClientRequestDuration.
			WithLabelValues(c.target, method).
			Observe(dur.Seconds())

		c.log.Info("http_client_request",
			"target", c.target,
			"method", method,
			"url", url,
			"status", status,
			"duration_ms", dur.Milliseconds(),
		)
	}()

	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		// status остаётся "error" — это и есть сигнал "запрос не дошёл".
		// Различать таймаут, refused connection и DNS-фейл можно через
		// errors.As (net.OpError и т.д.), но для стенда хватит одной категории.
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	status = strconv.Itoa(resp.StatusCode)

	if resp.StatusCode >= 400 {
		// читаем тело только для лога, не больше 1 KiB чтобы не разнести память
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("non-2xx from %s: %d %s", c.target, resp.StatusCode, string(body))
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}
