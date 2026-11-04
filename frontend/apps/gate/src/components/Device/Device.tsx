// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {AuthPageLayout, useDesign} from '@thunderid/design';
import {useThunderID} from '@thunderid/react';
import type {JSX} from 'react';
import DeviceBox from './DeviceBox';

export default function Device(): JSX.Element {
  const {isMetaLoading} = useThunderID();
  const {isLoading: isDesignLoading} = useDesign();

  return (
    <AuthPageLayout isLoading={isMetaLoading || isDesignLoading} variant="SignIn">
      <DeviceBox />
    </AuthPageLayout>
  );
}
