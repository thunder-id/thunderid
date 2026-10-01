// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMemo} from 'react';
import {SAMPLE_BUNDLES, type SampleBundle} from '../data/sampleBundles';

export const useGetSampleBundles = (): Record<string, SampleBundle> => useMemo(() => SAMPLE_BUNDLES, []);

export const useGetSampleBundle = (key: string): SampleBundle | undefined => useMemo(() => SAMPLE_BUNDLES[key], [key]);
