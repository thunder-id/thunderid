// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {GithubIcon, GoogleIcon} from '@thunderid/components';
import {IdentityProviderTypes, getConnectionIcon, type IdentityProvider} from '@thunderid/configure-connections';
import {Divider, Box} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import AuthenticationMethodItem from './AuthenticationMethodItem';
import AuthMethodGroup from './AuthMethodGroup';

export interface SocialLoginGroupProps {
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

  /**
   * Whether this group is disabled because a pre-configured flow is selected instead of
   * individual toggles.
   */
  disabled?: boolean;
}

/**
 * "Social Login" category on the Sign-in Experience step: Google, GitHub, and any other connected
 * identity providers. Google and GitHub always show (as "Not configured" when no connection
 * exists yet), matching every row in a list keeping the same trailing edge.
 *
 * The group's master checkbox reflects only the *available* (configured) providers: checked when
 * all of them are enabled, indeterminate when some are. Checking it enables just the first
 * available provider as a sensible default; unchecking clears all of them. Disabled entirely if no
 * provider is configured yet, since there's nothing to enable.
 */
export default function SocialLoginGroup({
  integrations,
  availableIntegrations,
  onIntegrationToggle,
  disabled = false,
}: SocialLoginGroupProps): JSX.Element {
  const {t} = useTranslation();

  const googleProvider = availableIntegrations.find(
    (idp: IdentityProvider) => idp.type === IdentityProviderTypes.GOOGLE,
  );
  const githubProvider = availableIntegrations.find(
    (idp: IdentityProvider) => idp.type === IdentityProviderTypes.GITHUB,
  );

  const otherProviders = availableIntegrations.filter(
    (provider: IdentityProvider) =>
      provider.type !== IdentityProviderTypes.GOOGLE && provider.type !== IdentityProviderTypes.GITHUB,
  );

  const availableProviderIds = [googleProvider?.id, githubProvider?.id, ...otherProviders.map((p) => p.id)].filter(
    (id): id is string => !!id,
  );
  const enabledProviderIds = availableProviderIds.filter((id) => integrations[id]);
  const checked = availableProviderIds.length > 0 && enabledProviderIds.length === availableProviderIds.length;
  const indeterminate = enabledProviderIds.length > 0 && enabledProviderIds.length < availableProviderIds.length;

  const handleCheckedChange = (next: boolean): void => {
    if (next) {
      const [firstProviderId] = availableProviderIds;
      if (firstProviderId && !integrations[firstProviderId]) onIntegrationToggle(firstProviderId);
    } else {
      enabledProviderIds.forEach(onIntegrationToggle);
    }
  };

  return (
    <AuthMethodGroup
      title={t('applications:onboarding.configure.security.socialLogin.title', 'Social Login')}
      checked={checked}
      indeterminate={indeterminate}
      onCheckedChange={handleCheckedChange}
      disabled={disabled}
      checkboxDisabled={disabled || availableProviderIds.length === 0}
    >
      {/* Google */}
      <AuthenticationMethodItem
        id={googleProvider?.id ?? 'google'}
        name={t('applications:onboarding.configure.SignInOptions.google')}
        icon={<GoogleIcon size={20} />}
        isEnabled={googleProvider ? (integrations[googleProvider.id] ?? false) : false}
        isAvailable={!!googleProvider}
        isDisabled={disabled}
        onToggle={onIntegrationToggle}
      />

      <Divider variant="middle" />

      {/* GitHub */}
      <AuthenticationMethodItem
        id={githubProvider?.id ?? 'github'}
        name={t('applications:onboarding.configure.SignInOptions.github')}
        icon={<GithubIcon size={20} />}
        isEnabled={githubProvider ? (integrations[githubProvider.id] ?? false) : false}
        isAvailable={!!githubProvider}
        isDisabled={disabled}
        onToggle={onIntegrationToggle}
      />

      {/* Other Social Login Providers */}
      {otherProviders.map((provider: IdentityProvider) => (
        <Box key={provider.id}>
          <Divider variant="middle" />
          <AuthenticationMethodItem
            id={provider.id}
            name={provider.name}
            icon={getConnectionIcon(provider.type)}
            isEnabled={integrations[provider.id] ?? false}
            isAvailable
            isDisabled={disabled}
            onToggle={onIntegrationToggle}
          />
        </Box>
      ))}
    </AuthMethodGroup>
  );
}
