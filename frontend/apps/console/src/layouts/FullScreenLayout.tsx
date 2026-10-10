// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useEnvironment} from '@thunderid/contexts';
import {Box, Layout} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {Outlet} from 'react-router';
import EnvironmentView from '../components/environment-view/EnvironmentView';

export default function FullScreenLayout(): JSX.Element {
  const {selected} = useEnvironment();
  return (
    <Layout sx={{minHeight: '100vh'}}>
      <Layout.Content>
        <Box sx={{minHeight: '100vh'}}>
          {selected ? (
            <EnvironmentView key={selected.id} environment={selected}>
              <Outlet />
            </EnvironmentView>
          ) : (
            <Outlet />
          )}
        </Box>
      </Layout.Content>
    </Layout>
  );
}
