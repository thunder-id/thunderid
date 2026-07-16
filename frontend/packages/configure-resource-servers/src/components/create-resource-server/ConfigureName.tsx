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

import {generateRandomHumanReadableIdentifiers} from '@thunderid/utils';
import {Box, Chip, FormControl, FormLabel, Stack, TextField, Typography, useTheme} from '@wso2/oxygen-ui';
import {Lightbulb} from '@wso2/oxygen-ui-icons-react';
import {useEffect, useMemo, type ChangeEvent, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import type {ResourceServerType} from '../../models/resource-server';

interface ConfigureNameProps {
  name: string;
  identifier: string;
  /** The resource server type selected in the previous step, used to tailor copy for MCP servers. */
  selectedType?: ResourceServerType;
  onNameChange: (name: string) => void;
  onIdentifierChange: (identifier: string) => void;
  onReadyChange?: (isReady: boolean) => void;
}

export default function ConfigureName({
  name,
  identifier,
  selectedType = undefined,
  onNameChange,
  onIdentifierChange,
  onReadyChange = undefined,
}: ConfigureNameProps): JSX.Element {
  const {t} = useTranslation();
  const theme = useTheme();

  const suggestions: string[] = useMemo((): string[] => generateRandomHumanReadableIdentifiers(), []);

  useEffect((): void => {
    if (onReadyChange) {
      onReadyChange(name.trim().length > 0 && identifier.trim().length > 0);
    }
  }, [name, identifier, onReadyChange]);

  const handleNameChange = (e: ChangeEvent<HTMLInputElement>): void => {
    onNameChange(e.target.value);
  };

  const handleSuggestionClick = (suggestion: string): void => {
    onNameChange(suggestion);
  };

  const handleIdentifierChange = (e: ChangeEvent<HTMLInputElement>): void => {
    onIdentifierChange(e.target.value);
  };

  return (
    <Stack direction="column" spacing={4}>
      <Typography variant="h1" gutterBottom>
        {selectedType === 'MCP'
          ? t('resourceServers:create.name.titleMcp', 'Name your MCP server')
          : t('resourceServers:create.name.title', 'Name your resource server')}
      </Typography>

      <FormControl fullWidth required>
        <FormLabel htmlFor="resource-server-name-input">
          {selectedType === 'MCP'
            ? t('resourceServers:create.name.nameLabelMcp', 'MCP Server Name')
            : t('resourceServers:create.name.nameLabel', 'Resource Server Name')}
        </FormLabel>
        <TextField
          id="resource-server-name-input"
          fullWidth
          value={name}
          onChange={handleNameChange}
          placeholder={t('resourceServers:create.name.namePlaceholder', 'e.g. Payments API')}
        />
      </FormControl>

      <Stack direction="column" spacing={2}>
        <Stack direction="row" alignItems="center" spacing={1}>
          <Lightbulb size={20} color={theme.vars?.palette.warning.main} />
          <Typography variant="body2" color="text.secondary">
            {t('resourceServers:create.name.suggestions', 'Need inspiration? Pick one:')}
          </Typography>
        </Stack>
        <Box sx={{display: 'flex', flexWrap: 'wrap', gap: 1}}>
          {suggestions.map(
            (suggestion: string): JSX.Element => (
              <Chip
                key={suggestion}
                label={suggestion}
                onClick={(): void => handleSuggestionClick(suggestion)}
                variant="outlined"
                clickable
                sx={{
                  '&:hover': {
                    bgcolor: 'primary.main',
                    color: 'primary.contrastText',
                    borderColor: 'primary.main',
                  },
                }}
              />
            ),
          )}
        </Box>
      </Stack>

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
              ? t('resourceServers:create.name.identifierPlaceholderMcp', 'https://mcp.example.com')
              : t('resourceServers:create.name.identifierPlaceholder', 'https://api.example.com')
          }
          helperText={
            selectedType === 'MCP'
              ? t(
                  'resourceServers:create.name.identifierHintMcp',
                  'A unique identifier for this MCP server. When set as an absolute URI, it becomes the token audience for RFC 8707 resource indicators.',
                )
              : t(
                  'resourceServers:create.name.identifierHint',
                  'A unique identifier for this resource server. When set as an absolute URI, it becomes the token audience for RFC 8707 resource indicators.',
                )
          }
        />
      </FormControl>
    </Stack>
  );
}
