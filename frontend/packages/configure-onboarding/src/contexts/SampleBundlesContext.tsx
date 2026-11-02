// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {createContext, type Context} from 'react';
import type {SampleBundle} from '../utils/sampleBundles';

/**
 * The quick start bundles the host application staged, keyed by bundle name.
 *
 * Defaults to no bundles, so a page renders standalone, in a test or a story, without a
 * `SampleBundlesProvider` above it. Read it through `useGetSampleBundles` rather than directly.
 *
 * @public
 */
const SampleBundlesContext: Context<Record<string, SampleBundle>> = createContext<Record<string, SampleBundle>>({});

export default SampleBundlesContext;
