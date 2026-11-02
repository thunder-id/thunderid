// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {createContext, createElement, useContext, type JSX, type ReactNode} from 'react';
import useGatewaySource from './gatewaySource';
import type {
  AppliedConfiguration,
  AppliedVersion,
  ApplyRequest,
  ApplyResult,
  StoredValue,
  ValueKind,
  ValueReference,
  Version,
} from '../models/deploy';
import type {Gateway} from '../models/gateway';

/** A value to set in a target's store. */
export interface StoredValueInput {
  value: string;
  /** The value's description, kept as it is when given. */
  description?: string;
}

/**
 * Where the Deploy page reads and writes: the places configuration runs, here called targets, and
 * the versions it is captured as.
 *
 * A control plane's targets are its gateways, and that is the source used when none is provided. A
 * console that deploys somewhere else, such as to environments a separate service keeps, provides its
 * own through `DeploySourceProvider`, answering in the same shapes, and every page and dialog in this
 * package works against it unchanged.
 */
export interface DeploySource {
  /** The targets, the default one first. */
  targets(): Promise<Gateway[]>;
  /**
   * The versions, newest first, without their content: the versions of one target when versions are
   * kept per target, otherwise every version.
   */
  versions(targetId?: string): Promise<Version[]>;
  /** One version, with where it refers to values. */
  version(ref: string, targetId?: string): Promise<Version>;
  /** Which version a target runs. */
  appliedVersion(targetId: string): Promise<AppliedVersion>;
  /** Everything a target runs, each resource as its own read returns it. */
  appliedConfiguration(targetId: string): Promise<AppliedConfiguration>;
  /** What applying a version would change on a target, and what it lacks for it. Nothing is written. */
  applyCheck(targetId: string, version: string): Promise<ApplyResult>;
  /** Applies a version to a target, or works out what it would do when `dryRun` is set. */
  apply(targetId: string, request: ApplyRequest): Promise<ApplyResult>;
  /**
   * Puts a target back on the version it ran before, or works out what that would do. A rollback that
   * removes more resources than the threshold is refused until `confirmRemovals` is set.
   */
  revert(targetId: string, dryRun: boolean, confirmRemovals?: boolean): Promise<ApplyResult>;
  /** Every variable or every secret a target holds. A secret's value is never read back. */
  values(targetId: string, kind: ValueKind): Promise<StoredValue[]>;
  /** Sets a variable or a secret on a target, creating it when the target does not hold it. */
  setValue(targetId: string, kind: ValueKind, name: string, input: StoredValueInput): Promise<StoredValue>;
  /** Removes a variable or a secret from a target. */
  deleteValue(targetId: string, kind: ValueKind, name: string): Promise<void>;
  /** Where the version a target runs refers to values. */
  runningReferences(targetId: string): Promise<ValueReference[]>;
}

const DeploySourceContext = createContext<DeploySource | null>(null);

/** Provides the source every page and dialog in this package reads and writes through. */
export function DeploySourceProvider({source, children}: {source: DeploySource; children: ReactNode}): JSX.Element {
  return createElement(DeploySourceContext.Provider, {value: source}, children);
}

/** The source in effect: the one provided, or the control plane's gateways. */
export default function useDeploySource(): DeploySource {
  const gateways = useGatewaySource();
  return useContext(DeploySourceContext) ?? gateways;
}
