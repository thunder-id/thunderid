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

import Link from '@docusaurus/Link';
import React, {ReactNode} from 'react';

/* ─── NextStepsCard ───────────────────────────────────────────────────────── */

interface NextStepsCardProps {
  title: string;
  description?: string;
  href: string;
}

export function NextStepsCard({title, description, href}: NextStepsCardProps) {
  const isExternal = href.startsWith('http');

  return (
    <Link
      to={href}
      {...(isExternal ? {target: '_blank', rel: 'noopener noreferrer'} : {})}
      style={{textDecoration: 'none', display: 'block', height: '100%'}}
    >
      <div className="next-steps-card" style={{height: '100%'}}>
        <span className="next-steps-card__title">{title}</span>
        {description && <span className="next-steps-card__desc">{description}</span>}
        <span className="next-steps-card__arrow" aria-hidden>→</span>
      </div>
    </Link>
  );
}

/* ─── NextStepsGroup ──────────────────────────────────────────────────────── */

interface NextStepsGroupProps {
  label: string;
  children: ReactNode;
}

export function NextStepsGroup({label, children}: NextStepsGroupProps) {
  return (
    <div className="next-steps-group">
      <span className="next-steps-group__label">{label}</span>
      <div className="next-steps-group__cards">{children}</div>
    </div>
  );
}

/* ─── NextSteps (wrapper) ─────────────────────────────────────────────────── */

interface NextStepsProps {
  children: ReactNode;
}

export function NextSteps({children}: NextStepsProps) {
  return <div className="next-steps">{children}</div>;
}
