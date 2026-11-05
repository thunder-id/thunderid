// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Button, Tooltip} from '@wso2/oxygen-ui';
import {Rocket} from '@wso2/oxygen-ui-icons-react';
import {useMemo, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import useAppliedVersions from '../api/useAppliedVersions';
import {useApplyCheck} from '../api/useGatewayVersions';
import {usePipeline} from '../api/usePipeline';
import {useVersions} from '../api/useVersions';
import useDeployPanels from '../hooks/useDeployPanels';
import useDeployRoutes from '../hooks/useDeployRoutes';
import type {Change, Version} from '../models/deploy';
import type {Gateway} from '../models/gateway';
import type {PipelineStage} from '../models/pipeline';

/**
 * The console header's Deploy: how many changes saved here the first environment lacks, and a click that
 * opens the deploy of the latest version to it. Without an environment it goes to the Deploy page.
 */
export default function DeployHeaderButton(): JSX.Element {
  const {t} = useTranslation('deploy');
  const navigate = useNavigate();
  const routes = useDeployRoutes();
  const pipeline = usePipeline();
  const versions = useVersions();
  const stages: PipelineStage[] = useMemo(() => pipeline.data?.stages ?? [], [pipeline.data]);
  const gateways: Gateway[] = useMemo(() => stages.map((stage: PipelineStage) => stage.gateway), [stages]);
  const applied = useAppliedVersions(gateways.map((gateway: Gateway) => gateway.id));
  const all: Version[] = useMemo(() => versions.data ?? [], [versions.data]);
  const panels = useDeployPanels(gateways, all, applied.byGateway, stages);
  const first = stages[0];
  const latest = all[0];
  const check = useApplyCheck(first?.gateway.id ?? '', latest?.version);
  // What deploying the latest version would change there, less what an earlier deploy left out on purpose.
  const waiting = (check.data?.diff.changes ?? []).filter(
    (change: Change) => change.change !== 'unchanged' && !change.heldBack,
  ).length;

  const open = (): void => {
    if (first && latest) panels.deploy(latest, first.gateway.id, undefined, [first.gateway]);
    else void navigate(routes.deploy.list());
  };
  const label =
    waiting > 0 ? t('header.deployCount', 'Deploy {{count}} changes', {count: waiting}) : t('header.title', 'Deploy');
  return (
    <>
      <Tooltip
        title={
          first && waiting > 0
            ? t('header.waiting', '{{count}} saved changes are not deployed to {{target}} yet.', {
                count: waiting,
                target: first.name,
              })
            : ''
        }
      >
        <Box component="span">
          <Button
            variant="contained"
            size="small"
            startIcon={<Rocket size={16} />}
            onClick={open}
            sx={{borderRadius: 100, mr: 1}}
            data-testid="header-deploy"
            data-waiting={waiting}
          >
            {label}
          </Button>
        </Box>
      </Tooltip>
      {panels.panels}
    </>
  );
}
