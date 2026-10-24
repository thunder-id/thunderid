// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {DocumentTitle} from '@thunderid/components';
import {useToast} from '@thunderid/contexts';
import {getErrorMessage} from '@thunderid/utils';
import {Alert, Box, CircularProgress, Typography} from '@wso2/oxygen-ui';
import {useCallback, useEffect, useMemo, useRef, useState, type JSX, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate, useParams} from 'react-router';
import useGateways from '../api/useGateways';
import {useAppliedConfiguration, useRunningReferences} from '../api/useGatewayVersions';
import useHeldBack from '../api/useHeldBack';
import {useSetStoredValue, useStoredValues} from '../api/useStoredValues';
import {useVersions} from '../api/useVersions';
import ExplorerCanvas from '../components/explorer/ExplorerCanvas';
import ExplorerPanel from '../components/explorer/ExplorerPanel';
import ExplorerToolbar from '../components/explorer/ExplorerToolbar';
import FullScreenHeader from '../components/FullScreenHeader';
import {buildGraph, type GraphNode, type Label} from '../explorer/model';
import useDeployRoutes from '../hooks/useDeployRoutes';
import useGatewayErrorTranslator from '../hooks/useGatewayErrorTranslator';
import type {Change, StoredValue, ValueReference, Version} from '../models/deploy';
import type {Gateway} from '../models/gateway';
import {versionLabel} from '../utils/format';
import {fromListText, toListText} from '../utils/listValue';

/**
 * The explorer of one gateway: a map of every resource it runs and the lines between them, with a
 * panel that sets the values the gateway holds for the selected resource.
 */
export default function GatewayExplorerPage(): JSX.Element {
  const {t} = useTranslation('deploy');
  const navigate = useNavigate();
  const routes = useDeployRoutes();
  const {showToast} = useToast();
  const tForErrors = useGatewayErrorTranslator();
  const {gatewayId = ''} = useParams<{gatewayId: string}>();
  const gateways = useGateways();
  const gateway = gateways.data?.find((entry: Gateway) => entry.id === gatewayId);
  const configuration = useAppliedConfiguration(gatewayId);
  const heldBackChanges = useHeldBack(gatewayId, configuration.data?.version);
  const heldBack = useMemo(
    () => new Set(heldBackChanges.map((change: Change) => `${change.resourceType}/${change.id ?? ''}`)),
    [heldBackChanges],
  );
  const versions = useVersions(gatewayId);
  const references = useRunningReferences(gatewayId);
  const variables = useStoredValues(gatewayId, 'variable');
  const secrets = useStoredValues(gatewayId, 'secret');
  const setVariable = useSetStoredValue(gatewayId, 'variable');
  const setSecret = useSetStoredValue(gatewayId, 'secret');

  const [selected, setSelected] = useState<string | null>(null);
  const [drafts, setDrafts] = useState<Map<string, string>>(new Map());
  const [revealed, setRevealed] = useState<Set<string>>(new Set());
  const [updated, setUpdated] = useState<Set<string>>(new Set());
  const [isSaving, setIsSaving] = useState(false);

  // Another gateway starts afresh: nothing typed, nothing revealed, the same resource kept if it runs it.
  useEffect(() => {
    setDrafts(new Map());
    setRevealed(new Set());
    setUpdated(new Set());
  }, [gatewayId]);

  const graph = useMemo(
    () => buildGraph(configuration.data?.resources ?? [], references.data ?? []),
    [configuration.data?.resources, references.data],
  );
  const node: GraphNode | null = useMemo(
    () => graph.nodes.find((entry: GraphNode) => entry.key === selected) ?? null,
    [graph, selected],
  );
  const held = useMemo(
    () =>
      new Map(
        (variables.data ?? []).flatMap((value: StoredValue) =>
          value.value === undefined ? [] : [[value.name, toListText(value.value)] as [string, string]],
        ),
      ),
    [variables.data],
  );
  const heldSecrets = useMemo(
    () =>
      new Set(
        (secrets.data ?? [])
          .filter((value: StoredValue) => value.exists !== false)
          .map((value: StoredValue) => value.name),
      ),
    [secrets.data],
  );
  const labelFor = useCallback(
    (label: Label): string => t(label.key.replace(/^deploy:/, ''), label.defaultValue, label.values),
    [t],
  );
  const all: Version[] = versions.data ?? [];
  const runningIndex = all.findIndex((entry: Version) => entry.version === configuration.data?.version);
  const running =
    runningIndex < 0
      ? configuration.data?.version?.slice(0, 7)
      : versionLabel(
          all[runningIndex].hash ?? configuration.data?.version,
          all[runningIndex].number ?? all.length - runningIndex,
        );
  const counts =
    variables.data || secrets.data
      ? t('stage.valuesSummary', '{{variables}} variables, {{secrets}} secrets', {
          variables: held.size,
          secrets: heldSecrets.size,
        })
      : undefined;

  const forget = (references: ValueReference[]): void => {
    setDrafts((previous: Map<string, string>) => {
      const next = new Map(previous);
      references.forEach((reference: ValueReference) => next.delete(reference.name));
      return next;
    });
  };
  const discard = (): void => {
    if (node) forget(node.values);
  };
  const save = async (): Promise<void> => {
    if (!node || !gateway) return;
    const dirty = node.values.filter((reference: ValueReference) => {
      const draft = drafts.get(reference.name);
      if (draft === undefined) return false;
      return reference.kind === 'secret' ? draft !== '' : draft !== (held.get(reference.name) ?? '');
    });
    if (dirty.length === 0) return;
    setIsSaving(true);
    // What saved stays saved when a later write fails: it is marked updated and its draft cleared, and
    // only the drafts that failed are kept to try again.
    const saved: ValueReference[] = [];
    try {
      for (const reference of dirty) {
        const draft = drafts.get(reference.name) ?? '';
        const value = reference.list ? fromListText(draft) : draft;
        if (reference.kind === 'secret') await setSecret.mutateAsync({name: reference.name, value});
        else await setVariable.mutateAsync({name: reference.name, value});
        saved.push(reference);
      }
      showToast(
        t('explorer.saved', 'Saved {{count}} values to {{gateway}}.', {count: dirty.length, gateway: gateway.name}),
        'success',
      );
    } catch (caught: unknown) {
      const reason = getErrorMessage(
        caught as Error,
        tForErrors,
        'deploy:explorer.saveFailed',
        'The values could not be saved.',
      );
      showToast(
        saved.length > 0
          ? t('explorer.savedSome', 'Saved {{saved}} of {{count}} values to {{gateway}}. {{reason}}', {
              saved: saved.length,
              count: dirty.length,
              gateway: gateway.name,
              reason,
            })
          : reason,
        'error',
      );
    } finally {
      setUpdated(
        (previous: Set<string>) =>
          new Set([...previous, ...saved.filter((r) => r.kind === 'secret').map((r) => r.name)]),
      );
      forget(saved);
      setIsSaving(false);
    }
  };

  // Full screen takes the canvas and the panel together, so a resource's values stay at hand.
  const stageRef = useRef<HTMLDivElement>(null);
  const [fullscreen, setFullscreen] = useState<boolean>(false);
  useEffect(() => {
    const onChange = (): void => setFullscreen(document.fullscreenElement === stageRef.current);
    document.addEventListener('fullscreenchange', onChange);
    return () => document.removeEventListener('fullscreenchange', onChange);
  }, []);
  const toggleFullscreen = (): void => {
    if (document.fullscreenElement) {
      void document.exitFullscreen().catch(() => undefined);
    } else {
      void stageRef.current?.requestFullscreen().catch(() => undefined);
    }
  };

  let body: ReactNode;
  if (gateways.isLoading || configuration.isLoading) {
    body = (
      <Box sx={{display: 'flex', justifyContent: 'center', alignItems: 'center', flex: 1}}>
        <CircularProgress size={28} />
      </Box>
    );
  } else if (!gateway) {
    body = (
      <Box sx={{p: 4}}>
        <Alert severity="error">{t('explorer.noGateway', 'No gateway is registered under this id.')}</Alert>
      </Box>
    );
  } else if (configuration.isError) {
    body = (
      <Box sx={{p: 4}}>
        <Alert severity="error">
          {t('explorer.readFailed', 'What {{gateway}} runs could not be read.', {gateway: gateway.name})}
        </Alert>
      </Box>
    );
  } else if (!configuration.data?.version) {
    body = (
      <Box sx={{p: 4}}>
        <Typography variant="body2" color="text.secondary">
          {t('explorer.nothingDeployed', 'Nothing has been deployed to {{gateway}} yet.', {gateway: gateway.name})}
        </Typography>
      </Box>
    );
  } else {
    body = (
      <>
        <ExplorerCanvas
          graph={graph}
          selected={selected}
          onSelect={setSelected}
          labelFor={labelFor}
          heldBack={heldBack}
          fullscreen={fullscreen}
          onToggleFullscreen={toggleFullscreen}
        />
        <ExplorerPanel
          gateway={gateway}
          graph={graph}
          version={running}
          counts={counts}
          selected={node}
          onSelect={setSelected}
          labelFor={labelFor}
          variables={held}
          secrets={heldSecrets}
          updated={updated}
          drafts={drafts}
          onDraft={(name: string, value: string) =>
            setDrafts((previous: Map<string, string>) => new Map(previous).set(name, value))
          }
          revealed={revealed}
          onReveal={(name: string) =>
            setRevealed((previous: Set<string>) => {
              const next = new Set(previous);
              if (next.has(name)) next.delete(name);
              else next.add(name);
              return next;
            })
          }
          isSaving={isSaving}
          onSave={() => void save()}
          onDiscard={discard}
        />
      </>
    );
  }

  return (
    <Box
      sx={{height: '100vh', display: 'flex', flexDirection: 'column'}}
      onKeyDown={(event) => {
        if (event.key === 'Escape') setSelected(null);
      }}
    >
      <DocumentTitle title={gateway?.name ?? t('header.title', 'Deploy')} />
      <FullScreenHeader
        breadcrumbs={[
          {key: 'deploy', label: t('header.title', 'Deploy'), onClick: () => void navigate(routes.deploy.list())},
          {key: 'gateway', label: gateway?.name ?? gatewayId},
        ]}
        onExit={() => void navigate(routes.home.list())}
      />
      {gateway && (
        <ExplorerToolbar
          gateway={gateway}
          gateways={gateways.data ?? []}
          version={running}
          counts={counts}
          onSwitch={(id: string) => void navigate(routes.deploy.explore(id))}
        />
      )}
      {references.isError && configuration.data?.version && (
        <Alert severity="error" sx={{mx: 4, mt: 2}}>
          {t('explorer.referencesFailed', 'Which values {{gateway}} refers to could not be read, so none are shown.', {
            gateway: gateway?.name ?? gatewayId,
          })}
        </Alert>
      )}
      <Box
        ref={stageRef}
        data-testid="explorer-stage"
        sx={{display: 'flex', flex: 1, minHeight: 0, bgcolor: 'background.default'}}
      >
        {body}
      </Box>
    </Box>
  );
}
