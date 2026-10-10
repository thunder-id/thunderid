// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useContext} from 'react';
import EnvironmentContext, {type EnvironmentContextType} from './EnvironmentContext';

const NONE: EnvironmentContextType = {environments: [], select: () => undefined};

/**
 * Reads whether the console shows the configuration or one environment. Without an
 * `EnvironmentProvider` it is always the configuration.
 *
 * @public
 */
export default function useEnvironment(): EnvironmentContextType {
  return useContext(EnvironmentContext) ?? NONE;
}
