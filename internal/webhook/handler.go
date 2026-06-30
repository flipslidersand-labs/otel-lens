// Package webhook provides an HTTP endpoint for receiving deploy events.
package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/flipslidersand/otel-lens/internal/store"
)

// deployPayload is the expected JSON body for POST /events/deploy.
type deployPayload struct {
	ServiceName string `json:"service_name"`
	Version     string `json:"version"`
	Author      string `json:"author"`
	// DeployedAt is optional; defaults to now if omitted.
	DeployedAt string `json:"deployed_at"` // RFC3339
}

// Server wraps an http.Server for the webhook endpoints.
type Server struct {
	st     *store.ClickHouseStore
	logger *zap.Logger
	addr   string
}

func New(addr string, st *store.ClickHouseStore, logger *zap.Logger) *Server {
	return &Server{addr: addr, st: st, logger: logger}
}

func (s *Server) ListenAndServe() error {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /events/deploy", s.handleDeploy)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	s.logger.Info("webhook server listening", zap.String("addr", s.addr))
	return http.ListenAndServe(s.addr, mux)
}

func (s *Server) handleDeploy(w http.ResponseWriter, r *http.Request) {
	var p deployPayload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, fmt.Sprintf("invalid JSON: %v", err), http.StatusBadRequest)
		return
	}
	if p.ServiceName == "" || p.Version == "" {
		http.Error(w, "service_name and version are required", http.StatusBadRequest)
		return
	}

	deployedAt := time.Now().UTC()
	if p.DeployedAt != "" {
		var err error
		deployedAt, err = time.Parse(time.RFC3339, p.DeployedAt)
		if err != nil {
			http.Error(w, fmt.Sprintf("invalid deployed_at: %v", err), http.StatusBadRequest)
			return
		}
	}

	evt := store.DeployEvent{
		DeployedAt:  deployedAt,
		ServiceName: p.ServiceName,
		Version:     p.Version,
		Author:      p.Author,
	}
	if err := s.st.InsertDeployEvent(r.Context(), evt); err != nil {
		s.logger.Error("insert deploy event", zap.Error(err))
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	s.logger.Info("deploy event recorded",
		zap.String("service", p.ServiceName),
		zap.String("version", p.Version),
		zap.Time("deployed_at", deployedAt),
	)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{ //nolint:errcheck
		"status":      "recorded",
		"service":     p.ServiceName,
		"version":     p.Version,
		"deployed_at": deployedAt.Format(time.RFC3339),
	})
}

// ProximityBoost returns a score bonus (0.0–0.3) based on how close the nearest
// deploy event is to the anomaly window midpoint. Closer = higher bonus.
func ProximityBoost(events []store.DeployEvent, from, to time.Time, service string) float64 {
	mid := from.Add(to.Sub(from) / 2)
	minDist := time.Duration(1<<63 - 1) // max duration

	for _, e := range events {
		if service != "" && e.ServiceName != service {
			continue
		}
		d := e.DeployedAt.Sub(mid)
		if d < 0 {
			d = -d
		}
		if d < minDist {
			minDist = d
		}
	}
	if minDist == time.Duration(1<<63-1) {
		return 0
	}
	// Linear decay: 0 minutes → +0.3, 30+ minutes → 0
	const maxWindow = 30 * time.Minute
	if minDist >= maxWindow {
		return 0
	}
	return 0.3 * (1 - float64(minDist)/float64(maxWindow))
}

// StartBackground starts the webhook server in a goroutine and returns immediately.
func StartBackground(ctx context.Context, srv *Server) {
	go func() {
		if err := srv.ListenAndServe(); err != nil && ctx.Err() == nil {
			srv.logger.Error("webhook server stopped", zap.Error(err))
		}
	}()
}
