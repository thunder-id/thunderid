// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {ActionTypes} from '../models/actions';
import {BlockTypes, ElementTypes} from '../models/elements';
import {ExecutionTypes, StepTypes} from '../models/steps';
import {TemplateTypes} from '../models/templates';
import {WidgetTypes} from '../models/widget';

class VisualFlowConstants {
  /**
   * Private constructor to avoid object instantiation from outside
   * the class.
   */
  // eslint-disable-next-line @typescript-eslint/no-empty-function
  private constructor() {}

  public static readonly FLOW_BUILDER_CANVAS_ID: string = 'flow-builder-canvas';

  public static readonly FLOW_BUILDER_VIEW_ID: string = 'flow-builder-view';

  public static readonly FLOW_BUILDER_FORM_ID: string = 'flow-builder-form';

  public static readonly FLOW_BUILDER_DRAGGABLE_ID: string = 'flow-builder-draggable';

  public static readonly FLOW_BUILDER_DROPPABLE_CANVAS_ID: string = 'flow-builder-droppable-canvas';

  public static readonly FLOW_BUILDER_DROPPABLE_VIEW_ID: string = 'flow-builder-droppable-view';

  public static readonly FLOW_BUILDER_DROPPABLE_FORM_ID: string = 'flow-builder-droppable-form';

  public static readonly FLOW_BUILDER_STACK_ID: string = 'flow-builder-stack';

  public static readonly FLOW_BUILDER_DROPPABLE_STACK_ID: string = 'flow-builder-droppable-stack';

  public static readonly FLOW_BUILDER_NEXT_HANDLE_SUFFIX: string = `_${ActionTypes.Next}`;

  public static readonly FLOW_BUILDER_PREVIOUS_HANDLE_SUFFIX: string = `_${ActionTypes.Previous}`;

  public static readonly FLOW_BUILDER_INCOMPLETE_HANDLE_SUFFIX: string = `_${ActionTypes.Incomplete}`;

  // Rendered size of the circular executor chip in compact (non-verbose) mode.
  // Must match the dimensions in ExecutionCompact.tsx; used to feed the
  // auto-layout engine the post-toggle node size before it is measured.
  public static readonly FLOW_BUILDER_COMPACT_EXECUTION_NODE_SIZE: number = 48;

  public static readonly FLOW_BUILDER_CANVAS_ALLOWED_RESOURCE_TYPES: string[] = [
    StepTypes.View,
    StepTypes.Rule,
    StepTypes.Execution,
    StepTypes.Call,
    TemplateTypes.Basic,
    TemplateTypes.BasicFederated,
    TemplateTypes.Blank,
    TemplateTypes.PasskeyLogin,
    BlockTypes.Form, // Form is allowed for drop detection, but handled specially to show dialog
    // Input types are allowed for drop detection, but handled specially to show dialog
    ElementTypes.TextInput,
    ElementTypes.PasswordInput,
    ElementTypes.EmailInput,
    ElementTypes.PhoneInput,
    ElementTypes.NumberInput,
    ElementTypes.DateInput,
    ElementTypes.OtpInput,
    ElementTypes.Checkbox,
    ElementTypes.Dropdown,
    ElementTypes.Select,
    // Widgets are allowed for drop detection, but handled specially to show dialog
    WidgetTypes.GoogleFederation,
    WidgetTypes.IdentifierPassword,
    WidgetTypes.SMSOTP,
    WidgetTypes.EmailOTP,
    WidgetTypes.GithubFederation,
    WidgetTypes.EUDIWallet,
    WidgetTypes.PasskeyAuthentication,
    WidgetTypes.Provisioning,
    WidgetTypes.Consent,
    WidgetTypes.MagicLink,
    WidgetTypes.SelfSignUpLink,
    WidgetTypes.SignInLink,
    WidgetTypes.RecoveryLink,
    ElementTypes.Timer,
  ];

  public static readonly FLOW_BUILDER_VIEW_ALLOWED_RESOURCE_TYPES: string[] = [
    BlockTypes.Form,
    ElementTypes.Action,
    ElementTypes.Resend,
    ElementTypes.Icon,
    ElementTypes.Stack,
    ElementTypes.Text,
    ElementTypes.RichText,
    ElementTypes.Divider,
    ElementTypes.Image,
    ElementTypes.Captcha,
    ElementTypes.Custom,
    // Input types are allowed for drop detection, but handled specially to show dialog
    ElementTypes.TextInput,
    ElementTypes.PasswordInput,
    ElementTypes.EmailInput,
    ElementTypes.PhoneInput,
    ElementTypes.NumberInput,
    ElementTypes.DateInput,
    ElementTypes.OtpInput,
    ElementTypes.Checkbox,
    ElementTypes.Dropdown,
    ElementTypes.Select,
    WidgetTypes.GoogleFederation,
    WidgetTypes.IdentifierPassword,
    WidgetTypes.SMSOTP,
    WidgetTypes.EmailOTP,
    WidgetTypes.GithubFederation,
    WidgetTypes.EUDIWallet,
    WidgetTypes.PasskeyAuthentication,
    WidgetTypes.MagicLink,
    WidgetTypes.SelfSignUpLink,
    WidgetTypes.SignInLink,
    WidgetTypes.RecoveryLink,
    ElementTypes.Timer,
  ];

  public static readonly FLOW_BUILDER_FLOW_COMPLETION_VIEW_ALLOWED_RESOURCE_TYPES: string[] = [
    ElementTypes.Icon,
    ElementTypes.Stack,
    ElementTypes.Text,
    ElementTypes.RichText,
    ElementTypes.Divider,
    ElementTypes.Image,
  ];

  public static readonly FLOW_BUILDER_FORM_ALLOWED_RESOURCE_TYPES: string[] = [
    ElementTypes.TextInput,
    ElementTypes.PasswordInput,
    ElementTypes.EmailInput,
    ElementTypes.PhoneInput,
    ElementTypes.NumberInput,
    ElementTypes.DateInput,
    ElementTypes.OtpInput,
    ElementTypes.Checkbox,
    ElementTypes.Dropdown,
    ElementTypes.Select,
    ElementTypes.Action,
    ElementTypes.Resend,
    ElementTypes.Icon,
    ElementTypes.Stack,
    ElementTypes.Text,
    ElementTypes.RichText,
    ElementTypes.Divider,
    ElementTypes.Image,
    ElementTypes.DynamicInputPlaceholder,
    ElementTypes.Timer,
    ElementTypes.Custom,
  ];

  public static readonly FLOW_BUILDER_STACK_ALLOWED_RESOURCE_TYPES: string[] = [
    ElementTypes.Action,
    ElementTypes.Resend,
    ElementTypes.Icon,
    ElementTypes.Stack,
    ElementTypes.Text,
    ElementTypes.RichText,
    ElementTypes.Divider,
    ElementTypes.Image,
  ];

  public static readonly FLOW_BUILDER_STATIC_CONTENT_ALLOWED_RESOURCE_TYPES: string[] = [
    ElementTypes.Icon,
    ElementTypes.Stack,
    ElementTypes.Text,
    ElementTypes.RichText,
    ElementTypes.Divider,
    ElementTypes.Image,
  ];

  public static readonly FLOW_BUILDER_STATIC_CONTENT_ALLOWED_EXECUTION_TYPES: ExecutionTypes[] = [
    ExecutionTypes.MagicLinkExecutor,
  ];
}

export default VisualFlowConstants;
