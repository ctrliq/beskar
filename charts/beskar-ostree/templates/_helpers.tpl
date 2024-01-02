{{/*
Expand the name of the chart.
*/}}
{{- define "beskar-ostree.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "beskar-ostree.fullname" -}}
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
{{- define "beskar-ostree.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "beskar-ostree.labels" -}}
helm.sh/chart: {{ include "beskar-ostree.chart" . }}
{{ include "beskar-ostree.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "beskar-ostree.selectorLabels" -}}
app.kubernetes.io/name: {{ include "beskar-ostree.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "beskar-ostree.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "beskar-ostree.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{- define "beskar-ostree.envs" -}}
- name: BESKAROSTREE_GOSSIP_KEY
  valueFrom:
    secretKeyRef:
      name: beskar-gossip-secret
      key: gossipKey
{{- if eq .Values.configData.storage.driver "filesystem" }}
- name: BESKAROSTREE_STORAGE_FILESYSTEM_DIRECTORY
  value: {{ .Values.configData.storage.filesystem.directory }}
{{- else if eq .Values.configData.storage.driver "azure" }}
- name: BESKAROSTREE_STORAGE_AZURE_ACCOUNTNAME
  valueFrom:
    secretKeyRef:
      name: {{ template "beskar-ostree.fullname" . }}-secret
      key: azureAccountName
- name: BESKAROSTREE_STORAGE_AZURE_ACCOUNTKEY
  valueFrom:
    secretKeyRef:
      name: {{ template "beskar-ostree.fullname" . }}-secret
      key: azureAccountKey
{{- else if eq .Values.configData.storage.driver "s3" }}
  {{- if and .Values.secrets.s3.secretKey .Values.secrets.s3.accessKey }}
- name: BESKAROSTREE_STORAGE_S3_ACCESSKEYID
  valueFrom:
    secretKeyRef:
      name: {{ template "beskar-ostree.fullname" . }}-secret
      key: s3AccessKey
- name: BESKAROSTREE_STORAGE_S3_SECRETACCESSKEY
  valueFrom:
    secretKeyRef:
      name: {{ template "beskar-ostree.fullname" . }}-secret
      key: s3SecretKey
  {{- end }}
{{- else if eq .Values.configData.storage.driver "gcs" }}
- name: BESKAROSTREE_STORAGE_GCS_KEYFILE
  value: /etc/gcs-keyfile
{{- end -}}

{{- with .Values.extraEnvVars }}
{{ toYaml . }}
{{- end -}}

{{- end -}}

{{- define "beskar-ostree.volumeMounts" -}}
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

{{- define "beskar-ostree.volumes" -}}
- name: config
  configMap:
    name: {{ template "beskar-ostree.fullname" . }}-config

{{- if eq .Values.configData.storage.driver "filesystem" }}
- name: data
  {{- if .Values.persistence.enabled }}
  persistentVolumeClaim:
    claimName: {{ if .Values.persistence.existingClaim }}{{ .Values.persistence.existingClaim }}{{- else }}{{ template "beskar-ostree.fullname" . }}{{- end }}
  {{- else }}
  emptyDir: {}
  {{- end -}}
{{- else if eq .Values.configData.storage.driver "gcs" }}
- name: gcs
  secret:
    secretName: {{ template "beskar-ostree.fullname" . }}-secret
{{- end }}

{{- with .Values.extraVolumes }}
{{ toYaml . }}
{{- end }}
{{- end -}}