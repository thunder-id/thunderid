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

import {List, Divider} from '@wso2/oxygen-ui';
import {UserRound, Google, GitHub, KeyRound} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import AuthenticationMethodItem from './AuthenticationMethodItem';
import {AuthenticatorTypes} from '@/features/integrations/models/authenticators';
import {type IdentityProvider, IdentityProviderTypes} from '@/features/integrations/models/identity-provider';
import getIntegrationIcon from '@/features/integrations/utils/getIntegrationIcon';

/**
 * Props for the IndividualMethodsToggleView component
 */
export interface IndividualMethodsToggleViewProps {
  /**
   * Record of enabled authentication integrations
   */
  integrations: Record<string, boolean>;

  /**
   * Available identity providers
   */
  availableIntegrations: IdentityProvider[];

  /**
   * Callback when an integration is toggled
   */
  onIntegrationToggle: (integrationId: string) => void;
}

/**
 * Component that renders the individual authentication methods toggle view
 */
export default function IndividualMethodsToggleView({
  integrations,
  availableIntegrations,
  onIntegrationToggle,
}: IndividualMethodsToggleViewProps): JSX.Element {
  const {t} = useTranslation();

  const googleProvider = availableIntegrations.find(
    (idp: IdentityProvider) => idp.type === IdentityProviderTypes.GOOGLE,
  );
  const githubProvider = availableIntegrations.find(
    (idp: IdentityProvider) => idp.type === IdentityProviderTypes.GITHUB,
  );
  const hasUsernamePassword = integrations[AuthenticatorTypes.CREDENTIALS_AUTH] ?? false;

  const otherProviders = availableIntegrations.filter(
    (provider: IdentityProvider) =>
      provider.type !== IdentityProviderTypes.GOOGLE && provider.type !== IdentityProviderTypes.GITHUB,
  );

  return (
    <>
      {/* Authentication Methods List */}
      <List sx={{bgcolor: 'background.paper', borderRadius: 1, border: 1, borderColor: 'divider'}}>
        {/* Username & Password */}
        <AuthenticationMethodItem
          id={AuthenticatorTypes.CREDENTIALS_AUTH}
          name={t('applications:onboarding.configure.SignInOptions.usernamePassword')}
          icon={<UserRound size={24} />}
          isEnabled={hasUsernamePassword}
          isAvailable
          onToggle={onIntegrationToggle}
        />

        <Divider component="li" />

        {/* Passkey */}
        <AuthenticationMethodItem
          id={AuthenticatorTypes.PASSKEY}
          name={t('applications:onboarding.configure.SignInOptions.passkey')}
          icon={<KeyRound size={24} />}
          isEnabled={integrations[AuthenticatorTypes.PASSKEY] ?? false}
          isAvailable
          onToggle={onIntegrationToggle}
        />

        <Divider component="li" />

        {/* Google */}
        <AuthenticationMethodItem
          id={googleProvider?.id ?? 'google'}
          name={t('applications:onboarding.configure.SignInOptions.google')}
          icon={<Google size={24} />}
          isEnabled={googleProvider ? (integrations[googleProvider.id] ?? false) : false}
          isAvailable={!!googleProvider}
          onToggle={onIntegrationToggle}
        />

        <Divider component="li" />

        {/* GitHub */}
        <AuthenticationMethodItem
          id={githubProvider?.id ?? 'github'}
          name={t('applications:onboarding.configure.SignInOptions.github')}
          icon={<GitHub size={24} />}
          isEnabled={githubProvider ? (integrations[githubProvider.id] ?? false) : false}
          isAvailable={!!githubProvider}
          onToggle={onIntegrationToggle}
        />

        {/* Other Social Login Providers */}
        {otherProviders.map((provider: IdentityProvider) => (
          <div key={provider.id}>
            <Divider component="li" />
            <AuthenticationMethodItem
              id={provider.id}
              name={provider.name}
              icon={getIntegrationIcon(provider.type)}
              isEnabled={integrations[provider.id] ?? false}
              isAvailable
              onToggle={onIntegrationToggle}
            />
          </div>
        ))}
      </List>
    </>
  );
}
