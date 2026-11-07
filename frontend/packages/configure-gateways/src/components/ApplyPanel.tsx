// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {QueryErrorNotice} from '@thunderid/components';
import {getErrorMessage} from '@thunderid/utils';
import {
  Alert,
  AlertTitle,
  Box,
  Button,
  Checkbox,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Drawer,
  FormControlLabel,
  IconButton,
  MenuItem,
  Select,
  Stack,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import {Rocket, Send, X} from '@wso2/oxygen-ui-icons-react';
import {useMemo, useState, type JSX, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import ChangedValuesNotice from './ChangedValuesNotice';
import ChangeList from './ChangeList';
import DeployChanges from './DeployChanges';
import MissingValuesFields from './MissingValuesFields';
import RefusedFixes from './RefusedFixes';
import {useApplyCheck, useApplyVersion} from '../api/useGatewayVersions';
import {useRefused} from '../api/useMaster';
import {useSetStoredValue, useStoredValues} from '../api/useStoredValues';
import {useVersion} from '../api/useVersions';
import useProjectScope from '../contexts/ProjectScope';
import useChangeName from '../hooks/useChangeName';
import useGatewayErrorTranslator from '../hooks/useGatewayErrorTranslator';
import type {ApplyResult, Change, MasterEntry, Refusal, StoredValue, ValueReference, Version} from '../models/deploy';
import type {Gateway} from '../models/gateway';
import {errorCodeOf, isForbidden, REMOVALS_NEED_CONFIRMATION} from '../utils/apiError';
import {carriedWithProject, projectNeeds, sharedMissing} from '../utils/changeSummary';
import {fromListText, toListText} from '../utils/listValue';
import {fieldKey, missingFields, type MissingField, type RefusedFix} from '../utils/missingValues';
import {withNeeded} from '../utils/needs';

export interface ApplyPanelProps {
  version: Version;
  /** The version's label, "v6". */
  label: string;
  gateways: Gateway[];
  /** The gateway chosen to begin with; the first otherwise. */
  gatewayId?: string;
  /**
   * The keys of the changes ticked to begin with, every change otherwise: the resources the gateway
   * refused, to fix and deploy them again.
   */
  only?: string[];
  onClose: () => void;
  /** Called once the version is written, with what the gateway reported. */
  onDeployed: (gateway: Gateway, result: ApplyResult) => void;
  /** A dialog over the page, or a drawer at its right with the deploy path above the changes. */
  variant?: 'dialog' | 'drawer';
  /** Shown above the changes, such as where along the path the deploy goes. */
  before?: ReactNode;
  /** Shown below the changes, such as a setting of the gateway deployed to. */
  after?: ReactNode;
  /** In the drawer, the name of the gateway deployed to, which the drawer speaks of. */
  targetName?: string;
  /** In the drawer, the gateway a promotion carries the version from; absent for a deploy. */
  sourceName?: string;
  /** In the drawer, what it says the deploy does, under its title. */
  subtitle?: ReactNode;
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
  only = undefined,
  onClose,
  onDeployed,
  variant = 'dialog',
  before = undefined,
  after = undefined,
  targetName = undefined,
  sourceName = undefined,
  subtitle = undefined,
}: ApplyPanelProps): JSX.Element {
  const nameOfChange = useChangeName();
  const {t} = useTranslation('deploy');
  const tForErrors = useGatewayErrorTranslator();
  const [target, setTarget] = useState<string>(gatewayId ?? gateways[0]?.id ?? '');
  const gateway = gateways.find((entry: Gateway) => entry.id === target);
  const drawer = variant === 'drawer';
  const promote = drawer && sourceName !== undefined;
  // What the drawer calls where the version goes: the gateway, not the gateway it runs on.
  const envName = targetName ?? gateway?.name ?? '';
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

  const refusedEntries = useRefused(target);
  const references = useVersion(version.version, true, target).data?.references;
  const storedVariables = useStoredValues(target, 'variable').data;
  // What was typed over a value a refused resource uses, by its field key.
  const [fixValues, setFixValues] = useState<Record<string, string>>({});
  // Only the panel opened to retry what was refused starts with just those ticked; another gateway
  // chosen in it starts over with everything.
  const [startWith, setStartWith] = useState<string[] | undefined>(only);

  const changes: Change[] = useMemo(() => check.data?.diff.changes ?? [], [check.data]);
  // Under a project whose check places each change, the project's own changes start ticked and can be
  // unticked one by one, what the ticked ones need that the target lacks goes with them, and only what
  // else is ticked goes too; another project's changes are never carried. A check that places none, as
  // where a project's deploys are not scoped, deploys as the whole organization does.
  const project = useProjectScope();
  const scoped = project !== undefined && changes.some((change: Change) => change.section !== undefined);
  const active = useMemo(() => changes.filter((change: Change) => change.change !== 'unchanged'), [changes]);

  // What starts unticked: with a retry, all but what it retries; otherwise what an earlier deploy left
  // out for the deploys after it.
  const excluded = useMemo(
    () =>
      chosen ??
      (startWith
        ? new Set(
            active
              .filter((change: Change) => !startWith.includes(change.key) && !(scoped && sharedMissing(change)))
              .map((change: Change) => change.key),
          )
        : scoped
          ? // A shared resource the target lacks goes by default; one it holds is replaced only when ticked.
            new Set(
              active
                .filter((change: Change) => !carriedWithProject(change) && !sharedMissing(change))
                .map((change: Change) => change.key),
            )
          : new Set(active.filter((change: Change) => change.heldBack === true).map((change: Change) => change.key))),
    [chosen, startWith, active, scoped],
  );

  // A new target starts over with everything ticked and nothing typed. Nothing else resets them, so a
  // refetch of the same target keeps what the user chose and typed.
  const chooseTarget = (next: string): void => {
    setTarget(next);
    setChosen(null);
    setStartWith(undefined);
    setValues({});
    setFixValues({});
    setError(null);
    setConfirmRemovals(false);
  };

  const ticked = useMemo(
    () =>
      new Set(
        active
          .filter((change: Change) => {
            if (scoped) return change.section !== 'other' && !projectNeeds(change) && !excluded.has(change.key);
            return !excluded.has(change.key);
          })
          .map((change: Change) => change.key),
      ),
    [active, excluded, scoped],
  );
  // What the ticked changes bring along, as the control plane widens the selection; shown ticked and
  // marked, not chosen by hand.
  const required = useMemo(() => {
    if (scoped) {
      // What the ticked resources need that the target lacks goes with them, locked. A check that names no
      // needs says only that the project needs it, so it goes whatever is ticked.
      const named = active.some((change: Change) => (change.needs ?? []).length > 0);
      const needed = active.filter(projectNeeds);
      if (!named) return new Map(needed.map((change: Change) => [change.key, change.requiredBy ?? '']));
      const reached = withNeeded(ticked, changes);
      return new Map(
        needed
          .filter((change: Change) => reached.has(change.key))
          .map((change: Change) => [change.key, reached.get(change.key) ?? '']),
      );
    }
    return includeNeeded ? withNeeded(ticked, changes) : new Map<string, string>();
  }, [active, includeNeeded, scoped, ticked, changes]);
  const selected = useMemo(() => new Set([...ticked, ...required.keys()]), [ticked, required]);
  // The values the deploy lacks that a ticked change uses. A value only unticked changes use is not
  // asked for, nor one whose user the check does not name; a value no reference ties to a change is.
  const fields: MissingField[] = useMemo(() => {
    const byResource = new Map<string, string>(
      changes.map((change: Change) => [`${change.resourceType}/${change.id ?? ''}`, change.key]),
    );
    return missingFields(check.data?.missing).filter((field: MissingField) => {
      if (field.references.length === 0) return true;
      return field.references.some((reference) => {
        const key = byResource.get(`${reference.resourceType}/${reference.resourceId ?? ''}`);
        return key === undefined || selected.has(key);
      });
    });
  }, [check.data?.missing, changes, selected]);
  // What the check, a dry run with the values the gateway holds, says the gateway would refuse, by key.
  const predicted = useMemo(() => {
    const byKey = new Map<string, Refusal>();
    for (const result of check.data?.import?.results ?? []) {
      if (result.status === 'failed' && result.resourceId) {
        byKey.set(`${result.resourceType}/${result.resourceId}`, {
          code: result.code,
          message: result.message,
          description: result.description,
        });
      }
    }
    return byKey;
  }, [check.data]);
  // The ticked resources the gateway refused last time or would refuse now, each with why and the
  // values it uses that the gateway holds, which can be set right first. A value the gateway lacks is
  // asked for above instead.
  const fixes: RefusedFix[] = useMemo(() => {
    const lacking = new Set(fields.map(fieldKey));
    return active
      .filter((change: Change) => (change.failed === true || predicted.has(change.key)) && ticked.has(change.key))
      .map((change: Change) => {
        const own = (references ?? []).filter(
          (reference: ValueReference) =>
            reference.resourceType === change.resourceType && reference.resourceId === change.id,
        );
        const byKey = new Map<string, MissingField>();
        for (const reference of own) {
          const key = `${reference.kind}:${reference.name}`;
          if (lacking.has(key)) continue;
          const field = byKey.get(key) ?? {kind: reference.kind, name: reference.name, list: false, references: []};
          field.references.push(reference);
          field.list = field.list || reference.list === true;
          byKey.set(key, field);
        }
        const entry = refusedEntries.find((refused: MasterEntry) => refused.key === change.key);
        return {change, failure: entry?.failure, predicted: predicted.get(change.key), fields: [...byKey.values()]};
      });
  }, [active, ticked, references, fields, refusedEntries, predicted]);
  const heldValue = (field: MissingField): string => {
    if (field.kind === 'secret') return '';
    const held = storedVariables?.find((stored: StoredValue) => stored.name === field.name)?.value ?? '';
    return field.list ? toListText(held) : held;
  };
  const fixValueOf = (field: MissingField): string => fixValues[fieldKey(field)] ?? heldValue(field);
  // Each value typed over, once per name, that differs from what the gateway holds.
  const fixed: MissingField[] = [
    ...new Map(
      fixes
        .flatMap((fix: RefusedFix) => fix.fields)
        .filter((field: MissingField) => {
          const typed = fixValues[fieldKey(field)];
          return typed !== undefined && (field.kind === 'secret' ? typed !== '' : typed !== heldValue(field));
        })
        .map((field: MissingField) => [fieldKey(field), field]),
    ).values(),
  ];
  const unfilled = fields.filter((field: MissingField) => (values[fieldKey(field)] ?? '').trim() === '');
  const canDeploy = Boolean(gateway) && check.isSuccess && selected.size > 0 && unfilled.length === 0 && !isWriting;
  const removals = active.filter((change: Change) => change.change === 'deleted' && selected.has(change.key)).length;

  // A confirmation covers the removals the user was warned about, so changing what is ticked drops it.
  const toggle = (key: string): void => {
    setConfirmRemovals(false);
    setChosen((previous: Set<string> | null) => {
      const next = new Set(previous ?? excluded);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  };
  // Ticks or unticks one section's changes, leaving the other sections as they are.
  const toggleAll = (keys: string[], checked: boolean): void => {
    setConfirmRemovals(false);
    setChosen((previous: Set<string> | null) => {
      const next = new Set(previous ?? excluded);
      keys.forEach((key: string) => (checked ? next.delete(key) : next.add(key)));
      return next;
    });
  };

  const deploy = async (): Promise<void> => {
    if (!gateway || !canDeploy) return;
    setIsWriting(true);
    setError(null);
    try {
      // Values first, one at a time, so a refusal names the value and leaves the rest unsent.
      const write = async (field: MissingField, entered: string): Promise<void> => {
        const value = field.list ? fromListText(entered) : entered;
        if (field.kind === 'secret') await setSecret.mutateAsync({name: field.name, value});
        else await setVariable.mutateAsync({name: field.name, value});
      };
      for (const field of fields) await write(field, values[fieldKey(field)] ?? '');
      for (const field of fixed) await write(field, fixValues[fieldKey(field)] ?? '');
      // What is ticked is sent as the selection; the control plane brings along what it needs when
      // asked to, so the two agree on what is deployed.
      // What is left unticked stays out of later deploys too, until one ticks it. A retry unticks the
      // rest only to send what was refused, so it leaves that choice as it was.
      const leaveOut =
        startWith || scoped
          ? []
          : active
              .filter((change: Change) => !change.sourceExcluded && !selected.has(change.key))
              .map((change: Change) => change.key);
      const result = await apply.mutateAsync({
        gatewayId: gateway.id,
        data: {
          version: version.version,
          selection: [...(scoped ? selected : ticked)],
          ...(leaveOut.length > 0 ? {leaveOut} : {}),
          ...(includeNeeded && !scoped ? {} : {withDependencies: false}),
          ...(scoped ? {project} : {}),
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

  const header = drawer ? (
    <Stack direction="row" alignItems="flex-start" spacing={1.5}>
      <Box sx={{display: 'flex', color: 'primary.main', pt: 0.5}}>
        {promote ? <Send size={18} /> : <Rocket size={18} />}
      </Box>
      <Box sx={{flex: 1, minWidth: 0}}>
        <Typography variant="h6" component="h2" fontWeight={500}>
          {promote
            ? t('applyPanel.promoteTo', 'Promote to {{target}}', {target: envName})
            : t('applyPanel.deployThroughPipeline', 'Deploy through pipeline')}
        </Typography>
        {subtitle && (
          <Typography variant="body2" color="text.secondary">
            {subtitle}
          </Typography>
        )}
      </Box>
      <IconButton aria-label={t('applyPanel.close', 'Close')} onClick={onClose} disabled={isWriting}>
        <X size={18} />
      </IconButton>
    </Stack>
  ) : (
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
  // Shared resources the target lacks that the deploy leaves out, which its pages may need.
  const leftOut = scoped ? active.filter((change: Change) => sharedMissing(change) && !selected.has(change.key)) : [];
  let submitLabel = t('applyPanel.submit', 'Deploy');
  if (confirmRemovals) submitLabel = t('applyPanel.submitConfirm', 'Deploy and remove {{count}}', {count: removals});
  else if (drawer && check.isSuccess) {
    const count = active.filter((change: Change) => selected.has(change.key)).length;
    if (count === 0) submitLabel = t('applyPanel.nothingToDeploy', 'Nothing to deploy');
    else if (promote)
      submitLabel = t('applyPanel.promoteCount', 'Promote {{count}} changes to {{gateway}}', {count, gateway: envName});
    else
      submitLabel = t('applyPanel.submitCount', 'Deploy {{count}} changes to {{gateway}}', {count, gateway: envName});
  }
  if (!confirmRemovals && leftOut.length > 0 && check.isSuccess)
    submitLabel = promote
      ? t('applyPanel.promoteAnyway', 'Promote anyway')
      : t('applyPanel.deployAnyway', 'Deploy anyway');
  const leftOutWarning =
    leftOut.length > 0 && check.isSuccess ? (
      <Alert severity="warning" data-testid="deploy-shared-left-out" sx={{mx: drawer ? 3 : 0, mb: drawer ? 0 : 2}}>
        <AlertTitle>
          {t('applyPanel.sharedLeftOutTitle', "Some shared resources won't be on {{target}}", {target: envName})}
        </AlertTitle>
        {t('applyPanel.sharedLeftOut', {
          count: leftOut.length,
          names: leftOut.slice(0, 3).map(nameOfChange).join(', '),
          target: envName,
          defaultValue_one:
            "{{count}} shared resource this gateway doesn't have yet is left out: {{names}}. Sign-in pages, branding or text in {{target}} may not work as expected until it is deployed.",
          defaultValue_other:
            "{{count}} shared resources this gateway doesn't have yet are left out, such as {{names}}. Sign-in pages, branding or text in {{target}} may not work as expected until they are deployed.",
        })}
      </Alert>
    ) : null;
  const footer = (
    <>
      <Button variant="text" color="inherit" onClick={onClose} disabled={isWriting}>
        {t('applyPanel.cancel', 'Cancel')}
      </Button>
      <Button
        variant="contained"
        startIcon={
          isWriting ? (
            <CircularProgress size={14} color="inherit" />
          ) : promote ? (
            <Send size={14} />
          ) : (
            <Rocket size={14} />
          )
        }
        onClick={() => void deploy()}
        disabled={!canDeploy}
        sx={{borderRadius: 100}}
      >
        {submitLabel}
      </Button>
    </>
  );
  const missingValues =
    fields.length > 0 ? (
      <MissingValuesFields
        fields={fields}
        values={values}
        onChange={(field, value) => setValues((previous) => ({...previous, [fieldKey(field)]: value}))}
        disabled={isWriting}
        target={drawer ? envName : undefined}
      />
    ) : null;
  const body = (
    <Stack spacing={2.5}>
      {before}
      {check.isLoading && (
        <Box sx={{display: 'flex', alignItems: 'center', gap: 1.5}}>
          <CircularProgress size={18} />
          <Typography variant="body2" color="text.secondary">
            {t('applyPanel.checking', 'Working out what {{gateway}} would write...', {gateway: envName})}
          </Typography>
        </Box>
      )}
      {check.isError && (
        <QueryErrorNotice
          error={check.error}
          t={tForErrors}
          variant="inline"
          fallbackKey="deploy:applyPanel.checkFailedFor"
          fallbackDefaultValue={t('applyPanel.checkFailedFor', 'What {{target}} would receive could not be read.', {
            target: envName,
          })}
          resolveErrorMessage={(error, translate, fallbackKey, fallbackDefaultValue) =>
            isForbidden(error)
              ? t('applyPanel.forbiddenFor', 'Your account may not deploy to {{target}}.', {target: envName})
              : getErrorMessage(
                  error,
                  (key: string, options?: Record<string, unknown>) => translate(key, {target: envName, ...options}),
                  fallbackKey,
                  fallbackDefaultValue,
                )
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
            {count: removals, gateway: envName},
          )}
        </Alert>
      )}
      {!drawer && missingValues}
      {check.isSuccess && fixes.length > 0 && (
        <RefusedFixes
          fixes={fixes}
          target={envName}
          checked={check.isSuccess}
          valueOf={fixValueOf}
          onChange={(field, value) => setFixValues((previous) => ({...previous, [fieldKey(field)]: value}))}
          disabled={isWriting}
        />
      )}
      {check.isSuccess &&
        (drawer || scoped ? (
          <DeployChanges
            changes={changes}
            selected={selected}
            onToggle={toggle}
            onToggleAll={toggleAll}
            target={envName}
            source={sourceName}
            required={required}
          />
        ) : (
          <ChangeList
            changes={changes}
            selected={selected}
            onToggle={toggle}
            onToggleAll={toggleAll}
            target={gateway?.name}
            required={required}
          />
        ))}
      {check.isSuccess && active.length > 0 && !scoped && (
        <Tooltip
          title={t(
            'applyPanel.includeNeededHint',
            'A resource names the flows, units and other resources it needs. Ticked, deploying one brings along what it needs that {{gateway}} lacks.',
            {gateway: envName},
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
              <Typography variant="body2">{t('applyPanel.includeNeeded', 'Also deploy what these need')}</Typography>
            }
          />
        </Tooltip>
      )}
      {drawer && missingValues}
      {error && (
        <Alert severity="error">
          {isForbidden(error)
            ? t('applyPanel.forbiddenFor', 'Your account may not deploy to {{target}}.', {target: envName})
            : getErrorMessage(error, tForErrors, 'deploy:applyPanel.failed', 'The version could not be deployed.')}
        </Alert>
      )}
      {after}
    </Stack>
  );
  if (drawer) {
    return (
      <Drawer
        anchor="right"
        open
        onClose={onClose}
        sx={{'& .MuiDrawer-paper': {width: 560, maxWidth: '100%', display: 'flex', flexDirection: 'column'}}}
      >
        <Box data-testid="deploy-apply-panel" sx={{display: 'flex', flexDirection: 'column', height: '100%'}}>
          <Box sx={{px: 3, py: 2.5, borderBottom: 1, borderColor: 'divider'}}>{header}</Box>
          <Box sx={{flex: 1, overflowY: 'auto', px: 3, py: 2.5}}>{body}</Box>
          {leftOutWarning}
          <Box
            sx={{
              display: 'flex',
              justifyContent: 'flex-end',
              gap: 1,
              px: 3,
              py: 2,
              borderTop: 1,
              borderColor: 'divider',
            }}
          >
            {footer}
          </Box>
        </Box>
      </Drawer>
    );
  }
  return (
    <Dialog open onClose={onClose} fullWidth maxWidth="md">
      <Box data-testid="deploy-apply-panel">
        <DialogTitle>{header}</DialogTitle>
        <DialogContent dividers>
          {body}
          {leftOutWarning}
        </DialogContent>
        <DialogActions>{footer}</DialogActions>
      </Box>
    </Dialog>
  );
}
