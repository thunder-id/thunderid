// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {ROUTE_SEGMENTS as RoutePaths} from '../../configs/RouteConfig';

type Fields = Record<string, unknown>;

/** A label as an i18n key and its default. An empty key shows the default as it is. */
export type Label = [key: string, defaultValue: string];

/**
 * A field a tab shows: its path in the resource (see `valueAt`), with the label the configuration's
 * own page gives it when that differs from the one its name reads as.
 */
export type FieldSpec = string | [path: string, label: string];

/** A titled group of fields within a tab, as the configuration's page groups them in cards. */
export interface SectionSpec {
  title?: Label;
  fields: FieldSpec[];
  /** Set on the cards whose values the configuration's page offers to copy, such as identifiers. */
  copy?: boolean;
}

/** One tab, named and ordered as on the configuration's own page for the resource. */
export interface TabSpec {
  key: string;
  title: Label;
  sections?: SectionSpec[];
  /** The parts the tab lists, such as a group's members, by the path its own API serves each at. */
  parts?: string[];
  /** Whether the tab ends with the environment's own endpoints, as an Overview does. */
  endpoints?: boolean;
}

/** How the environment view shows one console section's resources. */
export interface ResourceLayout {
  /** The console section's path segment, the same one the configuration's pages use. */
  segment: string;
  /** The resource types the section shows, as an export names them. */
  types: string[];
  /** The section's title, as an i18n key and its default. */
  title: Label;
  /**
   * The tabs a resource shows, as its page in the configuration has them. Tabs the configuration's
   * page has for actions alone, such as a danger zone, are left out. Fields no tab names go to an
   * "Other" section at the end, so nothing the environment runs is hidden.
   */
  tabs?: TabSpec[] | ((resource: Record<string, unknown>) => TabSpec[]);
  /** What a resource is called; the common name fields are tried when absent. */
  nameOf?: (resource: Record<string, unknown>) => string | undefined;
  /** Whether the section's resources carry a logo, shown beside their name. */
  logo?: boolean;
}

const tab = (key: string, title: string, sections: SectionSpec[], parts?: string[]): TabSpec => ({
  key,
  title: [`common:environment.tabs.${key}`, title],
  sections,
  parts,
});

/** The cards whose values the configuration's pages offer to copy. */
const COPIED_CARDS: readonly string[] = [
  'Quick Copy',
  'Quick copy',
  'Identifier',
  'Application details',
  'Agent details',
];

const section = (title: string | undefined, fields: FieldSpec[]): SectionSpec => ({
  title: title ? [`common:environment.cards.${title.replace(/[^A-Za-z0-9]+/g, '')}`, title] : undefined,
  fields,
  copy: title ? COPIED_CARDS.includes(title) : false,
});

const OU: SectionSpec = section('Organization Unit', [
  ['ouHandle', 'Handle'],
  ['ouId', 'ID'],
]);

const SIGN_IN_FLOWS: FieldSpec[] = [
  ['authFlowId', 'Sign-in Flow'],
  ['registrationFlowId', 'Sign-up Flow'],
  ['isRegistrationFlowEnabled', 'Sign-up enabled'],
  ['recoveryFlowId', 'Recovery Flow'],
  ['isRecoveryFlowEnabled', 'Recovery enabled'],
  ['signOutFlowId', 'Sign Out Flow'],
  ['isSignOutFlowEnabled', 'Sign out enabled'],
];

const APPEARANCE: SectionSpec = section('Appearance', [
  ['themeId', 'Theme'],
  ['layoutId', 'Layout'],
]);

const OAUTH_CONFIGURATION: SectionSpec = section('OAuth 2 Configuration', [
  'oauth.grantTypes',
  'oauth.responseTypes',
  ['oauth.redirectUris', 'Authorized redirect URIs'],
  ['oauth.postLogoutRedirectUris', 'Post-Logout Redirect URIs'],
  ['oauth.backchannelLogoutUri', 'Back-Channel Logout URI'],
  ['oauth.tokenEndpointAuthMethod', 'Client Authentication Method'],
  ['oauth.acrValues', 'ACR Values'],
  'oauth.publicClient',
  ['oauth.pkceRequired', 'PKCE Required'],
  ['oauth.requirePushedAuthorizationRequests', 'Require Pushed Authorization Requests'],
]);

const TOKENS: SectionSpec[] = [
  section('Access Token', ['oauth.token.accessToken']),
  section('ID Token', ['oauth.token.idToken']),
  section('Refresh Token', ['oauth.token.refreshToken']),
  section('User Info', ['oauth.userInfo']),
  section('Scopes & User Attribute Mappings', ['oauth.scopeClaims']),
  section('Token Attributes', ['assertion']),
];

const SCHEMA: SectionSpec = section(undefined, ['schema']);

const text = (value: unknown): string | undefined => (typeof value === 'string' && value.trim() ? value : undefined);

/** Every view leaves these out: the view's own header shows them, or they say nothing to a reader. */
export const ALWAYS_HIDDEN: readonly string[] = [
  'id',
  'createdAt',
  'updatedAt',
  'createdTime',
  'updatedTime',
  'isReadOnly',
];

/** The fields a resource is commonly named by, tried in this order. */
const NAME_FIELDS: readonly string[] = ['name', 'displayName', 'handle', 'language', 'username', 'title'];

/** The fields a resource is commonly described by, tried in this order. */
const DESCRIPTION_FIELDS: readonly string[] = ['description'];

export const RESOURCE_LAYOUTS: readonly ResourceLayout[] = [
  {
    segment: RoutePaths.applications,
    types: ['application'],
    title: ['common:environment.sections.applications', 'Applications'],
    logo: true,
    tabs: [
      {
        ...tab('overview', 'Overview', [
          section('Application details', [
            ['id', 'Application ID'],
            ['oauth.clientId', 'Client ID'],
            'ouId',
            'template',
          ]),
        ]),
        endpoints: true,
      },
      tab('access', 'Access', [
        section('Allowed User Types', ['allowedUserTypes']),
        section('Agent Sign-In', ['allowedAgentTypes']),
        section('Application Access', [['url', 'Access URL']]),
      ]),
      tab('credentials', 'Credentials', [
        section('Identifier', [['oauth.clientId', 'Client ID']]),
        section('Secret', [['oauth.clientSecret', 'Client Secret']]),
        section('Certificate', ['oauth.certificate']),
        section('Platform Attestation', ['attestation']),
      ]),
      tab('flows', 'Flows', [section(undefined, SIGN_IN_FLOWS)]),
      tab('customization', 'Customization', [
        APPEARANCE,
        section('URLs', [
          ['tosUri', 'Terms of Service URL'],
          ['policyUri', 'Privacy Policy URL'],
        ]),
        section('Contacts', ['contacts']),
      ]),
      tab('token', 'Token', TOKENS),
      tab('advanced', 'Advanced', [
        OAUTH_CONFIGURATION,
        section('Identity Assertions (ID-JAG)', ['oauth.token.idJag']),
        section('Default Audience', ['oauth.token.accessToken.defaultAudience']),
        section('Passkeys', ['passkeyAllowedOrigins']),
      ]),
    ],
  },
  {
    segment: RoutePaths.agents,
    types: ['agent'],
    title: ['common:environment.sections.agents', 'Agents'],
    logo: true,
    tabs: [
      {
        ...tab('overview', 'Overview', [
          section('Agent details', [
            ['id', 'Agent ID'],
            ['oauth.clientId', 'Client ID'],
            ['owner', 'Owner ID'],
            'ouId',
            'ouHandle',
            'type',
          ]),
        ]),
        endpoints: true,
      },
      tab('attributes', 'Attributes', [section(undefined, ['attributes'])]),
      tab('credentials', 'Credentials', [
        section('Identifier', [['oauth.clientId', 'Client ID']]),
        section('Secret', [['oauth.clientSecret', 'Client Secret']]),
        section('Certificate', ['oauth.certificate']),
      ]),
      tab('flows', 'Flows', [section(undefined, SIGN_IN_FLOWS)]),
      tab('tokens', 'Tokens', TOKENS),
      tab('advanced', 'Advanced', [
        section('Owner', ['owner']),
        section('Allowed User Types', ['allowedUserTypes']),
        section('Agent Sign-In', ['allowedAgentTypes']),
        OAUTH_CONFIGURATION,
        section('Default Audience', ['oauth.token.accessToken.defaultAudience']),
      ]),
    ],
  },
  {
    segment: RoutePaths.resourceServers,
    types: ['resource_server'],
    title: ['common:environment.sections.resourceServers', 'Resource Servers'],
    tabs: (server: Record<string, unknown>) => [
      server.type === 'MCP'
        ? tab('capabilities', 'Capabilities', [], ['actions', 'resources'])
        : tab('resources', 'Resources', [], ['resources', 'actions']),
      tab('advanced', 'Advanced', [
        section('Configurations', [
          ['identifier', 'Identifier (Audience)'],
          ['authorizationEngine', 'Authorization engine'],
          ['delimiter', 'Delimiter'],
          'handle',
          'type',
        ]),
      ]),
    ],
  },
  {
    segment: RoutePaths.users,
    types: ['user'],
    title: ['common:environment.sections.users', 'Users'],
    nameOf: (user: Record<string, unknown>) => {
      const attributes = (user.attributes ?? {}) as Record<string, unknown>;
      return text(attributes.username) ?? text(attributes.email) ?? text(user.display);
    },
    tabs: [
      tab('general', 'General', [section('Quick Copy', [['id', 'User ID'], 'type']), OU]),
      tab('attributes', 'Attributes', [section(undefined, ['attributes'])]),
    ],
  },
  {
    segment: RoutePaths.groups,
    types: ['group'],
    title: ['common:environment.sections.groups', 'Groups'],
    tabs: [
      tab('general', 'General', [section('Quick Copy', [['id', 'Group ID']]), OU]),
      tab('members', 'Members', [], ['members']),
    ],
  },
  {
    segment: RoutePaths.roles,
    types: ['role'],
    title: ['common:environment.sections.roles', 'Roles'],
    tabs: [
      tab('general', 'General', [section('Quick Copy', [['id', 'Role ID']]), OU]),
      tab('permissions', 'Permissions', [section(undefined, ['permissions'])]),
      tab('assignments', 'Assignments', [], ['assignments']),
    ],
  },
  {
    segment: RoutePaths.organizationUnits,
    types: ['organization_unit'],
    title: ['common:environment.sections.organizationUnits', 'Organization Units'],
    logo: true,
    tabs: [
      tab('general', 'General', [
        section('Quick Copy', ['handle', ['id', 'Organization Unit ID']]),
        section('Parent Organization Unit', ['parent']),
      ]),
      tab('childOus', 'Child OUs', [], ['ous']),
      tab('users', 'Users', [], ['users']),
      tab('groups', 'Groups', [], ['groups']),
      tab('flows', 'Flows', [section(undefined, SIGN_IN_FLOWS)]),
      tab('customization', 'Customization', [APPEARANCE]),
    ],
  },
  {
    segment: RoutePaths.userTypes,
    types: ['user_type'],
    title: ['common:environment.sections.userTypes', 'User Types'],
    tabs: [
      tab('general', 'General', [
        section('Quick Copy', [
          ['id', 'User Type ID'],
          ['handle', 'User Type Handle'],
        ]),
        section('Organization Unit', ['ouId', 'ouHandle']),
        section('Self Registration', ['allowSelfRegistration']),
        section('Display Attribute', [['systemAttributes.display', 'Display attribute']]),
      ]),
      tab('schema', 'Schema', [SCHEMA]),
    ],
  },
  {
    segment: RoutePaths.agentTypes,
    types: ['agent_type'],
    title: ['common:environment.sections.agentTypes', 'Agent Types'],
    tabs: [tab('schema', 'Schema', [SCHEMA])],
  },
  {
    segment: RoutePaths.connections,
    types: ['connection'],
    title: ['common:environment.sections.connections', 'Connections'],
    tabs: [
      tab('general', 'General', [section('Quick copy', ['id', 'type'])]),
      tab('authentication', 'Authentication', [section(undefined, ['authentication'])]),
      tab('attributeConfiguration', 'Attribute Configuration', [
        section(undefined, ['attributeConfiguration', 'subjectAttributeMappings']),
      ]),
    ],
  },
  {
    segment: RoutePaths.flows,
    types: ['flow'],
    title: ['common:environment.sections.flows', 'Flows'],
  },
  {
    segment: RoutePaths.design,
    types: ['theme', 'layout'],
    title: ['common:environment.sections.design', 'Design'],
    tabs: (design: Record<string, unknown>) =>
      'layout' in design
        ? [
            tab('screens', 'Screens', [section(undefined, ['layout.screens'])]),
            tab('stylesheets', 'Stylesheets', [section(undefined, ['layout.stylesheets'])]),
          ]
        : [
            tab('colors', 'Colors', [section(undefined, ['theme.colorSchemes'])]),
            tab('shape', 'Shape', [section(undefined, ['theme.shape'])]),
            tab('typography', 'Typography', [section(undefined, ['theme.typography'])]),
            tab('settings', 'Settings', [
              section(undefined, [
                ['theme.defaultColorScheme', 'Default Color Scheme'],
                ['theme.direction', 'Default Text Direction'],
              ]),
            ]),
          ],
  },
  {
    segment: RoutePaths.translations,
    types: ['translation'],
    title: ['common:environment.sections.translations', 'Translations'],
    // A tab to each namespace, as the configuration's page chooses one to edit.
    tabs: (translation: Record<string, unknown>) =>
      Object.keys((translation.translations ?? {}) as Record<string, unknown>)
        .sort()
        .map((namespace: string) => ({
          key: `ns:${namespace}`,
          // A namespace is named as it is, not translated.
          title: ['', namespace] as Label,
          sections: [{fields: [`translations.${namespace}`]}],
        })),
  },
  {
    segment: RoutePaths.verifiableCredentials,
    types: ['credential_configuration'],
    title: ['common:environment.sections.verifiableCredentials', 'Verifiable Credentials'],
    tabs: [
      tab('general', 'General', [section('Quick Copy', ['id', 'handle']), OU]),
      tab('settings', 'Settings', [
        section(undefined, [
          ['vct', 'VCT'],
          'format',
          ['display.locale', 'Locale'],
          ['display.logoUri', 'Logo URI'],
          'validitySeconds',
        ]),
      ]),
      tab('claims', 'Claims', [section(undefined, ['claims'])]),
    ],
  },
  {
    segment: RoutePaths.verifiablePresentations,
    types: ['presentation_definition'],
    title: ['common:environment.sections.verifiablePresentations', 'Verifiable Presentations'],
    tabs: [
      tab('general', 'General', [section('Quick Copy', ['id', 'handle']), OU]),
      tab('settings', 'Settings', [section(undefined, [['vct', 'VCT'], 'format'])]),
      tab('claims', 'Claims', [section(undefined, ['mandatoryClaims', 'optionalClaims', 'claimValues'])]),
      tab('issuerTrust', 'Issuer Trust', [section(undefined, ['enforceTrustedIssuer', 'trustedAuthorities'])]),
    ],
  },
  {
    segment: RoutePaths.settings,
    types: ['server_config'],
    title: ['common:environment.sections.settings', 'Settings'],
    // The settings the environment runs, with what its own configuration fixes merged in.
    tabs: [tab('settings', 'Settings', [section(undefined, ['merged'])])],
  },
];

/** The layout of the console section with this path segment. */
export function layoutOf(segment: string | undefined): ResourceLayout | undefined {
  return RESOURCE_LAYOUTS.find((layout: ResourceLayout) => layout.segment === segment);
}

/** The layout a resource type is shown under. */
export function layoutOfType(resourceType: string): ResourceLayout | undefined {
  return RESOURCE_LAYOUTS.find((layout: ResourceLayout) => layout.types.includes(resourceType));
}

function asFields(resource: unknown): Fields {
  return resource !== null && typeof resource === 'object' && !Array.isArray(resource) ? (resource as Fields) : {};
}

/** What a resource is called, falling back to its identifier. */
export function nameOf(resource: unknown, id: string, layout?: ResourceLayout): string {
  const fields = asFields(resource);
  return layout?.nameOf?.(fields) ?? NAME_FIELDS.map((field: string) => text(fields[field])).find(Boolean) ?? id;
}

/** How a resource is described, when it is. */
export function descriptionOf(resource: unknown): string | undefined {
  const fields = asFields(resource);
  return DESCRIPTION_FIELDS.map((field: string) => text(fields[field])).find(Boolean);
}
