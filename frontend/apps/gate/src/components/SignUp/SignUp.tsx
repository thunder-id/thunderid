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

import {AuthPageLayout, useDesign} from '@thunderid/design';
import {useThunderID} from '@thunderid/react';
import {ParticleBackground} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import SignUpBox from './SignUpBox';

export default function SignUp(): JSX.Element {
  const {isMetaLoading} = useThunderID();
  const {isDesignEnabled, isLoading: isDesignLoading} = useDesign();

  const showSlogan = !isDesignLoading && !isDesignEnabled;

  return (
    <AuthPageLayout isLoading={isMetaLoading} variant="SignUp">
      {showSlogan && <ParticleBackground opacity={0.5} />}
      <SignUpBox />
    </AuthPageLayout>
  );
}
