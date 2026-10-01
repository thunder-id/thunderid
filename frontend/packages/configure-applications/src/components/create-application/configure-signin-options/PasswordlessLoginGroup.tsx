// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {AuthenticatorTypes} from '@thunderid/configure-connections';
import {Divider} from '@wso2/oxygen-ui';
import {FingerprintPattern, Mail} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import AuthenticationMethodItem from './AuthenticationMethodItem';
import AuthMethodGroup from './AuthMethodGroup';

export interface PasswordlessLoginGroupProps {
  /**
   * Record of enabled authentication integrations
   */
  integrations: Record<string, boolean>;

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

const ITEMS = [AuthenticatorTypes.PASSKEY, AuthenticatorTypes.MAGIC_LINK];

/**
 * "Passwordless Login" category on the Sign-in Experience step: Passkeys and Magic Link. The
 * group's master checkbox is checked when at least one is enabled, indeterminate when only one of
 * the two is. Checking it enables just Passkey (the primary passwordless method) as a sensible
 * default — the admin can add Magic Link from the now-expanded list. Unchecking clears both.
 */
export default function PasswordlessLoginGroup({
  integrations,
  onIntegrationToggle,
  disabled = false,
}: PasswordlessLoginGroupProps): JSX.Element {
  const {t} = useTranslation();

  const isPasskeyEnabled = integrations[AuthenticatorTypes.PASSKEY] ?? false;
  const isMagicLinkEnabled = integrations[AuthenticatorTypes.MAGIC_LINK] ?? false;
  const enabledCount = [isPasskeyEnabled, isMagicLinkEnabled].filter(Boolean).length;
  const checked = enabledCount === ITEMS.length;
  const indeterminate = enabledCount > 0 && enabledCount < ITEMS.length;

  const handleCheckedChange = (next: boolean): void => {
    if (next) {
      if (!isPasskeyEnabled) onIntegrationToggle(AuthenticatorTypes.PASSKEY);
    } else {
      if (isPasskeyEnabled) onIntegrationToggle(AuthenticatorTypes.PASSKEY);
      if (isMagicLinkEnabled) onIntegrationToggle(AuthenticatorTypes.MAGIC_LINK);
    }
  };

  return (
    <AuthMethodGroup
      title={t('applications:onboarding.configure.security.passwordlessLogin.title', 'Passwordless Login')}
      checked={checked}
      indeterminate={indeterminate}
      onCheckedChange={handleCheckedChange}
      disabled={disabled}
    >
      <AuthenticationMethodItem
        id={AuthenticatorTypes.PASSKEY}
        name={t('applications:onboarding.configure.SignInOptions.passkey')}
        icon={<FingerprintPattern size={20} />}
        isEnabled={isPasskeyEnabled}
        isAvailable
        isDisabled={disabled}
        onToggle={onIntegrationToggle}
      />

      <Divider />

      <AuthenticationMethodItem
        id={AuthenticatorTypes.MAGIC_LINK}
        name={t('applications:onboarding.configure.SignInOptions.magicLink')}
        icon={<Mail size={20} />}
        isEnabled={isMagicLinkEnabled}
        isAvailable
        isDisabled={disabled}
        onToggle={onIntegrationToggle}
      />
    </AuthMethodGroup>
  );
}
