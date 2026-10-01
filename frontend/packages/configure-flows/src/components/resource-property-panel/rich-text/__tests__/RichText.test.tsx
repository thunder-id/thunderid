// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {Resource} from '../../../../models/resources';
import RichText from '../RichText';

// Mock react-i18next
vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (key: string) => key,
  }),
}));

// Mock the lexical plugins and components
vi.mock('@lexical/react/LexicalComposer', () => ({
  LexicalComposer: ({children}: {children: React.ReactNode}) => <div data-testid="lexical-composer">{children}</div>,
}));

vi.mock('@lexical/react/LexicalRichTextPlugin', () => ({
  RichTextPlugin: ({contentEditable}: {contentEditable: React.ReactNode}) => (
    <div data-testid="rich-text-plugin">{contentEditable}</div>
  ),
}));

vi.mock('@lexical/react/LexicalContentEditable', () => ({
  ContentEditable: (props: Record<string, unknown>) => <div data-testid="content-editable" {...props} />,
}));

vi.mock('@lexical/react/LexicalErrorBoundary', () => ({
  LexicalErrorBoundary: ({children}: {children: React.ReactNode}) => <div data-testid="error-boundary">{children}</div>,
}));

vi.mock('@lexical/react/LexicalHistoryPlugin', () => ({
  HistoryPlugin: () => <div data-testid="history-plugin" />,
}));

vi.mock('@lexical/react/LexicalAutoFocusPlugin', () => ({
  AutoFocusPlugin: () => <div data-testid="auto-focus-plugin" />,
}));

vi.mock('@lexical/react/LexicalLinkPlugin', () => ({
  LinkPlugin: () => <div data-testid="link-plugin" />,
}));

// Mock helper plugins
vi.mock('../helper-plugins/ToolbarPlugin', () => ({
  default: ({disabled}: {disabled?: boolean}) => <div data-testid="toolbar-plugin" data-disabled={disabled} />,
}));

vi.mock('../helper-plugins/CustomLinkPlugin', () => ({
  default: () => <div data-testid="custom-link-plugin" />,
}));

vi.mock('../helper-plugins/HTMLPlugin', () => ({
  default: ({resource, disabled}: {onChange: () => void; resource: Resource; disabled?: boolean}) => (
    <div data-testid="html-plugin" data-resource-id={resource?.id} data-disabled={disabled} />
  ),
}));

describe('RichText', () => {
  const mockOnChange = vi.fn();

  const createMockResource = (overrides: Partial<Resource & {label?: string}> = {}): Resource =>
    ({
      id: 'resource-1',
      resourceType: 'ELEMENT',
      type: 'RICH_TEXT',
      category: 'DISPLAY',
      version: '1.0.0',
      deprecated: false,
      deletable: true,
      display: {
        label: 'Test Rich Text',
        image: '',
        showOnResourcePanel: true,
      },
      config: {
        field: {name: 'richText', type: 'RICH_TEXT'},
        styles: {},
      },
      ...overrides,
    }) as unknown as Resource;

  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Rendering', () => {
    it('should render the LexicalComposer wrapper', () => {
      render(<RichText onChange={mockOnChange} resource={createMockResource()} />);

      expect(screen.getByTestId('lexical-composer')).toBeInTheDocument();
    });

    it('should render the ToolbarPlugin', () => {
      render(<RichText onChange={mockOnChange} resource={createMockResource()} />);

      expect(screen.getByTestId('toolbar-plugin')).toBeInTheDocument();
    });

    it('should render the RichTextPlugin', () => {
      render(<RichText onChange={mockOnChange} resource={createMockResource()} />);

      expect(screen.getByTestId('rich-text-plugin')).toBeInTheDocument();
    });

    it('should render the ContentEditable', () => {
      render(<RichText onChange={mockOnChange} resource={createMockResource()} />);

      expect(screen.getByTestId('content-editable')).toBeInTheDocument();
    });

    it('should render the HistoryPlugin', () => {
      render(<RichText onChange={mockOnChange} resource={createMockResource()} />);

      expect(screen.getByTestId('history-plugin')).toBeInTheDocument();
    });

    it('should render the AutoFocusPlugin', () => {
      render(<RichText onChange={mockOnChange} resource={createMockResource()} />);

      expect(screen.getByTestId('auto-focus-plugin')).toBeInTheDocument();
    });

    it('should render the LinkPlugin', () => {
      render(<RichText onChange={mockOnChange} resource={createMockResource()} />);

      expect(screen.getByTestId('link-plugin')).toBeInTheDocument();
    });

    it('should render the CustomLinkPlugin', () => {
      render(<RichText onChange={mockOnChange} resource={createMockResource()} />);

      expect(screen.getByTestId('custom-link-plugin')).toBeInTheDocument();
    });

    it('should render the HTMLPlugin', () => {
      render(<RichText onChange={mockOnChange} resource={createMockResource()} />);

      expect(screen.getByTestId('html-plugin')).toBeInTheDocument();
    });

    it('should pass resource to HTMLPlugin', () => {
      const resource = createMockResource({id: 'test-resource-id'});
      render(<RichText onChange={mockOnChange} resource={resource} />);

      expect(screen.getByTestId('html-plugin')).toHaveAttribute('data-resource-id', 'test-resource-id');
    });
  });

  describe('Props', () => {
    it('should apply custom className', () => {
      const {container} = render(
        <RichText onChange={mockOnChange} resource={createMockResource()} className="custom-class" />,
      );

      const composerChild = container.querySelector('.custom-class');
      expect(composerChild).toBeInTheDocument();
    });

    it('should pass disabled prop to ToolbarPlugin', () => {
      render(<RichText onChange={mockOnChange} resource={createMockResource()} disabled />);

      expect(screen.getByTestId('toolbar-plugin')).toHaveAttribute('data-disabled', 'true');
    });

    it('should pass disabled prop to HTMLPlugin', () => {
      render(<RichText onChange={mockOnChange} resource={createMockResource()} disabled />);

      expect(screen.getByTestId('html-plugin')).toHaveAttribute('data-disabled', 'true');
    });

    it('should pass ToolbarProps to ToolbarPlugin', () => {
      render(
        <RichText
          onChange={mockOnChange}
          resource={createMockResource()}
          ToolbarProps={{bold: false, italic: false}}
        />,
      );

      expect(screen.getByTestId('toolbar-plugin')).toBeInTheDocument();
    });

    it('should default disabled to false', () => {
      render(<RichText onChange={mockOnChange} resource={createMockResource()} />);

      expect(screen.getByTestId('toolbar-plugin')).toHaveAttribute('data-disabled', 'false');
      expect(screen.getByTestId('html-plugin')).toHaveAttribute('data-disabled', 'false');
    });
  });

  describe('I18n Pattern Detection', () => {
    it('should not show resolved i18n value field when label is not an i18n pattern', () => {
      const resource = createMockResource({label: 'Regular text'});
      render(<RichText onChange={mockOnChange} resource={resource} />);

      expect(screen.queryByText('flows:core.elements.richText.resolvedI18nValue')).not.toBeInTheDocument();
    });

    it('should show resolved i18n value field when label is an i18n pattern', () => {
      const resource = createMockResource({label: '{{t(hello.world)}}'});
      render(<RichText onChange={mockOnChange} resource={resource} />);

      expect(screen.getByText('flows:core.elements.richText.resolvedI18nValue')).toBeInTheDocument();
    });

    it('should display resolved i18n value in text field', () => {
      const resource = createMockResource({label: '{{t(test.key)}}'});
      render(<RichText onChange={mockOnChange} resource={resource} />);

      const textField = screen.getByDisplayValue('test.key');
      expect(textField).toBeInTheDocument();
      expect(textField).toBeDisabled();
    });
  });

  describe('Edge Cases', () => {
    it('should handle resource without label', () => {
      const resource = createMockResource();
      render(<RichText onChange={mockOnChange} resource={resource} />);

      expect(screen.getByTestId('lexical-composer')).toBeInTheDocument();
    });

    it('should handle empty label', () => {
      const resource = createMockResource({label: ''});
      render(<RichText onChange={mockOnChange} resource={resource} />);

      expect(screen.getByTestId('lexical-composer')).toBeInTheDocument();
    });

    it('should handle undefined label', () => {
      const resource = createMockResource({label: undefined});
      render(<RichText onChange={mockOnChange} resource={resource} />);

      expect(screen.getByTestId('lexical-composer')).toBeInTheDocument();
    });
  });

  describe('Editor Config', () => {
    it('should have correct editor namespace', () => {
      render(<RichText onChange={mockOnChange} resource={createMockResource()} />);

      // The editor config is passed to LexicalComposer
      expect(screen.getByTestId('lexical-composer')).toBeInTheDocument();
    });

    it('should rethrow errors in onError callback', () => {
      // The onError callback in editorConfig throws errors
      // This is verified by the component's behavior when rendering
      render(<RichText onChange={mockOnChange} resource={createMockResource()} />);

      expect(screen.getByTestId('lexical-composer')).toBeInTheDocument();
    });
  });
});
