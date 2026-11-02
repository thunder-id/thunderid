// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Button, Chip, CircularProgress, IconButton, Stack, Tooltip, Typography} from '@wso2/oxygen-ui';
import {ChevronRight, Rocket} from '@wso2/oxygen-ui-icons-react';
import {useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import ActorName from './ActorName';
import DeployCard from './DeployCard';
import Timestamp from './Timestamp';
import VersionName from './VersionName';
import type {Version} from '../models/deploy';

export interface VersionsCardProps {
  versions: Version[];
  isLoading?: boolean;
  /** Opens the deploy of a version; absent, the rows have no button. */
  onDeploy?: (version: Version) => void;
  /** The version whose deploy is open, drawn as pressed. */
  deploying?: string;
  /** The names of the targets each version is deployed to, by the version's key. */
  runners?: Record<string, string[]>;
  /** Every target's name, in path order, so each is tagged in a colour of its own. */
  targets?: string[];
  /**
   * The names of the targets a Deploy from this card can reach; every target unless given. Where
   * configuration is promoted along a path, only the first target takes a deploy.
   */
  deployTargets?: string[];
}

/** The colours targets are tagged in, by their place along the path. */
const TARGET_COLORS = ['#8b5cf6', '#14b8a6', '#f59e0b', '#ec4899', '#0ea5e9', '#84cc16'];

/** How many of the newest versions are listed before the rest are folded away. */
const SHOWN = 5;

function RowChip({label, color}: {label: string; color: string}): JSX.Element {
  return (
    <Chip
      size="small"
      label={label}
      icon={<Box component="span" sx={{width: 6, height: 6, borderRadius: '50%', bgcolor: color, ml: 1}} />}
      sx={{
        height: 20,
        fontSize: 11,
        borderRadius: 100,
        bgcolor: 'action.hover',
        color: 'text.secondary',
        '& .MuiChip-icon': {ml: 0.75, mr: -0.25},
      }}
    />
  );
}

function DeployedChip({name, color}: {name: string; color: string}): JSX.Element {
  return (
    <Chip
      size="small"
      variant="outlined"
      label={name}
      data-testid="version-deployed-to"
      data-color={color}
      sx={{height: 20, fontSize: 11, borderRadius: 100, color, borderColor: color, bgcolor: `${color}14`}}
    />
  );
}

/**
 * The captured versions, newest first, one open at a time. The newest is set apart and open to begin
 * with, saying when it was captured, who it was captured for, and to deploy it; the others are a line
 * each, and one opens in place, plainly, closing the other. Each names where it is deployed. The newest few
 * are listed with every deployed version, and the rest are a click away.
 */
export default function VersionsCard({
  versions,
  isLoading = false,
  onDeploy = undefined,
  deploying = undefined,
  runners = {},
  targets = [],
  deployTargets = undefined,
}: VersionsCardProps): JSX.Element {
  const {t} = useTranslation('deploy');
  // One version is open at a time, the newest until another is opened; closing it leaves none open.
  const [opened, setOpened] = useState<string | null | undefined>(undefined);
  const openVersion = opened === undefined ? versions[0]?.version : opened;
  const toggle = (key: string): void => setOpened(openVersion === key ? null : key);
  const [showAll, setShowAll] = useState(false);
  const listed = versions.filter(
    (version: Version, index: number) =>
      showAll || index < SHOWN || (runners[version.version]?.length ?? 0) > 0 || version.version === openVersion,
  );
  const folded = versions.length - listed.length;

  // A version that every target a deploy can reach already runs has nowhere to go, so its Deploy is
  // disabled and says where it runs.
  const reachable = deployTargets ?? targets;
  const deployButton = (version: Version, deployedTo: string[]): JSX.Element | null => {
    if (!onDeploy) return null;
    const everywhere = reachable.length > 0 && reachable.every((name: string) => deployedTo.includes(name));
    const button = (
      <Button
        variant="contained"
        size="small"
        startIcon={<Rocket size={14} />}
        onClick={(event) => {
          // The row opens and closes on a click; Deploy is not that click.
          event.stopPropagation();
          onDeploy(version);
        }}
        disabled={everywhere}
        aria-pressed={deploying === version.version}
        sx={{borderRadius: 100, flexShrink: 0, fontSize: 12}}
      >
        {t('versions.deployRow', 'Deploy')}
      </Button>
    );
    if (!everywhere) return button;
    return (
      <Tooltip
        title={t('versions.alreadyDeployed', 'Already deployed to {{targets}}', {targets: reachable.join(', ')})}
      >
        {/* A disabled button takes no hover, so the hint is held by what wraps it. */}
        <Box component="span" onClick={(event) => event.stopPropagation()} sx={{flexShrink: 0}}>
          {button}
        </Box>
      </Tooltip>
    );
  };

  return (
    <DeployCard
      title={t('versions.title', 'Configuration versions')}
      description={t('versions.description', 'Captured from the configuration, newest first.')}
      testId="deploy-versions"
    >
      {isLoading && (
        <Box sx={{display: 'flex', justifyContent: 'center', py: 4}}>
          <CircularProgress size={22} />
        </Box>
      )}
      {!isLoading && versions.length === 0 && (
        <Typography variant="body2" color="text.secondary" sx={{px: 3, py: 3}}>
          {t('versions.empty', 'No version has been captured yet.')}
        </Typography>
      )}
      {listed.map((version: Version) => {
        const index = versions.indexOf(version);
        const number = version.number ?? versions.length - index;
        const deployedTo = runners[version.version] ?? [];
        const colorOf = (name: string): string => {
          const place = targets.indexOf(name);
          return TARGET_COLORS[(place < 0 ? targets.length : place) % TARGET_COLORS.length];
        };
        const tags = deployedTo.map((name: string) => <DeployedChip key={name} name={name} color={colorOf(name)} />);
        const hash = version.hash ?? version.version;
        const isOpen = openVersion === version.version;
        const marker =
          index === 0 ? {name: t('versions.latest', 'Latest'), color: 'var(--oxygen-palette-success-main)'} : undefined;
        const chevron = (
          <IconButton
            size="small"
            aria-expanded={isOpen}
            aria-label={t('versions.details', 'Details of v{{number}}', {number})}
            onClick={(event) => {
              event.stopPropagation();
              toggle(version.version);
            }}
          >
            <Box
              component="span"
              sx={{display: 'flex', transform: isOpen ? 'rotate(90deg)' : 'none', transition: 'transform 0.2s'}}
            >
              <ChevronRight size={14} />
            </Box>
          </IconButton>
        );
        return (
          <Box
            key={version.version}
            data-testid={`deploy-version-${version.version}`}
            data-latest={index === 0 ? 'true' : undefined}
            data-open={isOpen ? 'true' : undefined}
            // The whole row opens and closes the version, not only its chevron.
            onClick={() => toggle(version.version)}
            sx={{
              borderTop: 1,
              borderColor: 'divider',
              cursor: 'pointer',
              px: isOpen ? 1.25 : 0,
              py: isOpen ? 1.25 : 0,
              ...(isOpen ? {} : {'&:hover': {bgcolor: 'action.hover'}}),
            }}
          >
            {isOpen ? (
              // The open version is set apart: its name and marker, where it is deployed, when it was
              // captured and for whom, and a prominent Deploy.
              <Box
                sx={{
                  border: 1,
                  borderColor: 'rgba(var(--oxygen-palette-primary-mainChannel) / 0.45)',
                  borderRadius: '8px',
                  bgcolor: 'rgba(var(--oxygen-palette-primary-mainChannel) / 0.05)',
                  pl: 0.5,
                  pr: 2,
                  py: 1.5,
                }}
              >
                <Stack direction="row" alignItems="flex-start" spacing={1}>
                  {chevron}
                  <Stack spacing={1} sx={{minWidth: 0, flex: 1, pt: 0.25}}>
                    <Stack direction="row" alignItems="center" spacing={1} sx={{flexWrap: 'wrap', rowGap: 0.5}}>
                      <VersionName number={number} hash={hash} variant="subtitle1" />
                      {marker && <RowChip label={marker.name} color={marker.color} />}
                      {tags}
                    </Stack>
                    <Timestamp value={version.createdAt} />
                    {version.createdBy && <ActorName actor={version.createdBy} />}
                    {version.name && (
                      <Typography variant="body2" color="text.secondary">
                        {version.name}
                      </Typography>
                    )}
                  </Stack>
                  {deployButton(version, deployedTo)}
                </Stack>
              </Box>
            ) : (
              <Stack direction="row" alignItems="center" spacing={1} sx={{py: 1, pl: 1.5, pr: 2.25}}>
                {chevron}
                <VersionName number={number} hash={hash} />
                {marker && <RowChip label={marker.name} color={marker.color} />}
                {tags}
                <Box sx={{flex: 1, minWidth: 0}}>
                  <Timestamp value={version.createdAt} dateOnly />
                </Box>
                {version.createdBy && <ActorName actor={version.createdBy} avatarOnly />}
              </Stack>
            )}
          </Box>
        );
      })}
      {(folded > 0 || showAll) && versions.length > SHOWN && (
        <Box sx={{borderTop: 1, borderColor: 'divider', px: 2, py: 0.75}}>
          <Button
            variant="text"
            size="small"
            color="inherit"
            onClick={() => setShowAll(!showAll)}
            sx={{fontSize: 12, color: 'text.secondary'}}
            data-testid="versions-show-all"
          >
            {showAll
              ? t('versions.showFewer', 'Show fewer')
              : t('versions.showOlder', 'Show {{count}} older', {count: folded})}
          </Button>
        </Box>
      )}
    </DeployCard>
  );
}
