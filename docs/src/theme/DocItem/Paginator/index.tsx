// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useDoc} from '@docusaurus/plugin-content-docs/client';
import {Box} from '@wso2/oxygen-ui';
import {JSX} from 'react';
import PrevNextNav from '@site/src/components/PrevNextNav';

/**
 * Same "Previous / Next" card pair as the blog post footer (PrevNextNav.tsx), instead of
 * Infima's plain default `.pagination-nav` styling — so pagination reads as one component
 * across the site rather than a docs-specific treatment and a blog-specific one.
 */
export default function DocItemPaginator(): JSX.Element {
  const {metadata} = useDoc();
  const {previous, next} = metadata;

  return (
    <Box sx={{mt: 4}}>
      <PrevNextNav prev={previous} next={next} />
    </Box>
  );
}
