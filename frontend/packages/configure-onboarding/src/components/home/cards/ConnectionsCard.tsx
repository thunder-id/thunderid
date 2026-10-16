// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useConfig} from '@thunderid/contexts';
import {Layers} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import HomeNextStepCard from './HomeNextStepCard';

export default function ConnectionsCard(): JSX.Element {
  const {t} = useTranslation('home');
  const {config} = useConfig();
  const {product_name: productName} = config.brand || {};

  return (
    <HomeNextStepCard
      icon={<Layers size={24} />}
      title={t('next_steps.connections.title', 'Connections')}
      description={t('next_steps.connections.description', {
        product: productName,
        defaultValue:
          'Manage the external services {{product}} connects to for social login, enterprise OIDC, SMS delivery, and more.',
      })}
      primaryLabel={t('next_steps.connections.actions.primary.label', 'Manage Connections')}
      primaryRoute="/connections"
    />
  );
}
