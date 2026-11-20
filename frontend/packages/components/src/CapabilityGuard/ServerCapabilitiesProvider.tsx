// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {CapabilitiesProvider} from '@thunderid/contexts';
import {useCapabilities} from '@thunderid/hooks';
import type {JSX, PropsWithChildren} from 'react';

/**
 * Reads `GET /capabilities` once and supplies the result to every page beneath it. Mount it at the app
 * root, so pages under any layout see the same capabilities.
 */
export default function ServerCapabilitiesProvider({children}: PropsWithChildren): JSX.Element {
  return <CapabilitiesProvider capabilities={useCapabilities()}>{children}</CapabilitiesProvider>;
}
