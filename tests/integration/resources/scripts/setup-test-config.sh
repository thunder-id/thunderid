#!/bin/bash

DB_TYPE=${DB_TYPE:-sqlite}

cat > tests/integration/resources/deployment.yaml <<EOF
server:
  hostname: localhost
  port: 8095
  security:
    # Shortened from the 60s default so revocation-enforcement tests (which check a token
    # immediately after revoking it) don't need a long sleep to observe the deny-list cache pick
    # up the revocation. Introspection (RFC 7009 hot path) is unaffected — it reads the store
    # directly rather than through this periodic cache.
    token_revocation:
      sync_interval_seconds: 2


tls:
  cert_file: "config/certs/server.cert"
  key_file: "config/certs/server.key"

database:
EOF

if [ "$DB_TYPE" = "postgres" ]; then
  cat >> tests/integration/resources/deployment.yaml <<EOF
  config:
    type: postgres
    postgres:
      hostname: localhost
      port: 5432
      name: configdb
      username: dbuser
      password: dbpassword
      sslmode: disable

  runtime_transient:
    type: postgres
    postgres:
      hostname: localhost
      port: 5432
      name: runtime_transient
      username: dbuser
      password: dbpassword
      sslmode: disable

  entity:
    type: postgres
    postgres:
      hostname: localhost
      port: 5432
      name: entitydb
      username: dbuser
      password: dbpassword
      sslmode: disable

  runtime_persistent:
    type: postgres
    postgres:
      hostname: localhost
      port: 5432
      name: runtime_persistent
      username: dbuser
      password: dbpassword
      sslmode: disable
EOF
elif [ "$DB_TYPE" = "redis" ]; then
  cat >> tests/integration/resources/deployment.yaml <<EOF
  config:
    type: sqlite
    sqlite:
      path: "database/configdb.db"
      options: "cache=shared"

  runtime_transient:
    type: redis
    redis:
      address: "localhost:6379"
      db: 0
      key_prefix: "thunderid"

  entity:
    type: sqlite
    sqlite:
      path: "database/entitydb.db"
      options: "cache=shared"

  runtime_persistent:
    type: sqlite
    sqlite:
      path: "database/runtime_persistent.db"
      options: "cache=shared"
EOF
else
  cat >> tests/integration/resources/deployment.yaml <<EOF
  config:
    type: sqlite
    sqlite:
      path: "database/configdb.db"
      options: "cache=shared"

  runtime_transient:
    type: sqlite
    sqlite:
      path: "database/runtime_transient.db"
      options: "cache=shared"

  entity:
    type: sqlite
    sqlite:
      path: "database/entitydb.db"
      options: "cache=shared"

  runtime_persistent:
    type: sqlite
    sqlite:
      path: "database/runtime_persistent.db"
      options: "cache=shared"
EOF
fi

cat >> tests/integration/resources/deployment.yaml <<EOF


flow:
  max_version_history: 3

server_config:
  store: composite

passkey:
  allowed_origins:
    - "https://localhost:8095"
    - "http://localhost:8095"

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
EOF
