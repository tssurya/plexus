{{- define "plexus.name" -}}
plexus-controller
{{- end -}}

{{- define "plexus.namespace" -}}
plexus-system
{{- end -}}

{{- define "plexus.labels" -}}
app.kubernetes.io/name: {{ include "plexus.name" . }}
app.kubernetes.io/part-of: plexus
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{- define "plexus.selectorLabels" -}}
app.kubernetes.io/name: {{ include "plexus.name" . }}
{{- end -}}
