// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  Alert,
  Box,
  Button,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  MenuItem,
  Select,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import {Rocket} from '@wso2/oxygen-ui-icons-react';
import {useEffect, useMemo, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import ChangeList from './ChangeList';
import MissingValuesFields from './MissingValuesFields';
import {useApplyCheck, useApplyVersion} from '../api/useGatewayVersions';
import {useSetStoredValue} from '../api/useStoredValues';
import type {ApplyResult, Change, Gateway, Version} from '../models/deploy';
import {fromListText} from '../utils/listValue';
import {fieldKey, missingFields, type MissingField} from '../utils/missingValues';

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
 * lacks set in place, and one button then writes everything.
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
  const [target, setTarget] = useState<string>(gatewayId ?? gateways[0]?.id ?? '');
  const gateway = gateways.find((entry: Gateway) => entry.id === target);
  const check = useApplyCheck(target, version.version);
  const apply = useApplyVersion();
  const setVariable = useSetStoredValue(target, 'variable');
  const setSecret = useSetStoredValue(target, 'secret');
  const [excluded, setExcluded] = useState<Set<string>>(new Set());
  const [values, setValues] = useState<Record<string, string>>({});
  const [isWriting, setIsWriting] = useState(false);
  const [error, setError] = useState<Error | null>(null);

  const changes: Change[] = useMemo(() => check.data?.diff.changes ?? [], [check.data]);
  const active = useMemo(() => changes.filter((change: Change) => change.change !== 'unchanged'), [changes]);

  // A new target or a fresh check starts over from what the gateway holds back, with nothing typed.
  useEffect(() => {
    setExcluded(new Set(active.filter((change: Change) => change.excluded).map((change: Change) => change.key)));
    setValues({});
    setError(null);
  }, [target, check.dataUpdatedAt, active]);

  const selected = useMemo(
    () => new Set(active.map((change: Change) => change.key).filter((key: string) => !excluded.has(key))),
    [active, excluded],
  );
  const fields: MissingField[] = useMemo(() => missingFields(check.data?.missing), [check.data?.missing]);
  const unfilled = fields.filter((field: MissingField) => (values[fieldKey(field)] ?? '').trim() === '');
  const canDeploy = Boolean(gateway) && check.isSuccess && selected.size > 0 && unfilled.length === 0 && !isWriting;

  const toggle = (key: string): void =>
    setExcluded((previous: Set<string>) => {
      const next = new Set(previous);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  const toggleAll = (keys: string[], checked: boolean): void => setExcluded(checked ? new Set() : new Set(keys));

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
      // The selection is always sent, so what is ticked is exactly what is applied: left to the
      // gateway's standing choice, a change it holds back would stay held back though ticked.
      const result = await apply.mutateAsync({
        gatewayId: gateway.id,
        data: {version: version.version, selection: [...selected]},
      });
      onDeployed(gateway, result);
    } catch (caught: unknown) {
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
        onChange={(event) => setTarget(String(event.target.value))}
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
        {t('applyPanel.submit', 'Deploy')}
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
              <Alert severity="error">
                {t('applyPanel.checkFailed', 'What the gateway would write could not be read.')}
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
              />
            )}
            {error && (
              <Alert severity="error">
                {error.message || t('applyPanel.failed', 'The version could not be deployed.')}
              </Alert>
            )}
          </Stack>
        </DialogContent>
        <DialogActions>{footer}</DialogActions>
      </Box>
    </Dialog>
  );
}
