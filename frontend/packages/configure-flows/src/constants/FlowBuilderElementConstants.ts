// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Constants related to the flow builder elements.
 *
 * @remarks
 * This class is not meant to be instantiated. It only provides static constants.
 *
 * @example
 * ```typescript
 * const confirmPasswordIdentifier = FlowBuilderElementConstants.CONFIRM_PASSWORD_IDENTIFIER;
 * ```
 */
class FlowBuilderElementConstants {
  /**
   * Private constructor to avoid object instantiation from outside the class.
   */
  // eslint-disable-next-line @typescript-eslint/no-empty-function
  private constructor() {}

  public static readonly PASSWORD_IDENTIFIER: string = 'password';

  public static readonly CONFIRM_PASSWORD_IDENTIFIER: string = 'confirmPassword';

  public static readonly DEFAULT_CAPTCHA_PROVIDER: string = 'ReCAPTCHA V2';
}

export default FlowBuilderElementConstants;
