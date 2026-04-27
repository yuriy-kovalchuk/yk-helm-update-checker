{{/*
Expand the name of the chart.
*/}}
{{- define "yk-update-checker.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name, capped at 63 chars.
*/}}
{{- define "yk-update-checker.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
API fullname.
*/}}
{{- define "yk-update-checker.api.fullname" -}}
{{- printf "%s-api" (include "yk-update-checker.fullname" .) | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Scanner fullname.
*/}}
{{- define "yk-update-checker.scanner.fullname" -}}
{{- printf "%s-scanner" (include "yk-update-checker.fullname" .) | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Dashboard fullname.
*/}}
{{- define "yk-update-checker.dashboard.fullname" -}}
{{- printf "%s-dashboard" (include "yk-update-checker.fullname" .) | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create the chart label value.
*/}}
{{- define "yk-update-checker.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels.
*/}}
{{- define "yk-update-checker.labels" -}}
helm.sh/chart: {{ include "yk-update-checker.chart" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
API labels.
*/}}
{{- define "yk-update-checker.api.labels" -}}
{{ include "yk-update-checker.labels" . }}
{{ include "yk-update-checker.api.selectorLabels" . }}
{{- end }}

{{/*
API selector labels.
*/}}
{{- define "yk-update-checker.api.selectorLabels" -}}
app.kubernetes.io/name: {{ include "yk-update-checker.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: api
{{- end }}

{{/*
Scanner labels.
*/}}
{{- define "yk-update-checker.scanner.labels" -}}
{{ include "yk-update-checker.labels" . }}
{{ include "yk-update-checker.scanner.selectorLabels" . }}
{{- end }}

{{/*
Scanner selector labels.
*/}}
{{- define "yk-update-checker.scanner.selectorLabels" -}}
app.kubernetes.io/name: {{ include "yk-update-checker.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: scanner
{{- end }}

{{/*
Dashboard labels.
*/}}
{{- define "yk-update-checker.dashboard.labels" -}}
{{ include "yk-update-checker.labels" . }}
{{ include "yk-update-checker.dashboard.selectorLabels" . }}
{{- end }}

{{/*
Dashboard selector labels.
*/}}
{{- define "yk-update-checker.dashboard.selectorLabels" -}}
app.kubernetes.io/name: {{ include "yk-update-checker.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: dashboard
{{- end }}

{{/*
ServiceAccount name.
*/}}
{{- define "yk-update-checker.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "yk-update-checker.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
PVC name.
*/}}
{{- define "yk-update-checker.pvcName" -}}
{{- if .Values.api.persistence.existingClaim }}
{{- .Values.api.persistence.existingClaim }}
{{- else }}
{{- include "yk-update-checker.fullname" . }}-data
{{- end }}
{{- end }}

{{/*
API internal URL (for scanner and dashboard to connect).
*/}}
{{- define "yk-update-checker.api.url" -}}
http://{{ include "yk-update-checker.api.fullname" . }}:8080
{{- end }}

{{/*
Extract "{org}/{repo}" from a git URL for use in secret mount paths.
*/}}
{{- define "yk-update-checker.repoOrgPath" -}}
{{- $url := . -}}
{{- if hasPrefix "http" $url -}}
  {{- regexReplaceAll "^https?://[^/]+/" $url "" | trimSuffix ".git" -}}
{{- else -}}
  {{- regexReplaceAll "^[^:]+:" $url "" | trimSuffix ".git" -}}
{{- end -}}
{{- end }}

{{/*
Safe volume name from repo name.
*/}}
{{- define "yk-update-checker.secretVolumeName" -}}
{{- printf "secret-%s" (. | lower | regexReplaceAll "[^a-z0-9]" "-") -}}
{{- end }}
