// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package testutils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// The Control Plane run also boots a Data Plane next to the Control Plane, so a suite can take what
// the Control Plane exports to the deployment it configures. The Data Plane is the all-in-one server,
// prepared as the all-in-one run prepares it, in a product home of its own.
const (
	// DataPlanePort is where the Data Plane answers. The Control Plane has serverPort.
	DataPlanePort = 8096
	// DataPlaneURL is the base URL of the Data Plane.
	DataPlaneURL = "https://localhost:8096"
	// DataPlaneExtractedDir holds the Data Plane's product home, beside the Control Plane's.
	DataPlaneExtractedDir = "../../target/out/.test-dataplane"

	// ManagementAPIKey is the key whose digest both planes' deployment.yaml configure. It is what a
	// deployment pipeline presents to a Data Plane to store values and import a configuration.
	ManagementAPIKey = "integration-management-api-key"
	// ManagementAPIKeyHeader is the header the management API key is presented in.
	ManagementAPIKeyHeader = "API-Key"
)

var (
	dataPlaneHome string
	dataPlaneCmd  *exec.Cmd
)

// PrepareDataPlane unzips and bootstraps the all-in-one server as the Data Plane, then moves it out
// of ExtractedDir so the Control Plane can be unzipped there as usual. Call it before UnzipProduct.
func PrepareDataPlane() error {
	ensureInitialized()

	controlPlaneYaml := deploymentYamlSource
	deploymentYamlSource = TestDeploymentYamlPath
	defer func() {
		deploymentYamlSource = controlPlaneYaml
		extractedProductHome = ""
	}()

	if err := UnzipProduct(); err != nil {
		return fmt.Errorf("failed to unzip the data plane: %w", err)
	}
	if err := ReplaceResources(zipFilePattern); err != nil {
		return fmt.Errorf("failed to replace the data plane resources: %w", err)
	}
	if err := setServerPort(filepath.Join(extractedProductHome, "deployment.yaml"), DataPlanePort); err != nil {
		return err
	}
	if err := RunInitScript(zipFilePattern); err != nil {
		return fmt.Errorf("failed to initialize the data plane databases: %w", err)
	}
	if err := RunSetupScript(); err != nil {
		return fmt.Errorf("failed to bootstrap the data plane: %w", err)
	}

	if err := os.RemoveAll(DataPlaneExtractedDir); err != nil {
		return fmt.Errorf("failed to clear the data plane directory: %w", err)
	}
	if err := os.MkdirAll(DataPlaneExtractedDir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create the data plane directory: %w", err)
	}
	home := filepath.Join(DataPlaneExtractedDir, filepath.Base(extractedProductHome))
	if err := os.Rename(extractedProductHome, home); err != nil {
		return fmt.Errorf("failed to move the data plane: %w", err)
	}
	dataPlaneHome = home
	return nil
}

// setServerPort rewrites server.port in the deployment.yaml at path, keeping everything else.
func setServerPort(path string, port int) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", path, err)
	}
	var cfg map[string]interface{}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("failed to parse %s: %w", path, err)
	}
	server, ok := cfg["server"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("%s has no server section", path)
	}
	server["port"] = port
	out, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal %s: %w", path, err)
	}
	return os.WriteFile(path, out, 0o644)
}

// StartDataPlane starts the Data Plane PrepareDataPlane made and waits until it answers. Its output
// goes to thunderid.log in its product home, so it does not interleave with the Control Plane's.
func StartDataPlane() error {
	if dataPlaneHome == "" {
		return fmt.Errorf("the data plane is not prepared, call PrepareDataPlane first")
	}
	logFile, err := os.Create(filepath.Join(dataPlaneHome, "thunderid.log"))
	if err != nil {
		return fmt.Errorf("failed to create the data plane log: %w", err)
	}
	defer func() { _ = logFile.Close() }()

	cmd := exec.Command(filepath.Join(dataPlaneHome, ServerBinary), "-serverHome="+dataPlaneHome)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Env = append(os.Environ(), "GATEWAY_TOKEN="+DeclaredGatewayToken)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start the data plane: %w", err)
	}
	dataPlaneCmd = cmd

	client := GetNoRedirectHTTPClient()
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := client.Get(DataPlaneURL + "/health/liveness")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return nil
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	StopDataPlane()
	return fmt.Errorf("the data plane did not become ready, see %s", logFile.Name())
}

// StopDataPlane stops the Data Plane, giving it time to write its coverage data.
func StopDataPlane() {
	if dataPlaneCmd == nil {
		return
	}
	log.Println("Stopping data plane...")
	done := make(chan error, 1)
	go func() { done <- dataPlaneCmd.Wait() }()
	if err := sendStopSignal(dataPlaneCmd.Process); err != nil {
		_ = dataPlaneCmd.Process.Kill()
	}
	select {
	case <-done:
		time.Sleep(100 * time.Millisecond)
	case <-time.After(10 * time.Second):
		_ = dataPlaneCmd.Process.Kill()
		<-done
	}
	dataPlaneCmd = nil
}

// ImportDeletion names one resource an import removes.
type ImportDeletion struct {
	ResourceType string `json:"resourceType"`
	ID           string `json:"id"`
}

// importOutcome reports what an import did with one resource.
type importOutcome struct {
	ResourceType string `json:"resourceType"`
	ResourceID   string `json:"resourceId"`
	Status       string `json:"status"`
	Code         string `json:"code"`
	Message      string `json:"message"`
}

// ImportWithManagementAPIKey imports the content into the server under test and removes the given
// resources, presenting the management API key as a deployment pipeline does. It fails unless every
// resource is imported or removed.
func ImportWithManagementAPIKey(content string, deletions ...ImportDeletion) error {
	body, err := json.Marshal(map[string]interface{}{
		"content":   content,
		"deletions": deletions,
		"options":   map[string]string{"target": "runtime"},
	})
	if err != nil {
		return fmt.Errorf("failed to marshal the import request: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, TestServerURL+"/import", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create the import request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(ManagementAPIKeyHeader, ManagementAPIKey)

	resp, err := GetNoRedirectHTTPClient().Do(req)
	if err != nil {
		return fmt.Errorf("failed to send the import request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read the import response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("import returned status %d: %s", resp.StatusCode, respBody)
	}

	var importResp struct {
		Results []importOutcome `json:"results"`
	}
	if err := json.Unmarshal(respBody, &importResp); err != nil {
		return fmt.Errorf("failed to parse the import response: %w", err)
	}
	if len(importResp.Results) == 0 {
		return fmt.Errorf("import returned no results")
	}
	for _, outcome := range importResp.Results {
		if outcome.Status != "success" {
			return fmt.Errorf("import of %s %s failed: %s %s",
				outcome.ResourceType, outcome.ResourceID, outcome.Code, outcome.Message)
		}
	}
	return nil
}
