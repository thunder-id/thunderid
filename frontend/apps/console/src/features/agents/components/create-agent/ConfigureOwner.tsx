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

import {useGetUsers} from '@thunderid/configure-users';
import {useThunderID} from '@thunderid/react';
import {FormControl, FormLabel, MenuItem, Select, Stack, Typography} from '@wso2/oxygen-ui';
import {useEffect, useMemo, type JSX} from 'react';
import {useTranslation} from 'react-i18next';

export interface ConfigureOwnerProps {
  selectedOwnerId: string | null;
  onOwnerIdChange: (id: string | null) => void;
  onReadyChange?: (isReady: boolean) => void;
}

interface UserOption {
  id: string;
  label: string;
}

const formatUserLabel = (user: {id: string; display?: string; attributes?: Record<string, unknown>}): string => {
  if (user.display) return user.display;
  const attrs = user.attributes ?? {};
  const username = typeof attrs.username === 'string' ? attrs.username : undefined;
  const email = typeof attrs.email === 'string' ? attrs.email : undefined;
  return username ?? email ?? user.id;
};

export default function ConfigureOwner({
  selectedOwnerId,
  onOwnerIdChange,
  onReadyChange = undefined,
}: ConfigureOwnerProps): JSX.Element {
  const {t} = useTranslation();
  const currentUser = useThunderID().user as {id?: string} | null | undefined;
  const currentUserId = currentUser?.id ?? null;

  const {data: usersData, isLoading: usersLoading} = useGetUsers({limit: 100, offset: 0});

  const options: UserOption[] = useMemo(
    () => (usersData?.users ?? []).map((user) => ({id: user.id, label: formatUserLabel(user)})),
    [usersData],
  );

  // Default to the current user once we have one and nothing is selected.
  useEffect(() => {
    if (!selectedOwnerId && currentUserId) {
      onOwnerIdChange(currentUserId);
    }
  }, [currentUserId, selectedOwnerId, onOwnerIdChange]);

  // Step is always ready — owner has either a default (current user) or a chosen user.
  useEffect(() => {
    onReadyChange?.(true);
  }, [onReadyChange]);

  return (
    <Stack direction="column" spacing={4} data-testid="configure-agent-owner">
      <div>
        <Typography variant="h1" gutterBottom>
          {t('agents:createWizard.owner.title', 'Owner')}
        </Typography>
        <Typography variant="body1" color="text.secondary">
          {t('agents:createWizard.owner.subtitle', 'Choose the user that owns this agent.')}
        </Typography>
      </div>

      <FormControl fullWidth required>
        <FormLabel htmlFor="agent-owner-user-select">{t('agents:createWizard.owner.userLabel', 'Owner')}</FormLabel>
        <Select
          id="agent-owner-user-select"
          value={selectedOwnerId ?? ''}
          onChange={(e) => onOwnerIdChange(e.target.value || null)}
          displayEmpty
          disabled={usersLoading}
        >
          <MenuItem value="" disabled>
            <em>
              {usersLoading
                ? t('common:status.loading', 'Loading…')
                : t('agents:createWizard.owner.userPlaceholder', 'Select a user')}
            </em>
          </MenuItem>
          {options.map((opt) => (
            <MenuItem key={opt.id} value={opt.id}>
              {opt.label}
            </MenuItem>
          ))}
        </Select>
      </FormControl>
    </Stack>
  );
}
