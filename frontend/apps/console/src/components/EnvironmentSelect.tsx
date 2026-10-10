// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useEnvironment, type Environment} from '@thunderid/contexts';
import {FormControl, ListItemIcon, ListItemText, ListSubheader, MenuItem, Select, Stack} from '@wso2/oxygen-ui';
import {Eye, Pencil} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';

/** The choice that stands for the configuration, which is not an environment. */
const CONFIGURATION = '';

/**
 * Chooses what the console shows: the configuration, which is edited, or one environment, which is
 * the configuration as that environment runs it and is only read. Shown only where there are
 * environments.
 */
export default function EnvironmentSelect(): JSX.Element | null {
  const {t} = useTranslation();
  const {environments, selected, select} = useEnvironment();

  if (environments.length === 0) {
    return null;
  }

  const configurationLabel = t('common:environment.configuration', 'Configuration');
  const label = (environment: Environment): string =>
    environment.isDefault
      ? t('common:environment.default', '{{name}} (default)', {name: environment.name})
      : environment.name;

  return (
    <FormControl size="small" sx={{minWidth: 200, mr: 1}}>
      <Select
        displayEmpty
        value={selected?.id ?? CONFIGURATION}
        onChange={(event) => select(String(event.target.value) || undefined)}
        inputProps={{'aria-label': t('common:environment.label', 'Shown')}}
        renderValue={(value: string) => (
          <Stack direction="row" spacing={1} alignItems="center">
            {value === CONFIGURATION ? <Pencil size={14} /> : <Eye size={14} />}
            <span>{value === CONFIGURATION ? configurationLabel : (selected?.name ?? '')}</span>
          </Stack>
        )}
      >
        <MenuItem value={CONFIGURATION}>
          <ListItemIcon>
            <Pencil size={16} />
          </ListItemIcon>
          <ListItemText
            primary={configurationLabel}
            secondary={t('common:environment.configurationHint', 'Edit the configuration')}
          />
        </MenuItem>
        <ListSubheader>{t('common:environment.environments', 'Environments (read only)')}</ListSubheader>
        {environments.map((environment: Environment) => (
          <MenuItem key={environment.id} value={environment.id}>
            <ListItemIcon>
              <Eye size={16} />
            </ListItemIcon>
            <ListItemText primary={label(environment)} />
          </MenuItem>
        ))}
      </Select>
    </FormControl>
  );
}
