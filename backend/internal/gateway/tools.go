// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// gatewayTools are the MCP tools that move configuration between the gateways this deployment
// administers: capturing a version, deploying it, promoting what one gateway runs to another, and
// rolling a gateway back.
type gatewayTools struct {
	gateways ServiceInterface
	versions VersionServiceInterface
}

// gatewayDeploymentOutput is one gateway with what it runs.
type gatewayDeploymentOutput struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	BaseURL   string `json:"baseUrl"`
	IsDefault bool   `json:"isDefault,omitempty"`
	// AppliedVersion is the hash of the version the gateway runs, empty while it runs none.
	AppliedVersion string `json:"appliedVersion,omitempty"`
	// PreviousVersion is the version a rollback returns the gateway to, empty when there is none.
	PreviousVersion string     `json:"previousVersion,omitempty"`
	AppliedAt       *time.Time `json:"appliedAt,omitempty"`
	AppliedBy       string     `json:"appliedBy,omitempty"`
}

type gatewayListOutput struct {
	Gateways []gatewayDeploymentOutput `json:"gateways"`
}

// versionOutput is a captured version without its content, which is the whole configuration.
type versionOutput struct {
	Number    int       `json:"number"`
	Version   string    `json:"version"`
	Name      string    `json:"name,omitempty"`
	Note      string    `json:"note,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	CreatedBy string    `json:"createdBy,omitempty"`
	Current   bool      `json:"current,omitempty"`
	// Unchanged is set when a capture found the configuration already captured as this version.
	Unchanged bool `json:"unchanged,omitempty"`
	// Skipped names the resources a capture could not write, which the version leaves out.
	Skipped []string `json:"skipped,omitempty"`
}

type versionListOutput struct {
	Versions []versionOutput `json:"versions"`
}

type captureInput struct {
	Name string `json:"name,omitempty" jsonschema:"A name for the version"`
	Note string `json:"note,omitempty" jsonschema:"What the version is for"`
}

type deployInput struct {
	GatewayID string `json:"gatewayId" jsonschema:"The ID of the gateway to deploy to"`
	//nolint:lll
	Version string `json:"version,omitempty" jsonschema:"The version's hash, a prefix of at least seven of its characters, or latest. Defaults to latest"`
	DryRun  bool   `json:"dryRun,omitempty" jsonschema:"Report what the deploy would do without changing the gateway"`
	//nolint:lll
	ConfirmRemovals bool `json:"confirmRemovals,omitempty" jsonschema:"Allow a deploy that removes more resources than the deployment's threshold. Set it only after a dry run showed the removals and the user agreed"`
}

type promoteInput struct {
	SourceGatewayID string `json:"sourceGatewayId" jsonschema:"The ID of the gateway whose running version is promoted"`
	TargetGatewayID string `json:"targetGatewayId" jsonschema:"The ID of the gateway to promote it to"`
	DryRun          bool   `json:"dryRun,omitempty" jsonschema:"Report what the promotion would do, changing nothing"`
	//nolint:lll
	ConfirmRemovals bool `json:"confirmRemovals,omitempty" jsonschema:"Allow a promotion that removes more resources than the deployment's threshold. Set it only after a dry run showed the removals and the user agreed"`
}

type rollbackInput struct {
	GatewayID string `json:"gatewayId" jsonschema:"The ID of the gateway to roll back"`
	DryRun    bool   `json:"dryRun,omitempty" jsonschema:"Report what the rollback would do, changing nothing"`
	//nolint:lll
	ConfirmRemovals bool `json:"confirmRemovals,omitempty" jsonschema:"Allow a rollback that removes more resources than the deployment's threshold. Set it only after a dry run showed the removals and the user agreed"`
}

// changeOutput is one resource a deploy writes, removes or leaves alone, without its line diff.
type changeOutput struct {
	ResourceType string     `json:"resourceType"`
	ID           string     `json:"id,omitempty"`
	Name         string     `json:"name,omitempty"`
	Change       ChangeKind `json:"change"`
	// HeldBack marks a change the gateway leaves alone from an earlier choice, or one this deploy skips.
	HeldBack bool `json:"heldBack,omitempty"`
}

// refusedOutput is a resource the gateway refused to write or remove.
type refusedOutput struct {
	ResourceType string `json:"resourceType"`
	ResourceID   string `json:"resourceId,omitempty"`
	ResourceName string `json:"resourceName,omitempty"`
	Operation    string `json:"operation,omitempty"`
	Code         string `json:"code,omitempty"`
	Message      string `json:"message,omitempty"`
}

// deployOutput is what a deploy, promotion or rollback did, or would do in a dry run.
type deployOutput struct {
	GatewayID   string      `json:"gatewayId"`
	DryRun      bool        `json:"dryRun"`
	FromVersion string      `json:"fromVersion,omitempty"`
	ToVersion   string      `json:"toVersion"`
	Summary     DiffSummary `json:"summary"`
	// Changes leaves out the resources the gateway already holds as they are.
	Changes []changeOutput `json:"changes"`
	// Recorded is set once the gateway runs the version. A dry run never records one.
	Recorded bool `json:"recorded"`
	// Refused lists the resources the gateway refused. The rest of the version was applied, and
	// deploying the same version again sends them again.
	Refused []refusedOutput `json:"refused,omitempty"`
	// MissingVariables and MissingSecrets are values the version refers to that the gateway does not
	// hold. A deploy is refused until they are set on the gateway.
	MissingVariables []string `json:"missingVariables,omitempty"`
	MissingSecrets   []string `json:"missingSecrets,omitempty"`
	// ChangedVariables and ChangedSecrets were set on the gateway since its version was deployed, and
	// take effect when a version is deployed to it again.
	ChangedVariables []string `json:"changedVariables,omitempty"`
	ChangedSecrets   []string `json:"changedSecrets,omitempty"`
	// Queued is set when another deploy to the gateway was running, and the latest version follows it.
	Queued        bool   `json:"queued,omitempty"`
	QueuedVersion string `json:"queuedVersion,omitempty"`
}

// registerMCPTools registers the deployment tools on the server.
func registerMCPTools(server *mcp.Server, gateways ServiceInterface, versions VersionServiceInterface) {
	tools := &gatewayTools{gateways: gateways, versions: versions}

	mcp.AddTool(server, &mcp.Tool{
		Name: "thunderid_list_gateways",
		Description: `List the gateways (data planes) this deployment deploys configuration to, ` +
			`each with the version it runs and the version a rollback would return it to.`,
		Annotations: &mcp.ToolAnnotations{Title: "List Gateways", ReadOnlyHint: true},
	}, tools.listGateways)

	mcp.AddTool(server, &mcp.Tool{
		Name: "thunderid_list_configuration_versions",
		Description: `List the captured configuration versions, newest first. A version is named by its hash, ` +
			`and is what thunderid_deploy_version deploys.`,
		Annotations: &mcp.ToolAnnotations{Title: "List Configuration Versions", ReadOnlyHint: true},
	}, tools.listVersions)

	mcp.AddTool(server, &mcp.Tool{
		Name: "thunderid_capture_configuration_version",
		Description: `Capture the configuration as it is now as a new version, to deploy to a gateway. ` +
			`A configuration already captured is answered with the version that holds it.`,
		Annotations: &mcp.ToolAnnotations{Title: "Capture Configuration Version", IdempotentHint: true},
	}, tools.capture)

	mcp.AddTool(server, &mcp.Tool{
		Name: "thunderid_deploy_version",
		Description: `Deploy a configuration version to a gateway.

Run it with dryRun first and show the user what it would add, update and remove, and any values the
gateway is missing. A deploy is refused while the gateway lacks a value the version refers to; set it
on the gateway, then deploy again. Deploying the version a gateway already runs again sends it whole,
which is how values changed on the gateway take effect and how refused resources are retried.`,
		Annotations: &mcp.ToolAnnotations{Title: "Deploy Configuration Version", DestructiveHint: boolPtr(true)},
	}, tools.deploy)

	mcp.AddTool(server, &mcp.Tool{
		Name: "thunderid_promote_gateway",
		Description: `Promote the version one gateway runs to another, such as from a development gateway to a
production one. Only what the source runs is promoted: changes the source holds back stay out.

Run it with dryRun first and show the user what it would change on the target.`,
		Annotations: &mcp.ToolAnnotations{Title: "Promote Between Gateways", DestructiveHint: boolPtr(true)},
	}, tools.promote)

	mcp.AddTool(server, &mcp.Tool{
		Name: "thunderid_rollback_gateway",
		Description: `Return a gateway to the version it ran before its last deploy. A gateway rolled back has no ` +
			`earlier version to return to until it is deployed again. Run it with dryRun first.`,
		Annotations: &mcp.ToolAnnotations{Title: "Roll Back Gateway", DestructiveHint: boolPtr(true)},
	}, tools.rollback)
}

func boolPtr(b bool) *bool { return &b }

// toolError turns a service error into what a tool call reports.
func toolError(action string, svcErr *tidcommon.ServiceError) error {
	description := svcErr.ErrorDescription.DefaultValue
	if description == "" {
		description = svcErr.Error.DefaultValue
	}
	return fmt.Errorf("failed to %s: %s (%s)", action, description, svcErr.Code)
}

func (t *gatewayTools) listGateways(ctx context.Context, _ *mcp.CallToolRequest, _ any,
) (*mcp.CallToolResult, *gatewayListOutput, error) {
	gateways, svcErr := t.gateways.List(ctx)
	if svcErr != nil {
		return nil, nil, toolError("list the gateways", svcErr)
	}
	output := &gatewayListOutput{Gateways: make([]gatewayDeploymentOutput, 0, len(gateways))}
	for _, gw := range gateways {
		item := gatewayDeploymentOutput{ID: gw.ID, Name: gw.Name, BaseURL: gw.BaseURL, IsDefault: gw.IsDefault}
		applied, svcErr := t.versions.GetApplied(ctx, gw.ID)
		if svcErr != nil {
			return nil, nil, toolError("read what a gateway runs", svcErr)
		}
		item.AppliedVersion, item.PreviousVersion, item.AppliedAt = applied.Applied, applied.Previous, applied.AppliedAt
		if applied.AppliedBy != nil {
			item.AppliedBy = applied.AppliedBy.Name
		}
		output.Gateways = append(output.Gateways, item)
	}
	return nil, output, nil
}

func (t *gatewayTools) listVersions(ctx context.Context, _ *mcp.CallToolRequest, _ any,
) (*mcp.CallToolResult, *versionListOutput, error) {
	versions, svcErr := t.versions.ListVersions(ctx)
	if svcErr != nil {
		return nil, nil, toolError("list the versions", svcErr)
	}
	output := &versionListOutput{Versions: make([]versionOutput, 0, len(versions))}
	for i := range versions {
		output.Versions = append(output.Versions, versionOutputOf(&versions[i]))
	}
	return nil, output, nil
}

func (t *gatewayTools) capture(ctx context.Context, _ *mcp.CallToolRequest, input captureInput,
) (*mcp.CallToolResult, *versionOutput, error) {
	version, svcErr := t.versions.Capture(ctx, CaptureRequest{Name: input.Name, Note: input.Note})
	if svcErr != nil {
		return nil, nil, toolError("capture a version", svcErr)
	}
	output := versionOutputOf(version)
	return nil, &output, nil
}

func (t *gatewayTools) deploy(ctx context.Context, _ *mcp.CallToolRequest, input deployInput,
) (*mcp.CallToolResult, *deployOutput, error) {
	result, svcErr := t.versions.Apply(ctx, input.GatewayID, ApplyRequest{
		Version: input.Version, DryRun: input.DryRun, ConfirmRemovals: input.ConfirmRemovals,
	})
	if svcErr != nil {
		return nil, nil, toolError("deploy the version", svcErr)
	}
	return nil, deployOutputOf(result), nil
}

// promote applies the version the source runs to the target, as the console promotes one stage to
// the next: the changes the source holds back are skipped, because the source does not run them.
func (t *gatewayTools) promote(ctx context.Context, _ *mcp.CallToolRequest, input promoteInput,
) (*mcp.CallToolResult, *deployOutput, error) {
	if input.SourceGatewayID == input.TargetGatewayID {
		return nil, nil, fmt.Errorf("failed to promote: the source and target are the same gateway")
	}
	source, svcErr := t.versions.GetApplied(ctx, input.SourceGatewayID)
	if svcErr != nil {
		return nil, nil, toolError("read what the source gateway runs", svcErr)
	}
	if source.Applied == "" {
		return nil, nil, fmt.Errorf("failed to promote: the source gateway runs no version yet")
	}
	// What the source holds back of the version it runs is what applying it there again leaves alone.
	held, svcErr := t.versions.Apply(ctx, input.SourceGatewayID, ApplyRequest{Version: source.Applied, DryRun: true})
	if svcErr != nil {
		return nil, nil, toolError("read what the source gateway holds back", svcErr)
	}
	skip := []string{}
	for _, change := range held.Diff.Changes {
		if change.Excluded && change.Change != ChangeUnchanged {
			skip = append(skip, change.Key)
		}
	}
	result, svcErr := t.versions.Apply(ctx, input.TargetGatewayID, ApplyRequest{
		Version: source.Applied, DryRun: input.DryRun, Skip: skip, ConfirmRemovals: input.ConfirmRemovals,
	})
	if svcErr != nil {
		return nil, nil, toolError("promote the version", svcErr)
	}
	return nil, deployOutputOf(result), nil
}

func (t *gatewayTools) rollback(ctx context.Context, _ *mcp.CallToolRequest, input rollbackInput,
) (*mcp.CallToolResult, *deployOutput, error) {
	result, svcErr := t.versions.Revert(ctx, input.GatewayID, RevertRequest{
		DryRun: input.DryRun, ConfirmRemovals: input.ConfirmRemovals,
	})
	if svcErr != nil {
		return nil, nil, toolError("roll back the gateway", svcErr)
	}
	return nil, deployOutputOf(result), nil
}

func versionOutputOf(version *Version) versionOutput {
	output := versionOutput{
		Number: version.Seq, Version: version.Hash, Name: version.Name, Note: version.Note,
		CreatedAt: version.CreatedAt, Current: version.Current, Unchanged: version.Unchanged,
	}
	if version.CreatedBy != nil {
		output.CreatedBy = version.CreatedBy.Name
	}
	for _, skipped := range version.Skipped {
		output.Skipped = append(output.Skipped,
			fmt.Sprintf("%s %s: %s", skipped.ResourceType, skipped.ResourceID, skipped.Error))
	}
	return output
}

// deployOutputOf reads an apply's result, with the gateway's own account of the import read for the
// resources it refused.
func deployOutputOf(result *ApplyResult) *deployOutput {
	output := &deployOutput{
		GatewayID: result.GatewayID, DryRun: result.DryRun,
		FromVersion: result.Diff.FromVersion, ToVersion: result.Diff.ToVersion,
		Summary: result.Diff.Summary, Changes: []changeOutput{}, Recorded: result.Recorded,
		Queued: result.Queued, QueuedVersion: result.QueuedVersion,
	}
	for _, change := range result.Diff.Changes {
		if change.Change == ChangeUnchanged {
			continue
		}
		output.Changes = append(output.Changes, changeOutput{
			ResourceType: change.ResourceType, ID: change.ID, Name: change.Name, Change: change.Change,
			HeldBack: change.Excluded || change.Skipped,
		})
	}
	if result.Missing != nil {
		output.MissingVariables, output.MissingSecrets = result.Missing.Variables, result.Missing.Secrets
	}
	if result.Changed != nil {
		output.ChangedVariables, output.ChangedSecrets = result.Changed.Variables, result.Changed.Secrets
	}
	output.Refused = refusedOf(result.Import)
	return output
}

// refusedOf reads the resources a gateway's import refused from its answer. An answer it cannot read
// is reported as refusing nothing; the summary still says what the deploy changed.
func refusedOf(answer json.RawMessage) []refusedOutput {
	if len(answer) == 0 {
		return nil
	}
	var parsed struct {
		Results []struct {
			ResourceType string `json:"resourceType"`
			ResourceID   string `json:"resourceId"`
			ResourceName string `json:"resourceName"`
			Operation    string `json:"operation"`
			Status       string `json:"status"`
			Code         string `json:"code"`
			Message      string `json:"message"`
		} `json:"results"`
	}
	if err := json.Unmarshal(answer, &parsed); err != nil {
		return nil
	}
	var refused []refusedOutput
	for _, item := range parsed.Results {
		if item.Status != "failed" {
			continue
		}
		refused = append(refused, refusedOutput{
			ResourceType: item.ResourceType, ResourceID: item.ResourceID, ResourceName: item.ResourceName,
			Operation: item.Operation, Code: item.Code, Message: item.Message,
		})
	}
	return refused
}
