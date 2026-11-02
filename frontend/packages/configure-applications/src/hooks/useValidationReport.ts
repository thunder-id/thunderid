// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useEffect, useRef} from 'react';

/**
 * Reports `hasError` to `onValidationChange` whenever it changes, and reports `errorAfterUnmount`
 * once the component unmounts. A draft-based error is gone with the component, so the default
 * after unmount is `false`; an error on a committed value can be kept by passing it here.
 *
 * The callback is held in a ref, so the unmount report goes to the latest callback and a new
 * callback identity does not report a mounted, invalid component as valid.
 *
 * @param onValidationChange - The parent's report callback, if any
 * @param hasError - Whether the component currently has a validation error
 * @param errorAfterUnmount - What to report when the component unmounts
 */
export default function useValidationReport(
  onValidationChange: ((hasError: boolean) => void) | undefined,
  hasError: boolean,
  errorAfterUnmount = false,
): void {
  useEffect(() => {
    onValidationChange?.(hasError);
  }, [hasError, onValidationChange]);

  const latest = useRef({onValidationChange, errorAfterUnmount});
  useEffect(() => {
    latest.current = {onValidationChange, errorAfterUnmount};
  }, [onValidationChange, errorAfterUnmount]);
  useEffect(() => () => latest.current.onValidationChange?.(latest.current.errorAfterUnmount), []);
}
