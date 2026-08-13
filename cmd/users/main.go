package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"obstand/internal/httpx"
	"obstand/internal/obs"
)

type User struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

func main() {
	log := obs.NewLogger("users")
	reg := obs.NewRegistry()
	metrics := obs.NewMetrics(reg)

	addr := getenv("ADDR", ":8081")

	root := http.NewServeMux()
	root.Handle("GET /metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{Registry: reg}))

	api := http.NewServeMux()
	api.HandleFunc("GET /users/{id}", usersHandler(log))
	root.Handle("/", httpx.Observe(log, metrics)(api))

	srv := &http.Server{
		Addr:              addr,
		Handler:           root,
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Info("listening", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server failed", "err", err)
			stop()
		}
	}()

	<-ctx.Done()
	log.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("shutdown error", "err", err)
	}
}

func usersHandler(log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		// Чистый stub: возвращаем «найденного» пользователя для любого id.
		// В будущем сюда удобно вкорячивать инжекции ошибок для отдельных id —
		// например, id="404" → 404, id="slow" → time.Sleep.
		u := User{
			ID:    id,
			Name:  "User " + id,
			Email: "user" + id + "@example.com",
		}

		log.Info("user_lookup", "user_id", id)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(u)
	}
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
