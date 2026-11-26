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
    {{- with .Values.configuration.brand.slogan }}
    slogan: {{ toJson . }},
    {{- end }}
  },
  client: {
    base: {{ .Values.configuration.gateClient.path | quote }},
  },
  {{- if .Values.configuration.server.publicUrl }}
  // Defaults to the origin this app is served from. Required only when the server's
  // external URL differs.
  server: {
    public_url: {{ .Values.configuration.server.publicUrl | quote }},
  },
  {{- end }}
};
