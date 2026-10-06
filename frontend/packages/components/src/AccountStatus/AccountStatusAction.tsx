// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useToast} from '@thunderid/contexts';
import type {AccessStatus, AccessStatusReport, LifecycleCategory} from '@thunderid/types';
import {getErrorMessage} from '@thunderid/utils';
import {Button, Menu, MenuItem, Typography} from '@wso2/oxygen-ui';
import {ChevronDown, ChevronUp} from '@wso2/oxygen-ui-icons-react';
import {useCallback, useState, type JSX, type MouseEvent} from 'react';
import {useTranslation} from 'react-i18next';
import {LIFECYCLE_ACTION_STEPS, LifecycleActionIncompleteError, type LifecycleAction} from './lifecycleAction';
import LifecycleConfirmDialog from './LifecycleConfirmDialog';
import useLifecycleAction from './useLifecycleAction';
import useLifecycleActions from './useLifecycleActions';

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
   * Called after a successful operation so the host can refetch the record it is showing.
   */
  onChanged?: () => void;
}

/**
 * Actions each status offers, most expected first. A suspended record offers no bare Unlock, since
 * the read does not show a lock beneath the suspension. There is no operator lock.
 */
const ACTIONS_FOR_STATUS: Record<AccessStatus, LifecycleAction[]> = {
  ACTIVE: ['suspend'],
  LOCKED: ['unlock', 'suspend'],
  SUSPENDED: ['unsuspend', 'unsuspendAndUnlock'],
};

/**
 * Actions offered only to some categories. Unsuspend and unlock exists to skip the post-suspension
 * lock, which only a user receives.
 */
const ACTION_CATEGORIES: Partial<Record<LifecycleAction, LifecycleCategory[]>> = {
  unsuspendAndUnlock: ['user'],
};

const ACTION_LABEL: Record<LifecycleAction, {key: string; fallback: string}> = {
  suspend: {fallback: 'Suspend', key: 'common:accountStatus.suspend.action'},
  unlock: {fallback: 'Unlock', key: 'common:accountStatus.unlock.action'},
  unsuspend: {fallback: 'Unsuspend', key: 'common:accountStatus.unsuspend.action'},
  unsuspendAndUnlock: {fallback: 'Unsuspend and unlock', key: 'common:accountStatus.unsuspendAndUnlock.action'},
};

/**
 * One-line menu hint per operation, with a user-specific wording where "sign in" applies.
 */
const ACTION_HINT: Record<LifecycleAction, Partial<Record<LifecycleCategory, string>> & {fallback: string}> = {
  suspend: {
    fallback: 'Prevent authentication and revoke existing access.',
    user: 'Prevent sign-in and revoke existing access.',
  },
  unlock: {fallback: 'Clear the failed sign-in lock.'},
  unsuspend: {
    fallback: 'Remove the suspension.',
    user: 'Remove the suspension. The account remains locked until an administrator unlocks it or the user completes self-recovery.',
  },
  unsuspendAndUnlock: {fallback: 'Remove the suspension and unlock. The account can sign in right away.'},
};

/**
 * Resolves an operation's hint for the category being rendered, falling back to the neutral wording.
 */
function hintFor(
  operation: LifecycleAction,
  category: LifecycleCategory,
  t: (key: string, fallback: string) => string,
): string {
  const hint = ACTION_HINT[operation];

  return t(`common:accountStatus.${operation}.hint.${category}`, hint[category] ?? hint.fallback);
}

/**
 * The operations that withhold access, which are the ones rendered in the destructive colour.
 */
const WITHHOLDING: ReadonlySet<LifecycleAction> = new Set<LifecycleAction>(['suspend']);

/**
 * The page header's Actions menu for the lifecycle operations the record's status offers.
 * Renders nothing while the flows resolve or when the deployment configures none.
 */
export default function AccountStatusAction({
  subject,
  status = undefined,
  category,
  onChanged = undefined,
}: AccountStatusActionProps): JSX.Element | null {
  const {t} = useTranslation('common');
  const {showToast} = useToast();
  const {available, isPending} = useLifecycleActions(category);
  const [confirming, setConfirming] = useState<LifecycleAction | null>(null);
  // Last operation confirmed, kept so the dialog's content holds still through its exit fade.
  const [lastConfirming, setLastConfirming] = useState<LifecycleAction>('suspend');
  const [menuAnchor, setMenuAnchor] = useState<HTMLElement | null>(null);

  const action = useLifecycleAction(() => {
    setConfirming(null);
    onChanged?.();
  });

  // Every step of an action needs its flow; an action with a missing step is not offered.
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

      action.mutate(
        {action: choice, flowIds, operatorNote, subject},
        {
          // Unlock has no dialog to carry a failure, so its error goes to the toast rail. The others
          // report inside the dialog they were fired from, which stays open on failure, unless an
          // earlier step already changed the record: then the dialog closes on the refetched status.
          onError: (error: Error): void => {
            if (error instanceof LifecycleActionIncompleteError) {
              setConfirming(null);
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
              showToast(
                getErrorMessage(
                  error,
                  t,
                  'common:accountStatus.error',
                  'The account operation could not be completed.',
                ),
                'error',
              );
            }
          },
        },
      );
    },
    [action, flowsFor, onChanged, showToast, subject, t],
  );

  const effective: AccessStatus = status?.value ?? 'ACTIVE';
  // Indexed, not asserted: a status this build does not know has no action table, and offering the
  // active account's Suspend for it would act on a state nobody here can describe.
  const offered = (ACTIONS_FOR_STATUS[effective] ?? []).filter(
    (choice) => (ACTION_CATEGORIES[choice]?.includes(category) ?? true) && flowsFor(choice),
  );

  // Nothing to render while the flows resolve, and nothing when the deployment configures none.
  if (isPending || offered.length === 0) {
    return null;
  }

  const choose = (operation: LifecycleAction): void => {
    setMenuAnchor(null);
    // A failure from the previous action must not appear in the next one's dialog.
    action.reset();

    // Unlock is remedial and reversible by simply failing again, so it runs straight from the menu.
    // The others change what the account can do and confirm first.
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
        aria-haspopup="menu"
        aria-expanded={menuAnchor !== null}
        // The caret points the way the menu will move, so the control says whether it is open
        // without the operator having to look at the menu to find out.
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
          <MenuItem key={operation} onClick={(): void => choose(operation)}>
            <span>
              <Typography variant="body2" color={WITHHOLDING.has(operation) ? 'error' : undefined}>
                {t(ACTION_LABEL[operation].key, ACTION_LABEL[operation].fallback)}
              </Typography>
              <Typography variant="caption" color="text.secondary" display="block">
                {hintFor(operation, category, t)}
              </Typography>
            </span>
          </MenuItem>
        ))}
      </Menu>

      <LifecycleConfirmDialog
        open={confirming !== null}
        operation={lastConfirming}
        category={category}
        submitting={action.isPending}
        error={
          action.isError
            ? getErrorMessage(
                action.error,
                t,
                'common:accountStatus.error',
                'The account operation could not be completed.',
              )
            : undefined
        }
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
