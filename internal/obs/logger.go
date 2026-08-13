package obs

import (
	"log/slog"
	"os"
)

// NewLogger возвращает JSON-логер с проставленным именем сервиса.
// JSON выбран осознанно: его легко парсит Loki/Promtail/любой shipper.
func NewLogger(service string) *slog.Logger {
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	return slog.New(h).With("service", service)
}
