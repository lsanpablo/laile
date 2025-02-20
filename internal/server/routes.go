package server

import (
	"encoding/json"
	"github.com/jackc/pgx/v5/pgtype"
	"html/template"
	"laile/internal/event"
	db_models "laile/internal/postgresql"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
)

var tmpl2 = template.Must(template.New("").Parse(`
{{ define "delivery_attempts" }}
<div class="bg-white shadow overflow-hidden rounded-lg">
  <table class="min-w-full divide-y divide-gray-200">
    <thead class="bg-gray-50">
      <tr>
        <th scope="col" class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Target ID</th>
        <th scope="col" class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Service</th>
        <th scope="col" class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Forwarder</th>
        <th scope="col" class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Status</th>
        <th scope="col" class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Attempts</th>
      </tr>
    </thead>
    <tbody class="bg-white divide-y divide-gray-200">
      {{ range .Items }}
      <tr class="hover:bg-gray-50 cursor-pointer" onclick="window.location='/admin/targets/{{ .ID }}'">
        <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-900">{{ .ID }}</td>
        <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-900">{{ .WebhookServiceID }}</td>
        <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-900">{{ .ForwarderID }}</td>
        <td class="px-6 py-4 whitespace-nowrap text-sm 
          {{ if eq .Status "success" }}text-green-600
          {{ else if eq .Status "failed" }}text-red-600
          {{ else if eq .Status "scheduled" }}text-yellow-600
          {{ else }}text-gray-600{{ end }}">
          {{ .Status }}
        </td>
        <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-900">{{ .AttemptCount }}</td>
      </tr>
      {{ end }}
    </tbody>
  </table>
  {{ if .HasMore }}
  <div class="p-4">
    <button class="w-full bg-blue-600 hover:bg-blue-700 text-white font-medium py-2 px-4 rounded-md"
            hx-get="/admin/delivery-attempts"
            hx-target="#results"
            hx-swap="outerHTML"
            hx-include="[name='service'],[name='forwarder'],[name='status']"
            hx-vals='{"cursor": "{{ .LastID }}"}'>
      Load More
    </button>
  </div>
  {{ end }}
</div>
{{ end }}

{{ define "error" }}
<div class="max-w-lg mx-auto bg-red-50 border border-red-400 text-red-700 px-4 py-3 rounded relative" role="alert">
  <strong class="font-bold">Error!</strong>
  <span class="block sm:inline">{{ . }}</span>
</div>
{{ end }}

{{ define "target_details" }}
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Target Details</title>
    <script src="https://unpkg.com/htmx.org@1.9.6"></script>
    <script src="https://cdn.tailwindcss.com"></script>
</head>
<body class="bg-gray-100">
    <header class="bg-white shadow">
        <div class="max-w-7xl mx-auto py-6 px-4 sm:px-6 lg:px-8">
            <div class="flex items-center">
                <a href="/admin/dashboard" class="text-blue-600 hover:text-blue-800 mr-4">
                    ← Back to Dashboard
                </a>
                <h1 class="text-3xl font-bold text-gray-900">Target Details</h1>
            </div>
        </div>
    </header>
    <main class="max-w-7xl mx-auto py-6 sm:px-6 lg:px-8">
        <div class="bg-white shadow overflow-hidden sm:rounded-lg mb-6">
            <div class="px-4 py-5 sm:px-6">
                <dl class="grid grid-cols-1 gap-x-4 gap-y-8 sm:grid-cols-2">
                    <div class="sm:col-span-1">
                        <dt class="text-sm font-medium text-gray-500">Target ID</dt>
                        <dd class="mt-1 text-sm text-gray-900">{{ .Target.ID }}</dd>
                    </div>
                    <div class="sm:col-span-1">
                        <dt class="text-sm font-medium text-gray-500">Service ID</dt>
                        <dd class="mt-1 text-sm text-gray-900">{{ .Target.WebhookServiceID }}</dd>
                    </div>
                    <div class="sm:col-span-1">
                        <dt class="text-sm font-medium text-gray-500">Forwarder ID</dt>
                        <dd class="mt-1 text-sm text-gray-900">{{ .Target.ForwarderID }}</dd>
                    </div>
                    <div class="sm:col-span-1">
                        <dt class="text-sm font-medium text-gray-500">Total Attempts</dt>
                        <dd class="mt-1 text-sm text-gray-900">{{ .Target.AttemptCount }}</dd>
                    </div>
                    <div class="sm:col-span-2">
                        <dt class="text-sm font-medium text-gray-500">Webhook URL</dt>
                        <dd class="mt-1 text-sm text-gray-900">{{ .Target.Url }}</dd>
                    </div>
                </dl>
            </div>
        </div>

        <div class="bg-white shadow overflow-hidden sm:rounded-lg">
            <div class="px-4 py-5 sm:px-6">
                <h3 class="text-lg leading-6 font-medium text-gray-900">Delivery Attempts</h3>
            </div>
            <div class="border-t border-gray-200">
                <div class="overflow-x-auto">
                    <table class="min-w-full divide-y divide-gray-200">
                        <thead class="bg-gray-50">
                            <tr>
                                <th scope="col" class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">ID</th>
                                <th scope="col" class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Status</th>
                                <th scope="col" class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Scheduled For</th>
                                <th scope="col" class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Executed At</th>
                                <th scope="col" class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Created At</th>
                                <th scope="col" class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Response</th>
                            </tr>
                        </thead>
                        <tbody class="bg-white divide-y divide-gray-200">
                            {{ range .Attempts }}
                            <tr class="hover:bg-gray-50">
                                <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-900">{{ .ID }}</td>
                                <td class="px-6 py-4 whitespace-nowrap text-sm">
                                    <span class="px-2 inline-flex text-xs leading-5 font-semibold rounded-full
                                        {{ if eq .Status "success" }}bg-green-100 text-green-800
                                        {{ else if eq .Status "failed" }}bg-red-100 text-red-800
                                        {{ else if eq .Status "scheduled" }}bg-yellow-100 text-yellow-800
                                        {{ else }}bg-gray-100 text-gray-800{{ end }}">
                                        {{ .Status }}
                                    </span>
                                </td>
                                <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-900">{{ .ScheduledFor.Time.Format "2006-01-02 15:04:05" }}</td>
                                <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-900">
                                    {{ if .ExecutedAt.Valid }}
                                        {{ .ExecutedAt.Time.Format "2006-01-02 15:04:05" }}
                                    {{ end }}
                                </td>
                                <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-900">{{ .CreatedAt.Time.Format "2006-01-02 15:04:05" }}</td>
                                <td class="px-6 py-4 text-sm text-gray-900">
                                    {{ if .ResponseBody }}
                                        <details class="cursor-pointer">
                                            <summary class="text-blue-600 hover:text-blue-800">View Response</summary>
                                            <pre class="mt-2 p-2 bg-gray-50 rounded text-xs overflow-x-auto">{{ .ResponseBody }}</pre>
                                        </details>
                                    {{ end }}
                                </td>
                            </tr>
                            {{ end }}
                        </tbody>
                    </table>
                </div>
            </div>
        </div>
    </main>
</body>
</html>
{{ end }}

{{ define "dashboard" }}
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Admin Dashboard</title>
  <script src="https://unpkg.com/htmx.org@1.9.6"></script>
  <script src="https://cdn.tailwindcss.com"></script>
</head>
<body class="bg-gray-100">
  <header class="bg-white shadow">
    <div class="max-w-7xl mx-auto py-6 px-4 sm:px-6 lg:px-8">
      <h1 class="text-3xl font-bold text-gray-900">Webhook Dashboard</h1>
    </div>
  </header>
  <main>
    <div class="max-w-7xl mx-auto py-6 sm:px-6 lg:px-8">
      <div class="px-4 py-6 sm:px-0">
        <div class="mb-6">
          <div class="flex flex-wrap gap-4">
            <input name="service" placeholder="Service ID" class="flex-1 min-w-[200px] border border-gray-300 rounded-md p-2 shadow-sm focus:outline-none focus:ring-2 focus:ring-indigo-500"
                   hx-get="/admin/delivery-attempts" hx-trigger="keyup changed delay:500ms" hx-target="#results">
            <input name="forwarder" placeholder="Forwarder ID" class="flex-1 min-w-[200px] border border-gray-300 rounded-md p-2 shadow-sm focus:outline-none focus:ring-2 focus:ring-indigo-500"
                   hx-get="/admin/delivery-attempts" hx-trigger="keyup changed delay:500ms" hx-target="#results">
            <select name="status" class="min-w-[200px] border border-gray-300 rounded-md p-2 shadow-sm focus:outline-none focus:ring-2 focus:ring-indigo-500"
                    hx-get="/admin/delivery-attempts" hx-trigger="change" hx-target="#results">
              <option value="">All Statuses</option>
              <option value="success">Success</option>
              <option value="failed">Failed</option>
              <option value="scheduled">Scheduled</option>
            </select>
          </div>
        </div>
        <div id="results" hx-get="/admin/delivery-attempts" hx-trigger="load">
          <!-- Delivery attempts table will load here -->
        </div>
      </div>
    </div>
  </main>
</body>
</html>
{{ end }}
`))

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

func (s *Server) HelloWorldHandler(w http.ResponseWriter, r *http.Request) {
	resp := map[string]string{
		"message": "Hello World",
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.db.Health())
}

func (s *Server) WebhookListenerHandler(w http.ResponseWriter, r *http.Request) {
	// Extract listener from path
	listener := strings.TrimPrefix(r.URL.Path, "/listener/")

	err := event.HandleEvent(s.db, listener, r, s.config)
	if err != nil {
		slog.Error("webhook handler error", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"status": "error"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) adminStatusHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "running"})
}

func (s *Server) adminDashboardHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	tmpl2.ExecuteTemplate(w, "dashboard", nil)
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
			slog.Error("invalid cursor", "error", err)
			tmpl2.ExecuteTemplate(w, "error", "Invalid cursor")
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
		slog.Error("failed to list webhook targets", "error", err)
		tmpl2.ExecuteTemplate(w, "error", "Failed to load webhook targets")
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
	tmpl2.ExecuteTemplate(w, "delivery_attempts", response)
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
		slog.Error("failed to get target details", "error", err)
		http.Error(w, "Failed to load target details", http.StatusInternalServerError)
		return
	}

	attempts, err := queries.GetDeliveryAttemptsByTargetId(ctx, pgtype.Int8{
		Int64: targetID,
		Valid: true,
	})
	if err != nil {
		slog.Error("failed to get delivery attempts", "error", err)
		http.Error(w, "Failed to load delivery attempts", http.StatusInternalServerError)
		return
	}

	data := TargetDetailsData{
		Target:   target,
		Attempts: attempts,
	}

	w.Header().Set("Content-Type", "text/html")
	tmpl2.ExecuteTemplate(w, "target_details", data)
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
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
