// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package importer

import (
	"context"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/thunder-id/thunderid/internal/gateway"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// fakeGatewayService records what an import asked of it.
type fakeGatewayService struct {
	registered []gateway.RegisterRequest
	updatedID  string
	updated    *gateway.UpdateRequest
	existing   []gateway.Gateway
	listErr    *common.ServiceError
}

func (f *fakeGatewayService) List(context.Context) ([]gateway.Gateway, *common.ServiceError) {
	return f.existing, f.listErr
}

func (f *fakeGatewayService) Register(
	_ context.Context, req gateway.RegisterRequest) (*gateway.Registration, *common.ServiceError) {
	f.registered = append(f.registered, req)
	return &gateway.Registration{
		Gateway: gateway.Gateway{ID: "gw-new", Name: req.Name, BaseURL: req.BaseURL},
		Key:     "issued",
	}, nil
}

func (f *fakeGatewayService) Update(
	_ context.Context, id string, req gateway.UpdateRequest) (*gateway.Gateway, *common.ServiceError) {
	f.updatedID, f.updated = id, &req
	name := ""
	if req.Name != nil {
		name = *req.Name
	}
	return &gateway.Gateway{ID: id, Name: name}, nil
}

func gatewayDoc(t *testing.T, body string) parsedDocument {
	t.Helper()
	var node yaml.Node
	if err := yaml.Unmarshal([]byte(body), &node); err != nil {
		t.Fatalf("parsing the document: %v", err)
	}
	return parsedDocument{ResourceType: resourceTypeGateway, Node: node.Content[0]}
}

func newGatewayImportService(svc gatewayAdapter) ImportServiceInterface {
	return newImportService(nil, nil, nil, nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, svc, nil)
}

const gatewayYAML = `
name: production
baseUrl: https://dp.example.com:8090
key: the-key
`

// A bootstrap file registers a gateway. This is the one-time path, so it writes through the service
// and what it writes outlives the file.
func TestImportGatewayRegistersANewOne(t *testing.T) {
	svc := &fakeGatewayService{}
	importer, ok := newGatewayImportService(svc).(*importService)
	if !ok {
		t.Fatal("unexpected service type")
	}

	outcome := importer.importGateway(context.Background(), gatewayDoc(t, gatewayYAML), nil, false)

	if outcome.Status != statusSuccess {
		t.Fatalf("expected success, got %+v", outcome)
	}
	if outcome.Operation != operationCreate {
		t.Errorf("expected a create, got %q", outcome.Operation)
	}
	if len(svc.registered) != 1 || svc.registered[0].Name != "production" {
		t.Fatalf("the document did not reach the service: %+v", svc.registered)
	}
	if svc.registered[0].Key != "the-key" {
		t.Errorf("the declared key was not passed through: %q", svc.registered[0].Key)
	}
}

// Re-running bootstrap over a gateway that is already registered updates it rather than failing,
// which is what upsert is for.
func TestImportGatewayUpdatesAnExistingOneWhenUpserting(t *testing.T) {
	svc := &fakeGatewayService{existing: []gateway.Gateway{{ID: "gw-1", Name: "production"}}}
	importer, _ := newGatewayImportService(svc).(*importService)
	upsert := true

	outcome := importer.importGateway(
		context.Background(), gatewayDoc(t, gatewayYAML), &ImportOptions{Upsert: &upsert}, false)

	if outcome.Status != statusSuccess || outcome.Operation != operationUpdate {
		t.Fatalf("expected an update, got %+v", outcome)
	}
	if svc.updatedID != "gw-1" {
		t.Errorf("updated the wrong gateway: %q", svc.updatedID)
	}
	if len(svc.registered) != 0 {
		t.Error("an existing gateway was registered again")
	}
}

// A file naming no key must not rotate the credential of a gateway already registered. Re-running
// bootstrap would otherwise leave the gateway unreachable until it was reconfigured.
func TestImportGatewayLeavesTheKeyAloneWhenTheFileNamesNone(t *testing.T) {
	svc := &fakeGatewayService{existing: []gateway.Gateway{{ID: "gw-1", Name: "production"}}}
	importer, _ := newGatewayImportService(svc).(*importService)
	upsert := true

	outcome := importer.importGateway(context.Background(), gatewayDoc(t, `
name: production
baseUrl: https://dp.example.com:8090
`), &ImportOptions{Upsert: &upsert}, false)

	if outcome.Status != statusSuccess {
		t.Fatalf("expected success, got %+v", outcome)
	}
	if svc.updated == nil {
		t.Fatal("no update was made")
	}
	if svc.updated.Key != nil {
		t.Errorf("a file naming no key still changed the credential: %v", *svc.updated.Key)
	}
}

// With upsert turned off, a name already registered is a conflict rather than a silent replacement.
// Upsert defaults to on, so this has to be asked for explicitly.
func TestImportGatewayRefusesADuplicateNameWithoutUpsert(t *testing.T) {
	svc := &fakeGatewayService{existing: []gateway.Gateway{{ID: "gw-1", Name: "production"}}}
	importer, _ := newGatewayImportService(svc).(*importService)
	upsert := false

	outcome := importer.importGateway(
		context.Background(), gatewayDoc(t, gatewayYAML), &ImportOptions{Upsert: &upsert}, false)

	if outcome.Status != statusFailed {
		t.Fatalf("expected a conflict, got %+v", outcome)
	}
	if len(svc.registered) != 0 {
		t.Error("a duplicate name was registered anyway")
	}
}

// A document missing what it takes to reach a gateway is refused before the service is called.
func TestImportGatewayRefusesAnIncompleteDocument(t *testing.T) {
	for name, body := range map[string]string{
		"no name":     "baseUrl: https://dp.example.com:8090\n",
		"no base url": "name: production\n",
	} {
		t.Run(name, func(t *testing.T) {
			svc := &fakeGatewayService{}
			importer, _ := newGatewayImportService(svc).(*importService)

			outcome := importer.importGateway(context.Background(), gatewayDoc(t, body), nil, false)

			if outcome.Status != statusFailed {
				t.Errorf("expected the document to be refused, got %+v", outcome)
			}
			if len(svc.registered) != 0 {
				t.Error("an incomplete document reached the service")
			}
		})
	}
}

// A dry run reports what would happen without calling the service.
func TestImportGatewayDryRunChangesNothing(t *testing.T) {
	svc := &fakeGatewayService{}
	importer, _ := newGatewayImportService(svc).(*importService)

	outcome := importer.importGateway(context.Background(), gatewayDoc(t, gatewayYAML), nil, true)

	if outcome.Status != statusSuccess || outcome.Operation != operationCreate {
		t.Fatalf("expected a create to be reported, got %+v", outcome)
	}
	if len(svc.registered) != 0 {
		t.Error("a dry run registered a gateway")
	}
}

// A deployment wired without a gateway service reports that rather than panicking.
func TestImportGatewayWithoutAServiceIsReported(t *testing.T) {
	importer, _ := newGatewayImportService(nil).(*importService)

	outcome := importer.importGateway(context.Background(), gatewayDoc(t, gatewayYAML), nil, false)

	if outcome.Status != statusFailed {
		t.Fatalf("expected the missing adapter to be reported, got %+v", outcome)
	}
}
