// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {Actor} from './deploy';

/**
 * A version some of a project's own resources on an environment were deployed from, with how many.
 *
 * @public
 */
export interface ProjectVersionShare {
  /** The version's hash. */
  hash: string;
  /** The version's number, when it has one. */
  number?: number;
  /** How many of the project's resources on the environment came from it. */
  resources: number;
}

/**
 * How one project stands on an environment, against the environment before it in the pipeline or, for
 * the first one, against the latest version. A promotion can carry part of a version, so a project is
 * described by its own resources, never by the version the environment last received.
 *
 * @public
 */
export interface ProjectStatus {
  project: {
    /** The versions the project's own resources there came from, the most resources first. */
    versions: ProjectVersionShare[];
    /** The project's own changes waiting to be brought there. */
    pending: number;
    /** What the project's resources need that the environment lacks or refused. */
    required: number;
  };
  /** Changes that belong to no project, which go with any project's promotion when ticked. */
  shared: {
    pending: number;
    byType: {resourceType: string; count: number}[];
  };
  /** What the project's resources need that the environment holds in another form. */
  needed: {differing: number};
  /** When the project's resources were last deployed, promoted or rolled back there, and by whom. */
  deployedAt?: string;
  /** What last brought the project's resources there, when the control plane says. */
  lastAction?: 'deployed' | 'promoted' | 'rolledBack';
  deployedBy?: Actor;
  /**
   * On the first environment: whether a change saved to the project deploys there on its own, whether
   * that follows the organization's default, and how its last run went.
   */
  autoSync?: {
    enabled: boolean;
    inherited: boolean;
    lastRun?: {at: string; version?: string; error?: string};
  };
  /** Whether the project can go back one step there, and to which version. */
  rollback?: {
    available: boolean;
    toVersion?: {hash: string; number?: number};
    /** How many of the project's resources the rollback changes. */
    changes?: number;
    /** Whether the version it goes back to is the one the project runs now, so it undoes the last deploy. */
    sameVersion?: boolean;
  };
}
