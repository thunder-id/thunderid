// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useLogger} from '@thunderid/logger/react';
import {Box, Button, PageContent, PageTitle, Tab, Tabs} from '@wso2/oxygen-ui';
import {Camera, Plus} from '@wso2/oxygen-ui-icons-react';
import {useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate, useSearchParams} from 'react-router';
import CaptureVersionDialog from '../components/CaptureVersionDialog';
import ConfigurationVersionsList from '../components/ConfigurationVersionsList';
import GatewaysList from '../components/GatewaysList';
import RegisterGatewayDialog from '../components/RegisterGatewayDialog';
import useGatewayRoutes from '../hooks/useGatewayRoutes';
import type {GatewayRegistration} from '../models/gateway';

const TAB_GATEWAYS = 'gateways';
const TAB_VERSIONS = 'versions';

export default function GatewaysListPage(): JSX.Element {
  const {t} = useTranslation();
  const navigate = useNavigate();
  const routes = useGatewayRoutes();
  const logger = useLogger('GatewaysListPage');
  const [searchParams, setSearchParams] = useSearchParams();
  const tab = searchParams.get('tab') === TAB_VERSIONS ? TAB_VERSIONS : TAB_GATEWAYS;
  const [registerOpen, setRegisterOpen] = useState(false);
  const [captureOpen, setCaptureOpen] = useState(false);

  const handleRegistered = (gateway: GatewayRegistration): void => {
    setRegisterOpen(false);
    (async (): Promise<void> => {
      await navigate(routes.detail(gateway.id));
    })().catch((err: unknown) => {
      logger.error('Failed to navigate to gateway detail', {error: err});
    });
  };

  return (
    <PageContent>
      <PageTitle>
        <PageTitle.Header>{t('gateways:list.title', 'Gateway Management')}</PageTitle.Header>
        <PageTitle.SubHeader>
          {t(
            'gateways:list.subtitle',
            'Register the gateways this deployment administers, capture versions of its configuration, and apply them.',
          )}
        </PageTitle.SubHeader>
        <PageTitle.Actions>
          {tab === TAB_VERSIONS ? (
            <Button variant="contained" startIcon={<Camera size={18} />} onClick={() => setCaptureOpen(true)}>
              {t('gateways:capture.action', 'Capture current configuration')}
            </Button>
          ) : (
            <Button variant="contained" startIcon={<Plus size={18} />} onClick={() => setRegisterOpen(true)}>
              {t('gateways:list.register', 'Register gateway')}
            </Button>
          )}
        </PageTitle.Actions>
      </PageTitle>

      <Tabs
        value={tab}
        onChange={(_event: unknown, value: string) => {
          setSearchParams(value === TAB_GATEWAYS ? {} : {tab: value});
        }}
        aria-label={t('gateways:list.tabs.ariaLabel', 'Gateway management sections')}
      >
        <Tab value={TAB_GATEWAYS} label={t('gateways:list.tabs.gateways', 'Gateways')} />
        <Tab value={TAB_VERSIONS} label={t('gateways:list.tabs.versions', 'Configuration versions')} />
      </Tabs>

      <Box sx={{pt: 3}}>{tab === TAB_VERSIONS ? <ConfigurationVersionsList /> : <GatewaysList />}</Box>

      <RegisterGatewayDialog
        open={registerOpen}
        onClose={() => setRegisterOpen(false)}
        onRegistered={handleRegistered}
      />
      <CaptureVersionDialog open={captureOpen} onClose={() => setCaptureOpen(false)} />
    </PageContent>
  );
}
