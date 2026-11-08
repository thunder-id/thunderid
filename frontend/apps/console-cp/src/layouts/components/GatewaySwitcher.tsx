// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  gatewayColor,
  gatewaySectionOf,
  useDeployRoutes,
  usePipeline,
  useProjectScope,
  useProjectStatuses,
  useVersions,
  type PipelineStage,
  type ProjectStatus,
} from '@thunderid/configure-gateways';
import {Box, Button, Divider, ListSubheader, Menu, MenuItem, Stack, Typography} from '@wso2/oxygen-ui';
import {ChevronDown} from '@wso2/oxygen-ui-icons-react';
import {useState, type MouseEvent, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import {useMatch, useNavigate} from 'react-router';
import RouteConfig, {ROUTE_SEGMENTS} from '../../configs/RouteConfig';

/** The dot a gateway is marked with, in its pipeline colour. */
function Dot({index}: {index: number}): ReactNode {
  return <Box sx={{width: 8, height: 8, borderRadius: '50%', bgcolor: gatewayColor(index).light, flexShrink: 0}} />;
}

/**
 * The header's gateway switcher on a gateway's pages: each gateway, to open one, and with a project
 * selected, the project's configuration and how each gateway stands with the project. It names the
 * gateway open.
 */
export default function GatewaySwitcher(): ReactNode {
  const {t} = useTranslation();
  const navigate = useNavigate();
  const routes = useDeployRoutes();
  const project = useProjectScope();
  const pipeline = usePipeline();
  const versions = useVersions();
  const stages: PipelineStage[] = pipeline.data?.stages ?? [];
  const statuses = useProjectStatuses(stages);
  const match = useMatch(`/${ROUTE_SEGMENTS.deployTargets}/:gatewayId/*`);
  const [anchor, setAnchor] = useState<HTMLElement | null>(null);

  if (stages.length === 0) {
    return null;
  }

  const openIndex = stages.findIndex((stage: PipelineStage) => stage.gateway.id === match?.params.gatewayId);
  const open = openIndex < 0 ? undefined : stages[openIndex];
  const latest = versions.data?.[0];
  const owes = (status: ProjectStatus | undefined): number =>
    status ? status.project.pending + status.project.required : 0;
  const notDeployed = owes(statuses.byGateway[stages[0].gateway.id]);
  const go = (path: string): void => {
    setAnchor(null);
    void navigate(path);
  };
  const standing = (stage: PipelineStage, index: number): string | undefined => {
    if (!project) return undefined;
    const status = statuses.byGateway[stage.gateway.id];
    const runs = status?.project.versions[0];
    const runsText = runs?.number ? `v${runs.number}` : t('common:header.runsNothing', 'Nothing deployed');
    const count = owes(status);
    if (!status) return runsText;
    if (count === 0) return `${runsText} · ${t('common:header.upToDate', 'Up to date')}`;
    return `${runsText} · ${
      index === 0
        ? t('common:header.changesBehind', {
            count,
            defaultValue_one: '{{count}} change behind',
            defaultValue_other: '{{count}} changes behind',
          })
        : t('common:header.changesToPromote', {
            count,
            defaultValue_one: '{{count}} change to promote',
            defaultValue_other: '{{count}} changes to promote',
          })
    }`;
  };

  return (
    <>
      <Button
        variant="text"
        size="small"
        color="inherit"
        startIcon={open ? <Dot index={openIndex} /> : undefined}
        endIcon={<ChevronDown size={16} />}
        onClick={(event: MouseEvent<HTMLElement>) => setAnchor(event.currentTarget)}
        aria-label={t('common:header.switchGateway', 'Switch gateway')}
        aria-haspopup="menu"
        aria-expanded={Boolean(anchor)}
        sx={{mr: 1, fontFamily: open ? 'monospace' : undefined}}
        data-testid="gateway-switcher"
      >
        {open ? open.name.toUpperCase() : t('common:header.projectConfiguration', 'Project configuration')}
      </Button>
      <Menu anchorEl={anchor} open={Boolean(anchor)} onClose={() => setAnchor(null)}>
        {project && <ListSubheader>{t('common:header.configure', 'Configure')}</ListSubheader>}
        {project && (
          <MenuItem onClick={() => go(RouteConfig.projects.home(project))}>
            <Stack>
              <Typography variant="body2">
                {t('common:header.projectConfiguration', 'Project configuration')}
              </Typography>
              <Typography variant="caption" color="text.secondary">
                {latest?.number
                  ? t('common:header.configVersion', {
                      version: `v${latest.number}`,
                      count: notDeployed,
                      defaultValue_one: 'Config {{version}} · {{count}} change not deployed',
                      defaultValue_other: 'Config {{version}} · {{count}} changes not deployed',
                    })
                  : t('common:header.configNoVersion', 'Nothing captured yet')}
              </Typography>
            </Stack>
          </MenuItem>
        )}
        {project && <Divider />}
        <ListSubheader>{t('common:header.gateways', 'Gateways')}</ListSubheader>
        {stages.map((stage: PipelineStage, index: number) => (
          <MenuItem
            key={stage.gateway.id}
            selected={stage.gateway.id === open?.gateway.id}
            onClick={() => go(gatewaySectionOf(routes, stage.gateway.id, undefined, 'overview'))}
          >
            <Stack direction="row" spacing={1.5} alignItems="center">
              <Dot index={index} />
              <Stack>
                <Typography variant="body2">
                  <Box component="span" sx={{fontFamily: 'monospace', mr: 1}}>
                    {stage.name.toUpperCase()}
                  </Box>
                </Typography>
                <Typography variant="caption" color="text.secondary">
                  {standing(stage, index)}
                </Typography>
              </Stack>
            </Stack>
          </MenuItem>
        ))}
      </Menu>
    </>
  );
}
