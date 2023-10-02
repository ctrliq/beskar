{{/*
Expand the name of the chart.
*/}}
{{- define "beskar-yum.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "beskar-yum.fullname" -}}
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
{{- define "beskar-yum.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "beskar-yum.labels" -}}
helm.sh/chart: {{ include "beskar-yum.chart" . }}
{{ include "beskar-yum.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "beskar-yum.selectorLabels" -}}
app.kubernetes.io/name: {{ include "beskar-yum.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "beskar-yum.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "beskar-yum.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{- define "beskar-yum.envs" -}}
- name: BESKARYUM_GOSSIP_KEY
  valueFrom:
    secretKeyRef:
      name: beskar-gossip-secret
      key: gossipKey
{{- if eq .Values.configData.storage.driver "filesystem" }}
- name: BESKARYUM_STORAGE_FILESYSTEM_DIRECTORY
  value: {{ .Values.configData.storage.filesystem.directory }}
{{- else if eq .Values.configData.storage.driver "azure" }}
- name: BESKARYUM_STORAGE_AZURE_ACCOUNTNAME
  valueFrom:
    secretKeyRef:
      name: {{ template "beskar-yum.fullname" . }}-secret
      key: azureAccountName
- name: BESKARYUM_STORAGE_AZURE_ACCOUNTKEY
  valueFrom:
    secretKeyRef:
      name: {{ template "beskar-yum.fullname" . }}-secret
      key: azureAccountKey
{{- else if eq .Values.configData.storage.driver "s3" }}
  {{- if and .Values.secrets.s3.secretKey .Values.secrets.s3.accessKey }}
- name: BESKARYUM_STORAGE_S3_ACCESSKEYID
  valueFrom:
    secretKeyRef:
      name: {{ template "beskar-yum.fullname" . }}-secret
      key: s3AccessKey
- name: BESKARYUM_STORAGE_S3_SECRETACCESSKEY
  valueFrom:
    secretKeyRef:
      name: {{ template "beskar-yum.fullname" . }}-secret
      key: s3SecretKey
  {{- end }}
{{- else if eq .Values.configData.storage.driver "gcs" }}
- name: BESKARYUM_STORAGE_GCS_KEYFILE
  value: /etc/gcs-keyfile
{{- end -}}

{{- with .Values.extraEnvVars }}
{{ toYaml . }}
{{- end -}}

{{- end -}}

{{- define "beskar-yum.volumeMounts" -}}
- name: config
  mountPath: "/etc/beskar"

{{- if eq .Values.configData.storage.driver "filesystem" }}
- name: data
  mountPath: {{ .Values.configData.storage.filesystem.directory }}
{{- else if eq .Values.configData.storage.driver "gcs" }}
- name: gcs
  mountPath: "/etc/gcs-keyfile"
  subPath: gcsKeyfile
  readOnly: true
{{- end }}

{{- with .Values.extraVolumeMounts }}
{{ toYaml . }}
{{- end }}

{{- end -}}

{{- define "beskar-yum.volumes" -}}
- name: config
  configMap:
    name: {{ template "beskar-yum.fullname" . }}-config

{{- if eq .Values.configData.storage.driver "filesystem" }}
- name: data
  {{- if .Values.persistence.enabled }}
  persistentVolumeClaim:
    claimName: {{ if .Values.persistence.existingClaim }}{{ .Values.persistence.existingClaim }}{{- else }}{{ template "beskar-yum.fullname" . }}{{- end }}
  {{- else }}
  emptyDir: {}
  {{- end -}}
{{- else if eq .Values.configData.storage.driver "gcs" }}
- name: gcs
  secret:
    secretName: {{ template "beskar-yum.fullname" . }}-secret
{{- end }}

{{- with .Values.extraVolumes }}
{{ toYaml . }}
{{- end }}
{{- end -}}