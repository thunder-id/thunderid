// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {QueryKey} from '@tanstack/react-query';
import {useToast} from '@thunderid/contexts';
import type {AccessStatus, AccessStatusReport} from '@thunderid/types';
import {getErrorMessage} from '@thunderid/utils';
import {Button, ListItemText, Menu, MenuItem} from '@wso2/oxygen-ui';
import {ChevronDown, ChevronUp} from '@wso2/oxygen-ui-icons-react';
import type {TFunction} from 'i18next';
import {useCallback, useEffect, useState, type JSX, type MouseEvent} from 'react';
import {useTranslation} from 'react-i18next';
import {AccessStatuses} from './describeAccessStatus';
import {
  LIFECYCLE_ACTION_STEPS,
  LifecycleActionIncompleteError,
  lifecycleActionLabel,
  type LifecycleAction,
  type LifecycleCategory,
} from './lifecycleAction';
import LifecycleConfirmDialog from './LifecycleConfirmDialog';
import useAvailableLifecycleActions from './useAvailableLifecycleActions';
import useLifecycleAction from './useLifecycleAction';

export interface AccountStatusActionProps {
  /**
   * Entity id the operations act on.
   */
  subject: string;
  /**
   * Effective access status, as returned by the read that rendered this page.
   */
  status?: AccessStatusReport;
  /**
   * Identity category, which decides the operations offered.
   */
  category: LifecycleCategory;
  /**
   * The host package's list query for this category, invalidated after an operation so the list shows
   * the new status.
   */
  listQueryKey: QueryKey;
  /**
   * Called after a successful operation so the host can refetch the record it is showing. Pass a stable
   * function.
   */
  onChanged?: () => void;
}

/**
 * Actions each status offers, most expected first. A suspended record offers no bare Unlock, since the
 * read does not show a lock beneath the suspension.
 */
const ACTIONS_FOR_STATUS: Record<AccessStatus, LifecycleAction[]> = {
  ACTIVE: ['suspend'],
  // A lock beneath an ended lifespan is not shown, so only Suspend is offered.
  EXPIRED: ['suspend'],
  LOCKED: ['unlock', 'suspend'],
  SUSPENDED: ['unsuspend', 'unsuspendAndUnlock'],
};

/**
 * Actions offered only to some categories. Only a user is left locked by an unsuspend.
 */
const ACTION_CATEGORIES: Partial<Record<LifecycleAction, LifecycleCategory[]>> = {
  unsuspendAndUnlock: ['user'],
};

/**
 * The menu hint for an action. Only the combinations offered are distinguished: applications are never
 * locked, and only users are offered Unsuspend and unlock.
 */
function actionHint(operation: LifecycleAction, category: LifecycleCategory, t: TFunction): string {
  switch (operation) {
    case 'unlock':
      return category === 'agent'
        ? t('common:accountStatus.unlock.hint.agent', 'Remove all authentication locks.')
        : t('common:accountStatus.unlock.hint.user', 'Remove all sign-in locks.');
    case 'unsuspend':
      if (category === 'agent') {
        return t('common:accountStatus.unsuspend.hint.agent', 'Remove the suspension.');
      }
      if (category === 'application') {
        return t('common:accountStatus.unsuspend.hint.application', 'Remove the suspension.');
      }
      return t(
        'common:accountStatus.unsuspend.hint.user',
        'Remove the suspension. The account remains locked until the user completes account recovery or an administrator unlocks it.',
      );
    case 'unsuspendAndUnlock':
      return t(
        'common:accountStatus.unsuspendAndUnlock.hint.user',
        'Remove the suspension and unlock. The account can sign in right away.',
      );
    default:
      if (category === 'agent') {
        return t('common:accountStatus.suspend.hint.agent', 'Prevent authentication and revoke existing access.');
      }
      if (category === 'application') {
        return t('common:accountStatus.suspend.hint.application', 'Prevent authentication and revoke existing access.');
      }
      return t('common:accountStatus.suspend.hint.user', 'Prevent sign-in and revoke existing access.');
  }
}

/**
 * The message for a failed action, named for the category.
 */
function errorMessage(error: Error, category: LifecycleCategory, t: TFunction): string {
  switch (category) {
    case 'agent':
      return getErrorMessage(
        error,
        t,
        'common:accountStatus.error.agent',
        'The agent operation could not be completed.',
      );
    case 'application':
      return getErrorMessage(
        error,
        t,
        'common:accountStatus.error.application',
        'The application operation could not be completed.',
      );
    default:
      return getErrorMessage(error, t, 'common:accountStatus.error.user', 'The user operation could not be completed.');
  }
}

const AVAILABILITY_ERROR_FALLBACK = 'The account actions could not be loaded. Reload the page to try again.';

/**
 * The page header's Actions menu for the lifecycle operations the record's status offers. Renders
 * nothing while the flows resolve or when none is available, and toasts when they cannot be resolved.
 */
export default function AccountStatusAction({
  subject,
  status = undefined,
  category,
  listQueryKey,
  onChanged = undefined,
}: AccountStatusActionProps): JSX.Element | null {
  const {t} = useTranslation('common');
  const {showToast} = useToast();
  const {available, error: availabilityError, isPending} = useAvailableLifecycleActions(category);
  const [confirming, setConfirming] = useState<LifecycleAction | null>(null);
  // Kept so the dialog's content holds still through its exit fade.
  const [lastConfirming, setLastConfirming] = useState<LifecycleAction>('suspend');
  const [menuAnchor, setMenuAnchor] = useState<HTMLElement | null>(null);

  // The page stays usable without the menu, so a failed resolution only toasts.
  useEffect(() => {
    if (availabilityError) {
      showToast(
        getErrorMessage(availabilityError, t, 'common:accountStatus.availabilityError', AVAILABILITY_ERROR_FALLBACK),
        'error',
      );
    }
  }, [availabilityError, showToast, t]);

  const action = useLifecycleAction(listQueryKey, () => {
    setConfirming(null);
    onChanged?.();
  });
  const {mutate, reset} = action;

  // An action is offered only when every one of its steps has a flow.
  const flowsFor = useCallback(
    (choice: LifecycleAction): string[] | undefined => {
      const flowIds = LIFECYCLE_ACTION_STEPS[choice].map((operation) => available[operation]);
      return flowIds.every(Boolean) ? (flowIds as string[]) : undefined;
    },
    [available],
  );

  const run = useCallback(
    (choice: LifecycleAction, operatorNote?: string): void => {
      const flowIds = flowsFor(choice);
      if (!flowIds) {
        return;
      }
      mutate(
        {action: choice, category, flowIds, operatorNote, subject},
        {
          // Dialog actions show their error in the dialog; unlock has no dialog, so it toasts.
          onError: (error: Error): void => {
            if (error instanceof LifecycleActionIncompleteError) {
              setConfirming(null);
              // The toast reports it; the dialog fading out must not show it too.
              reset();
              onChanged?.();
              showToast(
                t(
                  'common:accountStatus.unsuspendAndUnlock.incomplete',
                  'The suspension was removed, but the unlock failed. Unlock it from the Actions menu.',
                ),
                'error',
              );
              return;
            }
            if (choice === 'unlock') {
              showToast(errorMessage(error, category, t), 'error');
            }
          },
        },
      );
    },
    [category, flowsFor, mutate, onChanged, reset, showToast, subject, t],
  );

  const effective: AccessStatus = status?.value ?? AccessStatuses.ACTIVE;
  // Indexed, not asserted: an unknown status offers nothing.
  const offered = (ACTIONS_FOR_STATUS[effective] ?? []).filter(
    (choice) => (ACTION_CATEGORIES[choice]?.includes(category) ?? true) && flowsFor(choice),
  );

  if (isPending || offered.length === 0) {
    return null;
  }

  const choose = (operation: LifecycleAction): void => {
    setMenuAnchor(null);
    action.reset();
    // Unlock is remedial, so it runs straight from the menu.
    if (operation === 'unlock') {
      run(operation);
    } else {
      setLastConfirming(operation);
      setConfirming(operation);
    }
  };

  return (
    <>
      <Button
        size="small"
        variant="outlined"
        disabled={action.isPending}
        data-testid="account-status-actions"
        aria-haspopup="menu"
        aria-expanded={menuAnchor !== null}
        endIcon={menuAnchor !== null ? <ChevronUp size={16} /> : <ChevronDown size={16} />}
        onClick={(event: MouseEvent<HTMLElement>): void => setMenuAnchor(event.currentTarget)}
      >
        {t('common:accountStatus.actions', 'Actions')}
      </Button>
      <Menu
        anchorEl={menuAnchor}
        open={menuAnchor !== null}
        onClose={(): void => setMenuAnchor(null)}
        anchorOrigin={{horizontal: 'right', vertical: 'bottom'}}
        transformOrigin={{horizontal: 'right', vertical: 'top'}}
      >
        {offered.map((operation) => (
          <MenuItem key={operation} onClick={(): void => choose(operation)} sx={{maxWidth: 360, whiteSpace: 'normal'}}>
            <ListItemText
              primary={lifecycleActionLabel(operation, t)}
              secondary={actionHint(operation, category, t)}
              slotProps={{primary: {color: operation === 'suspend' ? 'error' : undefined}}}
            />
          </MenuItem>
        ))}
      </Menu>
      <LifecycleConfirmDialog
        open={confirming !== null}
        operation={lastConfirming}
        category={category}
        submitting={action.isPending}
        error={action.isError ? errorMessage(action.error, category, t) : undefined}
        onCancel={(): void => {
          setConfirming(null);
          action.reset();
        }}
        onConfirm={(operatorNote?: string): void => {
          if (confirming) {
            run(confirming, operatorNote);
          }
        }}
      />
    </>
  );
}
