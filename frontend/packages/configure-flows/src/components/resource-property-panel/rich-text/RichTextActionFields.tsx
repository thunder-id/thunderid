// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, FormControl, FormControlLabel, FormLabel, Stack, Switch, TextField, Typography} from '@wso2/oxygen-ui';
import {useEdges, useReactFlow} from '@xyflow/react';
import {useEffect, useMemo, useState, type ReactElement} from 'react';
import {useTranslation} from 'react-i18next';
import VisualFlowConstants from '../../../constants/VisualFlowConstants';
import type {Resource} from '../../../models/resources';

/**
 * Props interface for the RichTextActionFields component.
 */
export interface RichTextActionFieldsPropsInterface {
  resource: Resource;
  onChange: (propertyKey: string, newValue: unknown, resource: Resource, debounce?: boolean) => void;
}

/**
 * RichTextActionInterface defines the shape of the `action` property on a rich-text resource. It is used to
 * determine whether the rich text should behave as an interactive link and, if so, which step it
 * should trigger when clicked.
 */
export interface RichTextActionInterface {
  ref?: string;
}

/**
 * Matches a `data-action-ref` attribute and captures its value. The leading whitespace is an
 * attribute boundary, matching `syncLabelActionRef` and the adapter's own matcher, so an
 * attribute merely ending in `data-action-ref` cannot be mistaken for the sentinel.
 */
const ACTION_REF_SENTINEL = /\sdata-action-ref\s*=\s*(?:"([^"]*)"|'([^']*)')/gi;

/**
 * Reads the `data-action-ref` sentinels out of a rich text's label HTML, in document order.
 *
 * @param label - The rich text's label, which may be HTML.
 * @returns The sentinel values, empty when the label carries none.
 */
function getAnchorActionRefs(label: string | undefined): string[] {
  if (!label) {
    return [];
  }

  return Array.from(label.matchAll(ACTION_REF_SENTINEL))
    .map((match: RegExpMatchArray) => match[1] ?? match[2])
    .filter((ref: string) => ref !== '');
}

/**
 * RichTextActionFields is a React component that renders the action configuration fields for a rich-text resource.
 *
 * @param param0 - The props for the component, including the resource and onChange handler.
 * @returns A React element representing the action configuration fields.
 */
function RichTextActionFields({resource, onChange}: RichTextActionFieldsPropsInterface): ReactElement {
  const {t} = useTranslation();
  const {getEdges, setEdges} = useReactFlow();
  const edges = useEdges();

  const action: RichTextActionInterface | undefined = useMemo<RichTextActionInterface | undefined>(() => {
    const r = resource as Resource & {action?: RichTextActionInterface};
    return r.action;
  }, [resource]);

  const handleId = `${resource.id}${VisualFlowConstants.FLOW_BUILDER_NEXT_HANDLE_SUFFIX}`;
  // Show the connected target reactively — the "Connected step" field mirrors whatever
  // edge is currently sourced from this rich-text handle. No user typing; edges drive it.
  const connectedTarget: string = useMemo<string>(
    () => edges.find((edge) => edge.sourceHandle === handleId)?.target ?? '',
    [edges, handleId],
  );

  // ResourceProperties filters `propertyKey === 'action'` from updating the currently
  // selected resource view, so the parent-driven `action` reference never rebroadcasts
  // to this component. Track enabled state locally and seed it from the resource on mount.
  const [isEnabled, setIsEnabled] = useState<boolean>(Boolean(action));

  // Re-seed the local enabled state whenever a different resource is selected. `action`
  // itself isn't re-broadcast (see comment on isEnabled), so key off resource id.
  useEffect(() => {
    setIsEnabled(Boolean(action));
    // eslint-disable-next-line react-hooks/exhaustive-deps -- intentional: action is stale for the same resource
  }, [resource.id]);

  const handleToggle = (enabled: boolean): void => {
    setIsEnabled(enabled);
    if (enabled) {
      // The ref must equal the anchor's `data-action-ref` in the label HTML, otherwise the
      // rendered link dispatches nothing. Keep the existing ref only when an anchor still
      // carries it — `action` writes never rebroadcast (see isEnabled), so after a toggle
      // off and on again it can hold a value the label has since moved away from. Otherwise
      // take the label's first sentinel, and fall back to the component id when there is
      // none (a label with no sentinel dispatches on any anchor click).
      const labelRefs = getAnchorActionRefs((resource as Resource & {label?: string}).label);
      const existingRef = action?.ref === '' ? undefined : action?.ref;
      const derivedRef =
        (existingRef !== undefined && labelRefs.includes(existingRef) ? existingRef : undefined) ??
        labelRefs[0] ??
        existingRef ??
        resource.id;

      onChange('action', {ref: derivedRef}, resource);
    } else {
      // Drop any edges the author drew from this rich-text's source handle — the
      // handle is about to disappear, so orphaned edges would dangle otherwise.
      setEdges(getEdges().filter((edge) => edge.sourceHandle !== handleId));
      // Use `null` as an explicit "disabled" sentinel: lodash `set` treats it distinctly
      // from `undefined` (which some downstream passes may strip), and the adapter reads
      // it as falsy so the source Handle actually unmounts.
      onChange('action', null, resource);
    }
  };

  return (
    <Stack gap={2} data-testid="rich-text-action-fields">
      <Typography variant="body2" color="text.secondary">
        {t(
          'flows:core.elements.richText.action.description',
          'Turn this rich text into an interactive link. When on, the link inside triggers the connected step instead of navigating.',
        )}
      </Typography>
      <Box>
        <FormControlLabel
          sx={{ml: 0}}
          control={
            <Switch
              data-testid="rich-text-action-enabled"
              checked={isEnabled}
              onChange={(e) => handleToggle(e.target.checked)}
              sx={{mr: 1}}
            />
          }
          label={t('flows:core.elements.richText.action.enabled.label', 'Use as an interactive link')}
        />
      </Box>
      {isEnabled && (
        <FormControl fullWidth size="small">
          <FormLabel htmlFor="rich-text-action-ref">
            {t('flows:core.elements.richText.action.ref.label', 'Connected step')}
          </FormLabel>
          <TextField
            id="rich-text-action-ref"
            data-testid="rich-text-action-ref"
            value={connectedTarget}
            placeholder={t(
              'flows:core.elements.richText.action.ref.placeholder',
              'Draw an edge from the link to a step',
            )}
            size="small"
            fullWidth
            slotProps={{input: {readOnly: true}}}
          />
        </FormControl>
      )}
    </Stack>
  );
}

export default RichTextActionFields;
