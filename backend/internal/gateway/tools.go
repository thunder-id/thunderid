// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/thunder-id/thunderid/internal/variablestore"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// gatewayTools serve, as MCP tools, what the gateway, configuration version and gateway store APIs
// serve over REST: the gateways and what each runs, the captured versions, deploying a version,
// promoting what one gateway runs to another, rolling a gateway back, and the values a gateway fills
// configuration with.
//
// Each tool answers with the body its REST endpoint answers with, so a client that works with one
// works with the other: a list is given under a key, since a tool answers with an object.
type gatewayTools struct {
	gateways ServiceInterface
	versions VersionServiceInterface
	applied  AppliedConfigurationServiceInterface
	stores   StoreServiceInterface
}

// restBody is a tool's answer: the REST endpoint's body as an object.
type restBody = map[string]any

type gatewayInput struct {
	GatewayID string `json:"gatewayId" jsonschema:"The ID of the gateway"`
}

type versionInput struct {
	//nolint:lll
	Version string `json:"version" jsonschema:"The version's hash, a prefix of at least seven of its characters, latest, or current (the configuration as it stands, not captured)"`
}

type captureInput struct {
	Name string `json:"name,omitempty" jsonschema:"A name for the version"`
	Note string `json:"note,omitempty" jsonschema:"What the version is for"`
}

type diffInput struct {
	GatewayID string `json:"gatewayId" jsonschema:"The ID of the gateway"`
	//nolint:lll
	Version string `json:"version,omitempty" jsonschema:"The version's hash or a prefix of it, or latest. Defaults to latest"`
}

type deployInput struct {
	GatewayID string `json:"gatewayId" jsonschema:"The ID of the gateway to deploy to"`
	//nolint:lll
	Version string `json:"version,omitempty" jsonschema:"The version's hash or a prefix of it, or latest. Defaults to latest"`
	DryRun  bool   `json:"dryRun,omitempty" jsonschema:"Report what the deploy would do, changing nothing"`
	//nolint:lll
	Selection *[]string `json:"selection,omitempty" jsonschema:"The keys, from a dry run's changes, of the changes to deploy. The gateway goes on leaving the others alone until a later deploy selects them. Omit to keep the gateway's standing choice"`
	//nolint:lll
	Skip []string `json:"skip,omitempty" jsonschema:"The keys of changes to leave alone in this deploy only, keeping the gateway's standing choice for them"`
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

type listValuesInput struct {
	GatewayID string `json:"gatewayId" jsonschema:"The ID of the gateway"`
	Limit     int    `json:"limit,omitempty" jsonschema:"The most values to return"`
	Offset    int    `json:"offset,omitempty" jsonschema:"How many values to skip"`
	Names     string `json:"names,omitempty" jsonschema:"Comma-separated names to return, leaving the others out"`
}

type setValueInput struct {
	GatewayID   string `json:"gatewayId" jsonschema:"The ID of the gateway"`
	Name        string `json:"name" jsonschema:"The value's name, as a configuration version refers to it"`
	Value       string `json:"value" jsonschema:"The value"`
	Description string `json:"description,omitempty" jsonschema:"What the value is for"`
}

type valueNameInput struct {
	GatewayID string `json:"gatewayId" jsonschema:"The ID of the gateway"`
	Name      string `json:"name" jsonschema:"The value's name"`
}

// registerMCPTools registers the gateway tools on the server.
func registerMCPTools(server *mcp.Server, tools *gatewayTools) {
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true}
	destructive := true

	mcp.AddTool(server, &mcp.Tool{
		Name: "thunderid_list_gateways",
		Description: `List the gateways (data planes) this deployment deploys configuration to, as GET /gateways ` +
			`does. Each gateway's baseUrl is where applications sign their users in.`,
		Annotations: readOnly,
	}, tools.listGateways)

	mcp.AddTool(server, &mcp.Tool{
		Name: "thunderid_get_gateway_applied_version",
		Description: `Report the version a gateway runs, the one a rollback returns it to, and when and by whom ` +
			`it was deployed, as GET /gateways/{id}/applied-version does.`,
		Annotations: readOnly,
	}, tools.appliedVersion)

	mcp.AddTool(server, &mcp.Tool{
		Name: "thunderid_get_gateway_configuration",
		Description: `Show the configuration a gateway runs, each resource as its own read returns it, as ` +
			`GET /gateways/{id}/applied-configuration does.`,
		Annotations: readOnly,
	}, tools.appliedConfiguration)

	mcp.AddTool(server, &mcp.Tool{
		Name: "thunderid_list_configuration_versions",
		Description: `List the captured configuration versions, newest first and without their content, as ` +
			`GET /configuration-versions does. A version is named by its hash.`,
		Annotations: readOnly,
	}, tools.listVersions)

	mcp.AddTool(server, &mcp.Tool{
		Name: "thunderid_get_configuration_version",
		Description: `Read one configuration version with its content and each place it refers to a variable or ` +
			`a secret, as GET /configuration-versions/{version} does.`,
		Annotations: readOnly,
	}, tools.getVersion)

	mcp.AddTool(server, &mcp.Tool{
		Name: "thunderid_capture_configuration_version",
		Description: `Capture the configuration as it is now as a new version, to deploy to a gateway, as ` +
			`POST /configuration-versions does. A configuration already captured is answered with its version.`,
		Annotations: &mcp.ToolAnnotations{IdempotentHint: true},
	}, tools.capture)

	mcp.AddTool(server, &mcp.Tool{
		Name: "thunderid_diff_version",
		Description: `Report how a version differs from what a gateway runs, resource by resource and line by ` +
			`line, as GET /gateways/{id}/diff does. It changes nothing.`,
		Annotations: readOnly,
	}, tools.diff)

	mcp.AddTool(server, &mcp.Tool{
		Name: "thunderid_deploy_version",
		Description: `Deploy a configuration version to a gateway, as POST /gateways/{id}/apply does.

Run it with dryRun first and show the user what it would add, update and remove, and any values the
gateway is missing. A deploy is refused while the gateway lacks a value the version refers to; set it
on the gateway, then deploy again. The answer's import lists any resource the gateway refused: the rest
was deployed, and deploying the same version again retries it. Deploying the version a gateway already
runs again is also how values changed on the gateway take effect.`,
		Annotations: &mcp.ToolAnnotations{DestructiveHint: &destructive},
	}, tools.deploy)

	mcp.AddTool(server, &mcp.Tool{
		Name: "thunderid_promote_gateway",
		Description: `Deploy the version one gateway runs to another, as the Console promotes one stage to the ` +
			`next. Only what the source runs is promoted: changes the source holds back stay out.

Run it with dryRun first and show the user what it would change on the target.`,
		Annotations: &mcp.ToolAnnotations{DestructiveHint: &destructive},
	}, tools.promote)

	mcp.AddTool(server, &mcp.Tool{
		Name: "thunderid_rollback_gateway",
		Description: `Return a gateway to the version it ran before its last deploy, as POST /gateways/{id}/revert ` +
			`does. A gateway rolled back has no earlier version to return to until it is deployed again. Run it ` +
			`with dryRun first.`,
		Annotations: &mcp.ToolAnnotations{DestructiveHint: &destructive},
	}, tools.rollback)

	registerValueTools(server, collectionVariables, "variable", tools.listVariables, tools.setVariable,
		tools.deleteVariable)
	registerValueTools(server, collectionSecrets, "secret", tools.listSecrets, tools.setSecret, tools.deleteSecret)
}

// registerValueTools registers the tools for one of a gateway's value collections: its variables or
// its secrets.
func registerValueTools(server *mcp.Server, collection, kind string,
	list mcp.ToolHandlerFor[listValuesInput, restBody], set mcp.ToolHandlerFor[setValueInput, restBody],
	remove mcp.ToolHandlerFor[valueNameInput, restBody]) {
	destructive := true
	listNote := ""
	setNote := ""
	if kind == "secret" {
		listNote = " A secret's value is never returned."
		setNote = " Ask the user for the value rather than making one up, and do not repeat it back."
	}
	mcp.AddTool(server, &mcp.Tool{
		Name: "thunderid_list_gateway_" + collection,
		Description: fmt.Sprintf(`List the %ss a gateway holds, as GET /gateways/{id}/%s does.%s`,
			kind, collection, listNote),
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
	}, list)
	mcp.AddTool(server, &mcp.Tool{
		Name: "thunderid_set_gateway_" + kind,
		Description: fmt.Sprintf(`Set a %s on a gateway, creating it when the gateway does not hold it, as `+
			`PUT /gateways/{id}/%s/{name} does. A deployed configuration takes it up when a version is deployed `+
			`to the gateway again.%s`, kind, collection, setNote),
		Annotations: &mcp.ToolAnnotations{IdempotentHint: true},
	}, set)
	mcp.AddTool(server, &mcp.Tool{
		Name: "thunderid_delete_gateway_" + kind,
		Description: fmt.Sprintf(`Remove a %s from a gateway, as DELETE /gateways/{id}/%s/{name} does.`,
			kind, collection),
		Annotations: &mcp.ToolAnnotations{DestructiveHint: &destructive},
	}, remove)
}

// asBody gives a REST body as a tool's answer, so the two are the same JSON.
func asBody(value any) (restBody, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	body := restBody{}
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil, err
	}
	return body, nil
}

// answer is a tool's result: the body, or why the call failed.
func answer(action string, value any, svcErr *tidcommon.ServiceError) (*mcp.CallToolResult, restBody, error) {
	if svcErr != nil {
		return nil, nil, toolError(action, svcErr)
	}
	body, err := asBody(value)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to %s: %w", action, err)
	}
	return nil, body, nil
}

// toolError turns a service error into what a tool call reports, with its code, as REST reports it.
func toolError(action string, svcErr *tidcommon.ServiceError) error {
	description := svcErr.ErrorDescription.DefaultValue
	if description == "" {
		description = svcErr.Error.DefaultValue
	}
	return fmt.Errorf("failed to %s: %s (%s)", action, description, svcErr.Code)
}

func (t *gatewayTools) listGateways(ctx context.Context, _ *mcp.CallToolRequest, _ any,
) (*mcp.CallToolResult, restBody, error) {
	gateways, svcErr := t.gateways.List(ctx)
	return answer("list the gateways", map[string]any{"gateways": gateways}, svcErr)
}

func (t *gatewayTools) appliedVersion(ctx context.Context, _ *mcp.CallToolRequest, in gatewayInput,
) (*mcp.CallToolResult, restBody, error) {
	applied, svcErr := t.versions.GetApplied(ctx, in.GatewayID)
	return answer("read what the gateway runs", applied, svcErr)
}

func (t *gatewayTools) appliedConfiguration(ctx context.Context, _ *mcp.CallToolRequest, in gatewayInput,
) (*mcp.CallToolResult, restBody, error) {
	configuration, svcErr := t.applied.GetAppliedConfiguration(ctx, in.GatewayID)
	return answer("read the configuration the gateway runs", configuration, svcErr)
}

func (t *gatewayTools) listVersions(ctx context.Context, _ *mcp.CallToolRequest, _ any,
) (*mcp.CallToolResult, restBody, error) {
	versions, svcErr := t.versions.ListVersions(ctx)
	return answer("list the versions", map[string]any{"versions": versions}, svcErr)
}

func (t *gatewayTools) getVersion(ctx context.Context, _ *mcp.CallToolRequest, in versionInput,
) (*mcp.CallToolResult, restBody, error) {
	version, svcErr := t.versions.GetVersion(ctx, in.Version)
	return answer("read the version", version, svcErr)
}

func (t *gatewayTools) capture(ctx context.Context, _ *mcp.CallToolRequest, in captureInput,
) (*mcp.CallToolResult, restBody, error) {
	version, svcErr := t.versions.Capture(ctx, CaptureRequest{Name: in.Name, Note: in.Note})
	return answer("capture a version", version, svcErr)
}

func (t *gatewayTools) diff(ctx context.Context, _ *mcp.CallToolRequest, in diffInput,
) (*mcp.CallToolResult, restBody, error) {
	diff, svcErr := t.versions.Diff(ctx, in.GatewayID, in.Version)
	return answer("diff the version", diff, svcErr)
}

func (t *gatewayTools) deploy(ctx context.Context, _ *mcp.CallToolRequest, in deployInput,
) (*mcp.CallToolResult, restBody, error) {
	result, svcErr := t.versions.Apply(ctx, in.GatewayID, ApplyRequest{
		Version: in.Version, DryRun: in.DryRun, Selection: in.Selection, Skip: in.Skip,
		ConfirmRemovals: in.ConfirmRemovals,
	})
	return answer("deploy the version", result, svcErr)
}

// promote applies the version the source runs to the target, as the console promotes one stage to
// the next: the changes the source holds back are skipped, because the source does not run them.
func (t *gatewayTools) promote(ctx context.Context, _ *mcp.CallToolRequest, in promoteInput,
) (*mcp.CallToolResult, restBody, error) {
	if in.SourceGatewayID == in.TargetGatewayID {
		return nil, nil, fmt.Errorf("failed to promote: the source and target are the same gateway")
	}
	source, svcErr := t.versions.GetApplied(ctx, in.SourceGatewayID)
	if svcErr != nil {
		return nil, nil, toolError("read what the source gateway runs", svcErr)
	}
	if source.Applied == "" {
		return nil, nil, fmt.Errorf("failed to promote: the source gateway runs no version yet")
	}
	// What the source holds back of the version it runs is what applying it there again leaves alone.
	held, svcErr := t.versions.Apply(ctx, in.SourceGatewayID, ApplyRequest{Version: source.Applied, DryRun: true})
	if svcErr != nil {
		return nil, nil, toolError("read what the source gateway holds back", svcErr)
	}
	skip := []string{}
	for _, change := range held.Diff.Changes {
		if change.Excluded && change.Change != ChangeUnchanged {
			skip = append(skip, change.Key)
		}
	}
	result, svcErr := t.versions.Apply(ctx, in.TargetGatewayID, ApplyRequest{
		Version: source.Applied, DryRun: in.DryRun, Skip: skip, ConfirmRemovals: in.ConfirmRemovals,
	})
	return answer("promote the version", result, svcErr)
}

func (t *gatewayTools) rollback(ctx context.Context, _ *mcp.CallToolRequest, in rollbackInput,
) (*mcp.CallToolResult, restBody, error) {
	result, svcErr := t.versions.Revert(ctx, in.GatewayID, RevertRequest{
		DryRun: in.DryRun, ConfirmRemovals: in.ConfirmRemovals,
	})
	return answer("roll back the gateway", result, svcErr)
}

// storeAnswer is a tool's result for a call to a gateway's store: the value, nothing for a removal,
// or the gateway's refusal with its own status and body, as REST passes it on.
func storeAnswer[T any](action string, store *StoreAnswer[T], svcErr *tidcommon.ServiceError) (
	*mcp.CallToolResult, restBody, error) {
	switch {
	case svcErr != nil:
		return nil, nil, toolError(action, svcErr)
	case store.Refusal != nil:
		return nil, nil, fmt.Errorf("failed to %s: the gateway refused with %d: %s", action, store.Refusal.Status,
			string(store.Refusal.Body))
	case store.Value == nil:
		return nil, restBody{}, nil
	default:
		return answer(action, store.Value, nil)
	}
}

func listQueryOf(in listValuesInput) StoreListQuery {
	query := StoreListQuery{Names: in.Names}
	if in.Limit > 0 {
		query.Limit = strconv.Itoa(in.Limit)
	}
	if in.Offset > 0 {
		query.Offset = strconv.Itoa(in.Offset)
	}
	return query
}

func (t *gatewayTools) listVariables(ctx context.Context, _ *mcp.CallToolRequest, in listValuesInput,
) (*mcp.CallToolResult, restBody, error) {
	listed, svcErr := t.stores.ListVariables(ctx, in.GatewayID, listQueryOf(in))
	return storeAnswer("list the gateway's variables", listed, svcErr)
}

func (t *gatewayTools) setVariable(ctx context.Context, _ *mcp.CallToolRequest, in setValueInput,
) (*mcp.CallToolResult, restBody, error) {
	set, svcErr := t.stores.SetVariable(ctx, in.GatewayID, in.Name,
		variablestore.VariableUpdateRequest{Value: in.Value, Description: in.Description})
	return storeAnswer("set the variable", set, svcErr)
}

func (t *gatewayTools) deleteVariable(ctx context.Context, _ *mcp.CallToolRequest, in valueNameInput,
) (*mcp.CallToolResult, restBody, error) {
	removed, svcErr := t.stores.DeleteVariable(ctx, in.GatewayID, in.Name)
	return storeAnswer("remove the variable", removed, svcErr)
}

func (t *gatewayTools) listSecrets(ctx context.Context, _ *mcp.CallToolRequest, in listValuesInput,
) (*mcp.CallToolResult, restBody, error) {
	listed, svcErr := t.stores.ListSecrets(ctx, in.GatewayID, listQueryOf(in))
	return storeAnswer("list the gateway's secrets", listed, svcErr)
}

func (t *gatewayTools) setSecret(ctx context.Context, _ *mcp.CallToolRequest, in setValueInput,
) (*mcp.CallToolResult, restBody, error) {
	set, svcErr := t.stores.SetSecret(ctx, in.GatewayID, in.Name,
		variablestore.SecretUpdateRequest{Value: in.Value, Description: in.Description})
	return storeAnswer("set the secret", set, svcErr)
}

func (t *gatewayTools) deleteSecret(ctx context.Context, _ *mcp.CallToolRequest, in valueNameInput,
) (*mcp.CallToolResult, restBody, error) {
	removed, svcErr := t.stores.DeleteSecret(ctx, in.GatewayID, in.Name)
	return storeAnswer("remove the secret", removed, svcErr)
}
