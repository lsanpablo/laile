package server

import "html/template"

var adminTemplate = template.Must(template.New("").Parse(`
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
                        <dd class="mt-1 text-sm text-gray-900">{{ .Target.URL }}</dd>
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
