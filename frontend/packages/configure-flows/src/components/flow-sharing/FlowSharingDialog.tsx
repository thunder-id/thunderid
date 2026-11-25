// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {PageLoadingAnimation} from '@thunderid/components';
import {
  OrganizationAccessSettings,
  type OrganizationAccessState,
  type OverlayRules,
} from '@thunderid/configure-resource-sharing';
import type {FlowVariableDefinition} from '@thunderid/react';
import {getErrorMessage} from '@thunderid/utils';
import {
  Alert,
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogTitle,
  Divider,
  IconButton,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import {Pencil, X} from '@wso2/oxygen-ui-icons-react';
import {useCallback, useState, type ReactElement} from 'react';
import {useTranslation} from 'react-i18next';
import UnitTermsDrawer from './UnitTermsDrawer';
import VariableSharingRules from './VariableSharingRules';
import useConsoleSurface from '../variables-dialog/useConsoleSurface';
import VariableChangeDialog from '../variables-dialog/VariableChangeDialog';

/**
 * The flow the dialog shares.
 */
export interface SharedFlow {
  id: string;
  /**
   * Whether a resource file declares the flow, which leaves its sharing as it is.
   */
  isReadOnly?: boolean;
  name: string;
  /**
   * The organization unit that owns the flow.
   */
  ouId: string;
}

/**
 * Props interface of {@link FlowSharingDialog}
 */
export interface FlowSharingDialogProps {
  /**
   * The flow's organization access, held by the editor so the flow's own Save saves it, after the flow.
   */
  access: OrganizationAccessState;
  flow: SharedFlow;
  onClose: () => void;
  open: boolean;
  /**
   * The variables the flow declares now.
   */
  variables: FlowVariableDefinition[];
}

/**
 * A full-screen view for sharing a flow: which organization units can use it, laid out as the other shareable
 * resources lay it out, and what those units may do with the flow's variables.
 *
 * @param props - Props injected to the component.
 * @returns The FlowSharingDialog component.
 */
function FlowSharingDialog({access, flow, onClose, open, variables}: FlowSharingDialogProps): ReactElement {
  const {t} = useTranslation();
  // Resolves an error through the `flows` catalog. `t` defaults to the `common` namespace, so this forwards explicit
  // `ns:` prefixes unchanged and prefixes bare keys with `flows:`, per getErrorMessage's namespace-resolution contract.
  const tForErrors = useCallback(
    (key: string, options?: Record<string, unknown>): string => t(key.includes(':') ? key : `flows:${key}`, options),
    [t],
  );
  const {pageSx} = useConsoleSurface(open);
  const {draft, updateDraft} = access;
  const locked: boolean = flow.isReadOnly === true || access.isDeclared;
  const reachesAllChildUnits: boolean = draft.scope === 'chosen' && draft.childUnitsAccess === 'all';
  const reachesSelectedUnits: boolean = draft.scope === 'chosen' && draft.childUnitsAccess === 'selected';
  const [expandedOuId, setExpandedOuId] = useState<string | null>(null);
  const [isStartingOpen, setIsStartingOpen] = useState<boolean>(false);
  const [startingToReplace, setStartingToReplace] = useState<OverlayRules | null>(null);

  const unitRules = (ouId: string): OverlayRules | undefined =>
    draft.childUnits.find((unit) => unit.ouId === ouId)?.rules;

  const changeUnitRules = (ouId: string, rules: OverlayRules): void =>
    updateDraft((current) => ({
      ...current,
      childUnits: current.childUnits.map((unit) => (unit.ouId === ouId ? {...unit, rules} : unit)),
    }));

  // Starting terms are copied to each unit once, and each unit keeps its own copy after that.
  const copyStartingTerms = (rules: OverlayRules): void => {
    updateDraft((current) => ({
      ...current,
      childUnits: current.childUnits.map((unit) => ({...unit, rules: {...rules}})),
    }));
    setStartingToReplace(null);
    setIsStartingOpen(false);
  };

  // The copy replaces every unit's own terms, so it asks first when a unit already has some.
  const applyStartingTerms = (rules: OverlayRules): void => {
    if (draft.childUnits.some((unit) => Object.keys(unit.rules ?? {}).length > 0)) {
      setStartingToReplace(rules);
      return;
    }

    copyStartingTerms(rules);
  };

  const changeRules = (rules: OverlayRules): void => updateDraft((current) => ({...current, childUnitRules: rules}));

  return (
    <Dialog
      fullScreen
      open={open}
      onClose={onClose}
      slotProps={{paper: {sx: pageSx}}}
      data-testid="flow-sharing-dialog"
    >
      <DialogTitle sx={{alignItems: 'center', display: 'flex', justifyContent: 'space-between'}}>
        {t('flows:core.sharing.dialog.title', 'Share flow')}
        <IconButton onClick={onClose} aria-label={t('common:actions.close', 'Close')}>
          <X size={20} />
        </IconButton>
      </DialogTitle>
      <Divider />
      <Box sx={{flex: 1, overflowY: 'auto', p: 4}}>
        <Stack spacing={3} sx={{maxWidth: 1000, mx: 'auto', width: '100%'}}>
          {access.isLoading ? (
            <PageLoadingAnimation />
          ) : (
            <>
              <OrganizationAccessSettings
                access={access}
                ownerOuId={flow.ouId}
                resourceName={flow.name}
                readOnly={locked}
                hasUnitSettings
                errorNamespace="flows"
                renderChildUnitExtra={
                  variables.length > 0
                    ? (ouId: string, name: string) => (
                        <Box sx={{mt: 1}}>
                          <Button
                            size="small"
                            variant="text"
                            aria-label={t('flows:core.sharing.terms.editFor', 'Edit terms for {{name}}', {name})}
                            aria-expanded={expandedOuId === ouId}
                            startIcon={<Pencil size={14} />}
                            onClick={() => setExpandedOuId((current) => (current === ouId ? null : ouId))}
                            sx={{textTransform: 'none'}}
                          >
                            {t('flows:core.sharing.terms.edit', 'Edit terms')}
                          </Button>
                          {expandedOuId === ouId && (
                            <Box sx={{mt: 1}}>
                              <VariableSharingRules
                                bare
                                variables={variables}
                                ownerOuId={flow.ouId}
                                rules={unitRules(ouId)}
                                onChange={(rules: OverlayRules) => changeUnitRules(ouId, rules)}
                                disabled={locked}
                              />
                            </Box>
                          )}
                        </Box>
                      )
                    : undefined
                }
                childUnitsAction={
                  variables.length > 0 && draft.childUnits.length > 0 ? (
                    <Button
                      variant="outlined"
                      size="small"
                      onClick={() => setIsStartingOpen(true)}
                      data-testid="flow-sharing-starting-terms"
                    >
                      {t('flows:core.sharing.terms.setStarting', 'Set starting terms')}
                    </Button>
                  ) : undefined
                }
              />
              {variables.length === 0 && (
                <Alert severity="info" data-testid="flow-sharing-no-terms">
                  {t(
                    'flows:core.sharing.terms.noVariables',
                    'No terms are available. This flow declares no variables, so there is nothing for the organization units to set. Add a Variables step to the flow to offer some.',
                  )}
                </Alert>
              )}
              {variables.length > 0 &&
                !reachesSelectedUnits &&
                (reachesAllChildUnits ? (
                  <VariableSharingRules
                    variables={variables}
                    ownerOuId={flow.ouId}
                    rules={draft.childUnitRules}
                    onChange={changeRules}
                    disabled={locked}
                  />
                ) : (
                  <Typography variant="body2" color="text.secondary" data-testid="flow-sharing-variables-hint">
                    {t(
                      'flows:core.sharing.variables.needsChildUnits',
                      'Share the flow with child organization units to choose what they may do with its variables.',
                    )}
                  </Typography>
                ))}
            </>
          )}
          {access.saveError && (
            <Alert severity="error">
              {getErrorMessage(
                access.saveError,
                tForErrors,
                'core.sharing.dialog.saveError',
                'Failed to update how the flow is shared. Please try again.',
              )}
            </Alert>
          )}
        </Stack>
      </Box>
      {isStartingOpen && (
        <UnitTermsDrawer
          title={t(
            'flows:core.sharing.terms.startingTitle',
            'Starting terms for {{count}} granted organization units',
            {
              count: draft.childUnits.length,
            },
          )}
          subtitle={t(
            'flows:core.sharing.terms.startingSubtitle',
            'Fill them in once and they are copied to every organization unit below. Afterwards each organization unit keeps its own copy, changed from its own row.',
          )}
          applyLabel={t('flows:core.sharing.terms.copyTo', 'Copy to {{count}} organization units', {
            count: draft.childUnits.length,
          })}
          variables={variables}
          disabled={locked}
          onClose={() => setIsStartingOpen(false)}
          onApply={applyStartingTerms}
        />
      )}
      <Divider />
      <DialogActions sx={{px: 3, py: 2}}>
        {access.hasChanges && !locked && (
          <Typography variant="body2" color="text.secondary" sx={{flex: 1}} data-testid="flow-sharing-saved-with-flow">
            {t('flows:core.sharing.dialog.savedWithFlow', 'These changes are saved when you save the flow.')}
          </Typography>
        )}
        <Button variant="contained" onClick={onClose} data-testid="flow-sharing-done">
          {t('common:actions.done', 'Done')}
        </Button>
      </DialogActions>
      <VariableChangeDialog
        open={startingToReplace !== null}
        title={t('flows:core.sharing.terms.replaceTitle', 'Replace the terms of your organization units?')}
        details={[
          t(
            'flows:core.sharing.terms.replaceDetail',
            'Some organization units already have their own terms. Copying the starting terms replaces them for every organization unit.',
          ),
        ]}
        confirmLabel={t('flows:core.sharing.terms.replaceConfirm', 'Replace')}
        onClose={() => setStartingToReplace(null)}
        onConfirm={() => startingToReplace && copyStartingTerms(startingToReplace)}
      />
    </Dialog>
  );
}

export default FlowSharingDialog;
