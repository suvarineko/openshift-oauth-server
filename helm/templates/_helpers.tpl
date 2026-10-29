{{/*
Expand the name of the chart.
*/}}
{{- define "oauth-server.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "oauth-server.fullname" -}}
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
Create chart name and version as used by the chart label.
*/}}
{{- define "oauth-server.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "oauth-server.labels" -}}
helm.sh/chart: {{ include "oauth-server.chart" . }}
{{ include "oauth-server.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "oauth-server.selectorLabels" -}}
app.kubernetes.io/name: {{ include "oauth-server.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app: openshift-oauth-server
oauth-openshift-anti-affinity: 'true'
{{- end }}

{{/*
Preflight: require the OpenShift OAuth API group to be present in the cluster.
Aborts rendering/installation unless config.openshift.io/v1/OAuth is available.
Can be disabled by setting apiPreflightCheck=false (e.g. for `helm template` in CI).
*/}}
{{- define "oauth-server.requireOpenShiftOAuth" -}}
{{- if .Values.apiPreflightCheck }}
{{- if not (.Capabilities.APIVersions.Has "config.openshift.io/v1/OAuth") }}
{{- fail "\n\nThis chart requires the OpenShift OAuth API (config.openshift.io/v1/OAuth), which is not available in the target cluster.\nInstall this chart on an OpenShift cluster, or set apiPreflightCheck=false to bypass this check." }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "oauth-server.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "oauth-server.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}
