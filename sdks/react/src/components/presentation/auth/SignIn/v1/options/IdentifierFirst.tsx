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

import {EmbeddedSignInFlowAuthenticatorParamType, FieldType} from '@thunderid/browser';
import {FC, ReactElement, useEffect} from 'react';
// eslint-disable-next-line import/no-cycle
import {BaseSignInOptionProps} from './SignInOptionFactory';
import useFlow from '../../../../../../contexts/Flow/useFlow';
import useTheme from '../../../../../../contexts/Theme/useTheme';
import useTranslation from '../../../../../../hooks/useTranslation';
import {createField} from '../../../../../factories/FieldFactory';
import Button from '../../../../../primitives/Button/Button';

/**
 * Identifier First Sign-In Option Component.
 * Handles identifier-first authentication flow (username first, then password).
 */
const IdentifierFirst: FC<BaseSignInOptionProps> = ({
  authenticator,
  formValues,
  touchedFields,
  isLoading,
  onInputChange,
  inputClassName = '',
  buttonClassName = '',
  preferences,
}: BaseSignInOptionProps): ReactElement => {
  const {theme} = useTheme();
  const {t} = useTranslation(preferences?.i18n);
  const {setTitle, setSubtitle} = useFlow();

  const formFields: any = authenticator.metadata?.params?.sort((a: any, b: any) => a.order - b.order) || [];

  useEffect(() => {
    setTitle(t('identifier.first.heading'));
    setSubtitle(t('identifier.first.subheading'));
  }, [setTitle, setSubtitle, t]);

  return (
    <>
      {formFields.map((param: any) => (
        <div key={param.param}>
          {createField({
            className: inputClassName,
            disabled: isLoading,
            label: param.displayName,
            name: param.param,
            onChange: (value: any) => onInputChange(param.param, value),
            placeholder: t(`elements.fields.generic.placeholder`, {
              field: (param.displayName || param.param).toLowerCase(),
            }),
            required: authenticator.requiredParams.includes(param.param),
            touched: touchedFields[param.param] || false,
            type:
              param.type === EmbeddedSignInFlowAuthenticatorParamType.String && param.confidential
                ? FieldType.Password
                : FieldType.Text,
            value: formValues[param.param] || '',
          })}
        </div>
      ))}

      <Button
        fullWidth
        type="submit"
        color="primary"
        variant="solid"
        disabled={isLoading}
        loading={isLoading}
        className={buttonClassName}
        style={{marginBottom: `calc(${theme.vars.spacing.unit} * 2)`}}
      >
        {t('identifier.first.buttons.submit.text')}
      </Button>
    </>
  );
};

export default IdentifierFirst;
