// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import Layout from '@theme/Layout';
import {Box} from '@wso2/oxygen-ui';
import {JSX} from 'react';
import Detail from './Detail';
import type {EcosystemEntry} from '@site/src/types/ecosystem';

/**
 * Standalone page for one SDK, plugin, integration, or guide.
 *
 * These pages carry a wide banded header and a two-pane API explorer, neither
 * of which fits the docs prose column, and they are not versioned the way a doc
 * page is. So rather than bending the docs layout to hold them, the ecosystem
 * plugin routes them here, at `/sdks/<id>`, with their own shell. The docs keep
 * their own pages; `docs.overview` is what links the two together.
 */
export default function DetailPage({entry}: {entry: EcosystemEntry}): JSX.Element {
  return (
    <Layout title={entry.name} description={entry.description}>
      <Box sx={{display: 'flex', flexDirection: 'column', minHeight: '100%'}}>
        <Detail entry={entry} />
      </Box>
    </Layout>
  );
}
