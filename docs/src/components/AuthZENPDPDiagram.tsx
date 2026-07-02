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

import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import {SequenceDiagram} from './SequenceDiagram';
import type {DocusaurusProductConfig} from '@site/docusaurus.product.config';

export function AuthZENPDPDiagram() {
  const {siteConfig} = useDocusaurusContext();
  const productName =
    (siteConfig.customFields?.product as DocusaurusProductConfig | undefined)?.project.name ?? siteConfig.title;

  return (
    <SequenceDiagram
      actors={['Application', 'PEP', `${productName} PDP`, 'Protected Resource']}
      gaps={[260, 300, 300]}
      ariaLabel={`${productName} AuthZEN PDP authorization lifecycle: an application requests a protected resource through a policy enforcement point. The policy enforcement point asks ${productName} for an access decision. A false decision causes the policy enforcement point to deny access. A true decision allows the policy enforcement point to forward the request to the protected resource and return its response.`}
      rows={[
        {from: 0, to: 1, label: 'Request protected resource'},
        {
          from: 1,
          to: 2,
          label: 'POST /access/v1/evaluation',
          sublabel: ['subject, resource, action,', 'optional context'],
        },
        {
          from: 2,
          to: 1,
          label: 'Access decision',
          sublabel: ['decision: true | false,', 'optional context'],
        },
        {from: 1, to: 0, label: 'Deny access if false'},
        {from: 1, to: 3, label: 'Forward request if true'},
        {from: 3, to: 1, label: 'Resource response'},
        {from: 1, to: 0, label: 'Return resource response'},
      ]}
    />
  );
}
