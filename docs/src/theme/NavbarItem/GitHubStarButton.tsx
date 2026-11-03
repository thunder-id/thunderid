// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import React, {useState, type CSSProperties} from 'react';

interface ProductCustomFields {
  project: {
    source: {
      github: {
        fullName: string;
        url: string;
      };
    };
  };
}

interface Props {
  mobile?: boolean;
}

const buttonStyle: CSSProperties = {
  display: 'inline-flex',
  alignItems: 'center',
  gap: '0.375rem',
  height: '28px',
  padding: '0 0.75rem',
  border: '1px solid var(--oxygen-palette-divider)',
  borderRadius: '6px',
  color: 'var(--oxygen-palette-text-primary)',
  fontSize: '0.875rem',
  fontWeight: 'var(--ifm-font-weight-semibold)',
  lineHeight: 1,
  whiteSpace: 'nowrap',
  textDecoration: 'none',
  transition: 'background-color var(--ifm-transition-fast) ease, border-color var(--ifm-transition-fast) ease',
};

const buttonHoverStyle: CSSProperties = {
  backgroundColor: 'var(--ifm-color-emphasis-100)',
  borderColor: 'var(--ifm-color-emphasis-400)',
};

export default function GitHubStarButton({mobile = false}: Props): React.ReactElement {
  const {siteConfig} = useDocusaurusContext();
  const {project} = siteConfig.customFields?.product as ProductCustomFields;
  const {fullName, url} = project.source.github;
  const [hovered, setHovered] = useState(false);

  const button = (
    <a
      href={url}
      target="_blank"
      rel="noopener noreferrer"
      aria-label={`Star ${fullName} on GitHub`}
      style={hovered ? {...buttonStyle, ...buttonHoverStyle} : buttonStyle}
      onMouseEnter={() => setHovered(true)}
      onMouseLeave={() => setHovered(false)}
    >
      <svg viewBox="0 0 16 16" width="16" height="16" aria-hidden="true" focusable="false">
        <path
          fill="currentColor"
          d="M8 .25a.75.75 0 0 1 .673.418l1.882 3.815 4.21.612a.75.75 0 0 1 .416 1.279l-3.046 2.97.719 4.192a.751.751 0 0 1-1.088.791L8 12.347l-3.766 1.98a.75.75 0 0 1-1.088-.79l.72-4.194L.818 6.374a.75.75 0 0 1 .416-1.28l4.21-.611L7.327.668A.75.75 0 0 1 8 .25Zm0 2.445L6.615 5.5a.75.75 0 0 1-.564.41l-3.097.45 2.24 2.184a.75.75 0 0 1 .216.664l-.528 3.084 2.769-1.456a.75.75 0 0 1 .698 0l2.77 1.456-.53-3.084a.75.75 0 0 1 .216-.664l2.24-2.183-3.096-.45a.75.75 0 0 1-.564-.41L8 2.694Z"
        />
      </svg>
      <span>Star</span>
    </a>
  );

  if (mobile) {
    return (
      <li className="menu__list-item">
        <div style={{padding: '0.375rem 0.75rem'}}>{button}</div>
      </li>
    );
  }

  // navbar__item hides the desktop button below the mobile breakpoint.
  return <div className="navbar__item">{button}</div>;
}
