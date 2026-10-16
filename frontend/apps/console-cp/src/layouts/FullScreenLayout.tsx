// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Layout} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {Outlet} from 'react-router';

export default function FullScreenLayout(): JSX.Element {
  return (
    <Layout sx={{minHeight: '100vh'}}>
      <Layout.Content>
        <Box sx={{minHeight: '100vh'}}>
          <Outlet />
        </Box>
      </Layout.Content>
    </Layout>
  );
}
