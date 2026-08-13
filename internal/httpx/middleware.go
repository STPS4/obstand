package httpx

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"obstand/internal/obs"
)

// statusRecorder — обёртка над ResponseWriter, чтобы middleware узнал
// итоговый статус после того, как handler отработал.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// Observe оборачивает handler в замер длительности, инкремент счётчика
// и структурированный лог о запросе.
func Observe(log *slog.Logger, m *obs.Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(rec, r)

			dur := time.Since(start)

			// r.Pattern — это паттерн ServeMux, в который сматчился запрос
			// (например "GET /users/{id}"). Берём именно его, а не r.URL.Path,
			// иначе каждый уникальный id создаёт новую time series в Prometheus —
			// это cardinality explosion.
			path := routePattern(r)

			m.RequestsTotal.
				WithLabelValues(r.Method, path, strconv.Itoa(rec.status)).
				Inc()
			m.RequestDuration.
				WithLabelValues(r.Method, path).
				Observe(dur.Seconds())

			log.Info("http_request",
				"method", r.Method,
				"path", r.URL.Path, // в логи кладём реальный путь — он нужен для debug
				"route", path, // и шаблон — он нужен для корреляции с метриками
				"status", rec.status,
				"duration_ms", dur.Milliseconds(),
				"remote", r.RemoteAddr,
			)
		})
	}
}

// routePattern возвращает шаблон совпавшего роута без HTTP-метода.
// Если паттерн пустой или совпал catch-all корневого mux'а ("/"),
// возвращаем "unknown" — чтобы незарегистрированные пути не создавали
// уникальных серий по их реальным URL.
func routePattern(r *http.Request) string {
	p := r.Pattern
	if p == "" || p == "/" {
		return "unknown"
	}
	// Pattern имеет вид "METHOD /path/{param}" или "/path/{param}".
	// Отрежем метод, если он есть — он уже в отдельном лейбле.
	if i := strings.IndexByte(p, ' '); i >= 0 {
		return p[i+1:]
	}
	return p
}
