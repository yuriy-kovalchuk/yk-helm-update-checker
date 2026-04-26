{{/*
Expand the name of the chart.
*/}}
{{- define "yk-update-checker.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name, capped at 63 chars.
If the release name already contains the chart name it is used as-is.
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
Create the chart label value: name-version with + replaced by _ to satisfy DNS rules.
*/}}
{{- define "yk-update-checker.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels applied to every resource.
*/}}
{{- define "yk-update-checker.labels" -}}
helm.sh/chart: {{ include "yk-update-checker.chart" . }}
{{ include "yk-update-checker.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels used by Deployment.spec.selector and Service.spec.selector.
These must remain stable for the lifetime of the release.
*/}}
{{- define "yk-update-checker.selectorLabels" -}}
app.kubernetes.io/name: {{ include "yk-update-checker.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Name of the ServiceAccount to use.
*/}}
{{- define "yk-update-checker.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "yk-update-checker.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Extract "{org}/{repo}" from a git URL for use in secret mount paths.
Handles both HTTPS (https://host/org/repo) and SSH (git@host:org/repo.git) formats.
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
Safe volume name derived from a repo name: lowercase, non-alphanumeric replaced with "-".
*/}}
{{- define "yk-update-checker.secretVolumeName" -}}
{{- printf "secret-%s" (. | lower | regexReplaceAll "[^a-z0-9]" "-") -}}
{{- end }}