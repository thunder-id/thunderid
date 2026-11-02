// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useContext} from 'react';
import SampleBundlesContext from '../contexts/SampleBundlesContext';
import type {SampleBundle} from '../utils/sampleBundles';

/**
 * Reads every quick start bundle the host application supplied.
 *
 * @public
 */
export const useGetSampleBundles = (): Record<string, SampleBundle> => useContext(SampleBundlesContext);

/**
 * Reads one quick start bundle by name, or undefined when the host supplied none by that name.
 *
 * @public
 */
export const useGetSampleBundle = (key: string): SampleBundle | undefined => useContext(SampleBundlesContext)[key];
