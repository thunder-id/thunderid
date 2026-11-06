// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {createContext} from 'react';

/** The names the environments' maps give their resources, by `type/id` key. */
const ResourceNamesContext = createContext<ReadonlyMap<string, string>>(new Map());

export default ResourceNamesContext;
