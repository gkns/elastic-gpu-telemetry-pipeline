{{/*
Expand the name of the chart.
*/}}
{{- define "elastic-gpu-telemetry.name" -}}
{{- .Chart.Name | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "elastic-gpu-telemetry.fullname" -}}
{{- if .Release.Name }}
{{- printf "%s-%s" .Release.Name .Chart.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- .Chart.Name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}

{{/*
Chart label — name + version, used in helm.sh/chart annotation.
*/}}
{{- define "elastic-gpu-telemetry.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels applied to every resource.
*/}}
{{- define "elastic-gpu-telemetry.labels" -}}
helm.sh/chart: {{ include "elastic-gpu-telemetry.chart" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- with .Values.commonLabels }}
{{ toYaml . }}
{{- end }}
{{- end }}

{{/*
Selector labels for a given component.
Usage: include "elastic-gpu-telemetry.selectorLabels" (dict "component" "mq" "context" .)
*/}}
{{- define "elastic-gpu-telemetry.selectorLabels" -}}
app.kubernetes.io/name: {{ include "elastic-gpu-telemetry.name" .context }}
app.kubernetes.io/instance: {{ .context.Release.Name }}
app.kubernetes.io/component: {{ .component }}
{{- end }}

{{/*
Image pull secrets block.
*/}}
{{- define "elastic-gpu-telemetry.imagePullSecrets" -}}
{{- with .Values.imagePullSecrets }}
imagePullSecrets:
  {{- toYaml . | nindent 2 }}
{{- end }}
{{- end }}
