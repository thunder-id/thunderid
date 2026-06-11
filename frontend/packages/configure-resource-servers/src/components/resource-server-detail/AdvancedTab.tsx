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
import {useToast} from '@thunderid/contexts';
import {useLogger} from '@thunderid/logger/react';
import {Box, Button, Stack, TextField} from '@wso2/oxygen-ui';
import {useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import useUpdateResourceServer from '../../api/useUpdateResourceServer';
import type {ResourceServer} from '../../models/resource-server';

interface AdvancedTabProps {
  resourceServer: ResourceServer;
  onRefresh: () => void;
}

export default function AdvancedTab({resourceServer, onRefresh}: AdvancedTabProps): JSX.Element {
  const {t} = useTranslation();
  const {showToast} = useToast();
  const logger = useLogger('AdvancedTab');
  const updateRs = useUpdateResourceServer();

  const [identifier, setIdentifier] = useState(resourceServer.identifier ?? '');
  const [identifierDirty, setIdentifierDirty] = useState(false);

  const handleIdentifierSave = (): void => {
    updateRs.mutate(
      {id: resourceServer.id, data: {identifier: identifier || null}},
      {
        onSuccess: () => {
          showToast(t('resourceServers:edit.advanced.identifier.saved', 'Identifier saved.'), 'success');
          setIdentifierDirty(false);
          onRefresh();
        },
        onError: (err: Error) => {
          logger.error('Failed to save identifier', {error: err});
          showToast(t('resourceServers:edit.advanced.identifier.saveError', 'Failed to save identifier.'), 'error');
        },
      },
    );
  };

  const handleIdentifierDiscard = (): void => {
    setIdentifier(resourceServer.identifier ?? '');
    setIdentifierDirty(false);
  };

  return (
    <Stack spacing={3}>
      <SettingsCard
        title={t('resourceServers:edit.advanced.identifier.title', 'Configurations')}
        description={t(
          'resourceServers:edit.advanced.identifier.description',
          'Configuration settings for this resource server.',
        )}
      >
        <Stack spacing={2}>
          <TextField
            label={t('resourceServers:edit.advanced.identifier.label', 'Identifier (Audience)')}
            value={identifier}
            onChange={(e) => {
              setIdentifier(e.target.value);
              setIdentifierDirty(true);
            }}
            fullWidth
            size="small"
            helperText={t(
              'resourceServers:edit.advanced.identifier.hint',
              'A unique value that identifies this resource server. When set as an URI,enables RFC 8707 resource indicator support in OAuth2 authorization requests.',
            )}
            disabled={resourceServer.isReadOnly}
          />
          {!resourceServer.isReadOnly && identifierDirty && (
            <Box sx={{display: 'flex', gap: 1}}>
              <Button variant="outlined" size="small" onClick={handleIdentifierDiscard} disabled={updateRs.isPending}>
                {t('common:discard', 'Discard')}
              </Button>
              <Button variant="contained" size="small" onClick={handleIdentifierSave} disabled={updateRs.isPending}>
                {updateRs.isPending ? t('common:saving', 'Saving…') : t('common:save', 'Save')}
              </Button>
            </Box>
          )}
        </Stack>
      </SettingsCard>
    </Stack>
  );
}
