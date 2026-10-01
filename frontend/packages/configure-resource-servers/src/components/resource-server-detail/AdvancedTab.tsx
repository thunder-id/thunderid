// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {SettingsCard} from '@thunderid/components';
import {FormControl, FormHelperText, FormLabel, MenuItem, Select, Stack, TextField} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {Link} from 'react-router';
import useAuthZENPDPConnections from '../../api/useAuthZENPDPConnections';
import {useResourceServerConnectionRoutes} from '../../hooks/useResourceServerRoutes';
import {type AuthorizationEngine, AuthorizationEngines, type ResourceServer} from '../../models/resource-server';

const PDP_OPTION_PREFIX = `${AuthorizationEngines.AUTHZEN_PDP}:`;

interface AdvancedTabProps {
  resourceServer: ResourceServer;
  identifier: string;
  authorizationEngine: AuthorizationEngine;
  pdpConnectionId: string;
  onIdentifierChange: (value: string) => void;
  onAuthorizationEngineChange: (value: AuthorizationEngine) => void;
  onPDPConnectionChange: (value: string) => void;
}

export default function AdvancedTab({
  resourceServer,
  identifier,
  authorizationEngine,
  pdpConnectionId,
  onIdentifierChange,
  onAuthorizationEngineChange,
  onPDPConnectionChange,
}: AdvancedTabProps): JSX.Element {
  const {t} = useTranslation();
  const pdpConnections = useAuthZENPDPConnections();
  const connectionRoutes = useResourceServerConnectionRoutes();
  const authorizationEngineValue =
    authorizationEngine === AuthorizationEngines.AUTHZEN_PDP && pdpConnectionId
      ? `${PDP_OPTION_PREFIX}${pdpConnectionId}`
      : authorizationEngine;
  const hasSelectedPDP =
    authorizationEngine === AuthorizationEngines.AUTHZEN_PDP &&
    pdpConnectionId &&
    !(pdpConnections.data ?? []).some((connection) => connection.id === pdpConnectionId);

  return (
    <Stack spacing={3}>
      <SettingsCard
        title={t('resourceServers:edit.advanced.identifier.title', 'Configurations')}
        description={
          resourceServer.type === 'MCP'
            ? t(
                'resourceServers:edit.advanced.identifier.descriptionMcp',
                'Configuration settings for this MCP server.',
              )
            : t(
                'resourceServers:edit.advanced.identifier.description',
                'Configuration settings for this resource server.',
              )
        }
      >
        <FormControl fullWidth>
          <FormLabel htmlFor="resource-server-identifier">
            {t('resourceServers:edit.advanced.identifier.label', 'Identifier (Audience)')}
          </FormLabel>
          <TextField
            id="resource-server-identifier"
            value={identifier}
            onChange={(e) => onIdentifierChange(e.target.value)}
            fullWidth
            size="small"
            placeholder={
              resourceServer.type === 'MCP'
                ? t('resourceServers:edit.advanced.identifier.placeholderMcp', 'https://mcp.example.com')
                : t('resourceServers:edit.advanced.identifier.placeholder', 'https://api.example.com')
            }
            helperText={
              resourceServer.type === 'MCP'
                ? t(
                    'resourceServers:edit.advanced.identifier.hintMcp',
                    'A unique value that identifies this MCP server. When set as an URI, enables RFC 8707 resource indicator support in OAuth2 authorization requests.',
                  )
                : t(
                    'resourceServers:edit.advanced.identifier.hint',
                    'A unique value that identifies this resource server. When set as an URI, enables RFC 8707 resource indicator support in OAuth2 authorization requests.',
                  )
            }
            disabled={resourceServer.isReadOnly}
          />
        </FormControl>
        <FormControl fullWidth error={Boolean(pdpConnections.error)} sx={{mt: 3}}>
          <FormLabel id="resource-server-authorization-engine-label">
            {t('resourceServers:edit.advanced.authorizationEngine.label', 'Authorization engine')}
          </FormLabel>
          <Select
            id="resource-server-authorization-engine"
            labelId="resource-server-authorization-engine-label"
            value={authorizationEngineValue}
            onChange={(event) => {
              const value = event.target.value;
              if (value.startsWith(PDP_OPTION_PREFIX)) {
                onAuthorizationEngineChange(AuthorizationEngines.AUTHZEN_PDP);
                onPDPConnectionChange(value.slice(PDP_OPTION_PREFIX.length));
                return;
              }
              onAuthorizationEngineChange(AuthorizationEngines.RBAC);
              onPDPConnectionChange('');
            }}
            size="small"
            disabled={Boolean(resourceServer.isReadOnly) || pdpConnections.isLoading}
          >
            <MenuItem value={AuthorizationEngines.RBAC}>
              {t('resourceServers:edit.advanced.authorizationEngine.option.rbac', 'Local - Role Based Access Control')}
            </MenuItem>
            {hasSelectedPDP && (
              <MenuItem value={`${PDP_OPTION_PREFIX}${pdpConnectionId}`}>
                {t('resourceServers:edit.advanced.authorizationEngine.option.selectedPDP', 'Selected PDP')}
              </MenuItem>
            )}
            {(pdpConnections.data ?? []).map((connection) => (
              <MenuItem key={connection.id} value={`${PDP_OPTION_PREFIX}${connection.id}`}>
                {connection.name}
              </MenuItem>
            ))}
          </Select>
          <FormHelperText>
            {pdpConnections.error ? (
              t(
                'resourceServers:edit.advanced.authorizationEngine.loadPDPError',
                'Failed to load AuthZEN PDP connections.',
              )
            ) : pdpConnections.data?.length === 0 ? (
              <>
                {t(
                  'resourceServers:edit.advanced.authorizationEngine.noPDPConnections',
                  'No external PDP connections are available.',
                )}{' '}
                <Link to={connectionRoutes.create()}>
                  {t(
                    'resourceServers:edit.advanced.authorizationEngine.createPDPConnection',
                    'Create a PDP connection',
                  )}
                </Link>
              </>
            ) : (
              t(
                'resourceServers:edit.advanced.authorizationEngine.hint',
                'Choose Local - Role Based Access Control or a configured PDP connection for this resource server.',
              )
            )}
          </FormHelperText>
        </FormControl>
      </SettingsCard>
    </Stack>
  );
}
