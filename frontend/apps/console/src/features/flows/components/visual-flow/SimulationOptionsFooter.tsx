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
import {Box, Button, Stack, Typography} from '@wso2/oxygen-ui';
import {ArrowRight, CheckCircle, CircleAlert, CircleX, MousePointerClick} from '@wso2/oxygen-ui-icons-react';
import DOMPurify from 'dompurify';
import type {ReactElement} from 'react';
import {useTranslation} from 'react-i18next';
import {KIND_COLORS, KIND_LABEL_FALLBACKS, KIND_LABEL_KEYS} from '../../constants/simulationPreviewConstants';
import {SimulationOptionKinds, type SimulationOption} from '../../utils/getSimulationOptions';
import {containsTemplateLiteral} from '../resources/elements/adapters/TemplatePlaceholder';

function kindIcon(kind: SimulationOptionKinds): ReactElement {
  if (kind === SimulationOptionKinds.Success) return <CheckCircle size={15} />;
  if (kind === SimulationOptionKinds.Incomplete) return <CircleAlert size={15} />;
  if (kind === SimulationOptionKinds.Failure) return <CircleX size={15} />;
  return <MousePointerClick size={15} />;
}

/**
 * Props interface of {@link SimulationOptionsFooter}
 */
export interface SimulationOptionsFooterProps {
  /**
   * Transitions that cannot be triggered from the screen itself (e.g. executor outcomes).
   */
  options: SimulationOption[];
  /**
   * Whether the flow has completed (no outgoing transitions).
   */
  isComplete: boolean;
  /**
   * Whether some transitions are triggerable from the preview screen itself.
   */
  hasScreenOptions: boolean;
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
 * Footer of the simulation panel: lists the transitions the user can take from
 * the current step (color-coded by kind), the completion state, and a hint when
 * options are triggerable from the preview screen itself.
 *
 * @param props - Props injected to the component.
 * @returns The SimulationOptionsFooter component, or null when there is nothing to show.
 */
function SimulationOptionsFooter({
  options,
  isComplete,
  hasScreenOptions,
  onChoose,
  onPreview,
}: SimulationOptionsFooterProps): ReactElement | null {
  const {t} = useTranslation();
  const {resolveAll} = useTemplateLiteralResolver();

  if (!isComplete && options.length === 0 && !hasScreenOptions) {
    return null;
  }

  const optionLabel = (option: SimulationOption): string => {
    if (option.actionLabel) {
      // resolveAll handles mixed content; actions wired inside rich text carry
      // HTML labels, which are reduced to their plain text for the button.
      const resolved = containsTemplateLiteral(option.actionLabel)
        ? (resolveAll(option.actionLabel, {t}) ?? option.actionLabel)
        : option.actionLabel;
      return resolved.includes('<') ? DOMPurify.sanitize(resolved, {ALLOWED_TAGS: [], ALLOWED_ATTR: []}) : resolved;
    }
    return t(KIND_LABEL_KEYS[option.kind], KIND_LABEL_FALLBACKS[option.kind]);
  };

  return (
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
          <Typography variant="body2">
            {t('flows:core.simulation.complete', 'Flow complete — no outgoing transitions')}
          </Typography>
        </Stack>
      )}
      {!isComplete && options.length > 0 && (
        <>
          <Typography variant="overline" color="text.secondary" sx={{display: 'block', mb: 1, lineHeight: 1.5}}>
            {t('flows:core.simulation.chooseNext', 'Choose how the user proceeds from this step')}
          </Typography>
          <Stack direction="column" spacing={1}>
            {options.map((option: SimulationOption) => (
              <Button
                key={option.edgeId}
                fullWidth
                size="small"
                onClick={() => onChoose(option)}
                onMouseEnter={() => onPreview(option)}
                onMouseLeave={() => onPreview(null)}
                onFocus={() => onPreview(option)}
                onBlur={() => onPreview(null)}
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
          sx={{color: 'text.secondary', pt: options.length > 0 ? 1.25 : 0.5, pb: 0.5}}
        >
          <MousePointerClick size={15} />
          <Typography variant="caption">
            {options.length > 0
              ? t('flows:core.simulation.screenHintOr', 'or select an option on the preview screen')
              : t('flows:core.simulation.screenHint', 'Select an option on the preview screen to continue')}
          </Typography>
        </Stack>
      )}
    </Box>
  );
}

export default SimulationOptionsFooter;
