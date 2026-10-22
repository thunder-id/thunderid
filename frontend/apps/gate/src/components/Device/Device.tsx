// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {AuthPageLayout, useDesign} from '@thunderid/design';
import type {JSX} from 'react';
import DeviceBox from './DeviceBox';

export default function Device(): JSX.Element {
  const {isLoading: isDesignLoading} = useDesign();

  return (
    <AuthPageLayout isLoading={isDesignLoading} variant="SignIn">
      <DeviceBox />
    </AuthPageLayout>
  );
}
