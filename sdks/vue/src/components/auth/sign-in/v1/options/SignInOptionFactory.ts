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

import {
  ApplicationNativeAuthenticationConstants,
  EmbeddedSignInFlowAuthenticator,
  EmbeddedSignInFlowAuthenticatorKnownIdPType,
  EmbeddedSignInFlowAuthenticatorParamType,
  FieldType,
} from '@thunderid/browser';
import {type VNode, h} from 'vue';
import FacebookButton from '../../../../adapters/FacebookButton';
import GitHubButton from '../../../../adapters/GitHubButton';
import GoogleButton from '../../../../adapters/GoogleButton';
import MicrosoftButton from '../../../../adapters/MicrosoftButton';
import {createField} from '../../../../factories/FieldFactory';
import Button from '../../../../primitives/Button';

/**
 * Props shared by sign-in option rendering functions.
 */
export interface BaseSignInOptionProps {
  authenticator: EmbeddedSignInFlowAuthenticator;
  buttonClassName?: string;
  error?: string | null;
  formValues: Record<string, string>;
  inputClassName?: string;
  isLoading: boolean;
  onInputChange: (param: string, value: string) => void;
  onSubmit: (authenticator: EmbeddedSignInFlowAuthenticator, formData?: Record<string, string>) => void;
  t: (key: string, params?: Record<string, string>) => string;
  touchedFields: Record<string, boolean>;
}

/**
 * Renders form fields for authenticators that require user input (e.g. UsernamePassword, IdentifierFirst).
 */
const renderFormFields = (props: BaseSignInOptionProps): VNode[] => {
  const {authenticator, formValues, touchedFields, isLoading, onInputChange, inputClassName, buttonClassName, t} =
    props;

  const formFields: any[] =
    authenticator.metadata?.params
      ?.sort((a: any, b: any) => a.order - b.order)
      ?.filter((param: any) => param.param !== 'totp') || [];

  const fieldNodes: VNode[] = formFields.map((param: any) =>
    h(
      'div',
      {key: param.param},
      createField({
        className: inputClassName,
        disabled: isLoading,
        label: param.displayName,
        name: param.param,
        onChange: (value: string) => onInputChange(param.param, value),
        placeholder: t('elements.fields.generic.placeholder', {
          field: (param.displayName || param.param).toLowerCase(),
        }),
        required: authenticator.requiredParams.includes(param.param),
        touched: touchedFields[param.param] || false,
        type:
          param.type === EmbeddedSignInFlowAuthenticatorParamType.String && param.confidential
            ? FieldType.Password
            : FieldType.Text,
        value: formValues[param.param] || '',
      }),
    ),
  );

  fieldNodes.push(
    h(
      Button,
      {
        class: buttonClassName,
        color: 'primary',
        'data-testid': 'thunderid-signin-submit',
        disabled: isLoading,
        fullWidth: true,
        loading: isLoading,
        type: 'submit',
        variant: 'solid',
      },
      {default: () => t('username.password.buttons.submit.text')},
    ),
  );

  return fieldNodes;
};

/**
 * Renders a multi-option button for authenticators that require selection
 * but no immediate user input (e.g. EmailOtp, SmsOtp, Totp, Passkey).
 */
const renderMultiOptionButton = (props: BaseSignInOptionProps): VNode => {
  const {authenticator, isLoading, onSubmit, buttonClassName, t} = props;

  let authenticatorName: string = authenticator.authenticator;
  if (authenticator.idp !== EmbeddedSignInFlowAuthenticatorKnownIdPType.Local) {
    authenticatorName = authenticator.idp;
  }

  const displayName: string = t('elements.buttons.multi.option.text', {connection: authenticatorName});

  const iconPathMap: Record<string, string> = {
    [ApplicationNativeAuthenticationConstants.SupportedAuthenticators.SmsOtp]:
      'M20 15.5c-1.25 0-2.45-.2-3.57-.57a1.02 1.02 0 0 0-1.02.24l-2.2 2.2a15.074 15.074 0 0 1-6.59-6.59l2.2-2.2c.27-.27.35-.67.24-1.02A11.36 11.36 0 0 1 8.5 4c0-.55-.45-1-1-1H4c-.55 0-1 .45-1 1 0 9.39 7.61 17 17 17 .55 0 1-.45 1-1v-3.5c0-.55-.45-1-1-1M12 3v10l3-3h6V3z',
    [ApplicationNativeAuthenticationConstants.SupportedAuthenticators.EmailOtp]:
      'M20 4H4c-1.1 0-1.99.9-1.99 2L2 18c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V6c0-1.1-.9-2-2-2m0 4l-8 5l-8-5V6l8 5l8-5z',
    [ApplicationNativeAuthenticationConstants.SupportedAuthenticators.Totp]:
      'M12 1L3 5v6c0 5.55 3.84 10.74 9 12c5.16-1.26 9-6.45 9-12V5z',
  };

  const iconPath: string =
    iconPathMap[authenticator.authenticatorId] ||
    'M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10s10-4.48 10-10S17.52 2 12 2m-2 15l-5-5l1.41-1.41L10 14.17l7.59-7.59L19 8z';

  const icon: VNode = h('svg', {height: '18', viewBox: '0 0 24 24', width: '18', xmlns: 'http://www.w3.org/2000/svg'}, [
    h('path', {d: iconPath, fill: 'currentColor'}),
  ]);

  return h(
    Button,
    {
      class: buttonClassName,
      color: 'secondary',
      disabled: isLoading,
      fullWidth: true,
      onClick: () => onSubmit(authenticator),
      startIcon: icon,
      type: 'button',
      variant: 'solid',
    },
    {default: () => displayName},
  );
};

/**
 * Renders a generic social/federated login button for unknown federated authenticators.
 */
const renderSocialButton = (props: BaseSignInOptionProps): VNode => {
  const {authenticator, isLoading, onSubmit, buttonClassName, t} = props;

  const socialIcon: VNode = h(
    'svg',
    {height: '18', viewBox: '0 0 24 24', width: '18', xmlns: 'http://www.w3.org/2000/svg'},
    [
      h('path', {
        d: 'M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-2 15l-5-5 1.41-1.41L10 14.17l7.59-7.59L19 8l-9 9z',
        fill: 'currentColor',
      }),
    ],
  );

  return h(
    Button,
    {
      class: buttonClassName,
      color: 'secondary',
      disabled: isLoading,
      fullWidth: true,
      onClick: () => onSubmit(authenticator),
      startIcon: socialIcon,
      type: 'button',
      variant: 'outline',
    },
    {default: () => t('elements.buttons.social.text', {connection: authenticator.idp})},
  );
};

/**
 * Creates the appropriate sign-in VNode(s) based on the authenticator's ID.
 */
export const createSignInOption = (props: BaseSignInOptionProps): VNode | VNode[] => {
  const {authenticator, onSubmit, buttonClassName, isLoading} = props;
  const hasParams = !!(authenticator.metadata?.params && authenticator.metadata.params.length > 0);

  switch (authenticator.authenticatorId) {
    case ApplicationNativeAuthenticationConstants.SupportedAuthenticators.UsernamePassword:
    case ApplicationNativeAuthenticationConstants.SupportedAuthenticators.IdentifierFirst:
      return h('div', {}, renderFormFields(props));

    case ApplicationNativeAuthenticationConstants.SupportedAuthenticators.Google:
      return h(GoogleButton, {
        class: buttonClassName,
        isLoading,
        onClick: () => onSubmit(authenticator),
      });

    case ApplicationNativeAuthenticationConstants.SupportedAuthenticators.GitHub:
      return h(GitHubButton, {
        class: buttonClassName,
        isLoading,
        onClick: () => onSubmit(authenticator),
      });

    case ApplicationNativeAuthenticationConstants.SupportedAuthenticators.Microsoft:
      return h(MicrosoftButton, {
        class: buttonClassName,
        isLoading,
        onClick: () => onSubmit(authenticator),
      });

    case ApplicationNativeAuthenticationConstants.SupportedAuthenticators.Facebook:
      return h(FacebookButton, {
        class: buttonClassName,
        isLoading,
        onClick: () => onSubmit(authenticator),
      });

    case ApplicationNativeAuthenticationConstants.SupportedAuthenticators.EmailOtp:
    case ApplicationNativeAuthenticationConstants.SupportedAuthenticators.Totp:
    case ApplicationNativeAuthenticationConstants.SupportedAuthenticators.SmsOtp:
      return hasParams ? h('div', {}, renderFormFields(props)) : renderMultiOptionButton(props);

    default:
      // Federated (non-LOCAL) authenticators → generic social button
      if (authenticator.idp !== EmbeddedSignInFlowAuthenticatorKnownIdPType.Local) {
        return renderSocialButton(props);
      }
      // LOCAL with params → form fields fallback; otherwise multi-option button
      return hasParams ? h('div', {}, renderFormFields(props)) : renderMultiOptionButton(props);
  }
};

/**
 * Convenience function to create sign-in option VNode(s) from an authenticator.
 */
export const createSignInOptionFromAuthenticator = (
  authenticator: EmbeddedSignInFlowAuthenticator,
  formValues: Record<string, string>,
  touchedFields: Record<string, boolean>,
  isLoading: boolean,
  onInputChange: (param: string, value: string) => void,
  onSubmit: (authenticator: EmbeddedSignInFlowAuthenticator, formData?: Record<string, string>) => void,
  t: (key: string, params?: Record<string, string>) => string,
  options?: {
    buttonClassName?: string;
    error?: string | null;
    inputClassName?: string;
  },
): VNode | VNode[] =>
  createSignInOption({
    authenticator,
    formValues,
    isLoading,
    onInputChange,
    onSubmit,
    t,
    touchedFields,
    ...options,
  });
