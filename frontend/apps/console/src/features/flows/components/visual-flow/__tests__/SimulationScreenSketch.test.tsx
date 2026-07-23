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

import {render, screen, fireEvent} from '@testing-library/react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {SimulationOption} from '../../../utils/getSimulationOptions';
import {SimulationOptionKinds} from '../../../utils/getSimulationOptions';
import SimulationScreenSketch from '../SimulationScreenSketch';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (_key: string, fallback?: string) => fallback ?? _key,
  }),
}));

vi.mock('@thunderid/hooks', () => ({
  useTemplateLiteralResolver: () => ({
    resolve: (value: string) => value,
    resolveAll: (value: string) => value,
  }),
}));

const linkOption: SimulationOption = {
  edgeId: 'e-reset',
  targetNodeId: 'recovery-1',
  kind: SimulationOptionKinds.Action,
  actionLabel: 'Forgot password? Reset',
  sourceComponentId: 'rich_001',
};

const richTextComponent = {
  id: 'rich_001',
  type: 'RICH_TEXT',
  label: '<p>Forgot password? <a href="#">Reset</a></p>',
};

describe('SimulationScreenSketch', () => {
  const onChoose = vi.fn();
  const onPreview = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('should follow the wired transition when a rich-text link is clicked', () => {
    render(
      <SimulationScreenSketch
        components={[richTextComponent] as never}
        options={[linkOption]}
        onChoose={onChoose}
        onPreview={onPreview}
      />,
    );

    fireEvent.click(screen.getByText('Reset'));

    expect(onChoose).toHaveBeenCalledWith(linkOption);
  });

  it('should not follow the transition when the rich text is clicked outside the link', () => {
    render(
      <SimulationScreenSketch
        components={[richTextComponent] as never}
        options={[linkOption]}
        onChoose={onChoose}
        onPreview={onPreview}
      />,
    );

    fireEvent.click(screen.getByText(/Forgot password\?/));

    expect(onChoose).not.toHaveBeenCalled();
  });

  it('should preview the wired transition while hovering the link itself', () => {
    render(
      <SimulationScreenSketch
        components={[richTextComponent] as never}
        options={[linkOption]}
        onChoose={onChoose}
        onPreview={onPreview}
      />,
    );

    fireEvent.mouseOver(screen.getByText('Reset'));
    expect(onPreview).toHaveBeenCalledWith(linkOption);

    fireEvent.mouseLeave(screen.getByText('Reset'));
    expect(onPreview).toHaveBeenCalledWith(null);
  });

  it('should not preview anything when hovering the rich text outside the link', () => {
    render(
      <SimulationScreenSketch
        components={[richTextComponent] as never}
        options={[linkOption]}
        onChoose={onChoose}
        onPreview={onPreview}
      />,
    );

    fireEvent.mouseOver(screen.getByText(/Forgot password\?/));

    expect(onPreview).toHaveBeenCalledWith(null);
  });

  it('should fire the correct option when a rich text has two anchors that each carry an action ref', () => {
    const multiLinkComponent = {
      id: 'rich_002',
      type: 'RICH_TEXT',
      label:
        '<p>By continuing you agree to the <a href="#" data-action-ref="action_terms">Terms</a>. ' +
        '<a href="#" data-action-ref="action_reset">Reset</a></p>',
    };
    const termsOption: SimulationOption = {
      edgeId: 'action_terms',
      targetNodeId: 'terms-1',
      kind: SimulationOptionKinds.Action,
      actionLabel: 'Terms',
      sourceComponentId: 'rich_002',
    };
    const resetOption: SimulationOption = {
      edgeId: 'action_reset',
      targetNodeId: 'recovery-1',
      kind: SimulationOptionKinds.Action,
      actionLabel: 'Reset',
      sourceComponentId: 'rich_002',
    };
    render(
      <SimulationScreenSketch
        components={[multiLinkComponent] as never}
        options={[termsOption, resetOption]}
        onChoose={onChoose}
        onPreview={onPreview}
      />,
    );

    fireEvent.click(screen.getByText('Terms'));
    expect(onChoose).toHaveBeenCalledWith(termsOption);

    onChoose.mockClear();
    fireEvent.click(screen.getByText('Reset'));
    expect(onChoose).toHaveBeenCalledWith(resetOption);
  });

  it('should not fire any option for a plain anchor sharing a rich text with a wired link', () => {
    // The exact regression this guards against: a rich text mixing an unwired
    // link (no data-action-ref, e.g. an external Terms link) with a wired one.
    // Before the fix, clicking the unwired anchor incorrectly fired the wired
    // link's option because both were matched by the whole component's id.
    const mixedComponent = {
      id: 'rich_003',
      type: 'RICH_TEXT',
      label:
        '<p>By continuing you agree to the <a href="https://example.com/terms">Terms</a>. ' +
        '<a href="#" data-action-ref="action_reset">Reset</a></p>',
    };
    const resetOption: SimulationOption = {
      edgeId: 'action_reset',
      targetNodeId: 'recovery-1',
      kind: SimulationOptionKinds.Action,
      actionLabel: 'Reset',
      sourceComponentId: 'rich_003',
    };
    render(
      <SimulationScreenSketch
        components={[mixedComponent] as never}
        options={[resetOption]}
        onChoose={onChoose}
        onPreview={onPreview}
      />,
    );

    fireEvent.click(screen.getByText('Terms'));
    expect(onChoose).not.toHaveBeenCalled();

    fireEvent.click(screen.getByText('Reset'));
    expect(onChoose).toHaveBeenCalledWith(resetOption);
  });

  it('should render unwired rich text without click behavior', () => {
    render(
      <SimulationScreenSketch
        components={[richTextComponent] as never}
        options={[]}
        onChoose={onChoose}
        onPreview={onPreview}
      />,
    );

    fireEvent.click(screen.getByText('Reset'));

    expect(onChoose).not.toHaveBeenCalled();
  });
});
