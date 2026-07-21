/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

import {render} from '@testing-library/react';
import type {ReactNode} from 'react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import RichTextAdapter from '../RichTextAdapter';
import type {Element as FlowElement} from '@/features/flows/models/elements';

// Mock dependencies
vi.mock('../RichTextAdapter.scss', () => ({}));

const mockUpdateNodeInternals = vi.fn();
const mockUseNodeId = vi.fn<() => string | null>(() => 'node-1');

vi.mock('@xyflow/react', () => ({
  Handle: () => null,
  Position: {Left: 'left', Right: 'right', Top: 'top', Bottom: 'bottom'},
  useNodeId: () => mockUseNodeId(),
  useUpdateNodeInternals: () => mockUpdateNodeInternals,
}));

vi.mock('../NodeHandle', () => ({
  default: ({id, type, position}: {id: string; type: string; position: string}) => (
    <div data-testid="node-handle" data-handle-id={id} data-handle-type={type} data-position={position} />
  ),
}));

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
  }),
  Trans: ({children}: {children: ReactNode}) => children,
}));

describe('RichTextAdapter', () => {
  const createMockElement = (overrides: Partial<FlowElement> & Record<string, unknown> = {}): FlowElement =>
    ({
      id: 'richtext-1',
      type: 'RICH_TEXT',
      category: 'DISPLAY',
      config: {},
      label: 'Hello <b>World</b>',
      ...overrides,
    }) as FlowElement;

  beforeEach(() => {
    vi.clearAllMocks();
    mockUseNodeId.mockReturnValue('node-1');
  });

  describe('Rendering', () => {
    it('should render with rich-text-content class', () => {
      const resource = createMockElement();

      const {container} = render(<RichTextAdapter resource={resource} />);

      expect(container.querySelector('.rich-text-content')).toBeInTheDocument();
    });

    it('should render label content', () => {
      const resource = createMockElement();

      const {container} = render(<RichTextAdapter resource={resource} />);

      expect(container.querySelector('.rich-text-content')).toHaveTextContent('World');
    });

    it('should render label text', () => {
      const resource = createMockElement({label: 'Test Label'});

      const {container} = render(<RichTextAdapter resource={resource} />);

      expect(container.querySelector('.rich-text-content')).toHaveTextContent('Test Label');
    });
  });

  describe('HTML Sanitization', () => {
    it('should render sanitized HTML content', () => {
      const resource = createMockElement({label: '<p>Paragraph content</p>'});

      const {container} = render(<RichTextAdapter resource={resource} />);

      expect(container.querySelector('.rich-text-content')).toHaveTextContent('Paragraph content');
    });

    it('should handle plain text content', () => {
      const resource = createMockElement({label: 'Plain text without HTML'});

      const {container} = render(<RichTextAdapter resource={resource} />);

      expect(container.querySelector('.rich-text-content')).toHaveTextContent('Plain text without HTML');
    });
  });

  describe('Empty Content', () => {
    it('should handle empty label', () => {
      const resource = createMockElement({label: ''});

      const {container} = render(<RichTextAdapter resource={resource} />);

      expect(container.querySelector('.rich-text-content')).toHaveTextContent('');
    });

    it('should handle undefined label', () => {
      const resource = createMockElement({label: undefined});

      const {container} = render(<RichTextAdapter resource={resource} />);

      expect(container.querySelector('.rich-text-content')).toHaveTextContent('');
    });
  });

  describe('Different Resource IDs', () => {
    it('should render with different resource IDs', () => {
      const resource1 = createMockElement({id: 'richtext-1', label: 'First'});
      const resource2 = createMockElement({id: 'richtext-2', label: 'Second'});

      const {container: container1} = render(<RichTextAdapter resource={resource1} />);
      const {container: container2} = render(<RichTextAdapter resource={resource2} />);

      expect(container1.querySelector('.rich-text-content')).toBeInTheDocument();
      expect(container2.querySelector('.rich-text-content')).toBeInTheDocument();
    });
  });

  describe('Action-enabled source handle', () => {
    it('should render a source NodeHandle when the element has an action defined', () => {
      const resource = createMockElement({action: {ref: 'target-1'}} as Record<string, unknown>);

      const {getByTestId} = render(<RichTextAdapter resource={resource} />);

      const handle = getByTestId('node-handle');
      expect(handle).toHaveAttribute('data-handle-id', 'richtext-1_NEXT');
      expect(handle).toHaveAttribute('data-handle-type', 'source');
      expect(handle).toHaveAttribute('data-position', 'right');
    });

    it('should not render the source NodeHandle when the element has no action', () => {
      const resource = createMockElement();

      const {queryByTestId} = render(<RichTextAdapter resource={resource} />);

      expect(queryByTestId('node-handle')).toBeNull();
    });

    it('should call updateNodeInternals when isActionEnabled transitions from false to true', () => {
      const {rerender} = render(<RichTextAdapter resource={createMockElement()} />);
      // Initial render: prev=current, effect runs but returns early
      expect(mockUpdateNodeInternals).not.toHaveBeenCalled();

      rerender(<RichTextAdapter resource={createMockElement({action: {ref: 'x'}} as Record<string, unknown>)} />);

      expect(mockUpdateNodeInternals).toHaveBeenCalledWith('node-1');
    });

    it('should call updateNodeInternals when isActionEnabled transitions from true to false', () => {
      const {rerender} = render(
        <RichTextAdapter resource={createMockElement({action: {ref: 'x'}} as Record<string, unknown>)} />,
      );
      expect(mockUpdateNodeInternals).not.toHaveBeenCalled();

      rerender(<RichTextAdapter resource={createMockElement()} />);

      expect(mockUpdateNodeInternals).toHaveBeenCalledWith('node-1');
    });

    it('should skip updateNodeInternals when parentNodeId is null even after a transition', () => {
      mockUseNodeId.mockReturnValue(null);
      const {rerender} = render(<RichTextAdapter resource={createMockElement()} />);

      rerender(<RichTextAdapter resource={createMockElement({action: {ref: 'x'}} as Record<string, unknown>)} />);

      expect(mockUpdateNodeInternals).not.toHaveBeenCalled();
    });
  });

  describe('Anchor Tag Security', () => {
    it('should handle anchor tags with target="_blank"', () => {
      const resource = createMockElement({
        label: '<a href="https://example.com" target="_blank">External Link</a>',
      });

      const {container} = render(<RichTextAdapter resource={resource} />);

      expect(container.querySelector('.rich-text-content')).toHaveTextContent('External Link');
    });

    it('should handle anchor tags without target attribute', () => {
      const resource = createMockElement({
        label: '<a href="https://example.com">Regular Link</a>',
      });

      const {container} = render(<RichTextAdapter resource={resource} />);

      expect(container.querySelector('.rich-text-content')).toHaveTextContent('Regular Link');
    });

    it('should handle anchor tags with target="_self"', () => {
      const resource = createMockElement({
        label: '<a href="https://example.com" target="_self">Same Window Link</a>',
      });

      const {container} = render(<RichTextAdapter resource={resource} />);

      expect(container.querySelector('.rich-text-content')).toHaveTextContent('Same Window Link');
    });

    it('should handle multiple anchor tags', () => {
      const resource = createMockElement({
        label: '<a href="https://link1.com" target="_blank">Link 1</a> and <a href="https://link2.com">Link 2</a>',
      });

      const {container} = render(<RichTextAdapter resource={resource} />);

      expect(container.querySelector('.rich-text-content')).toHaveTextContent('Link 1 and Link 2');
    });
  });
});
