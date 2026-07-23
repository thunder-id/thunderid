/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {BuilderStaticPanel} from '@thunderid/components';
import {DefaultTheme, DesignResolveType, useGetDesignResolve, type Theme} from '@thunderid/design';
import {useTemplateLiteralResolver} from '@thunderid/hooks';
import type {EmbeddedFlowComponent} from '@thunderid/react';
import {
  Box,
  Chip,
  IconButton,
  InputAdornment,
  MenuItem,
  Stack,
  TextField,
  Tooltip,
  Typography,
  useColorScheme,
} from '@wso2/oxygen-ui';
import {
  AppWindow,
  ArrowLeft,
  Crosshair,
  Monitor,
  Moon,
  RotateCcw,
  Smartphone,
  Sun,
  Tablet,
  Workflow,
  X,
} from '@wso2/oxygen-ui-icons-react';
import type {Node} from '@xyflow/react';
import {useCallback, useEffect, useMemo, useRef, useState, type ReactElement} from 'react';
import {useTranslation} from 'react-i18next';
import SimulationOptionsFooter from './SimulationOptionsFooter';
import SimulationScreenSketch from './SimulationScreenSketch';
import {
  DEVICE_ASPECT_RATIOS,
  DEVICE_LABEL_FALLBACKS,
  GATE_DEFAULT_THEME,
  PREVIEW_APP_STORAGE_KEY,
  PREVIEW_DEVICE_STORAGE_KEY,
  PREVIEW_DEVICES,
  type PreviewDevice,
} from '../../constants/simulationPreviewConstants';
import type {FlowSimulation} from '../../hooks/useFlowSimulation';
import {ElementTypes} from '../../models/elements';
import type {StepData} from '../../models/steps';
import {
  resolveApplicationMeta,
  resolveTemplatesDeep,
  withDerivedEventTypes,
  withDynamicFieldStandIns,
  withRichTextActionRefs,
  type PreviewComponent,
} from '../../utils/gatePreviewTransforms';
import type {SimulationOption} from '../../utils/getSimulationOptions';
import GatePreview from '@/components/GatePreview/GatePreview';
import useGetApplication from '@/features/applications/api/useGetApplication';
import useGetApplications from '@/features/applications/api/useGetApplications';
import type {BasicApplication} from '@/features/applications/models/application';

/**
 * Props interface of {@link SimulationStepPreview}
 */
export interface SimulationStepPreviewProps {
  /**
   * The node currently focused by the simulation.
   */
  node: Node | null;
  /**
   * The flow simulation state and actions.
   */
  simulation: FlowSimulation;
}

/**
 * Read-only rendering of what the end user would see for the step focused by the
 * flow simulation. Action buttons that have an outgoing transition are live and
 * advance the simulation, exactly like the end user would.
 *
 * @param props - Props injected to the component.
 * @returns The SimulationStepPreview component.
 */
const readStored = (key: string): string | null => {
  try {
    return localStorage.getItem(key);
  } catch {
    return null;
  }
};

const writeStored = (key: string, value: string): void => {
  try {
    localStorage.setItem(key, value);
  } catch {
    // Persistence is best-effort (e.g. blocked storage access).
  }
};

export default function SimulationStepPreview({node, simulation}: SimulationStepPreviewProps): ReactElement | null {
  const {t} = useTranslation();
  const {resolveAll} = useTemplateLiteralResolver();
  const {mode, systemMode} = useColorScheme();
  const consoleScheme: 'light' | 'dark' = (mode === 'system' ? systemMode : mode) === 'dark' ? 'dark' : 'light';
  // Device and application picks persist across preview sessions.
  const [selectedAppId, setSelectedAppId] = useState<string>(() => readStored(PREVIEW_APP_STORAGE_KEY) ?? '');
  const [device, setDevice] = useState<PreviewDevice>(() => {
    const stored = readStored(PREVIEW_DEVICE_STORAGE_KEY);
    return stored && (PREVIEW_DEVICES as readonly string[]).includes(stored) ? (stored as PreviewDevice) : 'mobile';
  });
  const [previewColorScheme, setPreviewColorScheme] = useState<'light' | 'dark'>(consoleScheme);

  const {data: applicationsData} = useGetApplications();
  const applications = useMemo(() => applicationsData?.applications ?? [], [applicationsData]);
  // The preview opens themed as the built-in Console application by default —
  // there is no unthemed option, matching what an end user would actually see.
  const defaultAppId = useMemo(
    () => applications.find((application: BasicApplication) => application.name === 'Console')?.id ?? '',
    [applications],
  );
  const isKnownApp = applications.some((application: BasicApplication) => application.id === selectedAppId);
  const effectiveAppId = selectedAppId && isKnownApp ? selectedAppId : defaultAppId;
  const {data: selectedApplication} = useGetApplication(effectiveAppId);
  // The gate's own design resolution for the application. Responds 404 when the
  // application has no design configured at all — the gate then falls back to
  // the renderer's built-in default design, and so does the preview.
  const {data: resolvedDesign, isError: isDesignUnavailable} = useGetDesignResolve({
    type: DesignResolveType.APP,
    id: effectiveAppId,
  });

  const components = useMemo(
    () => ((node?.data as StepData | undefined)?.components ?? []) as PreviewComponent[],
    [node?.data],
  );

  // Interactive components rendered inside the screen (buttons, resend actions,
  // rich-text links) — options wired to these are triggerable by clicking the
  // screen itself, so they are left out of the footer to avoid duplication.
  // Deliberately restricted to types both renderers make clickable: an option
  // wired to anything else stays in the footer so its path remains reachable.
  const screenComponentIds = useMemo(() => {
    const triggerableTypes = new Set<string>([ElementTypes.Action, ElementTypes.Resend, ElementTypes.RichText]);
    const ids = new Set<string>();
    const collect = (list: PreviewComponent[]): void => {
      list.forEach((component: PreviewComponent) => {
        if (triggerableTypes.has(component.type)) {
          ids.add(component.id);
        }
        if (component.components?.length) {
          collect(component.components as PreviewComponent[]);
        }
      });
    };
    collect(components);
    return ids;
  }, [components]);

  // The consent attribute list is resolved per application at runtime — when the
  // step contains a consent input, hand the gate renderer placeholder purposes so
  // its real consent UI renders, just like during flow execution.
  const hasConsentInput = useMemo(() => {
    const containsConsent = (list: PreviewComponent[] | undefined): boolean =>
      Boolean(
        list?.some(
          (component: PreviewComponent) =>
            component.type === ElementTypes.Consent ||
            component.type === ElementTypes.ConsentInput ||
            containsConsent(component.components as PreviewComponent[] | undefined),
        ),
      );
    return containsConsent(components);
  }, [components]);

  const gateAdditionalData = useMemo(
    () =>
      hasConsentInput
        ? {
            consentPrompt: {
              purposes: [
                {
                  purposeId: 'preview_placeholder',
                  type: 'attributes',
                  essential: [
                    {
                      name: t(
                        'flows:core.simulation.preview.consentEssentialPlaceholder',
                        'Attributes requested by the application',
                      ),
                    },
                  ],
                  optional: [
                    {
                      name: t(
                        'flows:core.simulation.preview.consentOptionalPlaceholder',
                        'Optional attributes the user can toggle',
                      ),
                    },
                  ],
                },
              ],
            },
          }
        : undefined,
    [hasConsentInput, t],
  );

  // Themed mode: the selected application's meta placeholders (logo, name) and
  // i18n templates resolved into the step components, rendered by the real gate
  // renderer. resolveAll handles mixed content (e.g. rich text HTML containing
  // several templates) — resolve() would drop everything after the first one.
  // Maps each wired component id to its edge id (the SDK action ref) so rich-text
  // links can have their `action.ref` re-attached for the gate renderer.
  const refByComponentId = useMemo(() => {
    const map = new Map<string, string>();
    simulation.options.forEach((option: SimulationOption) => {
      if (option.sourceComponentId) {
        map.set(option.sourceComponentId, option.edgeId);
      }
    });
    return map;
  }, [simulation.options]);

  const themedComponents: EmbeddedFlowComponent[] = useMemo(
    () =>
      selectedApplication
        ? (resolveTemplatesDeep(
            resolveApplicationMeta(
              withRichTextActionRefs(
                withDerivedEventTypes(
                  withDynamicFieldStandIns(
                    components,
                    t('flows:core.simulation.preview.dynamicFieldsHint', 'Input fields resolved at runtime'),
                  ),
                ),
                refByComponentId,
              ),
              selectedApplication,
            ),
            (raw: string) => resolveAll(raw, {t}) ?? raw,
          ) as EmbeddedFlowComponent[])
        : [],
    [components, selectedApplication, resolveAll, t, refByComponentId],
  );

  // Depend on the stable members, not the aggregate object — new handler
  // identities would reconcile the whole iframe subtree on every state change.
  const {options: simulationOptions, choose: chooseOption, preview: previewOption} = simulation;

  // Both gate handlers resolve the first wired option in the reported
  // component's subtree — the gate may report a container (e.g. a social login
  // block) rather than the wired child itself.
  //
  // Rich-text links are reported as a synthetic action whose id is the
  // component's `action.ref`. That ref is also the edge id (and therefore the
  // option's `edgeId`), so options are matched on `sourceComponentId` OR
  // `edgeId` — the latter covers link clicks without depending on the loaded
  // component still carrying its `action.ref`.
  const findWiredOption = useCallback(
    (component: EmbeddedFlowComponent): SimulationOption | undefined => {
      const candidateIds = new Set<string>();
      const collect = (current: EmbeddedFlowComponent): void => {
        if (current.id) {
          candidateIds.add(current.id);
        }
        current.components?.forEach(collect);
      };
      collect(component);
      return simulationOptions.find(
        (candidate: SimulationOption) =>
          candidateIds.has(candidate.edgeId) ||
          (candidate.sourceComponentId !== undefined && candidateIds.has(candidate.sourceComponentId)),
      );
    },
    [simulationOptions],
  );

  const handleGateSubmit = useCallback(
    (component: EmbeddedFlowComponent): void => {
      const option = findWiredOption(component);
      if (option) {
        chooseOption(option);
      }
    },
    [findWiredOption, chooseOption],
  );

  const handleGateHover = useCallback(
    (component: EmbeddedFlowComponent | null): void => {
      previewOption(component ? (findWiredOption(component) ?? null) : null);
    },
    [findWiredOption, previewOption],
  );

  const footerRef = useRef<HTMLDivElement | null>(null);

  // Keyboard support while the preview is open: Escape exits, Backspace steps
  // back, ArrowUp/ArrowDown walk the transition options (a focused option is
  // previewed via its own focus handler; Enter activates it natively). Keys are
  // ignored while typing in an input, and cannot reach here while focus is
  // inside the themed gate's iframe (keydown does not cross the frame boundary).
  //
  // stop/back are read through refs, kept fresh every render, so the listener
  // effect below only depends on `isSimulating` — attaching/detaching the
  // window listener on preview start/stop, not on every step (stop/back's
  // identity changes every step because it closes over the traversal path).
  const {isSimulating} = simulation;
  const stopRef = useRef(simulation.stop);
  const backRef = useRef(simulation.back);
  useEffect(() => {
    stopRef.current = simulation.stop;
  }, [simulation.stop]);
  useEffect(() => {
    backRef.current = simulation.back;
  }, [simulation.back]);

  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent): void => {
      if (!isSimulating) {
        return;
      }
      const target = event.target instanceof HTMLElement ? event.target : null;
      if (target?.closest('input, textarea, select, [contenteditable="true"]')) {
        return;
      }
      if (event.key === 'Escape') {
        event.preventDefault();
        stopRef.current();
        return;
      }
      if (event.key === 'Backspace') {
        event.preventDefault();
        backRef.current();
        return;
      }
      if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
        const footer = footerRef.current;
        const optionButtons = footer
          ? Array.from(footer.querySelectorAll<HTMLButtonElement>('button:not([disabled])'))
          : [];
        if (optionButtons.length === 0) {
          return;
        }
        event.preventDefault();
        const activeIndex = optionButtons.findIndex((button) => button === document.activeElement);
        const delta = event.key === 'ArrowDown' ? 1 : -1;
        const nextIndex =
          activeIndex === -1
            ? delta === 1
              ? 0
              : optionButtons.length - 1
            : (activeIndex + delta + optionButtons.length) % optionButtons.length;
        optionButtons[nextIndex].focus();
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [isSimulating]);

  if (!simulation.isSimulating || !node) {
    return null;
  }

  // null shows the loading spinner while the design resolves; an empty theme
  // renders the gate's default design — used when no design is configured (the
  // resolve endpoint 404s) or when the resolved design carries no theme (e.g.
  // layout-only designs), which must not be treated as still loading.
  let theme: Theme | null = null;
  if (isDesignUnavailable) {
    theme = GATE_DEFAULT_THEME;
  } else if (resolvedDesign) {
    theme = resolvedDesign.theme ?? GATE_DEFAULT_THEME;
  }
  // Requires the loaded application (not just its id) — while the fetch is
  // pending or failed, the themed screen would render an empty card while the
  // footer still hides its screen-wired actions, stranding the walk.
  // Background steps (no screen components) get no preview chrome at all: the
  // device frame, device toggles, and application selector only make sense when
  // there is a screen to render.
  const hasScreen = components.length > 0;

  // Start and End are structural markers, not background work - the hint card
  // would only add noise there.
  const nodeType = node?.type?.toUpperCase() ?? '';
  const showBackgroundStepCard = !hasScreen && nodeType !== 'START' && nodeType !== 'END';

  // Identify the background step in the hint card: prefer the executor's
  // display label, then the executor name; Call steps invoke another flow.
  const backgroundStepData = node?.data as
    | {display?: {label?: string}; action?: {executor?: {name?: string}}}
    | undefined;
  const backgroundStepLabel =
    backgroundStepData?.display?.label ??
    backgroundStepData?.action?.executor?.name ??
    (nodeType === 'CALL' ? t('flows:core.simulation.preview.callStepLabel', 'Calls another flow') : undefined);
  const isThemedPreview = Boolean(effectiveAppId && selectedApplication && hasScreen);

  const isComplete = simulation.options.length === 0;
  const canGoBack = simulation.pathNodeIds.length > 1;

  const footerOptions = simulation.options.filter(
    (option: SimulationOption) => !(option.sourceComponentId && screenComponentIds.has(option.sourceComponentId)),
  );
  const hasScreenOptions = footerOptions.length < simulation.options.length;

  const footerProps = {
    options: footerOptions,
    isComplete,
    hasScreenOptions,
    onChoose: chooseOption,
    onPreview: previewOption,
  };

  const deviceIcon = (previewDevice: PreviewDevice): ReactElement => {
    if (previewDevice === 'mobile') return <Smartphone size={14} />;
    if (previewDevice === 'tablet') return <Tablet size={14} />;
    return <Monitor size={14} />;
  };

  return (
    <BuilderStaticPanel
      open
      width={380}
      anchor="right"
      paperSx={{overflow: 'hidden'}}
      header={
        <Box sx={{display: 'flex', alignItems: 'center', justifyContent: 'space-between', width: '100%'}}>
          <Stack direction="row" spacing={1} alignItems="center" sx={{minWidth: 0}}>
            <Typography variant="h6" noWrap>
              {t('flows:core.simulation.preview.title', 'End-user preview')}
            </Typography>
            <Chip
              size="small"
              variant="outlined"
              label={t('flows:core.simulation.stepCount', 'Step {{count}}', {count: simulation.pathNodeIds.length})}
              sx={{color: 'text.secondary', flexShrink: 0}}
            />
          </Stack>
          <Stack direction="row" spacing={0.5} sx={{flexShrink: 0}}>
            <Tooltip
              title={
                simulation.followCamera
                  ? t('flows:core.simulation.staticView', 'Switch to a static canvas view')
                  : t('flows:core.simulation.followSteps', 'Follow steps on the canvas')
              }
            >
              <IconButton
                size="small"
                onClick={simulation.toggleFollowCamera}
                aria-label={
                  simulation.followCamera
                    ? t('flows:core.simulation.staticView', 'Switch to a static canvas view')
                    : t('flows:core.simulation.followSteps', 'Follow steps on the canvas')
                }
                sx={{
                  borderRadius: 1,
                  color: simulation.followCamera ? 'primary.main' : 'text.secondary',
                  bgcolor: simulation.followCamera ? 'action.selected' : 'transparent',
                }}
              >
                <Crosshair size={14} />
              </IconButton>
            </Tooltip>
            <Tooltip title={t('flows:core.simulation.back', 'Go back one step')}>
              <span>
                <IconButton
                  size="small"
                  onClick={simulation.back}
                  disabled={!canGoBack}
                  aria-label={t('flows:core.simulation.back', 'Go back one step')}
                >
                  <ArrowLeft size={15} />
                </IconButton>
              </span>
            </Tooltip>
            <Tooltip title={t('flows:core.simulation.restart', 'Restart preview')}>
              <IconButton
                size="small"
                onClick={simulation.start}
                aria-label={t('flows:core.simulation.restart', 'Restart preview')}
              >
                <RotateCcw size={14} />
              </IconButton>
            </Tooltip>
            <Tooltip title={t('flows:core.simulation.exit', 'Exit preview')}>
              <IconButton
                size="small"
                onClick={simulation.stop}
                aria-label={t('flows:core.simulation.exit', 'Exit preview')}
                data-testid="simulation-preview-close"
              >
                <X size={16} />
              </IconButton>
            </Tooltip>
          </Stack>
        </Box>
      }
    >
      <Box
        data-testid="simulation-step-preview"
        data-device={device}
        sx={{flex: 1, minHeight: 0, display: 'flex', flexDirection: 'column'}}
      >
        {!hasScreen && (
          <Stack spacing={0.5} sx={{pt: 0.5}}>
            <SimulationOptionsFooter {...footerProps} placement="top" rootRef={footerRef} />
            {showBackgroundStepCard && (
              <Stack
                direction="row"
                spacing={1.25}
                alignItems="flex-start"
                data-testid="simulation-background-step"
                sx={{
                  mx: 0.5,
                  px: 1.5,
                  py: 1.25,
                  borderRadius: 1.5,
                  bgcolor: 'action.hover',
                  color: 'text.secondary',
                }}
              >
                <Box sx={{display: 'inline-flex', flexShrink: 0, mt: '2px'}}>
                  <Workflow size={16} />
                </Box>
                <Box>
                  <Typography variant="body2" color="text.primary">
                    {backgroundStepLabel ??
                      t('flows:core.simulation.preview.noScreen', 'No screen is shown for this step')}
                  </Typography>
                  <Typography variant="caption">
                    {node?.id
                      ? t(
                          'flows:core.simulation.preview.noScreenHintNamed',
                          '{{id}} runs in the background before the flow continues',
                          {id: node.id},
                        )
                      : t(
                          'flows:core.simulation.preview.noScreenHint',
                          'This step runs in the background before the flow continues',
                        )}
                  </Typography>
                </Box>
              </Stack>
            )}
          </Stack>
        )}

        {hasScreen && (
          <Stack
            direction="row"
            alignItems="center"
            justifyContent="space-between"
            sx={{px: 0.5, py: 1, flexShrink: 0}}
          >
            <Stack direction="row" spacing={0.25}>
              {PREVIEW_DEVICES.map((previewDevice: PreviewDevice) => (
                <Tooltip
                  key={previewDevice}
                  title={t(
                    `flows:core.simulation.preview.devices.${previewDevice}`,
                    DEVICE_LABEL_FALLBACKS[previewDevice],
                  )}
                >
                  <IconButton
                    size="small"
                    onClick={() => {
                      setDevice(previewDevice);
                      writeStored(PREVIEW_DEVICE_STORAGE_KEY, previewDevice);
                    }}
                    aria-label={t(
                      `flows:core.simulation.preview.devices.${previewDevice}`,
                      DEVICE_LABEL_FALLBACKS[previewDevice],
                    )}
                    aria-pressed={device === previewDevice}
                    sx={{
                      borderRadius: 1,
                      color: device === previewDevice ? 'primary.main' : 'text.secondary',
                      bgcolor: device === previewDevice ? 'action.selected' : 'transparent',
                    }}
                  >
                    {deviceIcon(previewDevice)}
                  </IconButton>
                </Tooltip>
              ))}
              {isThemedPreview && (
                <>
                  <Box sx={{width: '1px', height: 16, bgcolor: 'divider', mx: 0.5, alignSelf: 'center'}} />
                  <Tooltip
                    title={
                      previewColorScheme === 'dark'
                        ? t('flows:core.simulation.preview.lightMode', 'Switch to light preview')
                        : t('flows:core.simulation.preview.darkMode', 'Switch to dark preview')
                    }
                  >
                    <IconButton
                      size="small"
                      onClick={() =>
                        setPreviewColorScheme((prev: 'light' | 'dark') => (prev === 'dark' ? 'light' : 'dark'))
                      }
                      aria-label={
                        previewColorScheme === 'dark'
                          ? t('flows:core.simulation.preview.lightMode', 'Switch to light preview')
                          : t('flows:core.simulation.preview.darkMode', 'Switch to dark preview')
                      }
                      sx={{borderRadius: 1, color: 'text.secondary'}}
                    >
                      {previewColorScheme === 'dark' ? <Sun size={14} /> : <Moon size={14} />}
                    </IconButton>
                  </Tooltip>
                </>
              )}
            </Stack>
            <TextField
              select
              size="small"
              label={t('flows:core.simulation.preview.applicationLabel', 'Preview as application')}
              value={effectiveAppId}
              onChange={(event) => {
                setSelectedAppId(event.target.value);
                writeStored(PREVIEW_APP_STORAGE_KEY, event.target.value);
              }}
              sx={{
                minWidth: 180,
              }}
              slotProps={{
                inputLabel: {shrink: true},
                input: {
                  startAdornment: (
                    <InputAdornment position="start">
                      <AppWindow size={14} />
                    </InputAdornment>
                  ),
                },
              }}
            >
              {applications.map((application: BasicApplication) => (
                <MenuItem key={application.id} value={application.id}>
                  {application.name}
                </MenuItem>
              ))}
            </TextField>
          </Stack>
        )}

        {hasScreen &&
          (isThemedPreview ? (
            <Box sx={{flex: 1, minHeight: 0, overflow: 'auto'}}>
              <Box
                sx={{
                  width: '100%',
                  aspectRatio: DEVICE_ASPECT_RATIOS[device],
                  border: '1px solid',
                  borderColor: 'divider',
                  borderRadius: 1,
                  overflow: 'hidden',
                }}
              >
                <GatePreview
                  frameless
                  theme={theme}
                  // The gate merges resolved designs over its default theme and shows
                  // its default branding when no design is configured — mirror both.
                  baseTheme={DefaultTheme as Theme}
                  themelessBranding
                  displayName={selectedApplication?.name ?? ''}
                  showToolbar={false}
                  colorScheme={previewColorScheme}
                  mock={themedComponents}
                  stylesheets={resolvedDesign?.layout?.head?.stylesheets ?? []}
                  onSubmit={handleGateSubmit}
                  onComponentHover={handleGateHover}
                  additionalData={gateAdditionalData}
                />
              </Box>
            </Box>
          ) : (
            <Box sx={{flex: 1, minHeight: 0, overflow: 'auto', display: 'flex', justifyContent: 'center', py: 1}}>
              <SimulationScreenSketch
                components={components}
                options={simulationOptions}
                onChoose={chooseOption}
                onPreview={previewOption}
              />
            </Box>
          ))}

        {hasScreen && <SimulationOptionsFooter {...footerProps} rootRef={footerRef} />}
      </Box>
    </BuilderStaticPanel>
  );
}
