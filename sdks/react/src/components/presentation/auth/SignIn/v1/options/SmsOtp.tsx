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
import OtpField from '../../../../../primitives/OtpField/OtpField';

/**
 * SMS OTP Sign-In Option Component.
 * Handles SMS-based OTP authentication.
 */
const SmsOtp: FC<BaseSignInOptionProps> = ({
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
    setTitle(t('sms.otp.heading'));
    setSubtitle(t('sms.otp.subheading'));
  }, [setTitle, setSubtitle, t]);

  const hasOtpField: any = formFields.some(
    (param: any) => param.param.toLowerCase().includes('otp') || param.param.toLowerCase().includes('code'),
  );

  return (
    <>
      {formFields.map((param: any) => {
        const isOtpParam: any = param.param.toLowerCase().includes('otp') || param.param.toLowerCase().includes('code');

        return (
          <div key={param.param}>
            {isOtpParam && hasOtpField ? (
              <OtpField
                length={6}
                value={formValues[param.param] || ''}
                onChange={(event: any): void => onInputChange(param.param, event.target.value)}
                disabled={isLoading}
                className={inputClassName}
              />
            ) : (
              createField({
                className: inputClassName,
                disabled: isLoading,
                label: param.displayName,
                name: param.param,
                onChange: (value: any) => onInputChange(param.param, value),
                required: authenticator.requiredParams.includes(param.param),
                touched: touchedFields[param.param] || false,
                type:
                  param.type === EmbeddedSignInFlowAuthenticatorParamType.String && param.confidential
                    ? FieldType.Password
                    : FieldType.Text,
                value: formValues[param.param] || '',
              })
            )}
          </div>
        );
      })}

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
        {t('sms.otp.buttons.submit.text')}
      </Button>
    </>
  );
};

export default SmsOtp;
