/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

// Package flowmeta provides functionality for retrieving aggregated flow metadata.
package flowmeta

import (
	"context"
	"encoding/json"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/actorprovider"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/ou"
	i18nmgt "github.com/thunder-id/thunderid/internal/system/i18n/mgt"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// MetaType represents the type of metadata being requested.
type MetaType string

const (
	loggerComponentName = "FlowMetaService"
	// MetaTypeAPP represents the APP type for flow metadata.
	MetaTypeAPP MetaType = "APP"
	// MetaTypeOU represents the OU type for flow metadata.
	MetaTypeOU MetaType = "OU"
)

// IsValid checks if the MetaType is valid.
func (mt MetaType) IsValid() bool {
	return mt == MetaTypeAPP || mt == MetaTypeOU
}

// FlowMetaServiceInterface defines the interface for flow metadata operations.
type FlowMetaServiceInterface interface {
	GetFlowMetadata(
		ctx context.Context,
		metaType MetaType,
		id string,
		language *string,
		namespace *string,
	) (*FlowMetadataResponse, *tidcommon.ServiceError)
}

// flowMetaService is the implementation of FlowMetaServiceInterface.
type flowMetaService struct {
	actorProvider providers.ActorProvider
	ouService     providers.OrganizationUnitProvider
	designResolve providers.DesignProvider
	i18nService   providers.I18nProvider
	logger        *log.Logger
}

// newFlowMetaService creates a new instance of flowMetaService with injected dependencies.
func newFlowMetaService(
	actorProvider providers.ActorProvider,
	ouService providers.OrganizationUnitProvider,
	designResolve providers.DesignProvider,
	i18nService providers.I18nProvider,
) FlowMetaServiceInterface {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))
	return &flowMetaService{
		actorProvider: actorProvider,
		ouService:     ouService,
		designResolve: designResolve,
		i18nService:   i18nService,
		logger:        logger,
	}
}

// GetFlowMetadata retrieves aggregated metadata for a flow based on type and ID.
func (fms *flowMetaService) GetFlowMetadata(
	ctx context.Context,
	metaType MetaType,
	id string,
	language *string,
	namespace *string,
) (*FlowMetadataResponse, *tidcommon.ServiceError) {
	response := newFlowMetadataResponse()
	lang, ns := resolveLanguageAndNamespace(language, namespace)

	if metaType == "" {
		fms.populateI18nMetadata(ctx, response, lang, ns)
		return response, nil
	}

	if !metaType.IsValid() {
		return nil, &ErrorInvalidType
	}

	ouID, svcErr := fms.populateTypeMetadata(ctx, metaType, id, response)
	if svcErr != nil {
		return nil, svcErr
	}

	if svcErr := fms.populateOUMetadata(ctx, ouID, response); svcErr != nil {
		return nil, svcErr
	}

	fms.populateDesignMetadata(ctx, metaType, id, ouID, response)
	fms.populateI18nMetadata(ctx, response, lang, ns)

	fms.logger.Debug(ctx, "Successfully retrieved flow metadata",
		log.String("type", string(metaType)),
		log.String("id", id))

	return response, nil
}

func newFlowMetadataResponse() *FlowMetadataResponse {
	emptyJSON, _ := json.Marshal(map[string]interface{}{})

	return &FlowMetadataResponse{
		Design: DesignMetadata{
			Theme:  json.RawMessage(emptyJSON),
			Layout: json.RawMessage(emptyJSON),
		},
		I18n: I18nMetadata{
			Translations: make(map[string]map[string]string),
		},
	}
}

func resolveLanguageAndNamespace(language *string, namespace *string) (string, string) {
	lang := i18nmgt.SystemLanguage
	if language != nil && *language != "" {
		lang = *language
	}

	ns := ""
	if namespace != nil {
		ns = *namespace
	}

	return lang, ns
}

func (fms *flowMetaService) populateTypeMetadata(
	ctx context.Context,
	metaType MetaType,
	id string,
	response *FlowMetadataResponse,
) (string, *tidcommon.ServiceError) {
	if metaType == MetaTypeOU {
		response.IsRegistrationFlowEnabled = false
		return id, nil
	}

	client, svcErr := fms.actorProvider.GetInboundClientByID(ctx, id)
	if svcErr != nil {
		if svcErr.Code == actorprovider.ErrorActorNotFound.Code {
			return "", &ErrorApplicationNotFound
		}
		fms.logger.Error(ctx, "Failed to get inbound client", log.String("appID", id),
			log.String("error", svcErr.Error.DefaultValue))
		return "", &ErrorApplicationFetchFailed
	}
	if client == nil {
		return "", &ErrorApplicationNotFound
	}

	entity, epErr := fms.actorProvider.GetActor(id)
	if epErr != nil && epErr.Code != string(entityprovider.ErrorCodeEntityNotFound) {
		fms.logger.Error(ctx, "Failed to get actor", log.String("appID", id),
			log.String("error", epErr.Error.DefaultValue))
		return "", &ErrorApplicationFetchFailed
	}

	response.IsRegistrationFlowEnabled = client.IsRegistrationFlowEnabled
	response.IsRecoveryFlowEnabled = client.IsRecoveryFlowEnabled
	response.Application = actorprovider.BuildApplicationMetadata(client.ID, entity, client.Properties)

	ouList, ouErr := fms.ouService.GetOrganizationUnitList(ctx, 1, 0, nil)
	if ouErr != nil {
		if ouErr.Code == ou.ErrorOrganizationUnitNotFound.Code {
			return "", &ErrorOUNotFound
		}

		fms.logger.Error(ctx, "Failed to get root organization unit",
			log.String("error", ouErr.Error.DefaultValue),
			log.String("code", ouErr.Code))
		return "", &ErrorOUFetchFailed
	}

	if ouList != nil && ouList.TotalResults == 1 && len(ouList.OrganizationUnits) > 0 {
		return ouList.OrganizationUnits[0].ID, nil
	}

	return "", nil
}

func (fms *flowMetaService) populateOUMetadata(
	ctx context.Context,
	ouID string,
	response *FlowMetadataResponse,
) *tidcommon.ServiceError {
	if ouID == "" {
		return nil
	}

	orgUnit, svcErr := fms.ouService.GetOrganizationUnit(ctx, ouID)
	if svcErr != nil {
		if svcErr.Code == ou.ErrorOrganizationUnitNotFound.Code {
			return &ErrorOUNotFound
		}

		fms.logger.Error(ctx, "Failed to get organization unit",
			log.String("ouID", ouID),
			log.String("error", svcErr.Error.DefaultValue),
			log.String("code", svcErr.Code))
		return &ErrorOUFetchFailed
	}

	response.OU = &OUMetadata{
		ID:              orgUnit.ID,
		Handle:          orgUnit.Handle,
		Name:            orgUnit.Name,
		Description:     orgUnit.Description,
		LogoURL:         orgUnit.LogoURL,
		TosURI:          orgUnit.TosURI,
		PolicyURI:       orgUnit.PolicyURI,
		CookiePolicyURI: orgUnit.CookiePolicyURI,
	}

	return nil
}

func (fms *flowMetaService) populateDesignMetadata(
	ctx context.Context,
	metaType MetaType,
	id string,
	ouID string,
	response *FlowMetadataResponse,
) {
	designType := providers.DesignResolveTypeAPP
	designID := id
	if metaType == MetaTypeOU {
		designType = providers.DesignResolveTypeOU
		designID = ouID
	}

	designResp, svcErr := fms.designResolve.ResolveDesign(ctx, designType, designID)
	if svcErr != nil {
		fms.logger.Debug(ctx, "Failed to get design configuration",
			log.String("type", string(designType)),
			log.String("id", designID),
			log.String("error", svcErr.Error.DefaultValue))
		return
	}

	if designResp == nil {
		return
	}

	if designResp.Theme != nil {
		response.Design.Theme = designResp.Theme
	}
	if designResp.Layout != nil {
		response.Design.Layout = designResp.Layout
	}
}

func (fms *flowMetaService) populateI18nMetadata(
	ctx context.Context, response *FlowMetadataResponse, lang string, ns string) {
	i18nResp, i18nErr := fms.i18nService.ResolveTranslations(ctx, lang, ns)
	if i18nErr != nil {
		fms.logger.Debug(ctx, "Failed to get i18n translations",
			log.String("language", lang),
			log.String("namespace", ns),
			log.String("error", i18nErr.Error.DefaultValue))
	} else if i18nResp != nil {
		response.I18n.Language = i18nResp.Language
		response.I18n.TotalResults = i18nResp.TotalResults
		response.I18n.Translations = i18nResp.Translations
	}

	languages, i18nErr := fms.i18nService.ListLanguages(ctx)
	if i18nErr != nil {
		fms.logger.Debug(ctx, "Failed to list languages",
			log.String("error", i18nErr.Error.DefaultValue))
		response.I18n.Languages = []string{i18nmgt.SystemLanguage}
		return
	}

	response.I18n.Languages = languages
}
