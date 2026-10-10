// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {Environment} from '@thunderid/contexts';
import {createContext, useContext} from 'react';
import type {ValueReference} from './values';

/** A resource of the environment another resource names by its identifier. */
export interface IndexedResource {
  name: string;
  /** Where the environment view shows it. */
  path: string;
}

/** What every part of the environment view reads: the environment, its values and its resources. */
export interface EnvironmentViewContextType {
  environment: Environment;
  /** The variables the gateway holds, by name. */
  variables: ReadonlyMap<string, string>;
  /** The secrets the gateway holds a value for. */
  secrets: ReadonlySet<string>;
  /** The environment's resources by identifier, so a resource naming another links to it. */
  resources: ReadonlyMap<string, IndexedResource>;
  /** Opens the setting of a value the gateway fills, as a list's when `list` is set. */
  edit: (reference: ValueReference, list: boolean) => void;
}

const EnvironmentViewContext = createContext<EnvironmentViewContextType | undefined>(undefined);

export const EnvironmentViewContextProvider = EnvironmentViewContext.Provider;

/** Reads the environment view's context. Only the environment view renders what reads it. */
export function useEnvironmentView(): EnvironmentViewContextType {
  const context = useContext(EnvironmentViewContext);
  if (!context) {
    throw new Error('useEnvironmentView is read outside the environment view');
  }
  return context;
}
