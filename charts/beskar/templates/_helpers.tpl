{{/* vim: set filetype=mustache: */}}
{{/*
Expand the name of the chart.
*/}}
{{- define "beskar.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
*/}}
{{- define "beskar.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "beskar.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "beskar.labels" -}}
helm.sh/chart: {{ include "beskar.chart" . }}
{{ include "beskar.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "beskar.selectorLabels" -}}
app.kubernetes.io/name: {{ include "beskar.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{- define "beskar.envs" -}}
- name: BESKAR_REGISTRY_HTTP_SECRET
  valueFrom:
    secretKeyRef:
      name: {{ template "beskar.fullname" . }}-secret
      key: haSharedSecret
- name: BESKAR_GOSSIP_KEY
  valueFrom:
    secretKeyRef:
      name: {{ template "beskar.fullname" . }}-secret
      key: gossipKey

{{- if .Values.secrets.htpasswd }}
- name: BESKAR_REGISTRY_AUTH
  value: "htpasswd"
- name: BESKAR_REGISTRY_AUTH_HTPASSWD_REALM
  value: "Registry Realm"
- name: BESKAR_REGISTRY_AUTH_HTPASSWD_PATH
  value: "/auth/htpasswd"
{{- end }}

{{- if .Values.groupcache.size }}
- name: BESKAR_CACHE_SIZE
  value: {{ .Values.groupcache.size | quote }}
{{- end }}

{{- if .Values.secrets.beskarPassword }}
- name: BESKAR_REGISTRY_AUTH
  value: "beskar"
- name: BESKAR_REGISTRY_AUTH_BESKAR_ACCOUNT
  value: {{ htpasswd "beskar" .Values.secrets.beskarPassword }}
{{- end }}

{{- if .Values.tlsSecretName }}
- name: BESKAR_REGISTRY_HTTP_TLS_CERTIFICATE
  value: /etc/ssl/docker/tls.crt
- name: BESKAR_REGISTRY_HTTP_TLS_KEY
  value: /etc/ssl/docker/tls.key
{{- end -}}

{{- if eq .Values.storage "filesystem" }}
- name: BESKAR_REGISTRY_STORAGE_FILESYSTEM_ROOTDIRECTORY
  value: "/var/lib/registry"
{{- else if eq .Values.storage "azure" }}
- name: BESKAR_REGISTRY_STORAGE_AZURE_ACCOUNTNAME
  valueFrom:
    secretKeyRef:
      name: {{ template "beskar.fullname" . }}-secret
      key: azureAccountName
- name: BESKAR_REGISTRY_STORAGE_AZURE_ACCOUNTKEY
  valueFrom:
    secretKeyRef:
      name: {{ template "beskar.fullname" . }}-secret
      key: azureAccountKey
- name: BESKAR_REGISTRY_STORAGE_AZURE_CONTAINER
  valueFrom:
    secretKeyRef:
      name: {{ template "beskar.fullname" . }}-secret
      key: azureContainer
- name: BESKAR_REGISTRY_STORAGE_AZURE_ROOTDIRECTORY
  value: {{ if .Values.azure }}{{ .Values.azure.rootdirectory | default "" | quote }}{{ else }}""{{ end }}
{{- else if eq .Values.storage "s3" }}
- name: BESKAR_REGISTRY_STORAGE_S3_REGION
  value: {{ required ".Values.s3.region is required" .Values.s3.region }}
- name: BESKAR_REGISTRY_STORAGE_S3_BUCKET
  value: {{ required ".Values.s3.bucket is required" .Values.s3.bucket }}
{{- if or (and .Values.secrets.s3.secretKey .Values.secrets.s3.accessKey) .Values.secrets.s3.secretRef }}
- name: BESKAR_REGISTRY_STORAGE_S3_ACCESSKEY
  valueFrom:
    secretKeyRef:
      name: {{ if .Values.secrets.s3.secretRef }}{{ .Values.secrets.s3.secretRef }}{{ else }}{{ template "beskar.fullname" . }}-secret{{ end }}
      key: s3AccessKey
- name: BESKAR_REGISTRY_STORAGE_S3_SECRETKEY
  valueFrom:
    secretKeyRef:
      name: {{ if .Values.secrets.s3.secretRef }}{{ .Values.secrets.s3.secretRef }}{{ else }}{{ template "beskar.fullname" . }}-secret{{ end }}
      key: s3SecretKey
{{- end -}}

{{- if .Values.s3.regionEndpoint }}
- name: BESKAR_REGISTRY_STORAGE_S3_REGIONENDPOINT
  value: {{ .Values.s3.regionEndpoint }}
{{- end -}}

{{- if .Values.s3.rootdirectory }}
- name: BESKAR_REGISTRY_STORAGE_S3_ROOTDIRECTORY
  value: {{ .Values.s3.rootdirectory | quote }}
{{- end -}}

{{- if .Values.s3.encrypt }}
- name: BESKAR_REGISTRY_STORAGE_S3_ENCRYPT
  value: {{ .Values.s3.encrypt | quote }}
{{- end -}}

{{- if .Values.s3.secure }}
- name: BESKAR_REGISTRY_STORAGE_S3_SECURE
  value: {{ .Values.s3.secure | quote }}
{{- end -}}

{{- else if eq .Values.storage "swift" }}
- name: BESKAR_REGISTRY_STORAGE_SWIFT_AUTHURL
  value: {{ required ".Values.swift.authurl is required" .Values.swift.authurl }}
- name: BESKAR_REGISTRY_STORAGE_SWIFT_USERNAME
  valueFrom:
    secretKeyRef:
      name: {{ template "beskar.fullname" . }}-secret
      key: swiftUsername
- name: BESKAR_REGISTRY_STORAGE_SWIFT_PASSWORD
  valueFrom:
    secretKeyRef:
      name: {{ template "beskar.fullname" . }}-secret
      key: swiftPassword
- name: BESKAR_REGISTRY_STORAGE_SWIFT_CONTAINER
  value: {{ required ".Values.swift.container is required" .Values.swift.container }}

{{- else if eq .Values.storage "gcs" }}
- name: BESKAR_REGISTRY_STORAGE_GCS_KEYFILE
  value: /etc/gcs-keyfile
- name: BESKAR_REGISTRY_STORAGE_GCS_BUCKET
  value: {{ required ".Values.gcs.bucket is required" .Values.gcs.bucket }}
- name: BESKAR_REGISTRY_STORAGE_GCS_CHUNKSIZE
  value: {{ .Values.gcs.chunksize | default "5242880" | quote }}
- name: BESKAR_REGISTRY_STORAGE_GCS_ROOTDIRECTORY
  value: {{ .Values.gcs.rootdirectory | default "/" | quote }}
{{- end -}}

{{- if .Values.proxy.enabled }}
- name: BESKAR_REGISTRY_PROXY_REMOTEURL
  value: {{ required ".Values.proxy.remoteurl is required" .Values.proxy.remoteurl }}
- name: BESKAR_REGISTRY_PROXY_USERNAME
  valueFrom:
    secretKeyRef:
      name: {{ if .Values.proxy.secretRef }}{{ .Values.proxy.secretRef }}{{ else }}{{ template "beskar.fullname" . }}-secret{{ end }}
      key: proxyUsername
- name: BESKAR_REGISTRY_PROXY_PASSWORD
  valueFrom:
    secretKeyRef:
      name: {{ if .Values.proxy.secretRef }}{{ .Values.proxy.secretRef }}{{ else }}{{ template "beskar.fullname" . }}-secret{{ end }}
      key: proxyPassword
{{- end -}}

{{- if .Values.persistence.deleteEnabled }}
- name: BESKAR_REGISTRY_STORAGE_DELETE_ENABLED
  value: "true"
{{- end -}}

{{- with .Values.extraEnvVars }}
{{ toYaml . }}
{{- end -}}

{{- end -}}

{{- define "beskar.volumeMounts" -}}
- name: "{{ template "beskar.fullname" . }}-config"
  mountPath: "/etc/beskar"

{{- if .Values.secrets.htpasswd }}
- name: auth
  mountPath: /auth
  readOnly: true
{{- end }}

{{- if eq .Values.storage "filesystem" }}
- name: data
  mountPath: /var/lib/registry/
{{- else if eq .Values.storage "gcs" }}
- name: gcs
  mountPath: "/etc/gcs-keyfile"
  subPath: gcsKeyfile
  readOnly: true
{{- end }}

{{- if .Values.tlsSecretName }}
- mountPath: /etc/ssl/docker
  name: tls-cert
  readOnly: true
{{- end }}

{{- with .Values.extraVolumeMounts }}
{{ toYaml . }}
{{- end }}

{{- end -}}

{{- define "beskar.volumes" -}}
- name: {{ template "beskar.fullname" . }}-config
  configMap:
    name: {{ template "beskar.fullname" . }}-config

{{- if .Values.secrets.htpasswd }}
- name: auth
  secret:
    secretName: {{ template "beskar.fullname" . }}-secret
    items:
    - key: htpasswd
      path: htpasswd
{{- end }}

{{- if eq .Values.storage "filesystem" }}
- name: data
  {{- if .Values.persistence.enabled }}
  persistentVolumeClaim:
    claimName: {{ if .Values.persistence.existingClaim }}{{ .Values.persistence.existingClaim }}{{- else }}{{ template "beskar.fullname" . }}{{- end }}
  {{- else }}
  emptyDir: {}
  {{- end -}}
{{- else if eq .Values.storage "gcs" }}
- name: gcs
  secret:
    secretName: {{ template "beskar.fullname" . }}-secret
{{- end }}

{{- if .Values.tlsSecretName }}
- name: tls-cert
  secret:
    secretName: {{ .Values.tlsSecretName }}
{{- end }}

{{- with .Values.extraVolumes }}
{{ toYaml . }}
{{- end }}
{{- end -}}

{{- define "beskar.plugins" -}}
plugins:
{{- if .Values.plugins.yum.enabled }}
  yum:
    prefix: /yum
    mediatype: application/vnd.ciq.rpm-package.v1.config+json
    backends:
    - url: {{ .Values.plugins.yum.url }}
{{- end }}
{{- end -}}