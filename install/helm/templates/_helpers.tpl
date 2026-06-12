{{/*
Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).

WSO2 LLC. licenses this file to you under the Apache License,
Version 2.0 (the "License"); you may not use this file except
in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing,
software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
KIND, either express or implied. See the License for the
specific language governing permissions and limitations
under the License.
*/}}

{{/*
Expand the name of the chart.
*/}}

{{- define "thunderid.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "thunderid.fullname" -}}
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
{{- define "thunderid.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "thunderid.labels" -}}
helm.sh/chart: {{ include "thunderid.chart" . }}
{{ include "thunderid.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "thunderid.selectorLabels" -}}
app.kubernetes.io/name: {{ include "thunderid.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "thunderid.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "thunderid.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Check if auto-generated database credentials Secret should be included in checksum annotation.
Returns true if any database password is set without a passwordRef.key.
This is used to trigger pod restarts when auto-generated Secrets change.
*/}}
{{- define "thunderid.shouldIncludeSecretChecksum" -}}
{{- $configuration := default dict .Values.configuration -}}
{{- $database := default dict $configuration.database -}}
{{- $config := default dict $database.config -}}
{{- $runtime := default dict $database.runtime -}}
{{- $user := default dict $database.user -}}
{{- $configPostgres := default dict $config.postgres -}}
{{- $runtimePostgres := default dict $runtime.postgres -}}
{{- $runtimeRedis := default dict $runtime.redis -}}
{{- $userPostgres := default dict $user.postgres -}}
{{- $consent := default dict $configuration.consent -}}
{{- $consentDb := default dict $consent.database -}}
{{- $cache := default dict $configuration.cache -}}
{{- $redis := default dict $cache.redis -}}
{{- if or (and $configPostgres.password (not (default dict $configPostgres.passwordRef).key)) (and $runtimePostgres.password (not (default dict $runtimePostgres.passwordRef).key)) (and $runtimeRedis.password (not (default dict $runtimeRedis.passwordRef).key)) (and $userPostgres.password (not (default dict $userPostgres.passwordRef).key)) (and $consent.enabled $consentDb.password (not (default dict $consentDb.passwordRef).key)) (and $redis.password (eq $cache.type "redis") (not (default dict $redis.passwordRef).key)) }}true{{- end }}
{{- end }}

{{/*
Generate database password environment variable definitions for both deployment and setup job.
Injects DB_CONFIG_PASSWORD, DB_RUNTIME_PASSWORD, and DB_USER_PASSWORD from either auto-generated or external Secrets.
*/}}
{{- define "thunderid.databasePasswordEnvVars" -}}
{{- $defaultDbSecretName := printf "%s-db-credentials" (include "thunderid.fullname" .) -}}
{{- $configuration := default dict .Values.configuration -}}
{{- $database := default dict $configuration.database -}}
{{- $config := default dict $database.config -}}
{{- $runtime := default dict $database.runtime -}}
{{- $user := default dict $database.user -}}
{{- $configPostgres := default dict $config.postgres -}}
{{- $runtimePostgres := default dict $runtime.postgres -}}
{{- $runtimeRedis := default dict $runtime.redis -}}
{{- $userPostgres := default dict $user.postgres -}}
{{- $consent := default dict $configuration.consent -}}
{{- $consentDb := default dict $consent.database -}}
{{- $configPasswordRef := default dict $configPostgres.passwordRef -}}
{{- $runtimePasswordRef := default dict $runtimePostgres.passwordRef -}}
{{- $runtimeRedisPasswordRef := default dict $runtimeRedis.passwordRef -}}
{{- $userPasswordRef := default dict $userPostgres.passwordRef -}}
{{- $consentPasswordRef := default dict $consentDb.passwordRef -}}
{{- if or $configPostgres.password $configPasswordRef.key }}
- name: DB_CONFIG_PASSWORD
  valueFrom:
    secretKeyRef:
      name: {{ if $configPasswordRef.key }}{{ $configPasswordRef.name | default $defaultDbSecretName }}{{ else }}{{ $defaultDbSecretName }}{{ end }}
      key: {{ $configPasswordRef.key | default "config-db-password" }}
{{- end }}
{{- if or $runtimePostgres.password $runtimePasswordRef.key }}
- name: DB_RUNTIME_PASSWORD
  valueFrom:
    secretKeyRef:
      name: {{ if $runtimePasswordRef.key }}{{ $runtimePasswordRef.name | default $defaultDbSecretName }}{{ else }}{{ $defaultDbSecretName }}{{ end }}
      key: {{ $runtimePasswordRef.key | default "runtime-db-password" }}
{{- end }}
{{- if or $runtimeRedis.password $runtimeRedisPasswordRef.key }}
- name: DB_RUNTIME_REDIS_PASSWORD
  valueFrom:
    secretKeyRef:
      name: {{ if $runtimeRedisPasswordRef.key }}{{ $runtimeRedisPasswordRef.name | default $defaultDbSecretName }}{{ else }}{{ $defaultDbSecretName }}{{ end }}
      key: {{ $runtimeRedisPasswordRef.key | default "runtime-redis-password" }}
{{- end }}
{{- if or $userPostgres.password $userPasswordRef.key }}
- name: DB_USER_PASSWORD
  valueFrom:
    secretKeyRef:
      name: {{ if $userPasswordRef.key }}{{ $userPasswordRef.name | default $defaultDbSecretName }}{{ else }}{{ $defaultDbSecretName }}{{ end }}
      key: {{ $userPasswordRef.key | default "user-db-password" }}
{{- end }}
{{- if and $consent.enabled (or $consentDb.password $consentPasswordRef.key) }}
- name: DB_CONSENT_PASSWORD
  valueFrom:
    secretKeyRef:
      name: {{ if $consentPasswordRef.key }}{{ $consentPasswordRef.name | default $defaultDbSecretName }}{{ else }}{{ $defaultDbSecretName }}{{ end }}
      key: {{ $consentPasswordRef.key | default "consent-db-password" }}
{{- end }}
{{- end }}

{{/*
Generate Redis password environment variable definitions for both deployment and setup job.
Injects CACHE_REDIS_PASSWORD from auto-generated database credentials Secret when Redis cache is enabled.
*/}}
{{- define "thunderid.cacheRedisPasswordEnvVars" -}}
{{- $defaultDbSecretName := printf "%s-db-credentials" (include "thunderid.fullname" .) -}}
{{- $configuration := default dict .Values.configuration -}}
{{- $cache := default dict $configuration.cache -}}
{{- $redis := default dict $cache.redis -}}
{{- $redisPasswordRef := default dict $redis.passwordRef -}}
{{- if and (eq $cache.type "redis") (or $redis.password $redisPasswordRef.key) }}
- name: CACHE_REDIS_PASSWORD
  valueFrom:
    secretKeyRef:
      name: {{ if $redisPasswordRef.key }}{{ $redisPasswordRef.name | default $defaultDbSecretName }}{{ else }}{{ $defaultDbSecretName }}{{ end }}
      key: {{ $redisPasswordRef.key | default "cache-redis-password" }}
{{- end }}
{{- end }}

{{/*
Generate generic secret-backed environment variable definitions.
Expected input:
  - secretEnv: list of objects with fields {name, secretName, secretKey, optional}
*/}}
{{- define "thunderid.secretEnvVars" -}}
{{- $secretEnv := default (list) .secretEnv -}}
{{- range $index, $item := $secretEnv }}
{{- if not $item.name }}
{{- fail (printf "Invalid secretEnv entry at index %d: name is required." $index) }}
{{- end }}
{{- if not $item.secretName }}
{{- fail (printf "Invalid secretEnv entry for %s: secretName is required." $item.name) }}
{{- end }}
{{- if not $item.secretKey }}
{{- fail (printf "Invalid secretEnv entry for %s: secretKey is required." $item.name) }}
{{- end }}
- name: {{ $item.name }}
  valueFrom:
    secretKeyRef:
      name: {{ $item.secretName }}
      key: {{ $item.secretKey }}
      {{- if hasKey $item "optional" }}
      optional: {{ $item.optional }}
      {{- end }}
{{- end }}
{{- end }}

{{/*
Render ConfigMap/Secret volume items for declarative resources.
Supports both formats:
  - string item: "path/to/file.yaml" (used as key and path)
  - object item: { key: "source-key", path: "target/path.yaml" }
*/}}
{{- define "thunderid.declarativeResourceItems" -}}
{{- $items := default (list) .items -}}
{{- $field := default "declarativeResources.*.items" .field -}}
{{- range $index, $item := $items }}
{{- if kindIs "string" $item }}
- key: {{ $item }}
  path: {{ $item }}
{{- else if kindIs "map" $item }}
{{- if not $item.key }}
{{- fail (printf "Invalid %s entry at index %d: key is required for object items." $field $index) }}
{{- end }}
- key: {{ $item.key }}
  path: {{ default $item.key $item.path }}
{{- else }}
{{- fail (printf "Invalid %s entry at index %d: expected string or object with key/path." $field $index) }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Render file-level volumeMount entries for declarative resources.
When items are provided, mounting file-by-file with subPath preserves existing
files already present in config/resources.
Each item may optionally specify a mountPath to override the global base path.
*/}}
{{- define "thunderid.declarativeResourceVolumeMounts" -}}
{{- $items := default (list) .items -}}
{{- $field := default "declarativeResources.*.items" .field -}}
{{- $globalMountPath := .mountPath -}}
{{- $readOnly := .readOnly -}}
{{- $volumeName := default "declarative-resources" .volumeName -}}
{{- range $index, $item := $items }}
{{- if kindIs "string" $item }}
- name: {{ $volumeName }}
  mountPath: {{ printf "%s/%s" $globalMountPath $item }}
  subPath: {{ $item }}
  readOnly: {{ $readOnly }}
{{- else if kindIs "map" $item }}
{{- if not $item.key }}
{{- fail (printf "Invalid %s entry at index %d: key is required for object items." $field $index) }}
{{- end }}
{{- $path := default $item.key $item.path }}
{{- $effectiveMountPath := $item.mountPath | default (printf "%s/%s" $globalMountPath $path) }}
- name: {{ $volumeName }}
  mountPath: {{ $effectiveMountPath }}
  subPath: {{ $path }}
  readOnly: {{ $readOnly }}
{{- else }}
{{- fail (printf "Invalid %s entry at index %d: expected string or object with key/path." $field $index) }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Render volumeMount entries for singleFile.keys mode.
Each key is mounted as <key>.yaml directly under the given mountPath.
*/}}
{{- define "thunderid.singleFileVolumeMounts" -}}
{{- $keys := default (list) .keys -}}
{{- $mountPath := .mountPath -}}
{{- $readOnly := .readOnly -}}
{{- range $keys }}
- name: declarative-resources
  mountPath: {{ printf "%s/%s.yaml" $mountPath . }}
  subPath: {{ . }}
  readOnly: {{ $readOnly }}
{{- end }}
{{- end }}
