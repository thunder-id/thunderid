// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {StaticStepTypes, StepTypes} from '../models/steps';

class FlowConstants {
  /**
   * Private constructor to avoid object instantiation from outside
   * the class.
   */
  // eslint-disable-next-line @typescript-eslint/no-empty-function
  private constructor() {}

  // 1 min
  public static readonly AUTO_SAVE_INTERVAL: number = 60000;

  // Maximum number of history items to keep
  public static readonly MAX_HISTORY_ITEMS: number = 20;

  /**
   * The ID for the start step in the flow.
   * Uses lowercase to match ReactFlow node type conventions.
   */
  public static readonly START_STEP_ID: string = StaticStepTypes.Start.toLowerCase();

  /**
   * The ID for the end/user onboard step in the flow.
   */
  public static readonly END_STEP_ID: string = StepTypes.End;

  /**
   * Default edge type for the flow canvas.
   */
  public static readonly DEFAULT_EDGE_TYPE: string = 'base-edge';

  /**
   * Executor names for auto-assignment based on field types.
   */
  public static readonly ExecutorNames = {
    PASSWORD_PROVISIONING: 'AskPasswordFlowExecutorConstants.PASSWORD_PROVISIONING_EXECUTOR',
    EMAIL_OTP: 'AskPasswordFlowExecutorConstants.EMAIL_OTP_EXECUTOR',
  } as const;

  /**
   * Action types for button actions.
   */
  public static readonly ActionTypes = {
    EXECUTOR: 'EXECUTOR',
  } as const;
}

export default FlowConstants;
