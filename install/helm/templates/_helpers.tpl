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
{{- $runtimeTransient := default dict $database.runtime_transient -}}
{{- $entity := default dict $database.entity -}}
{{- $configPostgres := default dict $config.postgres -}}
{{- $runtimeTransientPostgres := default dict $runtimeTransient.postgres -}}
{{- $runtimeTransientRedis := default dict $runtimeTransient.redis -}}
{{- $entityPostgres := default dict $entity.postgres -}}
{{- $runtimePersistent := default dict $database.runtime_persistent -}}
{{- $runtimePersistentPostgres := default dict $runtimePersistent.postgres -}}
{{- $cache := default dict $configuration.cache -}}
{{- $redis := default dict $cache.redis -}}
{{- if or (and $configPostgres.password (not (default dict $configPostgres.passwordRef).key)) (and $runtimeTransientPostgres.password (not (default dict $runtimeTransientPostgres.passwordRef).key)) (and $runtimeTransientRedis.password (not (default dict $runtimeTransientRedis.passwordRef).key)) (and $entityPostgres.password (not (default dict $entityPostgres.passwordRef).key)) (and $runtimePersistentPostgres.password (not (default dict $runtimePersistentPostgres.passwordRef).key)) (and $redis.password (eq $cache.type "redis") (not (default dict $redis.passwordRef).key)) }}true{{- end }}
{{- end }}

{{/*
Generate database password environment variable definitions for both deployment and setup job.
Injects DB_CONFIG_PASSWORD, DB_RUNTIME_TRANSIENT_PASSWORD, DB_ENTITY_PASSWORD, and DB_RUNTIME_PERSISTENT_PASSWORD from either auto-generated or external Secrets.
*/}}
{{- define "thunderid.databasePasswordEnvVars" -}}
{{- $defaultDbSecretName := printf "%s-db-credentials" (include "thunderid.fullname" .) -}}
{{- $configuration := default dict .Values.configuration -}}
{{- $database := default dict $configuration.database -}}
{{- $config := default dict $database.config -}}
{{- $runtimeTransient := default dict $database.runtime_transient -}}
{{- $entity := default dict $database.entity -}}
{{- $runtimePersistent := default dict $database.runtime_persistent -}}
{{- $configPostgres := default dict $config.postgres -}}
{{- $runtimeTransientPostgres := default dict $runtimeTransient.postgres -}}
{{- $runtimeTransientRedis := default dict $runtimeTransient.redis -}}
{{- $entityPostgres := default dict $entity.postgres -}}
{{- $runtimePersistentPostgres := default dict $runtimePersistent.postgres -}}
{{- $configPasswordRef := default dict $configPostgres.passwordRef -}}
{{- $runtimeTransientPasswordRef := default dict $runtimeTransientPostgres.passwordRef -}}
{{- $runtimeTransientRedisPasswordRef := default dict $runtimeTransientRedis.passwordRef -}}
{{- $entityPasswordRef := default dict $entityPostgres.passwordRef -}}
{{- $runtimePersistentPasswordRef := default dict $runtimePersistentPostgres.passwordRef -}}
{{- if or $configPostgres.password $configPasswordRef.key }}
- name: DB_CONFIG_PASSWORD
  valueFrom:
    secretKeyRef:
      name: {{ if $configPasswordRef.key }}{{ $configPasswordRef.name | default $defaultDbSecretName }}{{ else }}{{ $defaultDbSecretName }}{{ end }}
      key: {{ $configPasswordRef.key | default "config-db-password" }}
{{- end }}
{{- if or $runtimeTransientPostgres.password $runtimeTransientPasswordRef.key }}
- name: DB_RUNTIME_TRANSIENT_PASSWORD
  valueFrom:
    secretKeyRef:
      name: {{ if $runtimeTransientPasswordRef.key }}{{ $runtimeTransientPasswordRef.name | default $defaultDbSecretName }}{{ else }}{{ $defaultDbSecretName }}{{ end }}
      key: {{ $runtimeTransientPasswordRef.key | default "runtime-transient-db-password" }}
{{- end }}
{{- if or $runtimeTransientRedis.password $runtimeTransientRedisPasswordRef.key }}
- name: DB_RUNTIME_TRANSIENT_REDIS_PASSWORD
  valueFrom:
    secretKeyRef:
      name: {{ if $runtimeTransientRedisPasswordRef.key }}{{ $runtimeTransientRedisPasswordRef.name | default $defaultDbSecretName }}{{ else }}{{ $defaultDbSecretName }}{{ end }}
      key: {{ $runtimeTransientRedisPasswordRef.key | default "runtime-transient-redis-password" }}
{{- end }}
{{- if or $entityPostgres.password $entityPasswordRef.key }}
- name: DB_ENTITY_PASSWORD
  valueFrom:
    secretKeyRef:
      name: {{ if $entityPasswordRef.key }}{{ $entityPasswordRef.name | default $defaultDbSecretName }}{{ else }}{{ $defaultDbSecretName }}{{ end }}
      key: {{ $entityPasswordRef.key | default "entity-db-password" }}
{{- end }}
{{- if or $runtimePersistentPostgres.password $runtimePersistentPasswordRef.key }}
- name: DB_RUNTIME_PERSISTENT_PASSWORD
  valueFrom:
    secretKeyRef:
      name: {{ if $runtimePersistentPasswordRef.key }}{{ $runtimePersistentPasswordRef.name | default $defaultDbSecretName }}{{ else }}{{ $defaultDbSecretName }}{{ end }}
      key: {{ $runtimePersistentPasswordRef.key | default "runtime-persistent-db-password" }}
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
