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

import React, {type ReactNode} from 'react';
import type {Maturity} from '@site/plugins/maturityPlugin';

interface MaturityBannerProps {
  maturity: Maturity;
}

const MATURITY_CONFIG: Record<Maturity, {label: string; message: string}> = {
  preview: {
    label: 'Preview',
    message: 'This feature is in Preview. APIs and behavior may change without notice.',
  },
  beta: {
    label: 'Beta',
    message: 'This feature is in Beta. It is functional but may have rough edges.',
  },
};

export default function MaturityBanner({maturity}: MaturityBannerProps): ReactNode {
  const config = MATURITY_CONFIG[maturity];

  return (
    <div className={`maturity-banner maturity-banner--${maturity}`} role="note">
      <span className="maturity-banner__badge">{config.label}</span>
      <span className="maturity-banner__message">{config.message}</span>
    </div>
  );
}
