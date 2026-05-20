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

/**
 * Application creation step identifiers used in application creation flow
 * to track the current step and navigate between steps.
 *
 * @public
 */
export const ApplicationCreateFlowStep = {
  STACK: 'STACK',
  NAME: 'NAME',
  ORGANIZATION_UNIT: 'ORGANIZATION_UNIT',
  DESIGN: 'DESIGN',
  OPTIONS: 'OPTIONS',
  EXPERIENCE: 'EXPERIENCE',
  CONFIGURE: 'CONFIGURE',
  COMPLETE: 'COMPLETE',
} as const;

/**
 * Sign-in approach identifiers for application creation flow.
 *
 * @public
 */
export const ApplicationCreateFlowSignInApproach = {
  INBUILT: 'INBUILT',
  EMBEDDED: 'EMBEDDED',
} as const;

/**
 * Configuration type identifiers for application creation flow.
 *
 * @public
 */
export const ApplicationCreateFlowConfiguration = {
  URL: 'URL',
  DEEPLINK: 'DEEPLINK',
  NONE: 'NONE',
} as const;

/**
 * Application creation step type
 *
 * @public
 */
export type ApplicationCreateFlowStep = keyof typeof ApplicationCreateFlowStep;

/**
 * Sign-in approach type
 *
 * @public
 */
export type ApplicationCreateFlowSignInApproach = keyof typeof ApplicationCreateFlowSignInApproach;

/**
 * Configuration type
 *
 * @public
 */
export type ApplicationCreateFlowConfiguration = keyof typeof ApplicationCreateFlowConfiguration;
