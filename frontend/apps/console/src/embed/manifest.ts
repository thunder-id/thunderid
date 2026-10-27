// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {RemoteManifest} from './contract';
import RouteConfig from '../configs/RouteConfig';

/** The console's navigation, for the host to render. Mirrors DashboardLayout's sidebar. */
const consoleManifest: RemoteManifest = {
  id: 'thunderid',
  nav: [
    {items: [{icon: 'Home', label: 'Home', path: RouteConfig.home.list()}]},
    {
      items: [
        {icon: 'LayoutGrid', label: 'Applications', path: RouteConfig.applications.list()},
        {icon: 'Server', label: 'Resource Servers', path: RouteConfig.resourceServers.list()},
      ],
      section: 'Resources',
    },
    {
      items: [
        {icon: 'UsersRound', label: 'Users', path: RouteConfig.users.list()},
        {icon: 'Bot', label: 'Agents', path: RouteConfig.agents.list()},
        {icon: 'Users', label: 'Groups', path: RouteConfig.groups.list()},
        {icon: 'ShieldCheck', label: 'Roles', path: RouteConfig.roles.list()},
        {icon: 'UserRoundCog', label: 'User Types', path: RouteConfig.userTypes.list()},
      ],
      section: 'Identities',
    },
    {
      items: [
        {icon: 'Building', label: 'Organization Units', path: RouteConfig.organizationUnits.list()},
        {icon: 'Workflow', label: 'Flows', path: RouteConfig.flows.list()},
        {icon: 'Layers', label: 'Connections', path: RouteConfig.connections.list()},
      ],
      section: 'Configure',
    },
    {
      items: [
        {icon: 'Palette', label: 'Design', path: RouteConfig.design.list()},
        {icon: 'Languages', label: 'Translations', path: RouteConfig.translations.list()},
      ],
      section: 'Customize',
    },
    {items: [{icon: 'Settings', label: 'Settings', path: RouteConfig.settings.list()}], section: 'System'},
  ],
  title: 'ThunderID',
};

export default consoleManifest;
