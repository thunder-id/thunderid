// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, CircularProgress} from '@wso2/oxygen-ui';
import type {JSX} from 'react';

export default function PageLoader(): JSX.Element {
  return (
    <Box sx={{display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100vh'}}>
      <CircularProgress aria-label="Loading content" />
    </Box>
  );
}
