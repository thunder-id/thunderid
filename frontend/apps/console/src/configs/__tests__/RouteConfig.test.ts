/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {describe, it, expect} from 'vitest';
import RouteConfig, {ROUTE_SEGMENTS} from '../RouteConfig';

describe('RouteConfig', () => {
  it('organizationUnits builds paths from the segment', () => {
    expect(RouteConfig.organizationUnits.list()).toBe('/organization-units');
    expect(RouteConfig.organizationUnits.detail('ou-1')).toBe('/organization-units/ou-1');
    expect(RouteConfig.organizationUnits.create()).toBe('/organization-units/create');
  });

  it('users builds paths from the segment', () => {
    expect(RouteConfig.users.list()).toBe('/users');
    expect(RouteConfig.users.detail('user-1')).toBe('/users/user-1');
    expect(RouteConfig.users.add()).toBe('/users/add');
    expect(RouteConfig.users.addCreate()).toBe('/users/add/create');
    expect(RouteConfig.users.addInvite()).toBe('/users/add/invite');
  });

  it('userTypes builds paths from the segment', () => {
    expect(RouteConfig.userTypes.list()).toBe('/user-types');
    expect(RouteConfig.userTypes.detail('type-1')).toBe('/user-types/type-1');
    expect(RouteConfig.userTypes.create()).toBe('/user-types/create');
  });

  it('agentTypes builds paths from the segment', () => {
    expect(RouteConfig.agentTypes.detail('agent-type-1')).toBe('/agent-types/agent-type-1');
  });

  it('agents builds paths from the segment', () => {
    expect(RouteConfig.agents.list()).toBe('/agents');
    expect(RouteConfig.agents.detail('agent-1')).toBe('/agents/agent-1');
    expect(RouteConfig.agents.create()).toBe('/agents/create');
  });

  it('connections builds paths from the segment', () => {
    expect(RouteConfig.connections.list()).toBe('/connections');
    expect(RouteConfig.connections.byType('google')).toBe('/connections/google');
    expect(RouteConfig.connections.detail('google', 'conn-1')).toBe('/connections/google/conn-1');
    expect(RouteConfig.connections.configure('google')).toBe('/connections/google/configure');
    expect(RouteConfig.connections.create()).toBe('/connections/create');
  });

  it('trustedIssuers builds paths from the segment', () => {
    expect(RouteConfig.trustedIssuers.detail('issuer-1')).toBe('/trusted-issuers/issuer-1');
  });

  it('resourceServers builds paths from the segment', () => {
    expect(RouteConfig.resourceServers.list()).toBe('/resource-servers');
    expect(RouteConfig.resourceServers.detail('rs-1')).toBe('/resource-servers/rs-1');
    expect(RouteConfig.resourceServers.create()).toBe('/resource-servers/create');
  });

  it('translations builds paths from the segment', () => {
    expect(RouteConfig.translations.list()).toBe('/translations');
    expect(RouteConfig.translations.detail('en-us')).toBe('/translations/en-us');
    expect(RouteConfig.translations.create()).toBe('/translations/create');
  });

  it('home builds paths from the segment', () => {
    expect(RouteConfig.home.list()).toBe('/home');
  });

  it('applications builds paths from the segment', () => {
    expect(RouteConfig.applications.list()).toBe('/applications');
    expect(RouteConfig.applications.detail('app-1')).toBe('/applications/app-1');
    expect(RouteConfig.applications.types()).toBe('/applications/types');
    expect(RouteConfig.applications.create()).toBe('/applications/create');
  });

  it('groups builds paths from the segment', () => {
    expect(RouteConfig.groups.list()).toBe('/groups');
    expect(RouteConfig.groups.detail('group-1')).toBe('/groups/group-1');
    expect(RouteConfig.groups.create()).toBe('/groups/create');
  });

  it('roles builds paths from the segment', () => {
    expect(RouteConfig.roles.list()).toBe('/roles');
    expect(RouteConfig.roles.detail('role-1')).toBe('/roles/role-1');
    expect(RouteConfig.roles.create()).toBe('/roles/create');
  });

  it('verifiableCredentials builds paths from the segment', () => {
    expect(RouteConfig.verifiableCredentials.list()).toBe('/verifiable-credentials');
    expect(RouteConfig.verifiableCredentials.detail('vc-1')).toBe('/verifiable-credentials/vc-1');
    expect(RouteConfig.verifiableCredentials.create()).toBe('/verifiable-credentials/create');
  });

  it('verifiablePresentations builds paths from the segment', () => {
    expect(RouteConfig.verifiablePresentations.list()).toBe('/verifiable-presentations');
    expect(RouteConfig.verifiablePresentations.detail('vp-1')).toBe('/verifiable-presentations/vp-1');
    expect(RouteConfig.verifiablePresentations.create()).toBe('/verifiable-presentations/create');
  });

  it('flows builds paths from the segment', () => {
    expect(RouteConfig.flows.list()).toBe('/flows');
    expect(RouteConfig.flows.create()).toBe('/flows/create');
    expect(RouteConfig.flows.byType('signin')).toBe('/flows/signin');
    expect(RouteConfig.flows.detail('signin', 'flow-1')).toBe('/flows/signin/flow-1');
  });

  it('design builds paths from the segment', () => {
    expect(RouteConfig.design.list()).toBe('/design');
    expect(RouteConfig.design.themesCreate()).toBe('/design/themes/create');
    expect(RouteConfig.design.themeDetail('theme-1')).toBe('/design/themes/theme-1');
    expect(RouteConfig.design.layoutDetail('layout-1')).toBe('/design/layouts/layout-1');
  });

  it('importExport builds paths from the segment', () => {
    expect(RouteConfig.importExport.list()).toBe('/import-export');
  });

  it('export builds paths from the segment', () => {
    expect(RouteConfig.export.page()).toBe('/export');
  });

  it('importConfiguration builds paths from the segment', () => {
    expect(RouteConfig.importConfiguration.upload()).toBe('/import-configuration');
    expect(RouteConfig.importConfiguration.validate()).toBe('/import-configuration/validate');
    expect(RouteConfig.importConfiguration.summary()).toBe('/import-configuration/summary');
  });

  it('welcome builds paths from the segment', () => {
    expect(RouteConfig.welcome.root()).toBe('/welcome');
    expect(RouteConfig.welcome.createProject()).toBe('/welcome/create-project');
    expect(RouteConfig.welcome.getStarted()).toBe('/welcome/get-started');
    expect(RouteConfig.welcome.getStartedApplicationsTypes()).toBe('/welcome/get-started/applications/types');
    expect(RouteConfig.welcome.getStartedApplicationsCreate()).toBe('/welcome/get-started/applications/create');
    expect(RouteConfig.welcome.tryoutSecuringApplication()).toBe('/welcome/tryout/securing-application');
    expect(RouteConfig.welcome.tryoutAiAgents()).toBe('/welcome/tryout/ai-agents');
    expect(RouteConfig.welcome.tryoutMcp()).toBe('/welcome/tryout/mcp');
    expect(RouteConfig.welcome.importConfigurationUpload()).toBe('/welcome/import-configuration');
    expect(RouteConfig.welcome.importConfigurationValidate()).toBe('/welcome/import-configuration/validate');
    expect(RouteConfig.welcome.importConfigurationSummary()).toBe('/welcome/import-configuration/summary');
  });

  it('settings builds paths from the segment', () => {
    expect(RouteConfig.settings.list()).toBe('/settings');
  });

  it('every builder derives its path from ROUTE_SEGMENTS rather than a separate literal', () => {
    expect(RouteConfig.organizationUnits.list()).toBe(`/${ROUTE_SEGMENTS.organizationUnits}`);
    expect(RouteConfig.users.list()).toBe(`/${ROUTE_SEGMENTS.users}`);
    expect(RouteConfig.welcome.root()).toBe(`/${ROUTE_SEGMENTS.welcome}`);
  });
});
