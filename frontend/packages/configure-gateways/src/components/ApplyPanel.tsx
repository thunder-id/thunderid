// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {QueryErrorNotice} from '@thunderid/components';
import {getErrorMessage} from '@thunderid/utils';
import {
  Alert,
  Box,
  Button,
  Checkbox,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControlLabel,
  MenuItem,
  Select,
  Stack,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import {Rocket} from '@wso2/oxygen-ui-icons-react';
import {useMemo, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import ChangedValuesNotice from './ChangedValuesNotice';
import ChangeList from './ChangeList';
import MissingValuesFields from './MissingValuesFields';
import {useApplyCheck, useApplyVersion} from '../api/useGatewayVersions';
import {useSetStoredValue} from '../api/useStoredValues';
import useGatewayErrorTranslator from '../hooks/useGatewayErrorTranslator';
import type {ApplyResult, Change, Version} from '../models/deploy';
import type {Gateway} from '../models/gateway';
import {errorCodeOf, isForbidden, REMOVALS_NEED_CONFIRMATION} from '../utils/apiError';
import {fromListText} from '../utils/listValue';
import {fieldKey, missingFields, type MissingField} from '../utils/missingValues';
import {withNeeded} from '../utils/needs';

export interface ApplyPanelProps {
  version: Version;
  /** The version's label, "v6". */
  label: string;
  gateways: Gateway[];
  /** The gateway chosen to begin with; the first otherwise. */
  gatewayId?: string;
  onClose: () => void;
  /** Called once the version is written, with what the gateway reported. */
  onDeployed: (gateway: Gateway, result: ApplyResult) => void;
}

/**
 * Deploys a version to a gateway in a dialog over the Deploy page: what the write would do is read as
 * soon as the panel opens, so the resources can be left out one by one and the values the gateway
 * lacks set in place, and one button then writes what is ticked, along with what it needs. Nothing
 * about the choice is kept: what is left out stays on offer.
 */
export default function ApplyPanel({
  version,
  label,
  gateways,
  gatewayId = undefined,
  onClose,
  onDeployed,
}: ApplyPanelProps): JSX.Element {
  const {t} = useTranslation('deploy');
  const tForErrors = useGatewayErrorTranslator();
  const [target, setTarget] = useState<string>(gatewayId ?? gateways[0]?.id ?? '');
  const gateway = gateways.find((entry: Gateway) => entry.id === target);
  const check = useApplyCheck(target, version.version);
  const apply = useApplyVersion();
  const setVariable = useSetStoredValue(target, 'variable');
  const setSecret = useSetStoredValue(target, 'secret');
  // Null until the user ticks or unticks something, so every change starts ticked.
  const [chosen, setChosen] = useState<Set<string> | null>(null);
  // Whether what is ticked brings along what it needs that the gateway lacks.
  const [includeNeeded, setIncludeNeeded] = useState(true);
  const [values, setValues] = useState<Record<string, string>>({});
  const [isWriting, setIsWriting] = useState(false);
  const [error, setError] = useState<Error | null>(null);
  // Set once the control plane refused the deploy for removing more than its threshold allows: the
  // removals are then pointed out, and the next click confirms them.
  const [confirmRemovals, setConfirmRemovals] = useState(false);

  const changes: Change[] = useMemo(() => check.data?.diff.changes ?? [], [check.data]);
  const active = useMemo(() => changes.filter((change: Change) => change.change !== 'unchanged'), [changes]);

  const excluded = useMemo(() => chosen ?? new Set<string>(), [chosen]);

  // A new target starts over with everything ticked and nothing typed. Nothing else resets them, so a
  // refetch of the same target keeps what the user chose and typed.
  const chooseTarget = (next: string): void => {
    setTarget(next);
    setChosen(null);
    setValues({});
    setError(null);
    setConfirmRemovals(false);
  };

  const ticked = useMemo(
    () => new Set(active.map((change: Change) => change.key).filter((key: string) => !excluded.has(key))),
    [active, excluded],
  );
  // What the ticked changes bring along, as the control plane widens the selection; shown ticked and
  // marked, not chosen by hand.
  const required = useMemo(
    () => (includeNeeded ? withNeeded(ticked, changes) : new Map<string, string>()),
    [includeNeeded, ticked, changes],
  );
  const selected = useMemo(() => new Set([...ticked, ...required.keys()]), [ticked, required]);
  const fields: MissingField[] = useMemo(() => missingFields(check.data?.missing), [check.data?.missing]);
  const unfilled = fields.filter((field: MissingField) => (values[fieldKey(field)] ?? '').trim() === '');
  const canDeploy = Boolean(gateway) && check.isSuccess && selected.size > 0 && unfilled.length === 0 && !isWriting;
  const removals = active.filter((change: Change) => change.change === 'deleted' && selected.has(change.key)).length;

  // A confirmation covers the removals the user was warned about, so changing what is ticked drops it.
  const toggle = (key: string): void => {
    setConfirmRemovals(false);
    setChosen((previous: Set<string> | null) => {
      const next = new Set(previous ?? []);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  };
  const toggleAll = (keys: string[], checked: boolean): void => {
    setConfirmRemovals(false);
    setChosen(checked ? new Set() : new Set(keys));
  };

  const deploy = async (): Promise<void> => {
    if (!gateway || !canDeploy) return;
    setIsWriting(true);
    setError(null);
    try {
      // Values first, one at a time, so a refusal names the value and leaves the rest unsent.
      for (const field of fields) {
        const entered = values[fieldKey(field)] ?? '';
        const value = field.list ? fromListText(entered) : entered;
        if (field.kind === 'secret') await setSecret.mutateAsync({name: field.name, value});
        else await setVariable.mutateAsync({name: field.name, value});
      }
      // What is ticked is sent as the selection; the control plane brings along what it needs when
      // asked to, so the two agree on what is deployed.
      const result = await apply.mutateAsync({
        gatewayId: gateway.id,
        data: {
          version: version.version,
          selection: [...ticked],
          ...(includeNeeded ? {} : {withDependencies: false}),
          ...(confirmRemovals ? {confirmRemovals: true} : {}),
        },
      });
      onDeployed(gateway, result);
    } catch (caught: unknown) {
      if (errorCodeOf(caught) === REMOVALS_NEED_CONFIRMATION) {
        setConfirmRemovals(true);
        return;
      }
      setError(caught instanceof Error ? caught : new Error(String(caught)));
    } finally {
      setIsWriting(false);
    }
  };

  const header = (
    <Stack direction="row" alignItems="center" spacing={2} sx={{flexWrap: 'wrap', rowGap: 1}}>
      <Box sx={{display: 'flex', color: 'primary.main'}}>
        <Rocket size={18} />
      </Box>
      <Typography variant="subtitle1" fontWeight={600}>
        {t('applyPanel.title', 'Deploy {{label}}', {label})}
      </Typography>
      <Typography variant="body2" color="text.secondary">
        {t('applyPanel.to', 'to')}
      </Typography>
      <Select
        size="small"
        value={target}
        onChange={(event) => chooseTarget(String(event.target.value))}
        inputProps={{'aria-label': t('applyPanel.gateway', 'Gateway')}}
        sx={{minWidth: 160}}
      >
        {gateways.map((entry: Gateway) => (
          <MenuItem key={entry.id} value={entry.id}>
            {entry.name}
          </MenuItem>
        ))}
      </Select>
    </Stack>
  );
  const footer = (
    <>
      <Button variant="text" color="inherit" onClick={onClose} disabled={isWriting}>
        {t('applyPanel.cancel', 'Cancel')}
      </Button>
      <Button
        variant="contained"
        startIcon={isWriting ? <CircularProgress size={14} color="inherit" /> : <Rocket size={14} />}
        onClick={() => void deploy()}
        disabled={!canDeploy}
        sx={{borderRadius: 100}}
      >
        {confirmRemovals
          ? t('applyPanel.submitConfirm', 'Deploy and remove {{count}}', {count: removals})
          : t('applyPanel.submit', 'Deploy')}
      </Button>
    </>
  );
  return (
    <Dialog open onClose={onClose} fullWidth maxWidth="md">
      <Box data-testid="deploy-apply-panel">
        <DialogTitle>{header}</DialogTitle>
        <DialogContent dividers>
          <Stack spacing={2.5}>
            {check.isLoading && (
              <Box sx={{display: 'flex', alignItems: 'center', gap: 1.5}}>
                <CircularProgress size={18} />
                <Typography variant="body2" color="text.secondary">
                  {t('applyPanel.checking', 'Working out what {{gateway}} would write...', {
                    gateway: gateway?.name ?? '',
                  })}
                </Typography>
              </Box>
            )}
            {check.isError && (
              <QueryErrorNotice
                error={check.error}
                t={tForErrors}
                variant="inline"
                fallbackKey="deploy:applyPanel.checkFailed"
                fallbackDefaultValue="What the gateway would write could not be read."
                resolveErrorMessage={(error, translate, fallbackKey, fallbackDefaultValue) =>
                  isForbidden(error)
                    ? t('applyPanel.forbidden', 'Your account may not deploy to this gateway.')
                    : getErrorMessage(error, translate, fallbackKey, fallbackDefaultValue)
                }
                onRetry={() => void check.refetch()}
              />
            )}
            {check.data?.changed && <ChangedValuesNotice changed={check.data.changed} />}
            {confirmRemovals && (
              <Alert severity="warning" data-testid="deploy-confirm-removals">
                {t(
                  'applyPanel.confirmRemovals',
                  'Deploying removes {{count}} resources from {{gateway}}. Check them in the list, then deploy again to go ahead.',
                  {count: removals, gateway: gateway?.name ?? ''},
                )}
              </Alert>
            )}
            {fields.length > 0 && (
              <MissingValuesFields
                fields={fields}
                values={values}
                onChange={(field, value) => setValues((previous) => ({...previous, [fieldKey(field)]: value}))}
                disabled={isWriting}
              />
            )}
            {check.isSuccess && (
              <ChangeList
                changes={changes}
                selected={selected}
                onToggle={toggle}
                onToggleAll={toggleAll}
                target={gateway?.name}
                required={required}
              />
            )}
            {check.isSuccess && active.length > 0 && (
              <Tooltip
                title={t(
                  'applyPanel.includeNeededHint',
                  'A resource names the flows, units and other resources it needs. Ticked, deploying one brings along what it needs that {{gateway}} lacks.',
                  {gateway: gateway?.name ?? ''},
                )}
              >
                <FormControlLabel
                  control={
                    <Checkbox
                      size="small"
                      checked={includeNeeded}
                      onChange={(_event, checked: boolean) => {
                        setConfirmRemovals(false);
                        setIncludeNeeded(checked);
                      }}
                      data-testid="deploy-include-needed"
                    />
                  }
                  label={
                    <Typography variant="body2">
                      {t('applyPanel.includeNeeded', 'Also deploy what these need')}
                    </Typography>
                  }
                />
              </Tooltip>
            )}
            {error && (
              <Alert severity="error">
                {isForbidden(error)
                  ? t('applyPanel.forbidden', 'Your account may not deploy to this gateway.')
                  : getErrorMessage(
                      error,
                      tForErrors,
                      'deploy:applyPanel.failed',
                      'The version could not be deployed.',
                    )}
              </Alert>
            )}
          </Stack>
        </DialogContent>
        <DialogActions>{footer}</DialogActions>
      </Box>
    </Dialog>
  );
}
