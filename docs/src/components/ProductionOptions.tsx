// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import Link from '@docusaurus/Link';
import Heading from '@theme/Heading';
import {Check} from '@wso2/oxygen-ui-icons-react';
import {ReactNode} from 'react';

/* ─── ProductionOption ────────────────────────────────────────────────────── */

interface ProductionOptionLink {
  label: string;
  href: string;
}

interface ProductionOptionProps {
  title: string;
  icon: ReactNode;
  badge?: string;
  recommended?: boolean;
  points?: string[];
  links: ProductionOptionLink[];
  children: ReactNode;
}

export function ProductionOption({
  title,
  icon,
  badge = undefined,
  recommended = false,
  points = [],
  links,
  children,
}: ProductionOptionProps) {
  return (
    <div className={`production-option${recommended ? ' production-option--recommended' : ''}`}>
      <div className="production-option__header">
        <span className="production-option__icon">{icon}</span>
        {badge && <span className="production-option__badge">{badge}</span>}
      </div>
      <span className="production-option__title">{title}</span>
      <div className="production-option__desc">{children}</div>
      {points.length > 0 && (
        <ul className="production-option__points">
          {points.map((point) => (
            <li key={point}>
              <Check size={14} />
              {point}
            </li>
          ))}
        </ul>
      )}
      <div className="production-option__links">
        {links.map(({label, href}) => {
          const isExternal = href.startsWith('http');
          return (
            <Link key={href} to={href} {...(isExternal ? {target: '_blank', rel: 'noopener noreferrer'} : {})}>
              {label}
              {isExternal ? '\u00a0↗' : '\u00a0→'}
            </Link>
          );
        })}
      </div>
    </div>
  );
}

/* ─── ProductionOptions (wrapper) ─────────────────────────────────────────── */

interface ProductionOptionsProps {
  id: string;
  eyebrow: string;
  title: string;
  intro: ReactNode;
  children: ReactNode;
}

export function ProductionOptions({id, eyebrow, title, intro, children}: ProductionOptionsProps) {
  return (
    <section className="production-options">
      <span className="production-options__eyebrow">{eyebrow}</span>
      <Heading as="h2" id={id} className="production-options__title">
        {title}
      </Heading>
      <div className="production-options__intro">{intro}</div>
      <div className="production-options__cards">{children}</div>
    </section>
  );
}
