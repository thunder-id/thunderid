// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Stack, Typography} from '@wso2/oxygen-ui';
import type {ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import CheckboxWithHint from './CheckboxWithHint';
import type {CommonResourcePropertiesPropsInterface} from './types';
import VariableBinding from './VariableBinding';
import {BOOLEAN_VARIABLES, TEXT_VARIABLES} from './variableBindingTypes';
import useResourceFieldError from '../../../../hooks/useResourceFieldError';
import PresentationDefinitionSelect from '../../PresentationDefinitionSelect';

/**
 * Properties editor for the OpenID4VP verifier executor: the presentation
 * definition (by handle) requested from the wallet, and whether holders without a
 * pre-existing local user are provisioned just-in-time.
 */
function OpenID4VPProperties({resource, onChange}: CommonResourcePropertiesPropsInterface): ReactNode {
  const {t} = useTranslation();

  const errorMessage: string = useResourceFieldError(resource?.id, 'data.properties.presentation_definition_id');

  return (
    <Stack gap={2}>
      <Typography variant="body2" color="text.secondary">
        {t('flows:core.executions.openid4vp.description')}
      </Typography>
      <VariableBinding
        resource={resource}
        onChange={onChange}
        propertyKey="presentation_definition_id"
        accepts={TEXT_VARIABLES}
        htmlFor="presentation_definition_id"
        label={t('verifiable-presentations:select.label')}
      >
        {({commit, value: stored}) => (
          <PresentationDefinitionSelect
            propertyKey="presentation_definition_id"
            value={(stored as string) ?? ''}
            onChange={(value: string) => commit(value)}
            errorMessage={errorMessage}
          />
        )}
      </VariableBinding>
      <VariableBinding
        inline
        resource={resource}
        onChange={onChange}
        propertyKey="allowAuthenticationWithoutLocalUser"
        accepts={BOOLEAN_VARIABLES}
        label={t(
          'flows:core.executions.openid4vp.allowAuthenticationWithoutLocalUser.label',
          'Allow authentication without a local user',
        )}
      >
        {({commit, label, value: stored}) => (
          <CheckboxWithHint
            checked={!!stored}
            onChange={commit}
            label={label}
            hint={t(
              'flows:core.executions.openid4vp.allowAuthenticationWithoutLocalUser.hint',
              'When enabled, a holder with no matching local user is provisioned just-in-time. When disabled, login requires an existing matching user.',
            )}
          />
        )}
      </VariableBinding>
    </Stack>
  );
}

export default OpenID4VPProperties;
