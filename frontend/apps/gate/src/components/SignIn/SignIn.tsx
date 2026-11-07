// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useDesign, AuthPageLayout} from '@thunderid/design';
import {useThunderID} from '@thunderid/react';
import {ParticleBackground} from '@wso2/oxygen-ui';
import {useState, type JSX} from 'react';
import SignInBox from './SignInBox';
import SignInCallbackLoading from './SignInCallbackLoading';
import SignInSlogan from './SignInSlogan';

/**
 * Whether the page is the callback of a social identity provider: the SDK exchanges a `code`, or
 * reports an `error`. A `state` with neither is left over from an abandoned or malformed redirect;
 * the SDK neither completes it nor starts a new flow while it is in the URL, so it is dropped here,
 * before the SDK reads the URL, and the page signs in as usual.
 */
const isReturningFromIdentityProvider = (): boolean => {
  const url = new URL(window.location.href);
  const {searchParams} = url;

  if (searchParams.has('code') || searchParams.has('error')) {
    return true;
  }

  if (searchParams.has('state')) {
    searchParams.delete('state');
    window.history.replaceState(window.history.state, '', url.toString());
  }

  return false;
};

export default function SignIn(): JSX.Element {
  const {isMetaLoading} = useThunderID();
  const {isDesignEnabled, isLoading: isDesignLoading} = useDesign();
  // Back from a social identity provider, the page has nothing to offer until the code exchange
  // answers, so one full-page loading state stands in for the slogan and card from the first paint
  // until the flow has a step or an error to show. A completed sign-in redirects away from it.
  const [isCompletingSignIn, setIsCompletingSignIn] = useState(isReturningFromIdentityProvider);

  const isPageLoading = isMetaLoading || isDesignLoading;
  const showSlogan = !isDesignLoading && !isDesignEnabled && !isCompletingSignIn;

  return (
    <AuthPageLayout isLoading={isPageLoading && !isCompletingSignIn} variant="SignIn">
      {showSlogan && <ParticleBackground opacity={0.5} />}
      {showSlogan && <SignInSlogan />}
      {isCompletingSignIn && <SignInCallbackLoading />}
      {!isPageLoading && (
        <SignInBox isCompletingSignIn={isCompletingSignIn} onSignInStepReady={() => setIsCompletingSignIn(false)} />
      )}
    </AuthPageLayout>
  );
}
