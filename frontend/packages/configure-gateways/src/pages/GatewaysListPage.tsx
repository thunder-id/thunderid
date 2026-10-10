// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useLogger} from '@thunderid/logger/react';
import {Button, PageContent, PageTitle} from '@wso2/oxygen-ui';
import {Plus} from '@wso2/oxygen-ui-icons-react';
import {useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import GatewaysList from '../components/GatewaysList';
import RegisterGatewayDialog from '../components/RegisterGatewayDialog';
import useGatewayRoutes from '../hooks/useGatewayRoutes';
import type {GatewayRegistration} from '../models/gateway';

export default function GatewaysListPage(): JSX.Element {
  const {t} = useTranslation();
  const navigate = useNavigate();
  const routes = useGatewayRoutes();
  const logger = useLogger('GatewaysListPage');
  const [registerOpen, setRegisterOpen] = useState(false);

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
          {t('gateways:list.subtitle', 'Register the gateways this deployment administers and how it reaches them.')}
        </PageTitle.SubHeader>
        <PageTitle.Actions>
          <Button variant="contained" startIcon={<Plus size={18} />} onClick={() => setRegisterOpen(true)}>
            {t('gateways:list.register', 'Register gateway')}
          </Button>
        </PageTitle.Actions>
      </PageTitle>

      <GatewaysList />

      <RegisterGatewayDialog
        open={registerOpen}
        onClose={() => setRegisterOpen(false)}
        onRegistered={handleRegistered}
      />
    </PageContent>
  );
}
