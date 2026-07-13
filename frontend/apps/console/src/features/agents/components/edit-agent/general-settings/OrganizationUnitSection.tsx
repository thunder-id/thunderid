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

import {SettingsCard} from '@thunderid/components';
import {FormControl, FormLabel, IconButton, InputAdornment, Stack, TextField, Tooltip} from '@wso2/oxygen-ui';
import {Check, Copy} from '@wso2/oxygen-ui-icons-react';
import {useTranslation} from 'react-i18next';
import type {Agent} from '../../../models/agent';

interface OrganizationUnitSectionProps {
  agent: Agent;
  copiedField: string | null;
  onCopyToClipboard: (text: string, fieldName: string) => Promise<void>;
}

export default function OrganizationUnitSection({agent, copiedField, onCopyToClipboard}: OrganizationUnitSectionProps) {
  const {t} = useTranslation();

  return (
    <SettingsCard
      title={t('agents:edit.general.sections.organizationUnit.title', 'Organization Unit')}
      description={t(
        'agents:edit.general.sections.organizationUnit.description',
        'The organization unit this agent belongs to.',
      )}
    >
      <Stack spacing={2}>
        <FormControl fullWidth>
          <FormLabel htmlFor="ou-handle-input">
            {t('groups:edit.general.sections.organizationUnit.handleLabel', 'Handle')}
          </FormLabel>
          <TextField
            id="ou-handle-input"
            value={agent.ouHandle ?? '-'}
            fullWidth
            size="small"
            slotProps={{
              input: {
                readOnly: true,
                endAdornment: agent.ouHandle ? (
                  <InputAdornment position="end">
                    <Tooltip
                      title={
                        copiedField === 'ouHandle'
                          ? t('common:actions.copied')
                          : t(
                              'groups:edit.general.sections.quickCopy.copyOrganizationUnitHandle',
                              'Copy Organization Unit Handle',
                            )
                      }
                    >
                      <IconButton
                        onClick={() => {
                          if (agent.ouHandle) {
                            onCopyToClipboard(agent.ouHandle, 'ouHandle').catch(() => null);
                          }
                        }}
                        edge="end"
                      >
                        {copiedField === 'ouHandle' ? <Check size={16} /> : <Copy size={16} />}
                      </IconButton>
                    </Tooltip>
                  </InputAdornment>
                ) : undefined,
              },
            }}
            sx={{'& input': {fontFamily: 'monospace', fontSize: '0.875rem'}}}
          />
        </FormControl>
        <FormControl fullWidth>
          <FormLabel htmlFor="ou-id-input">
            {t('groups:edit.general.sections.organizationUnit.idLabel', 'ID')}
          </FormLabel>
          <TextField
            id="ou-id-input"
            value={agent.ouId}
            fullWidth
            size="small"
            slotProps={{
              input: {
                readOnly: true,
                endAdornment: (
                  <InputAdornment position="end">
                    <Tooltip
                      title={
                        copiedField === 'ouId'
                          ? t('common:actions.copied')
                          : t(
                              'groups:edit.general.sections.quickCopy.copyOrganizationUnitId',
                              'Copy Organization Unit ID',
                            )
                      }
                    >
                      <IconButton
                        onClick={() => {
                          onCopyToClipboard(agent.ouId, 'ouId').catch(() => null);
                        }}
                        edge="end"
                      >
                        {copiedField === 'ouId' ? <Check size={16} /> : <Copy size={16} />}
                      </IconButton>
                    </Tooltip>
                  </InputAdornment>
                ),
              },
            }}
            sx={{'& input': {fontFamily: 'monospace', fontSize: '0.875rem'}}}
          />
        </FormControl>
      </Stack>
    </SettingsCard>
  );
}
