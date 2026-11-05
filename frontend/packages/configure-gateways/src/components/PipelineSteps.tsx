// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Stack, Typography} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import type {PipelineStage} from '../models/pipeline';

export interface PipelineStepsProps {
  stages: PipelineStage[];
  /** The gateway this deploy goes to. */
  targetId: string;
  /** The label of the version each environment runs, by gateway id; empty while it runs none. */
  runs?: Record<string, string>;
}

/** The environments in order, with the one a deploy goes to marked, and what each later one waits for. */
export default function PipelineSteps({stages, targetId, runs = {}}: PipelineStepsProps): JSX.Element {
  const {t} = useTranslation('deploy');
  const at = stages.findIndex((stage: PipelineStage) => stage.gateway.id === targetId);
  return (
    <Stack spacing={1} component="ol" sx={{m: 0, p: 0, listStyle: 'none'}} data-testid="pipeline-steps">
      {stages.map((stage: PipelineStage, index: number) => {
        const here = stage.gateway.id === targetId;
        const runsHere = runs[stage.gateway.id];
        let note = runsHere
          ? t('pipelineSteps.runs', 'Runs {{version}}', {version: runsHere})
          : t('pipelineSteps.runsNothing', 'Runs nothing yet');
        if (index > at && at >= 0)
          note = t('pipelineSteps.later', '{{runs}} · promote once {{before}} is verified', {
            runs: note,
            before: stages[index - 1].name,
          });
        return (
          <Stack
            key={stage.gateway.id}
            component="li"
            direction="row"
            spacing={1.5}
            alignItems="center"
            data-current={here ? 'true' : undefined}
            sx={{
              px: 1.5,
              py: 1,
              border: 1,
              borderRadius: 2,
              borderColor: here ? 'primary.main' : 'divider',
              bgcolor: here ? 'rgba(var(--oxygen-palette-primary-mainChannel) / 0.08)' : undefined,
            }}
          >
            <Box
              sx={{
                width: 24,
                height: 24,
                borderRadius: '50%',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                fontSize: 12,
                fontWeight: 600,
                bgcolor: here ? 'primary.main' : undefined,
                color: here ? 'primary.contrastText' : 'text.secondary',
                border: here ? 0 : 1,
                borderColor: 'divider',
              }}
              aria-hidden
            >
              {index + 1}
            </Box>
            <Box sx={{flex: 1, minWidth: 0}}>
              <Typography variant="body2" fontWeight={600}>
                {stage.name}
              </Typography>
              <Typography variant="caption" color="text.secondary">
                {note}
              </Typography>
            </Box>
            {here && (
              <Typography variant="caption" color="primary.main" fontWeight={600}>
                {t('pipelineSteps.thisDeploy', 'This deploy')}
              </Typography>
            )}
          </Stack>
        );
      })}
    </Stack>
  );
}
