// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {RuntimeEnvironment} from '@thunderid/contexts';
import {Box, Button, MenuItem, Paper, Select, Stack, Typography} from '@wso2/oxygen-ui';
import {ArrowRight} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {Link as RouterLink} from 'react-router';

/**
 * Props for the {@link EnvironmentBar} component.
 *
 * @public
 */
export interface EnvironmentBarProps {
  /** The environments to choose from, in the order a change moves through them */
  environments: RuntimeEnvironment[];
  /** The environment whose values the tab shows */
  selectedEnvironment: RuntimeEnvironment;
  /** Chooses an environment */
  onSelect: (environmentId: string) => void;
  /** The environments the resource is not deployed to yet, which are marked as such */
  notDeployedIds?: string[];
  /** Where the deployment pipeline is, when there is one to link to */
  deployPath?: string;
}

/** The environment's color, as a dot. */
function EnvironmentDot({environment}: {environment: RuntimeEnvironment}): JSX.Element {
  return (
    <Box
      aria-hidden
      sx={(theme) => ({
        width: 8,
        height: 8,
        borderRadius: '50%',
        flexShrink: 0,
        bgcolor: environment.color?.light ?? 'text.secondary',
        ...theme.applyStyles('dark', {bgcolor: environment.color?.dark ?? 'text.secondary'}),
      })}
    />
  );
}

/**
 * A bar above a tab whose values differ between environments: the environment whose values the
 * whole tab shows, a way to choose another, and the way to the deployment pipeline.
 *
 * @public
 */
export default function EnvironmentBar({
  environments,
  selectedEnvironment,
  onSelect,
  notDeployedIds = [],
  deployPath = undefined,
}: EnvironmentBarProps): JSX.Element {
  const {t} = useTranslation();
  // A deploy enters at the first environment; each later one is reached by promoting.
  const notDeployedLabel = (environment: RuntimeEnvironment): string =>
    environments[0]?.id === environment.id
      ? t('applications:edit.overview.environments.notDeployed', 'Not deployed to {{environment}} yet.', {
          environment: environment.name,
        })
      : t('applications:edit.overview.environments.notPromoted', 'Not promoted to {{environment}} yet.', {
          environment: environment.name,
        });

  return (
    <Paper variant="outlined" sx={{px: 2, py: 1.25}}>
      <Stack direction="row" alignItems="center" flexWrap="wrap" columnGap={2} rowGap={1}>
        <Typography id="environment-bar-label" variant="overline" color="text.secondary">
          {t('applications:edit.overview.environments.title', 'Environment')}
        </Typography>
        <Select
          size="small"
          value={selectedEnvironment.id}
          onChange={(event) => onSelect(String(event.target.value))}
          SelectDisplayProps={{'aria-labelledby': 'environment-bar-label'}}
          renderValue={() => (
            <Stack direction="row" alignItems="center" spacing={1}>
              <EnvironmentDot environment={selectedEnvironment} />
              <Typography variant="body2" component="span" sx={{fontFamily: 'monospace', textTransform: 'uppercase'}}>
                {selectedEnvironment.name}
              </Typography>
            </Stack>
          )}
        >
          {environments.map((environment) => (
            <MenuItem key={environment.id} value={environment.id}>
              <Stack direction="row" alignItems="center" spacing={1.25}>
                <EnvironmentDot environment={environment} />
                <Box>
                  <Typography variant="body2" sx={{fontFamily: 'monospace', textTransform: 'uppercase'}}>
                    {environment.name}
                  </Typography>
                  {notDeployedIds.includes(environment.id) && (
                    <Typography variant="caption" color="text.secondary">
                      {notDeployedLabel(environment)}
                    </Typography>
                  )}
                </Box>
              </Stack>
            </MenuItem>
          ))}
        </Select>
        <Typography variant="body2" color="text.secondary" role="status" sx={{flex: '1 1 260px', minWidth: 0}}>
          {t(
            'applications:edit.overview.environments.scope',
            'The guide, client ID, and endpoints on this tab use {{environment}} values.',
            {environment: selectedEnvironment.name},
          )}
        </Typography>
        {deployPath && (
          <Button component={RouterLink} to={deployPath} size="small" endIcon={<ArrowRight size={14} aria-hidden />}>
            {t('applications:edit.overview.environments.deploymentPipeline', 'Deployment pipeline')}
          </Button>
        )}
      </Stack>
    </Paper>
  );
}
