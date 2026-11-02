// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Alert, Snackbar} from '@wso2/oxygen-ui';
import {useState, useCallback, useMemo} from 'react';
import type {SyntheticEvent, PropsWithChildren, JSX} from 'react';
import ToastContext, {type ToastSeverity} from './ToastContext';

/**
 * Internal state shape for the active toast notification.
 */
interface ToastState {
  id: number;
  open: boolean;
  message: string;
  severity: ToastSeverity;
  durationMs: number;
}

const DEFAULT_TOAST_DURATION_MS = 6000;

/**
 * Props for the ToastProvider component.
 *
 * @public
 */
export type ToastProviderProps = PropsWithChildren;

/**
 * React context provider component that enables toast notifications throughout the application.
 *
 * This component manages the lifecycle of a single snackbar notification rendered at the
 * bottom-right of the viewport. It exposes a `showToast` function via `ToastContext` so
 * that any descendant component or hook can trigger a notification without needing to manage
 * local state.
 *
 * Wrap your application (or a subtree) with this provider and consume notifications using
 * the `useToast` hook.
 *
 * @example
 * Basic setup in the application root:
 * ```tsx
 * import ToastProvider from './ToastProvider';
 *
 * function App() {
 *   return (
 *     <ToastProvider>
 *       <Routes />
 *     </ToastProvider>
 *   );
 * }
 * ```
 *
 * @example
 * Triggering a toast from a mutation hook:
 * ```ts
 * import useToast from './useToast';
 *
 * function useCreateItem() {
 *   const { showToast } = useToast();
 *
 *   return useMutation({
 *     mutationFn: createItem,
 *     onSuccess: () => showToast('Item created successfully.', 'success'),
 *     onError: () => showToast('Failed to create item.', 'error'),
 *   });
 * }
 * ```
 *
 * @public
 */
export default function ToastProvider({children}: ToastProviderProps): JSX.Element {
  const [toast, setToast] = useState<ToastState>({
    id: 0,
    open: false,
    message: '',
    severity: 'success',
    durationMs: DEFAULT_TOAST_DURATION_MS,
  });

  const showToast = useCallback(
    (message: string, severity: ToastSeverity = 'success', durationMs: number = DEFAULT_TOAST_DURATION_MS): void => {
      // A new id remounts the Snackbar so its auto-hide timer restarts even when a toast is already open.
      setToast((prev) => ({id: prev.id + 1, open: true, message, severity, durationMs}));
    },
    [],
  );

  const handleClose = useCallback((_event?: SyntheticEvent | Event, reason?: string): void => {
    if (reason === 'clickaway') return;
    setToast((prev) => ({...prev, open: false}));
  }, []);

  const contextValue = useMemo(() => ({showToast}), [showToast]);

  return (
    <ToastContext.Provider value={contextValue}>
      {children}
      <Snackbar
        key={toast.id}
        open={toast.open}
        autoHideDuration={toast.durationMs}
        onClose={handleClose}
        anchorOrigin={{vertical: 'bottom', horizontal: 'right'}}
      >
        <Alert onClose={handleClose} severity={toast.severity} sx={{width: '100%'}}>
          {toast.message}
        </Alert>
      </Snackbar>
    </ToastContext.Provider>
  );
}
