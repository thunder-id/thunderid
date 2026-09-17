// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import Link from '@docusaurus/Link';
import {useLocation} from '@docusaurus/router';
import DocBreadcrumbs from '@theme-original/DocBreadcrumbs';
import {Box} from '@wso2/oxygen-ui';
import {ArrowLeft} from '@wso2/oxygen-ui-icons-react';
import {JSX} from 'react';
import sdkIdFromPath from '@site/src/components/Ecosystem/sdkDocPath';
import {useEcosystem} from '@site/src/hooks/useEcosystem';

/**
 * Replaces the docs breadcrumb with a single link back to the SDK, on pages
 * that belong to one.
 *
 * Those pages drop the sidebar (`displayed_sidebar: null`), so a trail through
 * a tree the reader cannot see would be of no use. One way back to where they
 * came from is.
 */
export default function DocBreadcrumbsWrapper(): JSX.Element {
  const {pathname} = useLocation();
  const entries = useEcosystem();
  const id = sdkIdFromPath(pathname);
  const entry = id ? entries.find((candidate) => candidate.id === id) : undefined;

  if (!entry?.sections?.length) {
    return <DocBreadcrumbs />;
  }

  return (
    <Box
      component={Link}
      to={`/sdks/${entry.id}`}
      sx={{
        display: 'inline-flex',
        alignItems: 'center',
        gap: 1,
        mb: 2,
        fontSize: '13.5px',
        fontWeight: 500,
        color: 'text.secondary',
        textDecoration: 'none',
        transition: 'color 0.15s',
        '&:hover': {color: 'text.primary'},
      }}
    >
      <ArrowLeft size={15} strokeWidth={2.2} />
      Back to {entry.name}
    </Box>
  );
}
