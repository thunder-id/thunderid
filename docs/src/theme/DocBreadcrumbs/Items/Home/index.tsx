// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import Link from '@docusaurus/Link';
import {useActiveVersion, useDoc} from '@docusaurus/plugin-content-docs/client';
import {translate} from '@docusaurus/Translate';
import IconHome from '@theme/Icon/Home';
import {type ReactNode} from 'react';

import styles from './styles.module.css';

const SDK_SIDEBARS = new Set(['browserSdkSidebar', 'expressSdkSidebar', 'reactSdkSidebar', 'vueSdkSidebar']);

export default function HomeBreadcrumbItem(): ReactNode {
  const activeVersion = useActiveVersion(undefined);
  const {metadata} = useDoc();
  // version.path is the docs version root (e.g. /thunder/docs/next)
  const docsHomeHref = activeVersion?.path ?? '/docs';
  const homeHref = SDK_SIDEBARS.has(metadata.sidebar ?? '') ? '/sdks-and-tools' : docsHomeHref;

  return (
    <li className="breadcrumbs__item">
      <Link
        aria-label={translate({
          id: 'theme.docs.breadcrumbs.home',
          message: 'Home page',
          description: 'The ARIA label for the home page in the breadcrumbs',
        })}
        className="breadcrumbs__link"
        href={homeHref}>
        <IconHome className={styles.breadcrumbHomeIcon} />
      </Link>
    </li>
  );
}
