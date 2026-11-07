// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useDesign, AuthPageLayout} from '@thunderid/design';
import {useThunderID} from '@thunderid/react';
import {ParticleBackground} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import SignInBox from './SignInBox';
import SignInSlogan from './SignInSlogan';

export default function SignIn(): JSX.Element {
  const {isMetaLoading} = useThunderID();
  const {isDesignEnabled, isLoading: isDesignLoading} = useDesign();

  const showSlogan = !isDesignLoading && !isDesignEnabled;

  return (
    <AuthPageLayout isLoading={isMetaLoading || isDesignLoading} variant="SignIn">
      {showSlogan && <ParticleBackground opacity={0.5} />}
      {showSlogan && <SignInSlogan />}
      <SignInBox />
    </AuthPageLayout>
  );
}
