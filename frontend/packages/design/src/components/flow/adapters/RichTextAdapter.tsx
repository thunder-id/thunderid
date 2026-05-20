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

import {cn, containsMetaTemplate, replaceMetaTemplate} from '@thunderid/utils';
import {Box} from '@wso2/oxygen-ui';
import DOMPurify from 'dompurify';
import type {JSX} from 'react';
import useDesign from '../../../contexts/Design/useDesign';
import type {FlowComponent} from '../../../models/flow';

/** The meta key used by the server to embed the application's sign-up URL. */
const SIGN_UP_URL_META_KEY = 'application.sign_up_url';

/** The meta key used by the server to embed the application's sign-in URL. */
const SIGN_IN_URL_META_KEY = 'application.sign_in_url';

/** The meta key used by the server to embed the application's forgot-password URL. */
const FORGOT_PASSWORD_URL_META_KEY = 'application.forgot_password_url';

/** The meta key used by the server to embed the application's access URL. */
const APPLICATION_URL_META_KEY = 'application.url';

const REGISTRATION_ENABLED_META_KEY = 'isRegistrationFlowEnabled';

const RECOVERY_ENABLED_META_KEY = 'isRecoveryFlowEnabled';

interface RichTextAdapterProps {
  component: FlowComponent;
  resolve: (template: string | undefined) => string | undefined;
  /**
   * Fallback sign-up URL used when the flow meta does not supply
   * `application.sign_up_url` but self registration is enabled.
   */
  signUpFallbackUrl?: string;
  /**
   * Fallback sign-in URL used when the flow meta does not supply
   * `application.sign_in_url`.
   */
  signInFallbackUrl?: string;
  /**
   * Fallback forgot-password URL used when the flow meta does not supply
   * `application.forgot_password_url` but recovery is enabled.
   */
  forgotPasswordFallbackUrl?: string;
}

export default function RichTextAdapter({
  component,
  resolve,
  signUpFallbackUrl = undefined,
  signInFallbackUrl = undefined,
  forgotPasswordFallbackUrl = undefined,
}: RichTextAdapterProps): JSX.Element | null {
  const {isDesignEnabled} = useDesign();
  const rawLabel = typeof component.label === 'string' ? component.label : undefined;

  // When any component label embeds a sign-up URL meta template we treat the
  // whole element as the "sign up link" block.  Show it only when self
  // registration is enabled; hide it entirely otherwise.
  if (rawLabel && containsMetaTemplate(rawLabel, SIGN_UP_URL_META_KEY)) {
    const isRegistrationEnabled = resolve(`{{meta(${REGISTRATION_ENABLED_META_KEY})}}`) === 'true';

    if (!isRegistrationEnabled) {
      return null;
    }

    // Resolve the label so all other meta/i18n tokens are processed first.
    let resolvedLabel = resolve(rawLabel) ?? rawLabel;

    // If the sign-up URL token is still present after resolution (i.e. the
    // server did not provide application.sign_up_url in meta), substitute the
    // fallback URL so the link still works.
    if (containsMetaTemplate(resolvedLabel, SIGN_UP_URL_META_KEY) && signUpFallbackUrl) {
      resolvedLabel = replaceMetaTemplate(resolvedLabel, SIGN_UP_URL_META_KEY, signUpFallbackUrl);
    }

    return (
      <Box
        className={cn('Flow--richText')}
        sx={{mb: 1, textAlign: isDesignEnabled ? 'center' : 'left'}}
        // eslint-disable-next-line react/no-danger
        dangerouslySetInnerHTML={{__html: DOMPurify.sanitize(resolvedLabel)}}
      />
    );
  }

  // When any component label embeds a forgot-password URL meta template, treat it
  // as the "forgot password link" block. Show it only when recovery is enabled.
  if (rawLabel && containsMetaTemplate(rawLabel, FORGOT_PASSWORD_URL_META_KEY)) {
    const isRecoveryEnabled = resolve(`{{meta(${RECOVERY_ENABLED_META_KEY})}}`) === 'true';

    if (!isRecoveryEnabled) {
      return null;
    }

    let resolvedLabel = resolve(rawLabel) ?? rawLabel;

    if (containsMetaTemplate(resolvedLabel, FORGOT_PASSWORD_URL_META_KEY) && forgotPasswordFallbackUrl) {
      resolvedLabel = replaceMetaTemplate(resolvedLabel, FORGOT_PASSWORD_URL_META_KEY, forgotPasswordFallbackUrl);
    }

    return (
      <Box
        className={cn('Flow--richText')}
        sx={{mb: 1, textAlign: isDesignEnabled ? 'center' : 'left'}}
        // eslint-disable-next-line react/no-danger
        dangerouslySetInnerHTML={{__html: DOMPurify.sanitize(resolvedLabel)}}
      />
    );
  }

  // When any component label embeds a sign-in URL meta template, treat it
  // as the "sign in link" block.
  if (rawLabel && containsMetaTemplate(rawLabel, SIGN_IN_URL_META_KEY)) {
    let resolvedLabel = resolve(rawLabel) ?? rawLabel;

    if (containsMetaTemplate(resolvedLabel, SIGN_IN_URL_META_KEY) && signInFallbackUrl) {
      resolvedLabel = replaceMetaTemplate(resolvedLabel, SIGN_IN_URL_META_KEY, signInFallbackUrl);
    }

    return (
      <Box
        className={cn('Flow--richText')}
        sx={{mb: 1, textAlign: isDesignEnabled ? 'center' : 'left'}}
        // eslint-disable-next-line react/no-danger
        dangerouslySetInnerHTML={{__html: DOMPurify.sanitize(resolvedLabel)}}
      />
    );
  }

  // When any component label embeds the application's URL, render it only when
  // the URL is present. There is no sensible local fallback route for this.
  if (rawLabel && containsMetaTemplate(rawLabel, APPLICATION_URL_META_KEY)) {
    const resolvedUrl = resolve(`{{meta(${APPLICATION_URL_META_KEY})}}`);

    if (!resolvedUrl || containsMetaTemplate(resolvedUrl, APPLICATION_URL_META_KEY)) {
      return null;
    }

    let resolvedLabel = resolve(rawLabel) ?? rawLabel;

    if (containsMetaTemplate(resolvedLabel, APPLICATION_URL_META_KEY)) {
      resolvedLabel = replaceMetaTemplate(resolvedLabel, APPLICATION_URL_META_KEY, resolvedUrl);
    }

    return (
      <Box
        className={cn('Flow--richText')}
        sx={{mb: 1, textAlign: isDesignEnabled ? 'center' : 'left'}}
        // eslint-disable-next-line react/no-danger
        dangerouslySetInnerHTML={{__html: DOMPurify.sanitize(resolvedLabel)}}
      />
    );
  }

  const resolvedLabel = resolve(rawLabel);

  return (
    <Box
      className={cn('Flow--richText')}
      sx={{mb: 1, textAlign: isDesignEnabled ? 'center' : 'left'}}
      // eslint-disable-next-line react/no-danger
      dangerouslySetInnerHTML={{
        __html: DOMPurify.sanitize(resolvedLabel ?? rawLabel ?? ''),
      }}
    />
  );
}
