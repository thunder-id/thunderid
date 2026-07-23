/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

import {Stack, Typography} from '@wso2/oxygen-ui';
import {useMemo, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import CheckboxWithHint from './CheckboxWithHint';
import type {CommonResourcePropertiesPropsInterface} from './types';
import PresentationDefinitionSelect from '@/features/flows/components/resource-property-panel/PresentationDefinitionSelect';
import type {StepData} from '@/features/flows/models/steps';

/**
 * Properties editor for the OpenID4VP verifier executor: the presentation
 * definition (by handle) requested from the wallet, and whether holders without a
 * pre-existing local user are provisioned just-in-time.
 */
function OpenID4VPProperties({resource, onChange}: CommonResourcePropertiesPropsInterface): ReactNode {
  const {t} = useTranslation();

  const properties = useMemo(() => {
    const stepData = resource?.data as StepData | undefined;
    return stepData?.properties ?? {};
  }, [resource]);

  return (
    <Stack gap={2}>
      <Typography variant="body2" color="text.secondary">
        {t('flows:core.executions.openid4vp.description')}
      </Typography>
      <PresentationDefinitionSelect
        propertyKey="presentation_definition_id"
        value={(properties.presentation_definition_id as string) ?? ''}
        onChange={(value: string) => onChange('data.properties.presentation_definition_id', value, resource)}
      />
      <CheckboxWithHint
        checked={!!properties.allowAuthenticationWithoutLocalUser}
        onChange={(checked) => onChange('data.properties.allowAuthenticationWithoutLocalUser', checked, resource)}
        label={t(
          'flows:core.executions.openid4vp.allowAuthenticationWithoutLocalUser.label',
          'Allow authentication without a local user',
        )}
        hint={t(
          'flows:core.executions.openid4vp.allowAuthenticationWithoutLocalUser.hint',
          'When enabled, a holder with no matching local user is provisioned just-in-time. When disabled, login requires an existing matching user.',
        )}
      />
    </Stack>
  );
}

export default OpenID4VPProperties;
