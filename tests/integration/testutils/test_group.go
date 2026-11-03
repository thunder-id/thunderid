// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package testutils

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// TestGroupEnv names the test group a process belongs to. The runner sets it on the process it starts
// for each group, and the group's test packages inherit it.
const TestGroupEnv = "INTEGRATION_GROUP"

// postgresSchemaDirs maps each database section of deployment.yaml to its schema under dbscripts/.
var postgresSchemaDirs = map[string]string{
	"config":             "configdb",
	"runtime_transient":  "runtime_transient",
	"entity":             "entitydb",
	"runtime_persistent": "runtime_persistent",
}

var (
	serverPortLine   = regexp.MustCompile(`(?m)^(\s*port:\s*)` + DefaultServerPort + `\s*$`)
	postgresNameLine = regexp.MustCompile(`(?m)^(\s*name:\s*)(configdb|runtime_transient|entitydb|runtime_persistent)\s*$`)
	redisPrefixLine  = regexp.MustCompile(`(?m)^(\s*key_prefix:\s*)"thunderid"\s*$`)
)

// TestGroup returns the test group this process runs, or "" when the run has no groups.
func TestGroup() string {
	return os.Getenv(TestGroupEnv)
}

// ConfigureTestGroup points the extracted deployment.yaml at the group's own server port and
// databases, so groups running at the same time share nothing. The file is edited as text because
// setup.sh reads the first port line of it without a YAML parser.
func ConfigureTestGroup(group, port string) error {
	path := filepath.Join(extractedProductHome, "deployment.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read deployment.yaml: %w", err)
	}

	content := strings.ReplaceAll(string(data), "localhost:"+DefaultServerPort, "localhost:"+port)
	content = serverPortLine.ReplaceAllString(content, "${1}"+port)
	content = postgresNameLine.ReplaceAllString(content, "${1}${2}_"+group)
	content = redisPrefixLine.ReplaceAllString(content, `${1}"thunderid-`+group+`"`)

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write deployment.yaml: %w", err)
	}
	return nil
}

// createPostgresDatabases creates each PostgreSQL database deployment.yaml names, from the schema the
// product ships, dropping one a previous run left behind.
func createPostgresDatabases() error {
	section, err := ReadDeploymentConfigKey("database")
	if err != nil {
		return err
	}
	databases, _ := section.(map[string]interface{})
	for key, schemaDir := range postgresSchemaDirs {
		database, _ := databases[key].(map[string]interface{})
		conn, _ := database["postgres"].(map[string]interface{})
		if conn == nil {
			continue
		}
		name := fmt.Sprint(conn["name"])
		log.Printf("Creating PostgreSQL database: %s", name)
		if _, err := runPsql(conn, "postgres", "-c", fmt.Sprintf("DROP DATABASE IF EXISTS %s", name)); err != nil {
			return err
		}
		if _, err := runPsql(conn, "postgres", "-c", fmt.Sprintf("CREATE DATABASE %s", name)); err != nil {
			return err
		}
		schema := filepath.Join(extractedProductHome, "dbscripts", schemaDir, "postgres.sql")
		if _, err := runPsql(conn, name, "-f", schema); err != nil {
			return fmt.Errorf("failed to create the %s schema: %w", name, err)
		}
	}
	return nil
}
