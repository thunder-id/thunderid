// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Link} from '@wso2/oxygen-ui';
import {ExternalLink as ExternalLinkIcon} from '@wso2/oxygen-ui-icons-react';
import type {JSX, ReactNode} from 'react';

// TODO: Move this to oxygen-ui and use.
export default function ExternalLink({href, children = null}: {href: string; children?: ReactNode}): JSX.Element {
  return (
    <Link
      href={href}
      target="_blank"
      rel="noopener noreferrer"
      style={{color: 'inherit', fontWeight: 600, display: 'inline-flex', alignItems: 'center', gap: 2}}
    >
      {children}
      <ExternalLinkIcon size={12} style={{flexShrink: 0, opacity: 0.7}} />
    </Link>
  );
}
