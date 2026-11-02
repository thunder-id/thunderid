// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useCallback, useState} from 'react';

/**
 * State for a snackbar notification.
 */
export interface SnackbarState {
  /** Whether the snackbar is open. */
  open: boolean;
  /** The message to display. */
  message: string;
}

/**
 * Return type for the useSnackbarNotifications hook.
 */
export interface UseSnackbarNotificationsReturn {
  /** Error snackbar state. */
  errorSnackbar: SnackbarState;
  /** Success snackbar state. */
  successSnackbar: SnackbarState;
  /** Info/hint snackbar state. */
  infoSnackbar: SnackbarState;
  /** Show an error notification. */
  showError: (message: string) => void;
  /** Show a success notification. */
  showSuccess: (message: string) => void;
  /** Show an info/hint notification. */
  showInfo: (message: string) => void;
  /** Close the error snackbar. */
  handleCloseErrorSnackbar: () => void;
  /** Close the success snackbar. */
  handleCloseSuccessSnackbar: () => void;
  /** Close the info snackbar. */
  handleCloseInfoSnackbar: () => void;
}

/**
 * Hook to manage snackbar notification state for error and success messages.
 *
 * @returns Snackbar state and handlers.
 *
 * @example
 * ```tsx
 * const { errorSnackbar, successSnackbar, showError, showSuccess, handleCloseErrorSnackbar, handleCloseSuccessSnackbar } = useSnackbarNotifications();
 *
 * // Show notifications
 * showError('Something went wrong');
 * showSuccess('Flow saved successfully');
 *
 * // In JSX
 * <Snackbar open={errorSnackbar.open} onClose={handleCloseErrorSnackbar}>
 *   <Alert severity="error">{errorSnackbar.message}</Alert>
 * </Snackbar>
 * ```
 */
const useSnackbarNotifications = (): UseSnackbarNotificationsReturn => {
  const [errorSnackbar, setErrorSnackbar] = useState<SnackbarState>({
    open: false,
    message: '',
  });

  const [successSnackbar, setSuccessSnackbar] = useState<SnackbarState>({
    open: false,
    message: '',
  });

  const [infoSnackbar, setInfoSnackbar] = useState<SnackbarState>({
    open: false,
    message: '',
  });

  /**
   * Show an error notification.
   */
  const showError = useCallback((message: string) => {
    setErrorSnackbar({open: true, message});
  }, []);

  /**
   * Show a success notification.
   */
  const showSuccess = useCallback((message: string) => {
    setSuccessSnackbar({open: true, message});
  }, []);

  /**
   * Show an info/hint notification.
   */
  const showInfo = useCallback((message: string) => {
    setInfoSnackbar({open: true, message});
  }, []);

  /**
   * Close the error snackbar.
   */
  const handleCloseErrorSnackbar = useCallback(() => {
    setErrorSnackbar((prev) => ({...prev, open: false}));
  }, []);

  /**
   * Close the success snackbar.
   */
  const handleCloseSuccessSnackbar = useCallback(() => {
    setSuccessSnackbar((prev) => ({...prev, open: false}));
  }, []);

  /**
   * Close the info snackbar.
   */
  const handleCloseInfoSnackbar = useCallback(() => {
    setInfoSnackbar((prev) => ({...prev, open: false}));
  }, []);

  return {
    errorSnackbar,
    successSnackbar,
    infoSnackbar,
    showError,
    showSuccess,
    showInfo,
    handleCloseErrorSnackbar,
    handleCloseSuccessSnackbar,
    handleCloseInfoSnackbar,
  };
};

export default useSnackbarNotifications;
