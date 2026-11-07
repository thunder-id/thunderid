// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  OrganizationAccessSettings,
  useOrganizationAccess,
  type OrganizationAccessState,
  type OverlayRules,
} from '@thunderid/configure-resource-sharing';
import type {FlowVariableDefinition} from '@thunderid/react';
import {
  Alert,
  Box,
  Button,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogTitle,
  Divider,
  IconButton,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import {X} from '@wso2/oxygen-ui-icons-react';
import {useState, type ReactElement} from 'react';
import {useTranslation} from 'react-i18next';
import UnitTermsDrawer from './UnitTermsDrawer';
import VariableSharingRules from './VariableSharingRules';
import useConsoleSurface from '../variables-dialog/useConsoleSurface';

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
  flow: SharedFlow;
  /**
   * Whether the flow has changes that are not saved. A variable added since the last save cannot be shared yet.
   */
  isDirty?: boolean;
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
function FlowSharingDialog({flow, isDirty = false, onClose, open, variables}: FlowSharingDialogProps): ReactElement {
  const {t} = useTranslation();
  const {pageSx} = useConsoleSurface();
  const access: OrganizationAccessState = useOrganizationAccess({resourcePath: `flows/${flow.id}`, enabled: open});

  const {draft, updateDraft} = access;
  const locked: boolean = flow.isReadOnly === true || access.isDeclared;
  const reachesAllChildUnits: boolean = draft.scope === 'chosen' && draft.childUnitsAccess === 'all';
  const reachesSelectedUnits: boolean = draft.scope === 'chosen' && draft.childUnitsAccess === 'selected';
  const [expandedOuId, setExpandedOuId] = useState<string | null>(null);
  const [isStartingOpen, setIsStartingOpen] = useState<boolean>(false);

  const unitRules = (ouId: string): OverlayRules | undefined =>
    draft.childUnits.find((unit) => unit.ouId === ouId)?.rules;

  const changeUnitRules = (ouId: string, rules: OverlayRules): void =>
    updateDraft((current) => ({
      ...current,
      childUnits: current.childUnits.map((unit) => (unit.ouId === ouId ? {...unit, rules} : unit)),
    }));

  // Starting terms are copied to each unit once, and each unit keeps its own copy after that.
  const applyStartingTerms = (rules: OverlayRules): void => {
    updateDraft((current) => ({
      ...current,
      childUnits: current.childUnits.map((unit) => ({...unit, rules: {...rules}})),
    }));
    setIsStartingOpen(false);
  };

  const close = (): void => {
    access.reset();
    onClose();
  };

  const save = async (): Promise<void> => {
    try {
      await access.save();
      onClose();
    } catch {
      // The failure is shown from the state the save leaves behind.
    }
  };

  const changeRules = (rules: OverlayRules): void => updateDraft((current) => ({...current, childUnitRules: rules}));

  return (
    <Dialog fullScreen open={open} onClose={close} slotProps={{paper: {sx: pageSx}}} data-testid="flow-sharing-dialog">
      <DialogTitle sx={{alignItems: 'center', display: 'flex', justifyContent: 'space-between'}}>
        {t('flows:core.sharing.dialog.title', 'Share flow')}
        <IconButton onClick={close} aria-label={t('flows:core.sharing.dialog.close', 'Close')}>
          <X size={20} />
        </IconButton>
      </DialogTitle>
      <Divider />
      <Box sx={{flex: 1, overflowY: 'auto', p: 4}}>
        <Stack spacing={3} sx={{maxWidth: 1000, mx: 'auto', width: '100%'}}>
          {isDirty && (
            <Alert severity="warning" data-testid="flow-sharing-unsaved">
              {t(
                'flows:core.sharing.dialog.unsaved',
                'This flow has unsaved changes. Save the flow first, or a variable you added cannot be shared yet.',
              )}
            </Alert>
          )}
          {access.isLoading ? (
            <Box sx={{display: 'flex', justifyContent: 'center', py: 6}}>
              <CircularProgress />
            </Box>
          ) : (
            <>
              <OrganizationAccessSettings
                access={access}
                ownerOuId={flow.ouId}
                resourceName={flow.name}
                readOnly={locked}
                hasUnitSettings
                errorNamespace="flows"
                childUnitTerms={
                  variables.length > 0
                    ? {
                        editLabel: (name: string) =>
                          t('flows:core.sharing.terms.editFor', 'Terms for {{name}}', {name}),
                        onEdit: (ouId: string) => setExpandedOuId((current) => (current === ouId ? null : ouId)),
                        expandedOuId,
                        renderPanel: (ouId: string) => (
                          <VariableSharingRules
                            bare
                            variables={variables}
                            rules={unitRules(ouId)}
                            onChange={(rules: OverlayRules) => changeUnitRules(ouId, rules)}
                            disabled={locked}
                          />
                        ),
                      }
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
                    'No terms are available. This flow declares no variables, so there is nothing for the units to set. Add a Variables step to the flow to offer some.',
                  )}
                </Alert>
              )}
              {variables.length > 0 &&
                !reachesSelectedUnits &&
                (reachesAllChildUnits ? (
                  <VariableSharingRules
                    variables={variables}
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
          {access.saveError && <Alert severity="error">{access.saveError.message}</Alert>}
        </Stack>
      </Box>
      {isStartingOpen && (
        <UnitTermsDrawer
          open
          title={t('flows:core.sharing.terms.startingTitle', 'Starting terms for {{count}} granted units', {
            count: draft.childUnits.length,
          })}
          subtitle={t(
            'flows:core.sharing.terms.startingSubtitle',
            'Fill them in once and they are copied to every unit below. Afterwards each unit keeps its own copy, changed from its own row.',
          )}
          applyLabel={t('flows:core.sharing.terms.copyTo', 'Copy to {{count}} units', {count: draft.childUnits.length})}
          variables={variables}
          rules={undefined}
          disabled={locked}
          onClose={() => setIsStartingOpen(false)}
          onApply={applyStartingTerms}
        />
      )}
      <Divider />
      <DialogActions sx={{px: 3, py: 2}}>
        <Button variant="text" onClick={close}>
          {t('flows:core.sharing.dialog.cancel', 'Cancel')}
        </Button>
        <Button
          variant="contained"
          disabled={locked || !access.hasChanges || access.isSaving}
          onClick={() => void save()}
          data-testid="flow-sharing-save"
        >
          {t('flows:core.sharing.dialog.save', 'Save')}
        </Button>
      </DialogActions>
    </Dialog>
  );
}

export default FlowSharingDialog;
