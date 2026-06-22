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

package thunderidengine

import (
	"fmt"
	"net/http"

	"github.com/thunder-id/thunderid/internal/attributecache"
	attributecacheconfig "github.com/thunder-id/thunderid/internal/attributecache/config"
	"github.com/thunder-id/thunderid/internal/authz"
	"github.com/thunder-id/thunderid/internal/consent"
	layoutmgt "github.com/thunder-id/thunderid/internal/design/layout/mgt"
	"github.com/thunder-id/thunderid/internal/design/resolve"
	thememgt "github.com/thunder-id/thunderid/internal/design/theme/mgt"
	"github.com/thunder-id/thunderid/internal/entitytype"
	flowcore "github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/flow/executor"
	flowmgt "github.com/thunder-id/thunderid/internal/flow/mgt"
	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/resource"
	"github.com/thunder-id/thunderid/internal/role"
	"github.com/thunder-id/thunderid/internal/system/cache"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
)

// sdkDeclarativeDesign carries theme and layout services built during SDK declarative
// initialization; they are needed to finish the flow provider and design resolve services.
type sdkDeclarativeDesign struct {
	themeService  thememgt.ThemeMgtServiceInterface
	layoutService layoutmgt.LayoutMgtServiceInterface
}

// buildSDKDeclarativeServices constructs only the system-of-record services the SDK endpoint
// groups require. Management REST routes are mounted on a throwaway mux. No database
// connection is opened.
func (c *engineConfig) buildSDKDeclarativeServices(
	cacheManager cache.CacheManagerInterface,
) (*sdkDeclarativeDesign, error) {
	mux := http.NewServeMux()

	ouAuthz, err := sysauthz.Initialize()
	if err != nil {
		return nil, fmt.Errorf("thunderidengine: system authorization service: %w", err)
	}
	ouService, ouHierarchyResolver, _, err := ou.Initialize(mux, nil, cacheManager, ouAuthz)
	if err != nil {
		return nil, fmt.Errorf("thunderidengine: organization unit service: %w", err)
	}
	ouAuthz.SetOUHierarchyResolver(ouHierarchyResolver)

	consentService := consent.Initialize()

	entityTypeService, _, err := entitytype.Initialize(
		mux, nil, cacheManager, ouService, ouAuthz, consentService)
	if err != nil {
		return nil, fmt.Errorf("thunderidengine: entity type service: %w", err)
	}

	resourceService, _, err := resource.Initialize(mux, ouService, consentService)
	if err != nil {
		return nil, fmt.Errorf("thunderidengine: resource service: %w", err)
	}

	if c.authZService == nil {
		if err := c.ensureRoleAndAuthZ(ouService, resourceService); err != nil {
			return nil, err
		}
	}

	idpService, _, err := idp.Initialize(cacheManager, mux, entityTypeService)
	if err != nil {
		return nil, fmt.Errorf("thunderidengine: idp service: %w", err)
	}

	attributeCacheService := attributecache.Initialize(attributecacheconfig.FromServerRuntime())

	themeMgtService, _, err := thememgt.Initialize(mux, nil)
	if err != nil {
		return nil, fmt.Errorf("thunderidengine: theme service: %w", err)
	}
	layoutMgtService, _, err := layoutmgt.Initialize(mux)
	if err != nil {
		return nil, fmt.Errorf("thunderidengine: layout service: %w", err)
	}

	if c.ouService == nil {
		c.ouService = ouService
	}
	if c.resourceService == nil {
		c.resourceService = resourceService
	}
	if c.idpService == nil {
		c.idpService = idpService
	}
	if c.attributeCacheSvc == nil {
		c.attributeCacheSvc = attributeCacheService
	}

	return &sdkDeclarativeDesign{
		themeService:  themeMgtService,
		layoutService: layoutMgtService,
	}, nil
}

func (c *engineConfig) ensureRoleAndAuthZ(
	ouService ou.OrganizationUnitServiceInterface,
	resourceService resource.ResourceServiceInterface,
) error {
	if c.roleService == nil {
		roleSvc, err := buildDeclarativeRoleService(ouService, resourceService)
		if err != nil {
			return fmt.Errorf("thunderidengine: declarative role service: %w", err)
		}
		c.roleService = roleSvc
	}
	c.authZService = authz.Initialize(c.roleService)
	return nil
}

// buildSDKDeclarativeFlowAndDesign finishes the SDK declarative graph once the executor registry
// exists: flow provider and design resolve service.
func (c *engineConfig) buildSDKDeclarativeFlowAndDesign(
	design *sdkDeclarativeDesign,
	cacheManager cache.CacheManagerInterface,
	flowFactory flowcore.FlowFactoryInterface,
	graphCache flowcore.GraphCacheInterface,
	execRegistry executor.ExecutorRegistryInterface,
) error {
	mux := http.NewServeMux()

	flowMgtService, _, err := flowmgt.Initialize(
		mux, nil, cacheManager, flowFactory, execRegistry, c.interceptorRegistry, graphCache)
	if err != nil {
		return fmt.Errorf("thunderidengine: flow management service: %w", err)
	}
	c.flowProvider = flowMgtService

	if c.designResolveService == nil {
		if c.hostActorProvider == nil {
			return fmt.Errorf(
				"thunderidengine: WithDesignResolveService or WithHostActorProvider " +
					"is required for declarative design resolution")
		}
		appService := newApplicationAdapter(c.hostActorProvider)
		c.designResolveService = resolve.Initialize(mux, design.themeService, design.layoutService, appService)
	}

	return nil
}

// buildDeclarativeRoleService constructs a file-backed role service from declarative YAML.
func buildDeclarativeRoleService(
	ouService ou.OrganizationUnitServiceInterface,
	resourceService resource.ResourceServiceInterface,
) (RoleService, error) {
	return role.InitializeDeclarativeReadOnly(ouService, resourceService)
}
