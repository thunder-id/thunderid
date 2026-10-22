// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {SettingsCard} from '@thunderid/components';
import {FormHelperText, FormLabel, MenuItem, Select, Stack} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import type {SyncConfiguration, SyncMode} from '../models/connection';

const ATTRIBUTES_MODES: SyncMode[] = ['override_received', 'no_sync', 'session_only'];
const GROUPS_AND_ROLES_MODES: SyncMode[] = ['no_sync', 'override_all', 'session_only'];
const MODE_LABELS: Record<SyncMode, string> = {
  no_sync: 'No sync',
  session_only: 'Session only',
  override_received: 'Override received',
  override_all: 'Override all',
};

export interface SyncSectionProps {
  sync?: SyncConfiguration;
  onChange: (sync: SyncConfiguration) => void;
}

/**
 * Sets how sign-ins through the connection keep the account in line with the provider. An unset mode
 * shows its server default: override received for attributes and no sync for groups and roles.
 */
export default function SyncSection({sync = undefined, onChange}: SyncSectionProps): JSX.Element {
  const {t} = useTranslation('connections');

  const renderSelect = (
    setting: keyof SyncConfiguration,
    label: string,
    hint: string,
    modes: SyncMode[],
    value: SyncMode,
  ): JSX.Element => (
    <Stack direction="column" spacing={1}>
      <FormLabel id={`sync-${setting}-label`}>{label}</FormLabel>
      <Select
        value={value}
        onChange={(e) => onChange({...sync, [setting]: e.target.value as SyncMode})}
        labelId={`sync-${setting}-label`}
      >
        {modes.map((mode) => (
          <MenuItem key={mode} value={mode}>
            {t(`attributeMapping.sync.mode.${mode}`, MODE_LABELS[mode])}
          </MenuItem>
        ))}
      </Select>
      <FormHelperText>{hint}</FormHelperText>
    </Stack>
  );

  return (
    <SettingsCard
      title={t('attributeMapping.sync.title', 'Sync')}
      description={t(
        'attributeMapping.sync.description',
        'How sign-ins through this connection keep the account in line with the provider.',
      )}
    >
      <Stack direction="column" spacing={3}>
        {renderSelect(
          'attributes',
          t('attributeMapping.sync.attributes.label', 'Attributes Sync'),
          t(
            'attributeMapping.sync.attributes.hint',
            'Override received updates only the attributes in the attribute mappings. Session only stores only the identifier and required attributes.',
          ),
          ATTRIBUTES_MODES,
          sync?.attributes ?? 'override_received',
        )}
        {renderSelect(
          'groupsAndRoles',
          t('attributeMapping.sync.groupsAndRoles.label', 'Groups and Roles Sync'),
          t(
            'attributeMapping.sync.groupsAndRoles.hint',
            'Groups and roles come from the authorization mapping. Override all removes groups and roles the mapping does not resolve, including ones assigned locally.',
          ),
          GROUPS_AND_ROLES_MODES,
          sync?.groupsAndRoles ?? 'no_sync',
        )}
      </Stack>
    </SettingsCard>
  );
}
