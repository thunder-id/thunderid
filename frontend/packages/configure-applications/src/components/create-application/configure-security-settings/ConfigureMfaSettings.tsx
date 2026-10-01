// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useSMSProviders} from '@thunderid/configure-connections';
import {Alert, Collapse, Divider} from '@wso2/oxygen-ui';
import {Mail, MessageSquare} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import useApplicationCreateContext from '../../../hooks/useApplicationCreateContext';
import AuthenticationMethodItem from '../configure-signin-options/AuthenticationMethodItem';
import AuthMethodGroup from '../configure-signin-options/AuthMethodGroup';

/**
 * Multi-Factor Login category of the Sign-in Experience step. Lets the admin require an
 * Email OTP and/or SMS OTP challenge after the primary sign-in method succeeds. Kept as separate
 * state from `integrations` (see `useApplicationCreateContext`) since MFA layers on top of
 * whichever first factor is chosen rather than being another alternative to it.
 *
 * Only meaningful for the wizard's own generated flow graph, so the section is disabled with an
 * explanatory note when a pre-configured flow is selected instead.
 */
export default function ConfigureMfaSettings(): JSX.Element {
  const {t} = useTranslation();
  const {
    integrations,
    isEmailOtpMfaEnabled,
    setIsEmailOtpMfaEnabled,
    isSmsOtpMfaEnabled,
    setIsSmsOtpMfaEnabled,
    smsOtpSenderId,
    setSmsOtpSenderId,
    selectedAuthFlow,
  } = useApplicationCreateContext();

  const {data: smsProviders, isLoading: isLoadingSmsProviders} = useSMSProviders();
  const hasSmsProvider = (smsProviders?.length ?? 0) > 0;
  // A flow only genuinely blocks MFA when it was manually picked from the pre-configured flows
  // list (an opaque, arbitrary graph). Auto-matched flows (integrations still enabled) are
  // regenerated on submit with the MFA subgraph included, so they don't need to disable this.
  const hasEnabledIntegrations = Object.values(integrations).some(Boolean);
  const isDisabledByPreConfiguredFlow = selectedAuthFlow !== null && !hasEnabledIntegrations;

  const handleEmailOtpToggle = (): void => {
    setIsEmailOtpMfaEnabled(!isEmailOtpMfaEnabled);
  };

  const handleSmsOtpToggle = (): void => {
    const next = !isSmsOtpMfaEnabled;
    setIsSmsOtpMfaEnabled(next);
    if (next && !smsOtpSenderId && smsProviders?.[0]) {
      setSmsOtpSenderId(smsProviders[0].id);
    }
  };

  // Only the currently available methods count toward the master checkbox — SMS OTP doesn't
  // count until a provider is connected, same as Social Login's per-provider availability.
  const availableEnabledStates = [isEmailOtpMfaEnabled, ...(hasSmsProvider ? [isSmsOtpMfaEnabled] : [])];
  const enabledCount = availableEnabledStates.filter(Boolean).length;
  const checked = enabledCount === availableEnabledStates.length;
  const indeterminate = enabledCount > 0 && enabledCount < availableEnabledStates.length;

  const handleCheckedChange = (next: boolean): void => {
    if (next) {
      if (!isEmailOtpMfaEnabled) setIsEmailOtpMfaEnabled(true);
    } else {
      if (isEmailOtpMfaEnabled) setIsEmailOtpMfaEnabled(false);
      if (isSmsOtpMfaEnabled) setIsSmsOtpMfaEnabled(false);
    }
  };

  return (
    <AuthMethodGroup
      data-testid="application-configure-mfa-settings"
      title={t('applications:onboarding.configure.security.mfa.title', 'Multi-Factor Login')}
      checked={checked}
      indeterminate={indeterminate}
      onCheckedChange={handleCheckedChange}
      disabled={isDisabledByPreConfiguredFlow}
      banner={
        <Collapse in={isDisabledByPreConfiguredFlow} unmountOnExit>
          <Alert severity="info" sx={{py: 0.5}}>
            {t(
              'applications:onboarding.configure.security.mfa.disabledForPreConfiguredFlow',
              "MFA isn't available for the selected pre-configured flow. Clear it, or choose individual sign-in methods above instead.",
            )}
          </Alert>
        </Collapse>
      }
    >
      <AuthenticationMethodItem
        id="email-otp"
        name={t('applications:onboarding.configure.security.mfa.emailOtp', 'Email OTP')}
        icon={<Mail size={20} />}
        isEnabled={isEmailOtpMfaEnabled}
        isAvailable
        isDisabled={isDisabledByPreConfiguredFlow}
        onToggle={handleEmailOtpToggle}
      />

      <Divider variant="middle" />

      <AuthenticationMethodItem
        id="sms-otp"
        name={t('applications:onboarding.configure.SignInOptions.smsOtp', 'SMS OTP')}
        icon={<MessageSquare size={20} />}
        isEnabled={isSmsOtpMfaEnabled}
        isAvailable={hasSmsProvider}
        isDisabled={isDisabledByPreConfiguredFlow || isLoadingSmsProviders}
        onToggle={handleSmsOtpToggle}
      />
    </AuthMethodGroup>
  );
}
