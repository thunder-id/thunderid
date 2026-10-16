// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useToast} from '@thunderid/contexts';
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
import type {Change, Gateway, StoredValue, ValueReference, Version} from '../models/deploy';
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

  const discard = (): void => {
    if (!node) return;
    setDrafts((previous: Map<string, string>) => {
      const next = new Map(previous);
      node.values.forEach((reference: ValueReference) => next.delete(reference.name));
      return next;
    });
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
    try {
      for (const reference of dirty) {
        const draft = drafts.get(reference.name) ?? '';
        const value = reference.list ? fromListText(draft) : draft;
        if (reference.kind === 'secret') await setSecret.mutateAsync({name: reference.name, value});
        else await setVariable.mutateAsync({name: reference.name, value});
      }
      setUpdated(
        (previous: Set<string>) =>
          new Set([...previous, ...dirty.filter((r) => r.kind === 'secret').map((r) => r.name)]),
      );
      discard();
      showToast(
        t('explorer.saved', 'Saved {{count}} values to {{gateway}}.', {count: dirty.length, gateway: gateway.name}),
        'success',
      );
    } catch (caught: unknown) {
      showToast(
        caught instanceof Error && caught.message
          ? caught.message
          : t('explorer.saveFailed', 'The values could not be saved.'),
        'error',
      );
    } finally {
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
      <FullScreenHeader
        breadcrumbs={[
          {key: 'deploy', label: t('header.title', 'Deploy'), onClick: () => void navigate(routes.deploy.list())},
          {key: 'gateway', label: gateway?.name ?? gatewayId},
        ]}
        onExit={() => void navigate('/')}
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
