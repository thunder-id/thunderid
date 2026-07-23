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

import {useTemplateLiteralResolver} from '@thunderid/hooks';
import {Box, Button, Checkbox, Divider, FormControlLabel, Stack, TextField, Typography} from '@wso2/oxygen-ui';
import {ImageIcon, Workflow} from '@wso2/oxygen-ui-icons-react';
import DOMPurify from 'dompurify';
import {Fragment, type MouseEvent, type ReactElement, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import {GATE_CARD_WIDTH, SKETCH_TEXT_INPUT_TYPES, SKETCH_ZOOM} from '../../constants/simulationPreviewConstants';
import {VARIANT_TO_MUI_MAP} from '../../constants/typographyVariantMaps';
import {ButtonVariants, ElementTypes} from '../../models/elements';
import type {PreviewComponent} from '../../utils/gatePreviewTransforms';
import type {SimulationOption} from '../../utils/getSimulationOptions';
import resolveStaticResourcePath from '../../utils/resolveStaticResourcePath';
import ConsentAdapter from '../resources/elements/adapters/ConsentAdapter';
import TemplatePlaceholder, {containsTemplateLiteral} from '../resources/elements/adapters/TemplatePlaceholder';

/**
 * Props interface of {@link SimulationScreenSketch}
 */
export interface SimulationScreenSketchProps {
  /**
   * Components of the previewed step's view.
   */
  components: PreviewComponent[];
  /**
   * Outgoing transitions of the current step — actions wired to one are live.
   */
  options: SimulationOption[];
  /**
   * Follows the given transition.
   */
  onChoose: (option: SimulationOption) => void;
  /**
   * Previews the given transition on the canvas (null clears the preview).
   */
  onPreview: (option: SimulationOption | null) => void;
}

/**
 * Unthemed sketch of what the end user sees for the previewed step. Action
 * buttons with an outgoing transition are live and advance the simulation.
 *
 * @param props - Props injected to the component.
 * @returns The SimulationScreenSketch component.
 */
function SimulationScreenSketch({components, options, onChoose, onPreview}: SimulationScreenSketchProps): ReactElement {
  const {t} = useTranslation();
  const {resolve, resolveAll} = useTemplateLiteralResolver();

  const resolveText = (raw: string | undefined): ReactNode => {
    if (!raw) {
      return null;
    }
    return containsTemplateLiteral(raw) ? <TemplatePlaceholder value={raw} t={t} /> : (resolve(raw, {t}) ?? raw);
  };

  const findOption = (componentId: string): SimulationOption | undefined =>
    options.find((option: SimulationOption) => option.sourceComponentId === componentId);

  const renderComponent = (component: PreviewComponent): ReactNode => {
    switch (component.type) {
      case ElementTypes.Text:
        return (
          <Typography
            variant={VARIANT_TO_MUI_MAP[component.variant as string] ?? 'body2'}
            align={component.align ?? 'left'}
          >
            {resolveText(component.label)}
          </Typography>
        );
      case ElementTypes.RichText: {
        // Mirrors RichTextAdapter: resolve templates, then render sanitized HTML.
        const raw = component.label ?? '';

        // A rich text can carry more than one anchor (e.g. a plain "Terms" link
        // next to a wired "Reset" link) — resolve the option from the specific
        // anchor interacted with, not from the whole component, so clicking an
        // unrelated anchor cannot fire the wired transition. This mirrors the
        // SDK's own RichTextAdapter: when any anchor in the container carries
        // `data-action-ref` (the sentinel, stamped at save time on the wired
        // anchor — see reactFlowTransformer), the clicked anchor's own ref must
        // match an option's `edgeId`. Only when NO anchor carries the attribute
        // at all (hand-authored content predating the convention, with a single
        // wired link) does any anchor click fall back to the component-level
        // match — safe there because there is no other anchor it could be.
        const resolveAnchorOption = (
          container: HTMLElement,
          target: EventTarget | null,
        ): SimulationOption | undefined => {
          const anchor = target instanceof HTMLElement ? target.closest('a') : null;
          if (!anchor) {
            return undefined;
          }
          const hasSentinel = container.querySelector('a[data-action-ref]') !== null;
          if (hasSentinel) {
            const actionRef = anchor.getAttribute('data-action-ref');
            return actionRef
              ? options.find((candidate: SimulationOption) => candidate.edgeId === actionRef)
              : undefined;
          }
          return findOption(component.id);
        };

        return (
          <Typography
            component="div"
            variant="body2"
            align={component.align ?? 'left'}
            // A wired rich text (e.g. a sign-up link) follows its transition when
            // the embedded link is clicked, like in the real gate. Unwired
            // anchors are neutralized - the sketch must never navigate away.
            onClick={(event: MouseEvent<HTMLElement>) => {
              if (!(event.target instanceof HTMLElement) || !event.target.closest('a')) {
                return;
              }
              event.preventDefault();
              const option = resolveAnchorOption(event.currentTarget, event.target);
              if (option) {
                onChoose(option);
              }
            }}
            onMouseOver={(event: MouseEvent<HTMLElement>) =>
              onPreview(resolveAnchorOption(event.currentTarget, event.target) ?? null)
            }
            onMouseLeave={() => onPreview(null)}
            sx={{'& a': {color: 'primary.main', cursor: 'pointer'}, '& p': {m: 0}}}
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
          <Box
            component="img"
            src={resolveStaticResourcePath(component.src)}
            alt={component.alt && !containsTemplateLiteral(component.alt) ? component.alt : ''}
            sx={{maxHeight: 60, mx: 'auto'}}
          />
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
              {t('flows:core.simulation.preview.dynamicFieldsHint', 'Input fields resolved at runtime')}
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
            onClick={option ? () => onChoose(option) : undefined}
            onMouseEnter={option ? () => onPreview(option) : undefined}
            onMouseLeave={option ? () => onPreview(null) : undefined}
            onFocus={option ? () => onPreview(option) : undefined}
            onBlur={option ? () => onPreview(null) : undefined}
            startIcon={
              // Unresolved template literals (e.g. application meta) would make
              // broken image requests in sketch mode — same guard as images.
              component.image && !containsTemplateLiteral(component.image) ? (
                // Decorative — the button label carries the accessible name.
                <Box component="img" src={resolveStaticResourcePath(component.image)} alt="" sx={{height: 18}} />
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

    if (SKETCH_TEXT_INPUT_TYPES.has(component.type)) {
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

  if (components.length === 0) {
    return (
      <Stack alignItems="center" spacing={1} sx={{px: 2, py: 4, color: 'text.secondary', textAlign: 'center'}}>
        <Workflow size={24} />
        <Typography variant="body2">
          {t('flows:core.simulation.preview.noScreen', 'No screen is shown for this step')}
        </Typography>
        <Typography variant="caption">
          {t(
            'flows:core.simulation.preview.noScreenHint',
            'This step runs in the background before the flow continues',
          )}
        </Typography>
      </Stack>
    );
  }

  return (
    <Box sx={{width: GATE_CARD_WIDTH, zoom: SKETCH_ZOOM}}>
      <Box sx={{p: 2, border: '1px solid', borderColor: 'divider', borderRadius: 2, bgcolor: 'background.paper'}}>
        {renderComponents(components)}
      </Box>
    </Box>
  );
}

export default SimulationScreenSketch;
