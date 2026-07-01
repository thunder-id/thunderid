#!/bin/bash

DB_TYPE=${DB_TYPE:-sqlite}

cat > tests/integration/resources/deployment.yaml <<EOF
server:
  hostname: localhost
  port: 8095


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

  runtime:
    type: postgres
    postgres:
      hostname: localhost
      port: 5432
      name: runtimedb
      username: dbuser
      password: dbpassword
      sslmode: disable

  user:
    type: postgres
    postgres:
      hostname: localhost
      port: 5432
      name: userdb
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

  runtime:
    type: redis
    redis:
      address: "localhost:6379"
      db: 0
      key_prefix: "thunderid"

  user:
    type: sqlite
    sqlite:
      path: "database/userdb.db"
      options: "cache=shared"
EOF
else
  cat >> tests/integration/resources/deployment.yaml <<EOF
  config:
    type: sqlite
    sqlite:
      path: "database/configdb.db"
      options: "cache=shared"

  runtime:
    type: sqlite
    sqlite:
      path: "database/runtimedb.db"
      options: "cache=shared"

  user:
    type: sqlite
    sqlite:
      path: "database/userdb.db"
      options: "cache=shared"
EOF
fi

cat >> tests/integration/resources/deployment.yaml <<EOF


flow:
  max_version_history: 3

server_config:
  store: composite

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
EOF
