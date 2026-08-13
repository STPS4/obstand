package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"obstand/internal/httpx"
	"obstand/internal/obs"
)

type Order struct {
	ID     string `json:"id"`
	UserID string `json:"user_id"`
	Item   string `json:"item"`
	Price  int    `json:"price"`
}

func main() {
	log := obs.NewLogger("orders")
	reg := obs.NewRegistry()
	metrics := obs.NewMetrics(reg)

	addr := getenv("ADDR", ":8082")

	// Корневой mux: /metrics регистрируем здесь напрямую, ВНЕ middleware Observe.
	// Иначе каждый scrape Prometheus'а инкрементил бы наш же счётчик —
	// /metrics стал бы самой популярной "ручкой" в графиках.
	root := http.NewServeMux()
	root.Handle("GET /metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{Registry: reg}))

	// Бизнес-роуты живут в отдельном mux'е и оборачиваются в Observe.
	api := http.NewServeMux()
	api.HandleFunc("GET /orders", ordersHandler)
	root.Handle("/", httpx.Observe(log, metrics)(api))

	srv := &http.Server{
		Addr:              addr,
		Handler:           root,
		ReadHeaderTimeout: 5 * time.Second, // защита от slow-loris и тихий gosec
	}

	// signal.NotifyContext отменит ctx по SIGINT (Ctrl+C) или SIGTERM
	// (его шлёт `docker stop`). Это наш сигнал начать graceful shutdown.
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

func ordersHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")

	orders := []Order{
		{ID: "o-100", UserID: userID, Item: "book", Price: 1200},
		{ID: "o-101", UserID: userID, Item: "pen", Price: 50},
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(orders)
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
