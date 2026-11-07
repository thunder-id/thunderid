// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Button, CircularProgress, Dialog, DialogContent, DialogTitle, Stack, Typography} from '@wso2/oxygen-ui';
import {useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import GatewayTag, {type VersionRunner} from './GatewayTag';
import useProjectScope from '../contexts/ProjectScope';
import type {Version} from '../models/deploy';
import {versionLabel} from '../utils/format';
import relativeTime from '../utils/relativeTime';

/** How many versions show before the rest are asked for. */
const SHOWN = 5;

export interface PipelineVersionsProps {
  /** The captured versions, newest first. */
  versions: Version[];
  isLoading?: boolean;
  /** The gateways that run each version, by its hash. */
  runners: Record<string, VersionRunner[]>;
  /** The version the first gateway runs, if any: newer ones can be deployed. */
  firstRuns?: string;
  /** The first gateway, which a deploy enters at. */
  firstName?: string;
  /** Opens a deploy of a version to the first gateway. */
  onDeploy: (version: Version) => void;
  /** The version a deploy is open for. */
  deploying?: string;
  /** How many changes the latest version brings to the first gateway, when known. */
  latestChanges?: number;
}

/**
 * The captured versions, newest first, each either ready to deploy to the first gateway or marked
 * with the gateways that run it.
 */
export default function PipelineVersions({
  versions,
  isLoading = false,
  runners,
  firstRuns = undefined,
  firstName = undefined,
  onDeploy,
  deploying = undefined,
  latestChanges = undefined,
}: PipelineVersionsProps): JSX.Element {
  const {t} = useTranslation('deploy');
  const project = useProjectScope();
  const [all, setAll] = useState(false);
  const firstIndex = firstRuns ? versions.findIndex((version: Version) => version.version === firstRuns) : -1;
  const shown = versions.slice(0, SHOWN);

  const rowOf = (version: Version, index: number): JSX.Element => {
    const ran = runners[version.version] ?? [];
    // Any version can be deployed to the first gateway, an older one to go back to it; the latest
    // stands out while the first gateway does not run it.
    const deployable = Boolean(firstName);
    // The latest's Deploy is always the main one. Its row stands out too while it brings changes to the
    // first gateway: in a project, the project's own changes, since a version can differ in another
    // project's resources alone.
    const waiting = project ? (latestChanges ?? 0) > 0 : firstIndex !== 0;
    const leading = index === 0 && deployable && waiting;
    const when = relativeTime(version.createdAt);
    const caption =
      index === 0
        ? [
            t('pipeline.latest', 'Latest · {{when}}', {when}),
            latestChanges
              ? t('pipeline.changeCount', {
                  count: latestChanges,
                  defaultValue_one: '{{count}} change',
                  defaultValue_other: '{{count}} changes',
                })
              : undefined,
          ]
            .filter(Boolean)
            .join(' · ')
        : when;
    return (
      <Stack
        key={version.version}
        direction="row"
        alignItems="center"
        spacing={1.25}
        data-testid={`pipeline-version-${version.version}`}
        sx={{
          px: 1.5,
          py: 1.25,
          borderRadius: '10px',
          border: 1,
          borderColor: leading ? 'primary.main' : 'divider',
          bgcolor: leading ? 'action.selected' : undefined,
        }}
      >
        <Box sx={{flex: 1, minWidth: 0}}>
          <Typography sx={{fontFamily: 'monospace', fontSize: 14}}>
            {versionLabel(version.version, version.number)}
          </Typography>
          <Typography variant="caption" color="text.secondary" component="div" noWrap>
            {caption}
          </Typography>
        </Box>
        {ran.length > 0 && (
          <Stack direction="row" spacing={0.5} sx={{flexWrap: 'wrap', justifyContent: 'flex-end'}}>
            {ran.map((runner: VersionRunner) => (
              <GatewayTag key={runner.name} name={runner.name} index={runner.index} />
            ))}
          </Stack>
        )}
        {deployable && (
          <Button
            size="small"
            variant={index === 0 && deployable ? 'contained' : 'outlined'}
            onClick={() => {
              setAll(false);
              onDeploy(version);
            }}
            disabled={deploying === version.version}
            sx={{borderRadius: 100, flexShrink: 0}}
          >
            {t('pipeline.deploy', 'Deploy')}
          </Button>
        )}
      </Stack>
    );
  };

  return (
    <Stack spacing={1.25} sx={{flex: '1 1 300px', minWidth: 0}} data-testid="pipeline-versions">
      <Typography variant="overline" color="text.secondary" sx={{fontWeight: 600, letterSpacing: '0.08em'}}>
        {t('pipeline.versionsLabel', 'Versions · ready to deploy')}
      </Typography>
      <Stack
        spacing={1.25}
        sx={{
          flex: 1,
          p: 2.5,
          borderRadius: '12px',
          border: 1,
          borderColor: 'divider',
          bgcolor: 'background.paper',
        }}
      >
        <Box sx={{mb: 0.5}}>
          <Typography variant="h6" fontWeight={500}>
            {project
              ? t('pipeline.projectVersionsTitle', 'Project versions')
              : t('pipeline.versionsTitle', 'Configuration versions')}
          </Typography>
          <Typography variant="body2" color="text.secondary">
            {firstName && project
              ? t(
                  'pipeline.projectVersionsHint',
                  'Every save creates a version. Deploys always enter the pipeline at {{first}}.',
                  {
                    first: firstName,
                  },
                )
              : firstName
                ? t('pipeline.versionsHint', 'Every change is captured as a version. A deploy enters at {{first}}.', {
                    first: firstName,
                  })
                : t('pipeline.versionsHintNone', 'Every change is captured as a version.')}
          </Typography>
        </Box>
        {isLoading && (
          <Box sx={{display: 'flex', justifyContent: 'center', py: 2}}>
            <CircularProgress size={20} />
          </Box>
        )}
        {!isLoading && versions.length === 0 && (
          <Typography variant="body2" color="text.secondary">
            {t('pipeline.noVersions', 'Nothing is captured yet. Make a change to capture the first version.')}
          </Typography>
        )}
        {shown.map(rowOf)}
        {versions.length > SHOWN && (
          <Button variant="text" size="small" onClick={() => setAll(true)} sx={{alignSelf: 'flex-start', px: 0}}>
            {t('pipeline.allVersions', 'View all versions')}
          </Button>
        )}
      </Stack>
      <Dialog open={all} onClose={() => setAll(false)} fullWidth maxWidth="sm">
        <DialogTitle>{t('pipeline.allVersionsTitle', 'All versions')}</DialogTitle>
        <DialogContent>
          <Stack spacing={1.25} sx={{pt: 1}} data-testid="pipeline-all-versions">
            {versions.map(rowOf)}
          </Stack>
        </DialogContent>
      </Dialog>
    </Stack>
  );
}
