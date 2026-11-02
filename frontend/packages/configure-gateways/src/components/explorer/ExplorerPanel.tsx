// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Button, IconButton, InputAdornment, Stack, TextField, Typography} from '@wso2/oxygen-ui';
import {Eye, EyeOff, KeyRound, PenLine, X} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {Tile} from './ResourceTile';
import {endpointsOf, gatewayEndpoints, type Endpoint} from '../../explorer/endpoints';
import {
  LANE_BY_ID,
  type Graph,
  type GraphEdge,
  type GraphNode,
  type Label,
  type LaneSummary,
} from '../../explorer/model';
import type {ValueReference} from '../../models/deploy';
import type {Gateway} from '../../models/gateway';
import CopyableUrl from '../CopyableUrl';

export interface ExplorerPanelProps {
  gateway: Gateway;
  graph: Graph;
  version?: string;
  counts?: string;
  selected: GraphNode | null;
  onSelect: (key: string | null) => void;
  labelFor: (label: Label) => string;
  /** What the gateway holds under each variable's name. */
  variables: ReadonlyMap<string, string>;
  /** The names of the secrets the gateway holds. */
  secrets: ReadonlySet<string>;
  /** The names whose stored value was replaced in this session and not captured since. */
  updated: ReadonlySet<string>;
  /** What has been typed for each value, by name, until saved or discarded. */
  drafts: ReadonlyMap<string, string>;
  onDraft: (name: string, value: string) => void;
  revealed: ReadonlySet<string>;
  onReveal: (name: string) => void;
  isSaving: boolean;
  onSave: () => void;
  onDiscard: () => void;
}

/**
 * How a value the panel lets one set reads as editable: a field on the card's surface, outlined more
 * firmly than the panel around it, taking the theme's own accent only while it is being typed in. No
 * colour of its own, so it sits in any console's theme and never reads as a warning.
 */
const EDITABLE = {
  '& .MuiOutlinedInput-root': {
    bgcolor: 'background.paper',
    '& fieldset': {borderColor: 'rgba(var(--oxygen-palette-text-primaryChannel) / 0.28)'},
    '&:hover fieldset': {borderColor: 'rgba(var(--oxygen-palette-text-primaryChannel) / 0.55)'},
  },
} as const;

/** URLs a resource or gateway is reached at on this gateway, each one copyable. */
function Endpoints({
  title,
  hint = undefined,
  endpoints,
}: {
  title: string;
  hint?: string;
  endpoints: Endpoint[];
}): JSX.Element | null {
  const {t} = useTranslation('deploy');
  if (endpoints.length === 0) return null;
  return (
    <Box data-testid="explorer-endpoints">
      <Typography variant="subtitle2">{title}</Typography>
      {hint && (
        <Typography variant="caption" color="text.secondary" component="div">
          {hint}
        </Typography>
      )}
      <Stack spacing={1} sx={{mt: 1}}>
        {endpoints.map((endpoint: Endpoint) => (
          <Box key={endpoint.key}>
            <Typography variant="caption" color="text.secondary" component="div" sx={{mb: 0.25}}>
              {t(endpoint.label[0], endpoint.label[1])}
            </Typography>
            <CopyableUrl url={endpoint.url} />
          </Box>
        ))}
      </Stack>
    </Box>
  );
}

/** The 360px panel beside the map: the gateway's overview, or the selected resource's values and links. */
export default function ExplorerPanel({
  gateway,
  graph,
  version = undefined,
  counts = undefined,
  selected,
  onSelect,
  labelFor,
  variables,
  secrets,
  updated,
  drafts,
  onDraft,
  revealed,
  onReveal,
  isSaving,
  onSave,
  onDiscard,
}: ExplorerPanelProps): JSX.Element {
  const {t} = useTranslation('deploy');
  const frame = {
    width: 360,
    flexShrink: 0,
    display: 'flex',
    flexDirection: 'column',
    borderLeft: 1,
    borderColor: 'divider',
    bgcolor: 'background.paper',
    overflow: 'auto',
  } as const;

  if (!selected) {
    return (
      <Box component="aside" data-testid="explorer-panel" sx={frame}>
        <Stack spacing={2} sx={{p: 2.5}}>
          <Box>
            <Typography variant="overline" color="text.secondary">
              {t('explorer.panel.environment', 'Gateway')}
            </Typography>
            <Typography variant="h6" fontWeight={600}>
              {gateway.name}
            </Typography>
            {gateway.baseUrl && (
              <Typography variant="caption" color="text.secondary" sx={{fontFamily: 'monospace'}}>
                {gateway.baseUrl}
              </Typography>
            )}
          </Box>
          <Stack direction="row" spacing={1.5}>
            <Summary title={t('explorer.panel.deployedVersion', 'Deployed version')} value={version ?? '—'} />
            <Summary title={t('explorer.panel.values', 'Values')} value={counts ?? '—'} />
          </Stack>
          <Endpoints
            title={t('explorer.panel.endpoints', 'Endpoints')}
            hint={t(
              'explorer.panel.gatewayEndpointsHint',
              'Where clients discover {{gateway}} and verify its tokens.',
              {
                gateway: gateway.name,
              },
            )}
            endpoints={gatewayEndpoints(gateway.baseUrl)}
          />
          <Box>
            <Typography variant="subtitle2" sx={{mb: 1}}>
              {t('explorer.panel.components', 'Components')}
            </Typography>
            <Stack spacing={0.75}>
              {graph.lanes.map((summary: LaneSummary) => (
                <Stack key={summary.lane.id} direction="row" alignItems="center" spacing={1}>
                  <Box sx={{width: 8, height: 8, borderRadius: 0.5, bgcolor: summary.lane.color}} />
                  <Typography variant="body2" sx={{flex: 1}}>
                    {t(summary.lane.title[0].replace(/^deploy:/, ''), summary.lane.title[1])}
                  </Typography>
                  <Typography variant="caption" color="text.secondary">
                    {summary.values > 0
                      ? t('explorer.panel.laneValues', '{{count}} · {{values}} values', {
                          count: summary.count,
                          values: summary.values,
                        })
                      : t('explorer.panel.laneShared', '{{count}} · shared', {count: summary.count})}
                  </Typography>
                </Stack>
              ))}
            </Stack>
          </Box>
          <Typography variant="body2" color="text.secondary">
            {t(
              'explorer.panel.selectHint',
              'Select a resource on the map to configure the values {{gateway}} holds for it.',
              {gateway: gateway.name},
            )}
          </Typography>
        </Stack>
      </Box>
    );
  }

  const lane = LANE_BY_ID[selected.lane];
  const own = selected.values;
  const variableRefs = own.filter((reference: ValueReference) => reference.kind === 'variable');
  const secretRefs = own.filter((reference: ValueReference) => reference.kind === 'secret');
  const dirty = own.some((reference: ValueReference) => {
    const draft = drafts.get(reference.name);
    if (draft === undefined) return false;
    return reference.kind === 'secret' ? draft !== '' : draft !== (variables.get(reference.name) ?? '');
  });
  const relationships = graph.edges
    .filter((edge: GraphEdge) => edge.from === selected.key || edge.to === selected.key)
    .map((edge: GraphEdge) => {
      const otherKey = edge.from === selected.key ? edge.to : edge.from;
      const other = graph.nodes.find((node: GraphNode) => node.key === otherKey);
      const from = graph.nodes.find((node: GraphNode) => node.key === edge.from);
      return other
        ? {
            other,
            label: edge.from === selected.key ? labelFor(edge.label) : `${from?.name ?? ''}: ${labelFor(edge.label)}`,
          }
        : null;
    })
    .filter((entry): entry is {other: GraphNode; label: string} => entry !== null);

  return (
    <Box component="aside" data-testid="explorer-panel" sx={frame}>
      <Stack direction="row" alignItems="center" spacing={1.5} sx={{p: 2.5, pb: 2}}>
        <Tile lane={lane} size={44} selected icon={selected.icon} letter={selected.letter} />
        <Box sx={{minWidth: 0, flex: 1}}>
          <Typography variant="subtitle1" fontWeight={600} noWrap>
            {selected.name}
          </Typography>
          <Typography variant="overline" color="text.secondary" sx={{lineHeight: 1.4}}>
            {labelFor(selected.kind)}
          </Typography>
        </Box>
        <IconButton size="small" aria-label={t('explorer.panel.close', 'Close')} onClick={() => onSelect(null)}>
          <X size={16} />
        </IconButton>
      </Stack>
      <Stack spacing={2.5} sx={{px: 2.5, pb: 2.5, flex: 1}}>
        {own.length === 0 && (
          <Typography variant="body2" color="text.secondary">
            {t(
              'explorer.panel.noValues',
              'This resource has no gateway-specific values. Its configuration is the same on every gateway and changes through a captured version.',
            )}
          </Typography>
        )}
        {own.length > 0 && (
          <Box
            data-testid="explorer-editable"
            sx={{
              p: 1.5,
              borderRadius: '8px',
              border: 1,
              borderColor: 'divider',
              bgcolor: 'action.hover',
            }}
          >
            <Stack direction="row" alignItems="center" spacing={1} sx={{mb: 1.5}}>
              <Box sx={{display: 'flex', color: 'text.secondary'}} aria-hidden>
                <PenLine size={14} />
              </Box>
              <Box sx={{minWidth: 0}}>
                <Typography variant="subtitle2" component="div">
                  {t('explorer.panel.valuesTitle', 'Values for {{gateway}}', {gateway: gateway.name})}
                </Typography>
                <Typography variant="caption" color="text.secondary" component="div">
                  {t('explorer.panel.valuesHint', 'Editable. Changes apply to {{gateway}} only.', {
                    gateway: gateway.name,
                  })}
                </Typography>
              </Box>
            </Stack>
            <Stack spacing={2}>
              {variableRefs.length > 0 && (
                <Box>
                  <Typography variant="subtitle2" sx={{mb: 1}}>
                    {t('explorer.panel.variables', 'Variables')}
                  </Typography>
                  <Stack spacing={1.5}>
                    {variableRefs.map((reference: ValueReference) => {
                      const stored = variables.get(reference.name) ?? '';
                      const value = drafts.get(reference.name) ?? stored;
                      const edited = drafts.has(reference.name) && value !== stored;
                      return (
                        <Box key={reference.name}>
                          <Stack direction="row" alignItems="center" spacing={1} sx={{mb: 0.5}}>
                            <Typography
                              component="label"
                              htmlFor={`value-${reference.name}`}
                              variant="caption"
                              sx={{fontFamily: 'monospace'}}
                            >
                              {reference.name}
                            </Typography>
                            {edited && (
                              <Typography variant="caption" color="warning.main">
                                {t('explorer.panel.edited', 'Edited')}
                              </Typography>
                            )}
                            {!variables.has(reference.name) && !edited && (
                              <Typography variant="caption" color="text.secondary">
                                {t('explorer.panel.notSet', 'Not set')}
                              </Typography>
                            )}
                          </Stack>
                          <TextField
                            id={`value-${reference.name}`}
                            size="small"
                            fullWidth
                            multiline={reference.list === true}
                            minRows={reference.list ? 2 : undefined}
                            value={value}
                            onChange={(event) => onDraft(reference.name, event.target.value)}
                            disabled={isSaving}
                            sx={EDITABLE}
                            slotProps={{input: {sx: {fontFamily: 'monospace', fontSize: 13}}}}
                            helperText={reference.list ? t('explorer.panel.listHint', 'One per line.') : undefined}
                          />
                        </Box>
                      );
                    })}
                  </Stack>
                </Box>
              )}
              {secretRefs.length > 0 && (
                <Box>
                  <Typography variant="subtitle2" sx={{mb: 1}}>
                    {t('explorer.panel.secrets', 'Secrets')}
                  </Typography>
                  <Stack spacing={1.5}>
                    {secretRefs.map((reference: ValueReference) => {
                      const held = secrets.has(reference.name);
                      const status = updated.has(reference.name)
                        ? t('explorer.panel.secretUpdated', 'Updated, not captured')
                        : held
                          ? t('explorer.panel.secretStored', 'Stored')
                          : t('explorer.panel.notSet', 'Not set');
                      return (
                        <Box key={reference.name}>
                          <Stack direction="row" alignItems="center" spacing={1} sx={{mb: 0.5}}>
                            <Box sx={{display: 'flex', color: 'text.secondary'}}>
                              <KeyRound size={13} />
                            </Box>
                            <Typography
                              component="label"
                              htmlFor={`value-${reference.name}`}
                              variant="caption"
                              sx={{fontFamily: 'monospace', flex: 1}}
                            >
                              {reference.name}
                            </Typography>
                            <Typography variant="caption" color="text.secondary">
                              {status}
                            </Typography>
                          </Stack>
                          <TextField
                            id={`value-${reference.name}`}
                            size="small"
                            fullWidth
                            type={revealed.has(reference.name) ? 'text' : 'password'}
                            autoComplete="new-password"
                            sx={EDITABLE}
                            placeholder={t('explorer.panel.secretPlaceholder', 'Enter a new value to replace')}
                            value={drafts.get(reference.name) ?? ''}
                            onChange={(event) => onDraft(reference.name, event.target.value)}
                            disabled={isSaving}
                            slotProps={{
                              input: {
                                endAdornment: (
                                  <InputAdornment position="end">
                                    <IconButton
                                      size="small"
                                      aria-label={
                                        revealed.has(reference.name)
                                          ? t('explorer.panel.hide', 'Hide')
                                          : t('explorer.panel.reveal', 'Show')
                                      }
                                      onClick={() => onReveal(reference.name)}
                                    >
                                      {revealed.has(reference.name) ? <EyeOff size={14} /> : <Eye size={14} />}
                                    </IconButton>
                                  </InputAdornment>
                                ),
                              },
                            }}
                          />
                        </Box>
                      );
                    })}
                  </Stack>
                </Box>
              )}
            </Stack>
          </Box>
        )}
        <Endpoints
          title={t('explorer.panel.endpoints', 'Endpoints')}
          hint={
            selected.resourceType === 'connection'
              ? t(
                  'explorer.panel.redirectHint',
                  'Register this with the provider so it returns users to {{gateway}}.',
                  {
                    gateway: gateway.name,
                  },
                )
              : t(
                  'explorer.panel.clientEndpointsHint',
                  'Where {{name}} signs users in and gets tokens on {{gateway}}.',
                  {
                    name: selected.name,
                    gateway: gateway.name,
                  },
                )
          }
          endpoints={endpointsOf(selected, gateway.baseUrl)}
        />
        {relationships.length > 0 && (
          <Box>
            <Typography variant="subtitle2" sx={{mb: 1}}>
              {t('explorer.panel.relationships', 'Relationships · {{count}}', {count: relationships.length})}
            </Typography>
            <Stack spacing={1}>
              {relationships.map((entry: {other: GraphNode; label: string}) => (
                <Stack
                  key={`${entry.other.key}:${entry.label}`}
                  component="button"
                  type="button"
                  direction="row"
                  alignItems="flex-start"
                  spacing={1}
                  onClick={() => onSelect(entry.other.key)}
                  sx={{
                    all: 'unset',
                    display: 'flex',
                    cursor: 'pointer',
                    borderRadius: 1,
                    '&:hover, &:focus-visible': {bgcolor: 'action.hover'},
                  }}
                >
                  <Box
                    sx={{
                      width: 8,
                      height: 8,
                      borderRadius: 0.5,
                      mt: 0.75,
                      flexShrink: 0,
                      bgcolor: LANE_BY_ID[entry.other.lane].color,
                    }}
                  />
                  <Box sx={{minWidth: 0}}>
                    <Typography variant="body2">{entry.other.name}</Typography>
                    <Typography variant="caption" color="text.secondary">
                      {entry.label}
                    </Typography>
                  </Box>
                </Stack>
              ))}
            </Stack>
          </Box>
        )}
      </Stack>
      {own.length > 0 && (
        <Stack
          direction="row"
          spacing={1}
          sx={{p: 2, borderTop: 1, borderColor: 'divider', justifyContent: 'flex-end'}}
        >
          <Button variant="text" color="inherit" onClick={onDiscard} disabled={!dirty || isSaving}>
            {t('explorer.panel.discard', 'Discard')}
          </Button>
          <Button variant="contained" onClick={onSave} disabled={!dirty || isSaving} sx={{borderRadius: 100}}>
            {isSaving ? t('explorer.panel.saving', 'Saving...') : t('explorer.panel.save', 'Save changes')}
          </Button>
        </Stack>
      )}
    </Box>
  );
}

function Summary({title, value}: {title: string; value: string}): JSX.Element {
  return (
    <Box sx={{flex: 1, p: 1.5, borderRadius: '8px', border: 1, borderColor: 'divider'}}>
      <Typography variant="caption" color="text.secondary" component="div">
        {title}
      </Typography>
      <Typography variant="body2" fontWeight={600} sx={{fontFamily: 'monospace'}}>
        {value}
      </Typography>
    </Box>
  );
}
