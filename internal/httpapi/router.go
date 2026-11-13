package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// NewRouter builds the service's HTTP routes. At this stage only the health
// probes exist; the ride endpoints arrive in Phase 1.
func NewRouter(db *sql.DB, logger *slog.Logger) http.Handler {
	api := &API{db: db, logger: logger}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", api.health)
	mux.HandleFunc("GET /readyz", api.ready)

	return requestLogger(mux, logger)
}

type API struct {
	db     *sql.DB
	logger *slog.Logger
}

// health is liveness: it reports whether the process is alive. It does not
// check dependencies.
func (a *API) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ready is readiness: it reports whether the service can actually serve traffic.
func (a *API) ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := a.db.PingContext(ctx); err != nil {
		a.logger.Warn("readiness check failed", "err", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func requestLogger(next http.Handler, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rec, r)

		attrs := []any{
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
		}
		if traceID := traceID(r); traceID != "" {
			attrs = append(attrs, "traceId", traceID)
		}
		if key := r.Header.Get("Idempotency-Key"); key != "" {
			attrs = append(attrs, "idempotencyKey", key)
		}

		logger.Info("request", attrs...)
	})
}

// traceID extracts the request's trace identifier: the W3C `traceparent`
// header when present, or the legacy `X-Trace-Id`.
func traceID(r *http.Request) string {
	if tp := r.Header.Get("traceparent"); tp != "" {
		// traceparent: version-traceid-parentid-flags
		if parts := strings.Split(tp, "-"); len(parts) >= 2 {
			return parts[1]
		}
	}
	return r.Header.Get("X-Trace-Id")
}
