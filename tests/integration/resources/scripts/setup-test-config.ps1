#!/usr/bin/env pwsh
# Copyright 2026 The ThunderID Authors
# SPDX-License-Identifier: Apache-2.0

# Setup Test Configuration
# Generates the deployment.yaml for integration tests

param(
    [string]$DbType = $env:DB_TYPE
)

if (-not $DbType) {
    $DbType = "sqlite"
}

$configPath = "tests/integration/resources/deployment.yaml"

$header = @"
server:
  hostname: localhost
  port: 8095
  security:
    # Exercised by tests/integration/managementapikey: a deployment pipeline authenticating to the
    # import API without first obtaining OAuth client credentials. This is the SHA-256 digest of
    # "integration-management-api-key"; the key itself is never configured here.
    management_api_key_hash: "4955d93012fc5e2ee07b527f13ffe55de32900c410cc198f4d7a9971bb3a9a2d"


gateway:
  # The default of one gateway would refuse the second registration in every uniqueness test before
  # the rule under test was reached.
  max_gateways: 5

tls:
  cert_file: "config/certs/server.cert"
  key_file: "config/certs/server.key"

database:
"@

if ($DbType -eq "postgres") {
    $dbConfig = @"
  config:
    type: postgres
    hostname: localhost
    port: 5432
    name: configdb
    username: dbuser
    password: dbpassword
    sslmode: disable
    path: ""
    options: ""

  runtime_transient:
    type: postgres
    hostname: localhost
    port: 5432
    name: runtime_transient
    username: dbuser
    password: dbpassword
    sslmode: disable
    path: ""
    options: ""

  entity:
    type: postgres
    hostname: localhost
    port: 5432
    name: entitydb
    username: dbuser
    password: dbpassword
    sslmode: disable
    path: ""
    options: ""

  runtime_persistent:
    type: postgres
    hostname: localhost
    port: 5432
    name: runtime_persistent
    username: dbuser
    password: dbpassword
    sslmode: disable
    path: ""
    options: ""
"@
} else {
    $dbConfig = @"
  config:
    type: sqlite
    hostname: ""
    port: 0
    name: ""
    username: ""
    password: ""
    sslmode: ""
    path: "database/configdb.db"
    options: "cache=shared"

  runtime_transient:
    type: sqlite
    hostname: ""
    port: 0
    name: ""
    username: ""
    password: ""
    sslmode: ""
    path: "database/runtime_transient.db"
    options: "cache=shared"

  entity:
    type: sqlite
    hostname: ""
    port: 0
    name: ""
    username: ""
    password: ""
    sslmode: ""
    path: "database/entitydb.db"
    options: "cache=shared"

  runtime_persistent:
    type: sqlite
    hostname: ""
    port: 0
    name: ""
    username: ""
    password: ""
    sslmode: ""
    path: "database/runtime_persistent.db"
    options: "cache=shared"
"@
}

$footer = @"


flow:
  max_version_history: 3

server_config:
  store: composite

oauth:
  allow_wildcard_redirect_uri: true
  send_server_errors_to_client: true
  auth_class:
    amrs:
      - PWD
      - OTP
      - BIO
    acr_amr:
      "urn:thunder:acr:password":
        - PWD
      "urn:thunder:acr:generated-code":
        - OTP
      "urn:thunder:acr:biometrics":
        - BIO
"@

$content = $header + "`n" + $dbConfig + $footer
Set-Content -Path $configPath -Value $content -NoNewline
Write-Host "Generated test config: $configPath (DB_TYPE=$DbType)"
