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
  Button,
  Checkbox,
  Chip,
  Divider,
  FormControlLabel,
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
  ArrowRight,
  CheckCircle,
  CircleAlert,
  CircleX,
  Crosshair,
  ImageIcon,
  Monitor,
  Moon,
  MousePointerClick,
  RotateCcw,
  Smartphone,
  Sun,
  Tablet,
  Workflow,
  X,
} from '@wso2/oxygen-ui-icons-react';
import type {Node} from '@xyflow/react';
import DOMPurify from 'dompurify';
import {Fragment, useCallback, useMemo, useState, type ReactElement, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import type {FlowSimulation} from '../../hooks/useFlowSimulation';
import {ButtonVariants, ElementTypes, TypographyVariants, type Element} from '../../models/elements';
import type {StepData} from '../../models/steps';
import {SimulationOptionKinds, type SimulationOption} from '../../utils/getSimulationOptions';
import resolveStaticResourcePath from '../../utils/resolveStaticResourcePath';
import ConsentAdapter from '../resources/elements/adapters/ConsentAdapter';
import TemplatePlaceholder, {containsTemplateLiteral} from '../resources/elements/adapters/TemplatePlaceholder';
import GatePreview from '@/components/GatePreview/GatePreview';
import useGetApplication from '@/features/applications/api/useGetApplication';
import useGetApplications from '@/features/applications/api/useGetApplications';
import type {Application, BasicApplication} from '@/features/applications/models/application';

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

const HEADING_VARIANT_MAP: Record<string, 'h5' | 'h6' | 'body1' | 'body2'> = {
  [TypographyVariants.H1]: 'h5',
  [TypographyVariants.H2]: 'h6',
  [TypographyVariants.H3]: 'h6',
  [TypographyVariants.H4]: 'body1',
  [TypographyVariants.H5]: 'body1',
  [TypographyVariants.H6]: 'body2',
  [TypographyVariants.Body1]: 'body1',
  [TypographyVariants.Body2]: 'body2',
};

const KIND_LABEL_KEYS: Record<SimulationOptionKinds, string> = {
  [SimulationOptionKinds.Action]: 'flows:core.simulation.kinds.action',
  [SimulationOptionKinds.Success]: 'flows:core.simulation.kinds.success',
  [SimulationOptionKinds.Incomplete]: 'flows:core.simulation.kinds.incomplete',
  [SimulationOptionKinds.Failure]: 'flows:core.simulation.kinds.failure',
};

const KIND_COLORS: Record<SimulationOptionKinds, 'primary' | 'success' | 'warning' | 'error'> = {
  [SimulationOptionKinds.Action]: 'primary',
  [SimulationOptionKinds.Success]: 'success',
  [SimulationOptionKinds.Incomplete]: 'warning',
  [SimulationOptionKinds.Failure]: 'error',
};

function kindIcon(kind: SimulationOptionKinds): ReactElement {
  if (kind === SimulationOptionKinds.Success) return <CheckCircle size={15} />;
  if (kind === SimulationOptionKinds.Incomplete) return <CircleAlert size={15} />;
  if (kind === SimulationOptionKinds.Failure) return <CircleX size={15} />;
  return <MousePointerClick size={15} />;
}

type PreviewDevice = 'mobile' | 'tablet' | 'desktop';

const PREVIEW_DEVICES: PreviewDevice[] = ['mobile', 'tablet', 'desktop'];

/**
 * Aspect ratio per device preset. The themed preview scales the login page to
 * fill a box of this shape within the panel, so the proportions convey the device.
 */
const DEVICE_ASPECT_RATIOS: Record<PreviewDevice, string> = {
  mobile: '360 / 640',
  tablet: '540 / 680',
  desktop: '780 / 560',
};

const TEXT_INPUT_TYPES = new Set<string>([
  ElementTypes.TextInput,
  ElementTypes.PasswordInput,
  ElementTypes.EmailInput,
  ElementTypes.PhoneInput,
  ElementTypes.NumberInput,
  ElementTypes.DateInput,
]);

interface PreviewComponent extends Element {
  label?: string;
  placeholder?: string;
  src?: string;
  align?: 'inherit' | 'left' | 'center' | 'right' | 'justify';
  image?: string;
}

/**
 * When an application has no design configured, the flow meta API hands the gate
 * an empty theme object (see backend flowmeta service), which the design provider
 * merges over its standard defaults. The preview mirrors that exact fallback.
 */
const GATE_DEFAULT_THEME = {} as Theme;

const APPLICATION_META_PATTERN = /\{\{\s*meta\(application\.(\w+)\)\s*\}\}/g;

/**
 * Deeply resolves `{{ meta(application.*) }}` placeholders in a component tree
 * against the selected application (e.g. logoUrl, name).
 */
function resolveApplicationMeta<T>(value: T, application: Application): T {
  if (typeof value === 'string') {
    return value.replaceAll(APPLICATION_META_PATTERN, (_match, property: string) => {
      const resolved = (application as unknown as Record<string, unknown>)[property];
      return typeof resolved === 'string' ? resolved : '';
    }) as unknown as T;
  }
  if (Array.isArray(value)) {
    return (value as unknown[]).map((item: unknown) => resolveApplicationMeta(item, application)) as unknown as T;
  }
  if (value && typeof value === 'object') {
    return Object.fromEntries(
      Object.entries(value as Record<string, unknown>).map(([key, entry]) => [
        key,
        resolveApplicationMeta(entry, application),
      ]),
    ) as unknown as T;
  }
  return value;
}

/**
 * Skeleton markup for runtime-resolved input fields, mirroring the consent
 * placeholder's look: a dashed box sketching mock fields (label bar + input
 * outline) with an italic caption. Rendered through the gate's RICH_TEXT
 * adapter, so `currentColor`-based tints adapt to the applied theme.
 */
function dynamicFieldsSkeletonHtml(caption: string): string {
  const field = (labelWidth: string): string =>
    '<div style="margin-bottom:12px;">' +
    `<div style="width:${labelWidth};height:6px;border-radius:4px;background:currentColor;opacity:0.25;margin-bottom:7px;"></div>` +
    '<div style="height:36px;border:1px solid rgba(128,128,128,0.4);border-radius:6px;"></div>' +
    '</div>';
  return (
    '<div style="border:1px dashed rgba(128,128,128,0.55);border-radius:8px;padding:12px 14px 0;">' +
    `${field('35%')}${field('50%')}` +
    '</div>' +
    `<div style="text-align:center;font-style:italic;opacity:0.65;font-size:0.75em;margin:7px 0 0;">${caption}</div>`
  );
}

/**
 * Replaces dynamic input placeholders with a skeleton sketch of the fields.
 * The server resolves these into actual input fields at runtime, so the
 * preview shows a placeholder where the runtime ones will appear. The canvas
 * element's own placeholder/hint texts ("Dynamic Input", …) are builder
 * chrome and are blanked out.
 */
function withDynamicFieldStandIns(list: PreviewComponent[], caption: string): PreviewComponent[] {
  return list.map((component: PreviewComponent) => {
    if (component.type === ElementTypes.DynamicInputPlaceholder) {
      return {
        ...component,
        type: ElementTypes.RichText,
        label: dynamicFieldsSkeletonHtml(caption),
        placeholder: '',
        hint: '',
      } as PreviewComponent;
    }
    if (component.components?.length) {
      return {...component, components: withDynamicFieldStandIns(component.components as PreviewComponent[], caption)};
    }
    return component;
  });
}

/**
 * Deeply resolves i18n templates (`{{ t(...) }}`) in a component tree using the
 * provided text resolver, so the gate renderer receives display-ready labels
 * instead of raw translation keys it cannot resolve inside the console.
 */
function resolveTemplatesDeep(value: unknown, resolveText: (raw: string) => string): unknown {
  if (typeof value === 'string') {
    return containsTemplateLiteral(value) ? resolveText(value) : value;
  }
  if (Array.isArray(value)) {
    return (value as unknown[]).map((item: unknown) => resolveTemplatesDeep(item, resolveText));
  }
  if (value && typeof value === 'object') {
    return Object.fromEntries(
      Object.entries(value as Record<string, unknown>).map(([key, entry]) => [
        key,
        resolveTemplatesDeep(entry, resolveText),
      ]),
    );
  }
  return value;
}

/**
 * Read-only rendering of what the end user would see for the step focused by the
 * flow simulation. Action buttons that have an outgoing transition are live and
 * advance the simulation, exactly like the end user would.
 *
 * @param props - Props injected to the component.
 * @returns The SimulationStepPreview component.
 */
export default function SimulationStepPreview({node, simulation}: SimulationStepPreviewProps): ReactElement | null {
  const {t} = useTranslation();
  const {resolve, resolveAll} = useTemplateLiteralResolver();
  const {mode, systemMode} = useColorScheme();
  const consoleScheme: 'light' | 'dark' = (mode === 'system' ? systemMode : mode) === 'dark' ? 'dark' : 'light';
  const [selectedAppId, setSelectedAppId] = useState<string>('');
  const [device, setDevice] = useState<PreviewDevice>('mobile');
  const [previewColorScheme, setPreviewColorScheme] = useState<'light' | 'dark'>(consoleScheme);

  const {data: applicationsData} = useGetApplications();
  const applications = useMemo(() => applicationsData?.applications ?? [], [applicationsData]);
  // The preview opens themed as the built-in Console application by default —
  // there is no unthemed option, matching what an end user would actually see.
  const defaultAppId = useMemo(
    () => applications.find((application: BasicApplication) => application.name === 'Console')?.id ?? '',
    [applications],
  );
  const effectiveAppId = selectedAppId || defaultAppId;
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

  // Action components rendered inside the screen — options wired to these are
  // triggerable by clicking the screen itself, so they are left out of the footer.
  const screenActionIds = useMemo(() => {
    const ids = new Set<string>();
    const collect = (list: PreviewComponent[]): void => {
      list.forEach((component: PreviewComponent) => {
        if (component.type === ElementTypes.Action || component.type === ElementTypes.Resend) {
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
                  essential: [{name: t('flows:core.simulation.preview.consentEssentialPlaceholder')}],
                  optional: [{name: t('flows:core.simulation.preview.consentOptionalPlaceholder')}],
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
  const themedComponents: EmbeddedFlowComponent[] = useMemo(
    () =>
      selectedApplication
        ? (resolveTemplatesDeep(
            resolveApplicationMeta(
              withDynamicFieldStandIns(components, t('flows:core.simulation.preview.dynamicFieldsHint')),
              selectedApplication,
            ),
            (raw: string) => resolveAll(raw, {t}) ?? raw,
          ) as EmbeddedFlowComponent[])
        : [],
    [components, selectedApplication, resolveAll, t],
  );

  const handleGateSubmit = useCallback(
    (component: EmbeddedFlowComponent): void => {
      const option = simulation.options.find(
        (candidate: SimulationOption) => candidate.sourceComponentId === component.id,
      );
      if (option) {
        simulation.choose(option);
      }
    },
    [simulation],
  );

  // Hovering a component inside the themed screen previews the edge of the first
  // wired action in that component's subtree (e.g. a social login block's button).
  const handleGateHover = useCallback(
    (component: EmbeddedFlowComponent | null): void => {
      if (!component) {
        simulation.preview(null);
        return;
      }
      const subtreeIds = new Set<string>();
      const collect = (current: EmbeddedFlowComponent): void => {
        if (current.id) {
          subtreeIds.add(current.id);
        }
        current.components?.forEach(collect);
      };
      collect(component);
      const option = simulation.options.find(
        (candidate: SimulationOption) => candidate.sourceComponentId && subtreeIds.has(candidate.sourceComponentId),
      );
      simulation.preview(option ?? null);
    },
    [simulation],
  );

  if (!simulation.isSimulating || !node) {
    return null;
  }

  // null shows the loading spinner while the design resolves; an empty theme
  // renders the gate's default design (used when no design is configured).
  const theme = isDesignUnavailable ? GATE_DEFAULT_THEME : (resolvedDesign?.theme ?? null);
  const isThemedPreview = Boolean(effectiveAppId && components.length > 0);

  const isComplete = simulation.options.length === 0;
  const canGoBack = simulation.pathNodeIds.length > 1;
  const footerOptions = simulation.options.filter(
    (option: SimulationOption) => !(option.sourceComponentId && screenActionIds.has(option.sourceComponentId)),
  );
  const hasScreenOptions = footerOptions.length < simulation.options.length;

  const optionLabel = (option: SimulationOption): string => {
    if (option.actionLabel) {
      return containsTemplateLiteral(option.actionLabel)
        ? (resolve(option.actionLabel, {t}) ?? option.actionLabel)
        : option.actionLabel;
    }
    return t(KIND_LABEL_KEYS[option.kind]);
  };

  const resolveText = (raw: string | undefined): ReactNode => {
    if (!raw) {
      return null;
    }
    return containsTemplateLiteral(raw) ? <TemplatePlaceholder value={raw} t={t} /> : (resolve(raw, {t}) ?? raw);
  };

  const findOption = (componentId: string): SimulationOption | undefined =>
    simulation.options.find((option: SimulationOption) => option.sourceComponentId === componentId);

  const renderComponent = (component: PreviewComponent): ReactNode => {
    switch (component.type) {
      case ElementTypes.Text:
        return (
          <Typography
            variant={HEADING_VARIANT_MAP[component.variant as string] ?? 'body2'}
            align={component.align ?? 'left'}
          >
            {resolveText(component.label)}
          </Typography>
        );
      case ElementTypes.RichText: {
        // Mirrors RichTextAdapter: resolve templates, then render sanitized HTML.
        const raw = component.label ?? '';
        return (
          <Typography
            component="div"
            variant="body2"
            align={component.align ?? 'left'}
            sx={{'& a': {color: 'primary.main'}, '& p': {m: 0}}}
            // eslint-disable-next-line react/no-danger
            dangerouslySetInnerHTML={{
              __html: DOMPurify.sanitize(resolveAll(raw, {t}) ?? raw, {
                ADD_ATTR: ['target'],
                RETURN_TRUSTED_TYPE: false,
              }),
            }}
          />
        );
      }
      case ElementTypes.Image:
        return component.src && !containsTemplateLiteral(component.src) ? (
          <Box component="img" src={resolveStaticResourcePath(component.src)} sx={{maxHeight: 60, mx: 'auto'}} />
        ) : (
          <Box
            sx={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              height: 56,
              borderRadius: 1,
              bgcolor: 'action.hover',
              color: 'text.disabled',
            }}
          >
            <ImageIcon size={22} />
          </Box>
        );
      case ElementTypes.Divider:
        return component.label ? (
          <Divider>
            <Typography variant="caption" color="text.secondary">
              {resolveText(component.label)}
            </Typography>
          </Divider>
        ) : (
          <Divider />
        );
      case ElementTypes.Checkbox:
        return <FormControlLabel control={<Checkbox size="small" />} label={resolveText(component.label)} />;
      case ElementTypes.OtpInput:
        return (
          <Stack direction="row" spacing={0.75} justifyContent="center">
            {['d1', 'd2', 'd3', 'd4', 'd5', 'd6'].map((digit) => (
              <Box
                key={digit}
                sx={{width: 34, height: 40, border: '1px solid', borderColor: 'divider', borderRadius: 1}}
              />
            ))}
          </Stack>
        );
      case ElementTypes.Captcha:
        return (
          <Box sx={{p: 1.5, border: '1px solid', borderColor: 'divider', borderRadius: 1, textAlign: 'center'}}>
            <Typography variant="caption" color="text.secondary">
              reCAPTCHA
            </Typography>
          </Box>
        );
      case ElementTypes.Consent:
      case ElementTypes.ConsentInput:
        // The consented attributes are resolved per application at runtime —
        // sketch them with the same placeholder used on the canvas.
        return <ConsentAdapter />;
      case ElementTypes.DynamicInputPlaceholder:
        // Resolved into actual input fields by the server at runtime — sketch
        // mock fields where the runtime ones will appear, like the consent
        // placeholder does for attributes.
        return (
          <Box data-testid="dynamic-fields-placeholder">
            <Box
              sx={{
                border: '1px dashed',
                borderColor: 'divider',
                borderRadius: 1.5,
                px: 1.5,
                py: 1.25,
                display: 'flex',
                flexDirection: 'column',
                gap: 1.25,
              }}
            >
              {['35%', '50%'].map((labelWidth) => (
                <Box key={labelWidth}>
                  <Box
                    sx={{width: labelWidth, height: 6, borderRadius: 1, bgcolor: 'action.disabledBackground', mb: 0.75}}
                  />
                  <Box sx={{height: 36, border: '1px solid', borderColor: 'divider', borderRadius: 1}} />
                </Box>
              ))}
            </Box>
            <Typography
              variant="caption"
              color="textSecondary"
              sx={{fontStyle: 'italic', display: 'block', mt: 0.75, textAlign: 'center'}}
            >
              {t('flows:core.simulation.preview.dynamicFieldsHint')}
            </Typography>
          </Box>
        );
      case ElementTypes.Action:
      case ElementTypes.Resend: {
        const option = findOption(component.id);
        return (
          <Button
            fullWidth
            variant={component.variant === ButtonVariants.Primary ? 'contained' : 'outlined'}
            disabled={!option}
            onClick={option ? () => simulation.choose(option) : undefined}
            onMouseEnter={option ? () => simulation.preview(option) : undefined}
            onMouseLeave={option ? () => simulation.preview(null) : undefined}
            startIcon={
              component.image ? (
                <Box component="img" src={resolveStaticResourcePath(component.image)} sx={{height: 18}} />
              ) : undefined
            }
            sx={{textTransform: 'none'}}
          >
            {resolveText(component.label)}
          </Button>
        );
      }
      default:
        break;
    }

    if (TEXT_INPUT_TYPES.has(component.type)) {
      const resolvedLabel = component.label ? resolve(component.label, {t}) : undefined;
      const placeholder =
        component.placeholder && !containsTemplateLiteral(component.placeholder) ? component.placeholder : undefined;
      return (
        <TextField
          fullWidth
          size="small"
          label={resolvedLabel === '' ? undefined : resolvedLabel}
          placeholder={placeholder}
          type={component.type === ElementTypes.PasswordInput ? 'password' : 'text'}
        />
      );
    }

    if (component.components?.length) {
      return renderComponents(component.components as PreviewComponent[]);
    }

    return null;
  };

  const renderComponents = (list: PreviewComponent[]): ReactNode => (
    <Stack direction="column" spacing={2}>
      {list.map((component: PreviewComponent) => (
        <Fragment key={component.id}>{renderComponent(component)}</Fragment>
      ))}
    </Stack>
  );

  const renderScreen = (): ReactNode => {
    if (components.length === 0) {
      return (
        <Stack alignItems="center" spacing={1} sx={{px: 2, py: 4, color: 'text.secondary', textAlign: 'center'}}>
          <Workflow size={24} />
          <Typography variant="body2">{t('flows:core.simulation.preview.noScreen')}</Typography>
          <Typography variant="caption">{t('flows:core.simulation.preview.noScreenHint')}</Typography>
        </Stack>
      );
    }

    return (
      <Box sx={{width: 380, maxWidth: '100%'}}>
        <Box sx={{p: 2, border: '1px solid', borderColor: 'divider', borderRadius: 2, bgcolor: 'background.paper'}}>
          {renderComponents(components)}
        </Box>
      </Box>
    );
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
              {t('flows:core.simulation.preview.title')}
            </Typography>
            <Chip
              size="small"
              variant="outlined"
              label={t('flows:core.simulation.stepCount', {count: simulation.pathNodeIds.length})}
              sx={{color: 'text.secondary', flexShrink: 0}}
            />
          </Stack>
          <Stack direction="row" spacing={0.5} sx={{flexShrink: 0}}>
            <Tooltip
              title={t(
                simulation.followCamera ? 'flows:core.simulation.staticView' : 'flows:core.simulation.followSteps',
              )}
            >
              <IconButton
                size="small"
                onClick={simulation.toggleFollowCamera}
                aria-label={t(
                  simulation.followCamera ? 'flows:core.simulation.staticView' : 'flows:core.simulation.followSteps',
                )}
                aria-pressed={simulation.followCamera}
                sx={{
                  borderRadius: 1,
                  color: simulation.followCamera ? 'primary.main' : 'text.secondary',
                  bgcolor: simulation.followCamera ? 'action.selected' : 'transparent',
                }}
              >
                <Crosshair size={14} />
              </IconButton>
            </Tooltip>
            <Tooltip title={t('flows:core.simulation.back')}>
              <span>
                <IconButton
                  size="small"
                  onClick={simulation.back}
                  disabled={!canGoBack}
                  aria-label={t('flows:core.simulation.back')}
                >
                  <ArrowLeft size={15} />
                </IconButton>
              </span>
            </Tooltip>
            <Tooltip title={t('flows:core.simulation.restart')}>
              <IconButton size="small" onClick={simulation.start} aria-label={t('flows:core.simulation.restart')}>
                <RotateCcw size={14} />
              </IconButton>
            </Tooltip>
            <Tooltip title={t('flows:core.simulation.exit')}>
              <IconButton
                size="small"
                onClick={simulation.stop}
                aria-label={t('flows:core.simulation.exit')}
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
        <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{px: 0.5, py: 1, flexShrink: 0}}>
          <Stack direction="row" spacing={0.25}>
            {PREVIEW_DEVICES.map((previewDevice: PreviewDevice) => (
              <Tooltip key={previewDevice} title={t(`flows:core.simulation.preview.devices.${previewDevice}`)}>
                <IconButton
                  size="small"
                  onClick={() => setDevice(previewDevice)}
                  aria-label={t(`flows:core.simulation.preview.devices.${previewDevice}`)}
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
                  title={t(
                    previewColorScheme === 'dark'
                      ? 'flows:core.simulation.preview.lightMode'
                      : 'flows:core.simulation.preview.darkMode',
                  )}
                >
                  <IconButton
                    size="small"
                    onClick={() =>
                      setPreviewColorScheme((prev: 'light' | 'dark') => (prev === 'dark' ? 'light' : 'dark'))
                    }
                    aria-label={t(
                      previewColorScheme === 'dark'
                        ? 'flows:core.simulation.preview.lightMode'
                        : 'flows:core.simulation.preview.darkMode',
                    )}
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
            label={t('flows:core.simulation.preview.applicationLabel')}
            value={effectiveAppId}
            onChange={(event) => setSelectedAppId(event.target.value)}
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

        {isThemedPreview ? (
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
            {renderScreen()}
          </Box>
        )}

        {/* Transitions that cannot be triggered from the screen itself (e.g. executor outcomes) */}
        {(isComplete || footerOptions.length > 0 || hasScreenOptions) && (
          <Box
            data-testid="simulation-preview-footer"
            sx={{
              borderTop: '1px solid',
              borderColor: 'divider',
              px: 0.5,
              py: 1.25,
              flexShrink: 0,
              maxHeight: '45%',
              overflow: 'auto',
            }}
          >
            {isComplete && (
              <Stack direction="row" spacing={1} alignItems="center" sx={{color: 'success.main', py: 0.5}}>
                <CheckCircle size={16} />
                <Typography variant="body2">{t('flows:core.simulation.complete')}</Typography>
              </Stack>
            )}
            {!isComplete && footerOptions.length > 0 && (
              <>
                <Typography variant="overline" color="text.secondary" sx={{display: 'block', mb: 1, lineHeight: 1.5}}>
                  {t('flows:core.simulation.chooseNext')}
                </Typography>
                <Stack direction="column" spacing={1}>
                  {footerOptions.map((option: SimulationOption) => (
                    <Button
                      key={option.edgeId}
                      fullWidth
                      size="small"
                      onClick={() => simulation.choose(option)}
                      onMouseEnter={() => simulation.preview(option)}
                      onMouseLeave={() => simulation.preview(null)}
                      onFocus={() => simulation.preview(option)}
                      onBlur={() => simulation.preview(null)}
                      startIcon={kindIcon(option.kind)}
                      endIcon={<ArrowRight size={14} />}
                      sx={{
                        textTransform: 'none',
                        justifyContent: 'flex-start',
                        px: 1.5,
                        py: 0.75,
                        borderRadius: 1.5,
                        border: '1px solid',
                        borderColor: 'divider',
                        color: 'text.primary',
                        fontWeight: 500,
                        '& .MuiButton-startIcon': {color: `${KIND_COLORS[option.kind]}.main`},
                        '& .MuiButton-endIcon': {ml: 'auto', opacity: 0, transition: 'opacity 0.15s ease'},
                        '&:hover': {
                          borderColor: `${KIND_COLORS[option.kind]}.main`,
                          bgcolor: 'action.hover',
                          '& .MuiButton-endIcon': {opacity: 1, color: `${KIND_COLORS[option.kind]}.main`},
                        },
                      }}
                    >
                      {optionLabel(option)}
                    </Button>
                  ))}
                </Stack>
              </>
            )}
            {!isComplete && hasScreenOptions && (
              <Stack
                direction="row"
                spacing={1}
                alignItems="center"
                data-testid="simulation-screen-hint"
                sx={{color: 'text.secondary', pt: footerOptions.length > 0 ? 1.25 : 0.5, pb: 0.5}}
              >
                <MousePointerClick size={15} />
                <Typography variant="caption">
                  {footerOptions.length > 0
                    ? t('flows:core.simulation.screenHintOr')
                    : t('flows:core.simulation.screenHint')}
                </Typography>
              </Stack>
            )}
          </Box>
        )}
      </Box>
    </BuilderStaticPanel>
  );
}
