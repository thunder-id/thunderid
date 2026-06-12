#!/usr/bin/env pwsh
# ----------------------------------------------------------------------------
# Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
#
# WSO2 LLC. licenses this file to you under the Apache License,
# Version 2.0 (the "License"); you may not use this file except
# in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing,
# software distributed under the License is distributed on an
# "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
# KIND, either express or implied. See the License for the
# specific language governing permissions and limitations
# under the License.
# ----------------------------------------------------------------------------

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

  runtime:
    type: postgres
    hostname: localhost
    port: 5432
    name: runtimedb
    username: dbuser
    password: dbpassword
    sslmode: disable
    path: ""
    options: ""

  user:
    type: postgres
    hostname: localhost
    port: 5432
    name: userdb
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

  runtime:
    type: sqlite
    hostname: ""
    port: 0
    name: ""
    username: ""
    password: ""
    sslmode: ""
    path: "database/runtimedb.db"
    options: "cache=shared"

  user:
    type: sqlite
    hostname: ""
    port: 0
    name: ""
    username: ""
    password: ""
    sslmode: ""
    path: "database/userdb.db"
    options: "cache=shared"
"@
}

$footer = @"


flow:
  max_version_history: 3

oauth:
  allow_wildcard_redirect_uri: true
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
