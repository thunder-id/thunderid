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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package importer

import (
	"github.com/thunder-id/thunderid/internal/agent"
	"github.com/thunder-id/thunderid/internal/application"
	layoutmgt "github.com/thunder-id/thunderid/internal/design/layout/mgt"
	thememgt "github.com/thunder-id/thunderid/internal/design/theme/mgt"
	"github.com/thunder-id/thunderid/internal/entitytype"
	flowmgt "github.com/thunder-id/thunderid/internal/flow/mgt"
	"github.com/thunder-id/thunderid/internal/group"
	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/notification"
	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/resource"
	"github.com/thunder-id/thunderid/internal/role"
	"github.com/thunder-id/thunderid/internal/vc/credential"
	"github.com/thunder-id/thunderid/internal/vc/presentation"
)

// The importer resolves runtime deletion by type-asserting the narrow deleter interfaces against the
// adapters it was constructed with. These assertions pin each interface to the concrete domain
// service, so a signature change there fails the build instead of silently degrading deletion to an
// "unsupported" outcome at runtime.
var (
	_ applicationDeleter             = (application.ApplicationServiceInterface)(nil)
	_ idpDeleter                     = (idp.IDPServiceInterface)(nil)
	_ senderDeleter                  = (notification.NotificationSenderMgtSvcInterface)(nil)
	_ flowDeleter                    = (flowmgt.FlowMgtServiceInterface)(nil)
	_ ouDeleter                      = (ou.OrganizationUnitServiceInterface)(nil)
	_ entityTypeDeleter              = (entitytype.EntityTypeServiceInterface)(nil)
	_ roleDeleter                    = (role.RoleServiceInterface)(nil)
	_ groupDeleter                   = (group.GroupServiceInterface)(nil)
	_ resourceServerDeleter          = (resource.ResourceServiceInterface)(nil)
	_ themeDeleter                   = (thememgt.ThemeMgtServiceInterface)(nil)
	_ layoutDeleter                  = (layoutmgt.LayoutMgtServiceInterface)(nil)
	_ agentDeleter                   = (agent.AgentServiceInterface)(nil)
	_ presentationDefinitionDeleter  = (presentation.PresentationDefinitionServiceInterface)(nil)
	_ credentialConfigurationDeleter = (credential.CredentialConfigurationServiceInterface)(nil)
)
