// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {NameSuggestion} from '@thunderid/components';
import {Checkbox, FormControl, FormControlLabel, FormLabel, Stack, TextField, Typography} from '@wso2/oxygen-ui';
import {useEffect, type ChangeEvent, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {isDefaultEligibleType, type ResourceServerType} from '../../models/resource-server';
import {isValidMcpServerIdentifier} from '../../utils/isValidMcpServerIdentifier';

interface ConfigureNameProps {
  name: string;
  identifier: string;
  /** The resource server type selected in the previous step, used to tailor copy for MCP servers. */
  selectedType?: ResourceServerType;
  /**
   * Whether the default resource server choice can be offered at all. False while the server config
   * is still loading and when a declarative default has locked it, since the backend rejects writes.
   */
  canSetDefault?: boolean;
  /** Whether the server being created should become the default resource server. */
  makeDefault?: boolean;
  onNameChange: (name: string) => void;
  onIdentifierChange: (identifier: string) => void;
  onReadyChange?: (isReady: boolean) => void;
  onMakeDefaultChange?: (makeDefault: boolean) => void;
}

export default function ConfigureName({
  name,
  identifier,
  selectedType = undefined,
  canSetDefault = false,
  makeDefault = false,
  onNameChange,
  onIdentifierChange,
  onReadyChange = undefined,
  onMakeDefaultChange = undefined,
}: ConfigureNameProps): JSX.Element {
  const {t} = useTranslation();

  const trimmedIdentifier = identifier.trim();
  const isIdentifierInvalid =
    selectedType === 'MCP' && trimmedIdentifier.length > 0 && !isValidMcpServerIdentifier(trimmedIdentifier);

  useEffect((): void => {
    if (onReadyChange) {
      onReadyChange(name.trim().length > 0 && trimmedIdentifier.length > 0 && !isIdentifierInvalid);
    }
  }, [name, trimmedIdentifier, isIdentifierInvalid, onReadyChange]);

  const handleNameChange = (e: ChangeEvent<HTMLInputElement>): void => {
    onNameChange(e.target.value);
  };

  const handleIdentifierChange = (e: ChangeEvent<HTMLInputElement>): void => {
    onIdentifierChange(e.target.value);
  };

  const handleMakeDefaultChange = (e: ChangeEvent<HTMLInputElement>): void => {
    onMakeDefaultChange?.(e.target.checked);
  };

  const isDefaultEligible = canSetDefault && isDefaultEligibleType(selectedType);

  return (
    <Stack direction="column" spacing={4}>
      <Typography variant="h1" gutterBottom>
        {selectedType === 'MCP'
          ? t('resourceServers:create.name.titleMcp', "Let's collect some details about your MCP server")
          : t('resourceServers:create.name.title', "Let's collect some details about your resource server")}
      </Typography>

      <FormControl fullWidth required>
        <FormLabel htmlFor="resource-server-name-input">
          {selectedType === 'MCP'
            ? t('resourceServers:create.name.nameLabelMcp', 'MCP server name')
            : t('resourceServers:create.name.nameLabel', 'Resource server name')}
        </FormLabel>
        <TextField
          id="resource-server-name-input"
          fullWidth
          value={name}
          onChange={handleNameChange}
          placeholder={t('resourceServers:create.name.namePlaceholder', 'e.g. Payments API')}
        />

        <NameSuggestion onSelect={onNameChange} />
      </FormControl>

      <FormControl fullWidth required>
        <FormLabel htmlFor="resource-server-identifier-input">
          {t('resourceServers:create.name.identifierLabel', 'Identifier')}
        </FormLabel>
        <TextField
          id="resource-server-identifier-input"
          fullWidth
          value={identifier}
          onChange={handleIdentifierChange}
          placeholder={
            selectedType === 'MCP'
              ? t('resourceServers:create.name.identifierPlaceholderMcp', 'https://mcp.example.com/mcp')
              : t('resourceServers:create.name.identifierPlaceholder', 'https://api.example.com')
          }
          error={isIdentifierInvalid}
          helperText={
            isIdentifierInvalid
              ? t(
                  'resourceServers:create.name.identifierInvalidMcp',
                  'Enter an http or https URL with no fragment, such as https://mcp.example.com/mcp.',
                )
              : selectedType === 'MCP'
                ? t(
                    'resourceServers:create.name.identifierHintMcp',
                    'The URL MCP clients use to reach this server, such as https://mcp.example.com/mcp. MCP clients send it as the resource parameter, and it becomes the token audience.',
                  )
                : t(
                    'resourceServers:create.name.identifierHint',
                    'A unique identifier for this resource server. When set as an absolute URI, it becomes the token audience for RFC 8707 resource indicators.',
                  )
          }
        />
      </FormControl>

      {isDefaultEligible && (
        <FormControl fullWidth>
          <FormControlLabel
            control={
              <Checkbox
                id="resource-server-make-default-input"
                checked={makeDefault}
                onChange={handleMakeDefaultChange}
              />
            }
            label={t('resourceServers:create.name.makeDefaultLabel', 'Make this the default resource server')}
          />
          <Typography variant="body2" color="text.secondary">
            {t(
              'resourceServers:setDefault.explanation',
              'When an application requests a token without naming a resource server, its permissions come from this one. Only one resource server can be the default at a time.',
            )}
          </Typography>
        </FormControl>
      )}
    </Stack>
  );
}
