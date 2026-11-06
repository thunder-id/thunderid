// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {Gateway} from './gateway';

/**
 * One environment a version is deployed and promoted through: a target, named.
 *
 * @public
 */
export interface PipelineStage {
  /** What the environment is called, such as Development. */
  name: string;
  /** Where the environment's own console is reached, to manage its users and sessions. */
  adminConsoleUrl?: string;
  /** The target the environment runs on. */
  gateway: Gateway;
}

/**
 * The environments a version moves through, in order: a deploy enters at the first, and each later one
 * is reached by promoting from the one before it.
 *
 * @public
 */
export interface Pipeline {
  stages: PipelineStage[];
}

/**
 * Whether a change saved here deploys to the first environment on its own.
 *
 * @public
 */
export interface AutoApplySetting {
  enabled: boolean;
  /** Set where it cannot be changed from here, such as a declarative configuration. */
  locked?: boolean;
}
