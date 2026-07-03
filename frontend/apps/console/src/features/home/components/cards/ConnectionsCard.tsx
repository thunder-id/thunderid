/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

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
