// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {RuntimeEnvironment} from '@thunderid/contexts';
import {Box, Stack, Typography} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {Trans, useTranslation} from 'react-i18next';
import EnvironmentPicker from '../edit-application/integration-guides/EnvironmentPicker';

/**
 * Props for the {@link EnvironmentChoice} component.
 *
 * @public
 */
export interface EnvironmentChoiceProps {
  /** The environments to choose from, in the order a change moves through them. */
  environments: RuntimeEnvironment[];
  /** The environment whose values are shown. */
  selectedEnvironment: RuntimeEnvironment;
  onSelect: (environmentId: string) => void;
  /** The environments the resource is not deployed to yet, which are marked as such. */
  notDeployedIds?: string[];
}

/**
 * The head of an overview card whose values differ between environments: the environments to choose
 * from, and a line naming the one whose values are shown.
 *
 * @public
 */
export default function EnvironmentChoice({
  environments,
  selectedEnvironment,
  onSelect,
  notDeployedIds = [],
}: EnvironmentChoiceProps): JSX.Element {
  const {t} = useTranslation();

  return (
    <>
      <Typography
        variant="caption"
        component="h3"
        sx={{
          display: 'block',
          fontWeight: 600,
          letterSpacing: '0.03em',
          color: 'text.secondary',
          textTransform: 'uppercase',
          mb: 1,
        }}
      >
        {t('applications:edit.overview.environments.title', 'Environment')}
      </Typography>
      <EnvironmentPicker
        environments={environments}
        selectedId={selectedEnvironment.id}
        onSelect={onSelect}
        notDeployedIds={notDeployedIds}
      />
      <Stack direction="row" alignItems="center" spacing={1} sx={{mt: 1.25}}>
        <Box
          aria-hidden
          sx={(theme) => ({
            width: 8,
            height: 8,
            borderRadius: '50%',
            flexShrink: 0,
            bgcolor: selectedEnvironment.color?.light ?? 'text.secondary',
            ...theme.applyStyles('dark', {bgcolor: selectedEnvironment.color?.dark ?? 'text.secondary'}),
          })}
        />
        <Typography variant="body2" role="status">
          <Trans
            i18nKey="applications:edit.overview.environments.description"
            defaults="Showing values for <strong>{{environment}}</strong>"
            values={{environment: selectedEnvironment.name}}
            components={{
              strong: (
                <Box
                  component="strong"
                  sx={(theme) => ({
                    textTransform: 'uppercase',
                    color: selectedEnvironment.color?.light ?? 'text.primary',
                    ...theme.applyStyles('dark', {color: selectedEnvironment.color?.dark ?? 'text.primary'}),
                  })}
                />
              ),
            }}
          />
        </Typography>
      </Stack>
    </>
  );
}
