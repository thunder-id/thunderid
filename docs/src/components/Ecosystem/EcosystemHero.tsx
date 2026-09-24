// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Typography} from '@wso2/oxygen-ui';
import {JSX} from 'react';
import ProductName from '@site/src/components/ProductName';

/**
 * Page header.
 *
 * Search and filtering used to live here; they now sit in the marketplace
 * layout below, beside the results they act on, so this is purely the title.
 *
 * Shares the same maxWidth/gutter as the filters+grid container in `index.tsx`
 * so the heading lines up with "Filters" and the search bar beneath it.
 */
export default function EcosystemHero(): JSX.Element {
  return (
    <Box sx={{position: 'relative', pt: {xs: 6, md: 8}, pb: {xs: 4, md: 5}}}>
      <Box sx={{maxWidth: 1200, mx: 'auto', px: {xs: 2, sm: 4}}}>
        <Box sx={{maxWidth: 780}}>
          <Box
            sx={{
              display: 'inline-flex',
              alignItems: 'center',
              gap: 1,
              mb: 2.5,
              fontFamily: 'monospace',
              fontSize: '10.5px',
              fontWeight: 600,
              letterSpacing: '0.18em',
              textTransform: 'uppercase',
              color: '#8bf9fa',
            }}
          >
            <Box
              component="span"
              sx={{width: 5, height: 5, borderRadius: '50%', bgcolor: '#8bf9fa', boxShadow: '0 0 10px #8bf9fa'}}
            />
            SDKs &amp; Tools
          </Box>

          <Typography
            variant="h1"
            sx={{
              fontSize: {xs: '2.25rem', sm: '2.75rem', md: '3.5rem'},
              fontWeight: 700,
              letterSpacing: '-0.04em',
              lineHeight: 1.04,
              color: 'text.primary',
              mb: 2.5,
            }}
          >
            Build <ProductName /> into any stack
          </Typography>

          <Typography sx={{fontSize: '16.5px', lineHeight: 1.65, color: 'text.secondary', maxWidth: 620}}>
            Official SDKs, framework integrations, community packages, and agent tooling, from the browser to the
            edge to the server.
          </Typography>
        </Box>
      </Box>
    </Box>
  );
}
