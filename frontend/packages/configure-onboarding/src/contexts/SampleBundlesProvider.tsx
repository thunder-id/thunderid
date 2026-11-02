// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {JSX, PropsWithChildren} from 'react';
import SampleBundlesContext from './SampleBundlesContext';
import type {SampleBundle} from '../utils/sampleBundles';

/**
 * Props for the SampleBundlesProvider component.
 *
 * @public
 */
export interface SampleBundlesProviderProps extends PropsWithChildren {
  /** The quick start bundles staged into the host application, keyed by bundle name. */
  bundles: Record<string, SampleBundle>;
}

/**
 * Supplies the quick start bundles the welcome pages can import. The bundles are files the build
 * stages into the host application, so the host reads them and hands them down here.
 *
 * @public
 */
export default function SampleBundlesProvider({bundles, children}: SampleBundlesProviderProps): JSX.Element {
  return <SampleBundlesContext.Provider value={bundles}>{children}</SampleBundlesContext.Provider>;
}
