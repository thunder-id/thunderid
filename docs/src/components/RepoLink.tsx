// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import Link from '@docusaurus/Link';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import type {CSSProperties, ReactNode} from 'react';
import type {DocusaurusProductConfig} from '@site/docusaurus.product.config';

export default function RepoLink({
  path = '',
  children,
  className = undefined,
  style = undefined,
}: {
  path?: string;
  children: ReactNode;
  className?: string;
  style?: CSSProperties;
}): ReactNode {
  const {siteConfig} = useDocusaurusContext();
  const config = siteConfig.customFields?.product as DocusaurusProductConfig;
  return (
    <Link className={className} href={config.project.source.github.url + path} style={style}>
      {children}
    </Link>
  );
}
