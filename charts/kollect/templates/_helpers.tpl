{{- define "kollect.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "kollect.fullname" -}}
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

{{- define "kollect.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "kollect.labels" -}}
helm.sh/chart: {{ include "kollect.chart" . }}
{{ include "kollect.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{- define "kollect.selectorLabels" -}}
app.kubernetes.io/name: kollect
control-plane: controller-manager
{{- end }}

{{- define "kollect.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (printf "%s-controller-manager" (include "kollect.fullname" .)) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{- define "kollect.image" -}}
{{- printf "%s:%s" .Values.image.repository .Values.image.tag }}
{{- end }}

{{- define "kollect.webhookServiceName" -}}
{{- printf "%s-webhook-service" (include "kollect.fullname" .) }}
{{- end }}

{{- define "kollect.hubRemoteClusters" -}}
{{- $ns := default $.Release.Namespace $.Values.hub.platformNamespace -}}
{{- $clusters := $.Values.hub.remoteClusters | default list -}}
{{- $parts := list -}}
{{- range $clusters }}
{{- $parts = append $parts (printf "%s/%s:%s" $ns . .) -}}
{{- end }}
{{- join "," $parts -}}
{{- end }}
