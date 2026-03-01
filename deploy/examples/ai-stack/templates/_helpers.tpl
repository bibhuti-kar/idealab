{{/*
Common labels for all resources.
*/}}
{{- define "ai-stack.labels" -}}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: ai-stack
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version }}
{{- end }}

{{/*
Selector labels for Ollama.
*/}}
{{- define "ai-stack.ollama.selectorLabels" -}}
app: ollama
app.kubernetes.io/name: ollama
app.kubernetes.io/component: inference
{{- end }}

{{/*
Selector labels for API server.
*/}}
{{- define "ai-stack.api.selectorLabels" -}}
app: api-server
app.kubernetes.io/name: api-server
app.kubernetes.io/component: api
{{- end }}
