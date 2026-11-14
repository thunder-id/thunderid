// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {SettingsCard} from '@thunderid/components';
import {FormControl, FormHelperText, FormLabel, MenuItem, Select, Stack} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import ConnectionConstants from '../constants/connection-constants';
import type {SyncConfiguration, SyncMode} from '../models/connection';

const ATTRIBUTES_MODES: SyncMode[] = ['override_received', 'no_sync', 'session_only'];
const GROUPS_AND_ROLES_MODES: SyncMode[] = ['no_sync', 'override_all', 'session_only'];
const MODE_LABELS: Record<SyncMode, string> = {
  no_sync: 'No sync',
  session_only: 'Session only',
  override_received: 'Override received',
  override_all: 'Override all',
};
const MODE_HINTS: Record<keyof SyncConfiguration, Partial<Record<SyncMode, string>>> = {
  attributes: {
    override_received:
      'Each sign-in updates the attributes in the attribute mappings when the provider sends them. Other attributes are kept.',
    no_sync: "The provider's attributes are stored when a sign-in creates the account, and are not updated after.",
    session_only:
      "The account stores only the required attributes and the values the user enters. The provider's other attributes apply to the session.",
  },
  groupsAndRoles: {
    no_sync:
      'The groups and roles the authorization mapping resolves are assigned when a sign-in creates the account, and are not updated after.',
    override_all:
      "Each sign-in matches the account's groups and roles to the authorization mapping, removing ones assigned locally.",
    session_only:
      'The groups and roles the authorization mapping resolves apply to the session and are not assigned to the account.',
  },
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
    modes: SyncMode[],
    value: SyncMode,
  ): JSX.Element => (
    <FormControl fullWidth>
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
      <FormHelperText>{t(`attributeMapping.sync.${setting}.hint.${value}`, MODE_HINTS[setting][value])}</FormHelperText>
    </FormControl>
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
          t('attributeMapping.sync.attributes.label', 'Attributes sync'),
          ATTRIBUTES_MODES,
          sync?.attributes ?? ConnectionConstants.DEFAULT_SYNC_MODES.attributes,
        )}
        {renderSelect(
          'groupsAndRoles',
          t('attributeMapping.sync.groupsAndRoles.label', 'Groups and roles sync'),
          GROUPS_AND_ROLES_MODES,
          sync?.groupsAndRoles ?? ConnectionConstants.DEFAULT_SYNC_MODES.groupsAndRoles,
        )}
      </Stack>
    </SettingsCard>
  );
}
