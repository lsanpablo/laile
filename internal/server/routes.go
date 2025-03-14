package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	"laile/internal/event"
	"laile/internal/log"
	db_models "laile/internal/postgresql"
)

func (s *Server) RegisterRoutes() http.Handler {
	mux := http.NewServeMux()

	// Add middleware
	handler := logMiddleware(recoverMiddleware(mux))

	// Register routes
	mux.HandleFunc("/", s.HelloWorldHandler)
	mux.HandleFunc("/health", s.healthHandler)
	mux.HandleFunc("/listener/", s.WebhookListenerHandler)

	return handler
}

func (s *Server) RegisterAdminRoutes() http.Handler {
	mux := http.NewServeMux()

	// Add middleware
	handler := logMiddleware(recoverMiddleware(mux))

	// Register admin routes
	mux.HandleFunc("/admin/status", s.adminStatusHandler)
	mux.HandleFunc("/admin/dashboard", s.adminDashboardHandler)
	mux.HandleFunc("/admin/delivery-attempts", s.deliveryAttemptsHandler)
	mux.HandleFunc("/admin/targets/", s.targetDetailsHandler)

	return handler
}

func (s *Server) HelloWorldHandler(w http.ResponseWriter, _ *http.Request) {
	resp := map[string]string{
		"message": "Hello World",
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) healthHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.db.Health())
}

func (s *Server) WebhookListenerHandler(w http.ResponseWriter, r *http.Request) {
	// Extract listener from path
	listener := strings.TrimPrefix(r.URL.Path, "/listener/")

	err := event.HandleEvent(s.db, listener, r, s.config)
	if err != nil {
		log.Logger.Error("webhook handler error", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"status": "error"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) adminStatusHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "running"})
}

func (s *Server) adminDashboardHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	err := adminTemplate.ExecuteTemplate(w, "dashboard", nil)
	if err != nil {
		http.Error(w, "Failed to render admin dashboard", http.StatusInternalServerError)
		return
	}
}

type DeliveryAttemptsResponse struct {
	Items   []db_models.GetWebhookTargetsListRow
	HasMore bool
	LastID  int64
}

func (s *Server) deliveryAttemptsHandler(w http.ResponseWriter, r *http.Request) {
	service := r.URL.Query().Get("service")
	forwarder := r.URL.Query().Get("forwarder")
	status := r.URL.Query().Get("status")
	cursor := r.URL.Query().Get("cursor")

	var cursorID int64
	if cursor != "" {
		var err error
		cursorID, err = strconv.ParseInt(cursor, 10, 64)
		if err != nil {
			log.Logger.Error("invalid cursor", "error", err)
			err = adminTemplate.ExecuteTemplate(w, "error", "Invalid cursor")
			if err != nil {
				http.Error(w, "failed to render error page", http.StatusInternalServerError)
			}
			return
		}
	}

	const pageSize = 20
	queries := s.db.Queries()
	ctx := r.Context()

	targets, err := queries.GetWebhookTargetsList(ctx, db_models.GetWebhookTargetsListParams{
		ServiceID:   service,
		ForwarderID: forwarder,
		Status:      status,
		Cursor:      cursorID,
		PageSize:    pageSize + 1, // fetch one extra to check if there are more
	})
	if err != nil {
		log.Logger.Error("failed to list webhook targets", "error", err)
		err = adminTemplate.ExecuteTemplate(w, "error", "Failed to load webhook targets")
		if err != nil {
			http.Error(w, "failed to render error page", http.StatusInternalServerError)
		}
		return
	}

	hasMore := len(targets) > pageSize
	if hasMore {
		targets = targets[:pageSize] // remove the extra item
	}

	var lastID int64
	if len(targets) > 0 {
		lastID = targets[len(targets)-1].ID
	}

	response := DeliveryAttemptsResponse{
		Items:   targets,
		HasMore: hasMore,
		LastID:  lastID,
	}

	w.Header().Set("Content-Type", "text/html")
	err = adminTemplate.ExecuteTemplate(w, "delivery_attempts", response)
	if err != nil {
		http.Error(w, "failed to render delivery attempts", http.StatusInternalServerError)
	}
}

type TargetDetailsData struct {
	Target   db_models.GetWebhookTargetDetailsRow
	Attempts []db_models.GetDeliveryAttemptsByTargetIdRow
}

func (s *Server) targetDetailsHandler(w http.ResponseWriter, r *http.Request) {
	targetID, err := strconv.ParseInt(strings.TrimPrefix(r.URL.Path, "/admin/targets/"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid target ID", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	queries := s.db.Queries()

	target, err := queries.GetWebhookTargetDetails(ctx, targetID)
	if err != nil {
		log.Logger.Error("failed to get target details", "error", err)
		http.Error(w, "Failed to load target details", http.StatusInternalServerError)
		return
	}

	attempts, err := queries.GetDeliveryAttemptsByTargetId(ctx, pgtype.Int8{
		Int64: targetID,
		Valid: true,
	})
	if err != nil {
		log.Logger.Error("failed to get delivery attempts", "error", err)
		http.Error(w, "Failed to load delivery attempts", http.StatusInternalServerError)
		return
	}

	data := TargetDetailsData{
		Target:   target,
		Attempts: attempts,
	}

	w.Header().Set("Content-Type", "text/html")
	err = adminTemplate.ExecuteTemplate(w, "target_details", data)
	if err != nil {
		http.Error(w, "failed to render target details", http.StatusInternalServerError)
	}
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	err := json.NewEncoder(w).Encode(data)
	if err != nil {
		log.Logger.Error("failed to write json response, json should be valid here", slog.Any("error", err))
	}
}

func logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slog.Info("request received",
			"method", r.Method,
			"path", r.URL.Path,
			"remote_addr", r.RemoteAddr,
		)
		next.ServeHTTP(w, r)
	})
}

func recoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				slog.Error("panic recovered", "error", err)
				writeJSON(w, http.StatusInternalServerError, map[string]string{"status": "error"})
			}
		}()
		next.ServeHTTP(w, r)
	})
}
