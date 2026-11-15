// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Button, Stack, Typography} from '@wso2/oxygen-ui';
import {ArrowRight, TriangleAlert} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {Link} from 'react-router';

/**
 * Props for the {@link EnvironmentNotDeployedNotice} component.
 *
 * @public
 */
export interface EnvironmentNotDeployedNoticeProps {
  /** The name of the application or agent that is not in the environment. */
  resourceName: string;
  environmentName: string;
  /** The environment it is promoted from. Left out for the first environment, which a deploy enters at. */
  previousEnvironmentName?: string;
  /** Where configuration is deployed to the environments. */
  deployPath?: string;
}

/**
 * Says that a resource is not in an environment yet, so it has no client ID or endpoints there, and
 * offers the way to get it there.
 *
 * @public
 */
export default function EnvironmentNotDeployedNotice({
  resourceName,
  environmentName,
  previousEnvironmentName = undefined,
  deployPath = undefined,
}: EnvironmentNotDeployedNoticeProps): JSX.Element {
  const {t} = useTranslation();

  return (
    // It takes the place of values that were shown, often after the page is first read, so it is announced.
    <Stack
      role="status"
      direction="row"
      spacing={1.5}
      sx={{
        px: 2,
        py: 1.5,
        border: 1,
        borderColor: 'warning.main',
        borderStyle: 'dashed',
        borderRadius: 1,
        bgcolor: 'action.hover',
        color: 'warning.main',
      }}
    >
      <Box sx={{display: 'flex', flexShrink: 0, pt: 0.25}}>
        <TriangleAlert size={18} aria-hidden />
      </Box>
      <Box sx={{minWidth: 0}}>
        <Typography variant="body2" color="inherit">
          <Box component="strong" sx={{fontWeight: 600, mr: 1}}>
            {previousEnvironmentName
              ? t('applications:edit.overview.environments.notPromotedTitle', 'Not promoted to {{environment}} yet', {
                  environment: environmentName,
                })
              : t('applications:edit.overview.environments.notDeployedTitle', 'Not deployed to {{environment}} yet', {
                  environment: environmentName,
                })}
          </Box>
          {previousEnvironmentName
            ? t(
                'applications:edit.overview.environments.notPromotedDetail',
                'There is no client ID or endpoints for {{name}} here. Promote it from {{previous}} to get them.',
                {name: resourceName, previous: previousEnvironmentName},
              )
            : t(
                'applications:edit.overview.environments.notDeployedDetail',
                'There is no client ID or endpoints for {{name}} here. Deploy it to {{environment}} to get them.',
                {name: resourceName, environment: environmentName},
              )}
        </Typography>
        {deployPath && (
          <Button
            component={Link}
            to={deployPath}
            variant="outlined"
            color="warning"
            size="small"
            endIcon={<ArrowRight size={16} aria-hidden />}
            sx={{mt: 1.25}}
          >
            {t('applications:edit.overview.environments.openPipeline', 'Open deployment pipeline')}
          </Button>
        )}
      </Box>
    </Stack>
  );
}
