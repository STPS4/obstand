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

type Order struct {
	ID     string `json:"id"`
	UserID string `json:"user_id"`
	Item   string `json:"item"`
	Price  int    `json:"price"`
}

type Response struct {
	User   *User   `json:"user,omitempty"`
	Orders []Order `json:"orders,omitempty"`
	Error  string  `json:"error,omitempty"`
}

func main() {
	log := obs.NewLogger("gateway")
	reg := obs.NewRegistry()
	metrics := obs.NewMetrics(reg)

	addr := getenv("ADDR", ":8080")
	usersURL := getenv("USERS_URL", "http://localhost:8081")
	ordersURL := getenv("ORDERS_URL", "http://localhost:8082")

	// Один Client на зависимость: target становится лейблом в метриках
	// исходящих запросов, поэтому "users" и "orders" различимы в графиках.
	usersClient := httpx.NewClient("users", usersURL, log, metrics)
	ordersClient := httpx.NewClient("orders", ordersURL, log, metrics)

	root := http.NewServeMux()
	root.Handle("GET /metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{Registry: reg}))

	api := http.NewServeMux()
	api.HandleFunc("GET /api/users/{id}", aggregateHandler(log, usersClient, ordersClient))
	root.Handle("/", httpx.Observe(log, metrics)(api))

	srv := &http.Server{
		Addr:              addr,
		Handler:           root,
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Info("listening", "addr", addr, "users_url", usersURL, "orders_url", ordersURL)
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

// aggregateHandler последовательно опрашивает users и orders и склеивает
// их ответы в один JSON. Оба вызова идут через httpx.Client, поэтому
// попадают в метрики http_client_requests_total.
func aggregateHandler(log *slog.Logger, users, orders *httpx.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		ctx := r.Context()

		var user User
		if err := users.GetJSON(ctx, "/users/"+id, &user); err != nil {
			log.Warn("users call failed", "user_id", id, "err", err)
			writeJSON(w, http.StatusBadGateway, Response{Error: "users unavailable: " + err.Error()})
			return
		}

		var userOrders []Order
		if err := orders.GetJSON(ctx, "/orders?user_id="+id, &userOrders); err != nil {
			log.Warn("orders call failed", "user_id", id, "err", err)
			writeJSON(w, http.StatusBadGateway, Response{Error: "orders unavailable: " + err.Error()})
			return
		}

		writeJSON(w, http.StatusOK, Response{User: &user, Orders: userOrders})
	}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
