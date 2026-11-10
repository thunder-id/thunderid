// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  gatewayColor,
  gatewaySectionOf,
  useDeployRoutes,
  usePipeline,
  type PipelineStage,
} from '@thunderid/configure-gateways';
import {Box, ComplexSelect, useAppShell} from '@wso2/oxygen-ui';
import {ExternalLink} from '@wso2/oxygen-ui-icons-react';
import type {ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import useOpenGatewayId from '../useOpenGatewayId';

/** The value of the entry that opens where gateways are managed, in place of a gateway. */
const MANAGE = '__manage__';

/** The dot a gateway is marked with, in its pipeline colour. */
function Dot({index}: {index: number}): ReactNode {
  return <Box aria-hidden sx={{width: 10, height: 10, borderRadius: '50%', bgcolor: gatewayColor(index).light}} />;
}

/**
 * The sidebar's gateway selector on a gateway's pages, as the design draws it: the gateway open, in its
 * pipeline colour, and each gateway to open instead, with where gateways are managed when the deployment
 * names it. Hidden while the sidebar shows icons alone.
 */
export default function GatewaySwitcher(): ReactNode {
  const {t} = useTranslation();
  const navigate = useNavigate();
  const routes = useDeployRoutes();
  const pipeline = usePipeline();
  const {state} = useAppShell();
  const stages: PipelineStage[] = pipeline.data?.stages ?? [];
  const openGatewayId = useOpenGatewayId();
  const manageUrl = window.__THUNDERID_RUNTIME_CONFIG__?.deploy?.manage_environments_url;

  const openIndex = stages.findIndex((stage: PipelineStage) => stage.gateway.id === openGatewayId);
  if (openIndex < 0 || state.sidebarCollapsed) {
    return null;
  }
  const open = stages[openIndex];

  const choose = (value: string): void => {
    if (value === MANAGE) {
      if (manageUrl) window.open(manageUrl, '_blank', 'noopener,noreferrer');
      return;
    }
    void navigate(gatewaySectionOf(routes, value, undefined, 'overview'));
  };

  return (
    <Box sx={{px: 1, pb: 1}}>
      <ComplexSelect
        value={open.gateway.id}
        size="small"
        fullWidth
        label={t('common:header.gateway', 'Gateway')}
        onChange={(event) => choose(String(event.target.value))}
        renderValue={() => (
          <Box sx={{display: 'flex', alignItems: 'center', gap: 1}}>
            <Dot index={openIndex} />
            <ComplexSelect.MenuItem.Text primary={open.name} />
          </Box>
        )}
        data-testid="gateway-switcher"
      >
        <ComplexSelect.ListHeader>{t('common:header.gateways', 'Gateways')}</ComplexSelect.ListHeader>
        {stages.map((stage: PipelineStage, index: number) => (
          <ComplexSelect.MenuItem key={stage.gateway.id} value={stage.gateway.id}>
            <ComplexSelect.MenuItem.Icon>
              <Dot index={index} />
            </ComplexSelect.MenuItem.Icon>
            <ComplexSelect.MenuItem.Text primary={stage.name} secondary={stage.gateway.name} />
          </ComplexSelect.MenuItem>
        ))}
        {manageUrl && <ComplexSelect.Divider />}
        {manageUrl && (
          <ComplexSelect.MenuItem value={MANAGE}>
            <ComplexSelect.MenuItem.Icon>
              <ExternalLink size={16} />
            </ComplexSelect.MenuItem.Icon>
            <ComplexSelect.MenuItem.Text primary={t('common:header.manageGateways', 'Manage gateways')} />
          </ComplexSelect.MenuItem>
        )}
      </ComplexSelect>
    </Box>
  );
}
