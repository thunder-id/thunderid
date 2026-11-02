// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package flowmgt

import (
	"context"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/flow/executormeta"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/flow/interceptormock"
)

const (
	testSystemFlowHandle = "system-application-delete"
	testSystemFlowJSON   = `{"id": "019a1f40-6a1c-7000-8000-5e5713d0aaaa", "handle": "system-test", ` +
		`"name": "Test", "flowType": "SYSTEM", "nodes": []}`
)

type SystemFlowTestSuite struct {
	suite.Suite
	validator *FlowValidatorInterfaceMock
}

func TestSystemFlowTestSuite(t *testing.T) {
	suite.Run(t, new(SystemFlowTestSuite))
}

func (s *SystemFlowTestSuite) SetupTest() {
	s.validator = NewFlowValidatorInterfaceMock(s.T())
}

// systemFlowBuild is a set of system flow files, each file's content keyed by its name.
func systemFlowBuild(files map[string]string) fstest.MapFS {
	build := fstest.MapFS{}
	for name, content := range files {
		build["system/"+name] = &fstest.MapFile{Data: []byte(content)}
	}
	return build
}

// The built-in system flows pass the validator a server runs them under, with the executors the
// static catalog describes.
func (s *SystemFlowTestSuite) TestBuiltInSystemFlowsAreValid() {
	registry, err := executormeta.NewRegistry(nil)
	s.Require().NoError(err)
	graphs := newGraphBuilderInterfaceMock(s.T())
	graphs.EXPECT().ValidateGraph(mock.Anything, mock.Anything).Return(nil)
	validator := newFlowValidator(
		registry, interceptormock.NewInterceptorRegistryInterfaceMock(s.T()), graphs, nil, nil)

	flows, err := loadSystemFlows(context.Background(), systemFlowFiles, validator)

	s.Require().NoError(err)
	s.Contains(flows, "system-application-delete")
	s.Contains(flows, "system-application-secret-rotate")
}

// Each system flow is validated as one, and is kept by its handle.
func (s *SystemFlowTestSuite) TestLoadValidatesEachFlow() {
	s.validator.EXPECT().ValidateSystemFlowDefinition(mock.Anything, mock.MatchedBy(func(def *FlowDefinition) bool {
		return def.Handle == "system-test" && def.FlowType == providers.FlowTypeSystem
	})).Return(nil).Once()

	flows, err := loadSystemFlows(context.Background(),
		systemFlowBuild(map[string]string{"test.json": testSystemFlowJSON}), s.validator)

	s.Require().NoError(err)
	s.Contains(flows, "system-test")
}

// A fault in a system flow is a fault in the build, so loading fails and startup with it.
func (s *SystemFlowTestSuite) TestLoadFailsOnAFaultyFlow() {
	invalid := tidcommon.ServiceError{Code: "FMS-1000", ErrorDescription: tidcommon.I18nMessage{DefaultValue: "bad"}}
	for name, tc := range map[string]struct {
		files map[string]string
		valid bool
	}{
		"not JSON":        {map[string]string{"a.json": "resource_type: flow"}, true},
		"no id":           {map[string]string{"a.json": `{"handle": "system-test", "flowType": "SYSTEM"}`}, true},
		"invalid":         {map[string]string{"a.json": testSystemFlowJSON}, false},
		"a reused handle": {map[string]string{"a.json": testSystemFlowJSON, "b.json": testSystemFlowJSON}, true},
		"a reused id": {map[string]string{"a.json": testSystemFlowJSON, "b.json": `{"id": ` +
			`"019a1f40-6a1c-7000-8000-5e5713d0aaaa", "handle": "system-other", "flowType": "SYSTEM"}`}, true},
	} {
		s.Run(name, func() {
			validator := NewFlowValidatorInterfaceMock(s.T())
			if tc.valid {
				validator.EXPECT().ValidateSystemFlowDefinition(mock.Anything, mock.Anything).Return(nil).Maybe()
			} else {
				validator.EXPECT().ValidateSystemFlowDefinition(mock.Anything, mock.Anything).Return(&invalid)
			}

			_, err := loadSystemFlows(context.Background(), systemFlowBuild(tc.files), validator)

			s.Error(err)
		})
	}
}

// A system flow is given by its handle, as a fresh copy each time; any other handle is not found.
func (s *SystemFlowTestSuite) TestGetSystemFlow() {
	s.validator.EXPECT().ValidateSystemFlowDefinition(mock.Anything, mock.Anything).Return(nil)
	flows, err := loadSystemFlows(context.Background(), systemFlowFiles, s.validator)
	s.Require().NoError(err)
	service := &flowMgtService{systemFlows: flows}

	flow, svcErr := service.GetSystemFlow(context.Background(), testSystemFlowHandle)
	s.Require().Nil(svcErr)
	s.Equal(providers.FlowTypeSystem, flow.FlowType)
	flow.Nodes = nil

	again, svcErr := service.GetSystemFlow(context.Background(), testSystemFlowHandle)
	s.Require().Nil(svcErr)
	s.NotEmpty(again.Nodes, "a caller's change reached the next caller")

	_, svcErr = service.GetSystemFlow(context.Background(), "basic-login")
	s.Equal(&ErrorFlowNotFound, svcErr)
}
