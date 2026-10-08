// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/* eslint-disable no-underscore-dangle */

window.__THUNDERID_RUNTIME_CONFIG__ = {
  brand: {
    product_name: {{ .Values.configuration.brand.productName | default "ThunderID" | quote }},
    favicon: {
      light: {{ .Values.configuration.brand.favicon.light | default "assets/images/favicon.ico" | quote }},
      dark: {{ .Values.configuration.brand.favicon.dark | default "assets/images/favicon-inverted.ico" | quote }},
    },
  },
  client: {
    base: {{ .Values.configuration.consoleClient.path | quote }},
    client_id: {{ .Values.configuration.consoleClient.clientId | quote }},
    scopes: {{ .Values.configuration.consoleClient.scopes }},
    {{- $resourceIdentifier := .Values.configuration.consoleClient.resourceIdentifier | default (printf "%s/mcp" .Values.configuration.server.publicUrl) }}
    {{- if $resourceIdentifier }}
    resource_identifier: {{ $resourceIdentifier | quote }},
    {{- end }}
  },
  // Mirrors direct_api.enabled in deployment.yaml. When false, the Console hides the Direct API
  // endpoints.
  direct_api: {
    enabled: {{ .Values.configuration.directApi.enabled }},
  },
  {{- if .Values.configuration.server.publicUrl }}
  // Defaults to the origin this app is served from. Required only when the server's
  // external URL differs.
  server: {
    public_url: {{ .Values.configuration.server.publicUrl | quote }},
  },
  {{- end }}
  {{- if .Values.configuration.consoleClient.trustedIssuer }}
  trusted_issuer: {
    hostname: {{ .Values.configuration.consoleClient.trustedIssuer.hostname | quote }},
    port: {{ .Values.configuration.consoleClient.trustedIssuer.port }},
    http_only: {{ .Values.configuration.consoleClient.trustedIssuer.httpOnly }},
    {{- if .Values.configuration.consoleClient.trustedIssuer.publicUrl }}
    public_url: {{ .Values.configuration.consoleClient.trustedIssuer.publicUrl | quote }},
    {{- end }}
    {{- if .Values.configuration.consoleClient.trustedIssuer.clientId }}
    client_id: {{ .Values.configuration.consoleClient.trustedIssuer.clientId | quote }},
    {{- end }}
    {{- if .Values.configuration.consoleClient.trustedIssuer.scopes }}
    scopes: {{ .Values.configuration.consoleClient.trustedIssuer.scopes }},
    {{- end }}
    {{- if .Values.configuration.consoleClient.trustedIssuer.type }}
    type: {{ .Values.configuration.consoleClient.trustedIssuer.type | quote }},
    {{- end }}
  },
  {{- end }}
  {{- if .Values.configuration.gateClient.hostname }}
  // Location of the login gate, used to build the OAuth redirect URI shown when configuring
  // social/OIDC connections. Omit to default to `${server public_url}/gate/callback`.
  gate_client: {
    hostname: {{ .Values.configuration.gateClient.hostname | quote }},
    {{- if .Values.configuration.gateClient.port }}
    port: {{ .Values.configuration.gateClient.port }},
    {{- end }}
    {{- if .Values.configuration.gateClient.scheme }}
    scheme: {{ .Values.configuration.gateClient.scheme | quote }},
    {{- end }}
  },
  {{- end }}
};
