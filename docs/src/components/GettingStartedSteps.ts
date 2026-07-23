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

export interface JourneyStep {
  label: string;
  href: string;
  docIds: string[];
}

export function createGettingStartedSteps(productName: string): JourneyStep[] {
  return [
    {
      label: `Run ${productName}`,
      href: '/docs/next/getting-started/get-thunderid',
      docIds: ['getting-started/get-thunderid'],
    },
    {
      label: 'Register an app',
      href: '/docs/next/getting-started/register-an-application',
      docIds: ['getting-started/register-an-application'],
    },
    {
      label: 'Build a flow',
      href: '/docs/next/getting-started/build-a-flow',
      docIds: ['getting-started/build-a-flow'],
    },
    {
      label: 'Connect your app',
      href: '/docs/next/getting-started/connect-your-application',
      docIds: [
        'getting-started/connect-your-application/index',
        'getting-started/connect-your-application/react',
        'getting-started/connect-your-application/vue',
        'getting-started/connect-your-application/browser',
        'getting-started/connect-your-application/express',
        'getting-started/connect-your-application/nuxt',
        'getting-started/connect-your-application/node',
        'getting-started/connect-your-application/nextjs',
      ],
    },
  ];
}

const STEP_DOC_IDS: string[][] = [];

export function getGettingStartedStepIndex(docId?: string): number | null {
  if (!docId) {
    return null;
  }

  const stepIndex = STEP_DOC_IDS.findIndex((ids) => ids.includes(docId));

  return stepIndex >= 0 ? stepIndex + 1 : null;
}
