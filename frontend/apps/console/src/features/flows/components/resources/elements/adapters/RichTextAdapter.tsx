/**
 * Copyright (c) 2025-2026, WSO2 LLC. (https://www.wso2.com).
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
import {Position, useNodeId, useUpdateNodeInternals} from '@xyflow/react';
import DOMPurify from 'dompurify';
import parse from 'html-react-parser';
import {useEffect, useMemo, useRef, type ReactElement} from 'react';
import {useTranslation} from 'react-i18next';
import NodeHandle from './NodeHandle';
import VisualFlowConstants from '@/features/flows/constants/VisualFlowConstants';
import type {Element as FlowElement} from '@/features/flows/models/elements';
import './RichTextAdapter.scss';

// Register DOMPurify hook once at module level to handle anchor tags.
(DOMPurify as unknown as {addHook: (name: string, fn: (node: globalThis.Element) => void) => void}).addHook(
  'afterSanitizeAttributes',
  (node: globalThis.Element) => {
    if (node.hasAttribute('target')) {
      const target: string | null = node.getAttribute('target');

      if (target === '_blank') {
        node.setAttribute('rel', 'noopener noreferrer');
      }
    }
  },
);

/**
 * RichText element type.
 */
export type RichTextElement = FlowElement & {
  label?: string;
  action?: {ref?: string; eventType?: string};
};

/**
 * Props interface of {@link RichTextAdapter}
 */
export interface RichTextAdapterPropsInterface {
  /**
   * The rich text element properties.
   */
  resource: FlowElement;
  /**
   * Index of this element within its parent container, used as a positionKey so React Flow
   * re-measures the source handle when the rich text is reordered.
   */
  elementIndex?: number;
}

/**
 * Adapter for the Rich Text component. When the component carries an `action.ref` (i.e.
 * a sentinel-marked anchor inside the HTML should dispatch a flow action), a source
 * NodeHandle is rendered so the author can draw an edge from the rich text to a target
 * step — matching the button-action wiring UX.
 */
function RichTextAdapter({resource, elementIndex = undefined}: RichTextAdapterPropsInterface): ReactElement {
  const {t} = useTranslation();

  const {resolveAll} = useTemplateLiteralResolver();

  const richTextElement = resource as RichTextElement;
  const textContent = richTextElement?.label ?? '';
  const isActionEnabled = Boolean(richTextElement.action);
  const parentNodeId = useNodeId();
  const updateNodeInternals = useUpdateNodeInternals();

  // React Flow does not observe subtree mutations, so it needs to be told to re-measure
  // when the source Handle is added or removed by the action-enabled toggle. Only fire
  // when the enabled value actually transitions — measuring on mount (or on any parent
  // re-render where isActionEnabled is the same) forces React Flow to re-run its initial
  // fitView, which locks the canvas into a zoomed-in state on flow load.
  const prevIsActionEnabledRef = useRef<boolean>(isActionEnabled);
  useEffect(() => {
    if (prevIsActionEnabledRef.current === isActionEnabled) {
      return;
    }
    prevIsActionEnabledRef.current = isActionEnabled;
    if (parentNodeId) {
      updateNodeInternals(parentNodeId);
    }
  }, [parentNodeId, updateNodeInternals, isActionEnabled]);

  const sanitizedHtml: string = useMemo(
    () =>
      DOMPurify.sanitize(resolveAll(textContent, {t}) ?? textContent, {
        ADD_ATTR: ['target', 'data-action-ref'],
        RETURN_TRUSTED_TYPE: false,
      }),
    // eslint-disable-next-line react-hooks/exhaustive-deps -- resolveAll is stable
    [textContent, t],
  );

  return (
    <div className="rich-text-content rich-text-adapter">
      {parse(sanitizedHtml)}
      {isActionEnabled && (
        <NodeHandle
          id={`${richTextElement.id}${VisualFlowConstants.FLOW_BUILDER_NEXT_HANDLE_SUFFIX}`}
          type="source"
          position={Position.Right}
          positionKey={elementIndex}
        />
      )}
    </div>
  );
}

export default RichTextAdapter;
