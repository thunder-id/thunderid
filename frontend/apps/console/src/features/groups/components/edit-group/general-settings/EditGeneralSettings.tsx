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
import {
  Stack,
  TextField,
  Button,
  IconButton,
  Typography,
  InputAdornment,
  Tooltip,
  FormControl,
  FormLabel,
} from '@wso2/oxygen-ui';
import {Copy, Check} from '@wso2/oxygen-ui-icons-react';
import {useState, useCallback, useRef, useEffect, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import QuickCopySection from './QuickCopySection';
import type {Group} from '../../../models/group';

interface EditGeneralSettingsProps {
  group: Group;
  onDeleteClick?: () => void;
}

/**
 * General settings tab content for the Group edit page.
 * Displays Organization Unit and Danger Zone sections.
 */
export default function EditGeneralSettings({group, onDeleteClick = undefined}: EditGeneralSettingsProps): JSX.Element {
  const {t} = useTranslation();
  const [copiedField, setCopiedField] = useState<string | null>(null);
  const copyTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(
    () => () => {
      if (copyTimeoutRef.current) {
        clearTimeout(copyTimeoutRef.current);
      }
    },
    [],
  );

  const handleCopyToClipboard = useCallback(async (text: string, fieldName: string): Promise<void> => {
    await navigator.clipboard.writeText(text);
    setCopiedField(fieldName);
    if (copyTimeoutRef.current) {
      clearTimeout(copyTimeoutRef.current);
    }
    copyTimeoutRef.current = setTimeout(() => {
      setCopiedField(null);
    }, 2000);
  }, []);

  return (
    <Stack spacing={3}>
      <QuickCopySection group={group} copiedField={copiedField} onCopyToClipboard={handleCopyToClipboard} />

      {/* Organization Unit */}
      <SettingsCard
        title={t('groups:edit.general.sections.organizationUnit.title')}
        description={t('groups:edit.general.sections.organizationUnit.description')}
      >
        <Stack spacing={2}>
          <FormControl fullWidth>
            <FormLabel htmlFor="ou-handle-input">
              {t('groups:edit.general.sections.organizationUnit.handleLabel', 'Handle')}
            </FormLabel>
            <TextField
              id="ou-handle-input"
              value={group.ouHandle ?? '-'}
              fullWidth
              size="small"
              slotProps={{
                input: {
                  readOnly: true,
                  endAdornment: group.ouHandle ? (
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
                          aria-label={t(
                            'groups:edit.general.sections.quickCopy.copyOrganizationUnitHandle',
                            'Copy Organization Unit Handle',
                          )}
                          onClick={() => {
                            handleCopyToClipboard(group.ouHandle!, 'ouHandle').catch(() => null);
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
              value={group.ouId}
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
                            : t('groups:edit.general.sections.quickCopy.copyOrganizationUnitId')
                        }
                      >
                        <IconButton
                          aria-label={t('groups:edit.general.sections.quickCopy.copyOrganizationUnitId')}
                          onClick={() => {
                            handleCopyToClipboard(group.ouId, 'ouId').catch(() => null);
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

      {/* Danger Zone */}
      {onDeleteClick && (
        <SettingsCard
          title={t('groups:edit.general.sections.dangerZone.title')}
          description={t('groups:edit.general.sections.dangerZone.description')}
        >
          <Typography variant="h6" gutterBottom color="error">
            {t('groups:edit.general.sections.dangerZone.deleteGroup')}
          </Typography>
          <Typography variant="body2" color="text.secondary" sx={{mb: 3}}>
            {t('groups:edit.general.sections.dangerZone.deleteGroupDescription')}
          </Typography>
          <Button variant="contained" color="error" onClick={onDeleteClick}>
            {t('common:actions.delete')}
          </Button>
        </SettingsCard>
      )}
    </Stack>
  );
}
