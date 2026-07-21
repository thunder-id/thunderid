/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

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
