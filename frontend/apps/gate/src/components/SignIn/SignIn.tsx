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

import {useThunderID} from '@thunderid/react';
import {useDesign, AuthPageLayout} from '@thunderid/design';
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
