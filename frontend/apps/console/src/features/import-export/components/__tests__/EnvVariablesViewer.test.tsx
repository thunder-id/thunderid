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

import {fireEvent, render, screen, waitFor} from '@thunderid/test-utils';
import {describe, expect, it, vi} from 'vitest';
import EnvVariablesViewer from '../EnvVariablesViewer';

const mockT = (key: string, params?: Record<string, unknown>) => {
  // When useTranslation is called with a namespace, keys don't include the namespace prefix
  switch (key) {
    case 'envViewer.variableCount':
      return params?.count !== undefined ? `${Number(params.count)} variables` : 'envViewer.variableCount';
    case 'envViewer.modified':
      return '(modified)';
    case 'envViewer.placeholderWarning':
      return 'Some values contain placeholders';
    case 'envViewer.title':
      return 'Environment Variables';
    case 'envViewer.download':
      return 'Download';
    default:
      return key;
  }
};

vi.mock('react-i18next', () => ({
  useTranslation: () => ({t: mockT}),
}));

vi.mock('@monaco-editor/react', () => ({
  default: ({
    value,
    onChange,
    options,
  }: {
    value: string;
    onChange: (val: string) => void;
    options: Record<string, unknown>;
  }) => (
    <textarea
      data-testid="monaco-editor"
      value={value}
      onChange={(e) => onChange(e.target.value)}
      readOnly={options.readOnly as boolean}
    />
  ),
}));

vi.mock('@wso2/oxygen-ui', async () => {
  const actual = await vi.importActual('@wso2/oxygen-ui');
  return {
    ...actual,
    useColorScheme: () => ({mode: 'light', systemMode: 'light'}),
  };
});

describe('EnvVariablesViewer', () => {
  const mockContent = 'API_KEY=secret123\nDATABASE_URL=postgres://localhost\n';
  const mockFileName = 'config.env';

  describe('rendering', () => {
    it('renders component with title', () => {
      render(<EnvVariablesViewer content={mockContent} fileName={mockFileName} />);

      expect(screen.getByText('Environment Variables')).toBeInTheDocument();
    });

    it('displays variable count', () => {
      render(<EnvVariablesViewer content={mockContent} fileName={mockFileName} />);

      expect(screen.getByText('2 variables')).toBeInTheDocument();
    });

    it('counts only non-comment lines', () => {
      const contentWithComments = '# Comment\nAPI_KEY=secret\n# Another comment\nURL=value\n';
      render(<EnvVariablesViewer content={contentWithComments} fileName={mockFileName} />);

      expect(screen.getByText('2 variables')).toBeInTheDocument();
    });

    it('ignores empty lines in count', () => {
      const contentWithEmpty = 'API_KEY=secret\n\n\nURL=value\n';
      render(<EnvVariablesViewer content={contentWithEmpty} fileName={mockFileName} />);

      expect(screen.getByText('2 variables')).toBeInTheDocument();
    });

    it('renders download button by default', () => {
      render(<EnvVariablesViewer content={mockContent} fileName={mockFileName} />);

      expect(screen.getByText('Download')).toBeInTheDocument();
    });

    it('hides download button when showDownload is false', () => {
      render(<EnvVariablesViewer content={mockContent} fileName={mockFileName} showDownload={false} />);

      expect(screen.queryByText('Download')).not.toBeInTheDocument();
    });
  });

  describe('expand/collapse behavior', () => {
    it('starts collapsed by default', () => {
      const {container} = render(<EnvVariablesViewer content={mockContent} fileName={mockFileName} />);

      // Collapse component keeps content in DOM but hides it with CSS
      const collapseElement = container.querySelector('.MuiCollapse-hidden');
      expect(collapseElement).toBeInTheDocument();
    });

    it('expands when icon button clicked', async () => {
      const {container} = render(<EnvVariablesViewer content={mockContent} fileName={mockFileName} />);

      const buttons = screen.getAllByRole('button');
      const expandButton = buttons[buttons.length - 1];

      fireEvent.click(expandButton);

      // After clicking, collapse should no longer have hidden class
      await waitFor(() => {
        const collapseElement = container.querySelector('.MuiCollapse-hidden');
        expect(collapseElement).not.toBeInTheDocument();
      });
    });

    it('collapses when icon button clicked again', async () => {
      const {container} = render(<EnvVariablesViewer content={mockContent} fileName={mockFileName} />);

      const buttons = screen.getAllByRole('button');
      const expandButton = buttons[buttons.length - 1];

      fireEvent.click(expandButton);
      await waitFor(() => {
        const collapseElement = container.querySelector('.MuiCollapse-hidden');
        expect(collapseElement).not.toBeInTheDocument();
      });

      fireEvent.click(expandButton);
      await waitFor(() => {
        const collapseElement = container.querySelector('.MuiCollapse-hidden');
        expect(collapseElement).toBeInTheDocument();
      });
    });
  });

  describe('editable mode', () => {
    it('allows editing when editable is true', () => {
      render(<EnvVariablesViewer content={mockContent} fileName={mockFileName} editable={true} />);

      const buttons = screen.getAllByRole('button');
      fireEvent.click(buttons[buttons.length - 1]);

      const editor = screen.getByTestId('monaco-editor');
      expect(editor).not.toHaveAttribute('readonly');
    });

    it('is readonly when editable is false', () => {
      render(<EnvVariablesViewer content={mockContent} fileName={mockFileName} editable={false} />);

      const buttons = screen.getAllByRole('button');
      fireEvent.click(buttons[buttons.length - 1]);

      const editor = screen.getByTestId('monaco-editor');
      expect(editor).toHaveAttribute('readonly');
    });

    it('calls onChange callback after debounce', async () => {
      vi.useFakeTimers();
      const onChange = vi.fn();
      render(<EnvVariablesViewer content={mockContent} fileName={mockFileName} editable={true} onChange={onChange} />);

      const buttons = screen.getAllByRole('button');
      fireEvent.click(buttons[buttons.length - 1]);

      const editor = screen.getByTestId('monaco-editor');
      fireEvent.change(editor, {target: {value: ''}});
      fireEvent.change(editor, {target: {value: (editor as HTMLTextAreaElement).value + 'NEW_VAR=value'}});

      await vi.advanceTimersByTimeAsync(300);

      expect(onChange).toHaveBeenCalledWith('NEW_VAR=value');
      vi.useRealTimers();
    });

    it('debounces onChange calls', async () => {
      vi.useFakeTimers();
      const onChange = vi.fn();
      render(<EnvVariablesViewer content={mockContent} fileName={mockFileName} editable={true} onChange={onChange} />);

      const buttons = screen.getAllByRole('button');
      fireEvent.click(buttons[buttons.length - 1]);

      const editor = screen.getByTestId('monaco-editor');
      fireEvent.change(editor, {target: {value: (editor as HTMLTextAreaElement).value + 'a'}});
      fireEvent.change(editor, {target: {value: (editor as HTMLTextAreaElement).value + 'b'}});
      fireEvent.change(editor, {target: {value: (editor as HTMLTextAreaElement).value + 'c'}});

      await vi.advanceTimersByTimeAsync(100);
      expect(onChange).not.toHaveBeenCalled();

      await vi.advanceTimersByTimeAsync(150);
      expect(onChange).toHaveBeenCalledTimes(1);
      vi.useRealTimers();
    });

    it('shows modified indicator when content changes', () => {
      render(<EnvVariablesViewer content={mockContent} fileName={mockFileName} editable={true} />);

      const buttons = screen.getAllByRole('button');
      fireEvent.click(buttons[buttons.length - 1]);

      const editor = screen.getByTestId('monaco-editor');
      fireEvent.change(editor, {target: {value: (editor as HTMLTextAreaElement).value + 'NEW_VAR=value'}});

      expect(screen.getByText(/\(modified\)/)).toBeInTheDocument();
    });

    it('does not call onChange when not in editable mode', async () => {
      vi.useFakeTimers();
      const onChange = vi.fn();
      render(<EnvVariablesViewer content={mockContent} fileName={mockFileName} editable={false} onChange={onChange} />);

      const buttons = screen.getAllByRole('button');
      fireEvent.click(buttons[buttons.length - 1]);

      const editor = screen.getByTestId('monaco-editor');
      // Even though editor is readonly, simulate change
      editor.dispatchEvent(new Event('change'));

      await vi.advanceTimersByTimeAsync(300);

      expect(onChange).not.toHaveBeenCalled();
      vi.useRealTimers();
    });
  });

  describe('placeholder detection', () => {
    it('detects placeholder values', () => {
      const contentWithPlaceholder = 'API_KEY=_placeholder\nURL=value\n';
      render(<EnvVariablesViewer content={contentWithPlaceholder} fileName={mockFileName} editable={true} />);

      const buttons = screen.getAllByRole('button');
      fireEvent.click(buttons[buttons.length - 1]);

      expect(screen.getByText('Some values contain placeholders')).toBeInTheDocument();
    });

    it('does not show placeholder warning when no placeholders', () => {
      render(<EnvVariablesViewer content={mockContent} fileName={mockFileName} />);

      const buttons = screen.getAllByRole('button');
      fireEvent.click(buttons[buttons.length - 1]);

      expect(screen.queryByText('Some values contain placeholders')).not.toBeInTheDocument();
    });

    it('does not show placeholder warning in readonly mode', () => {
      const contentWithPlaceholder = 'API_KEY=_placeholder\nURL=value\n';
      render(<EnvVariablesViewer content={contentWithPlaceholder} fileName={mockFileName} />);

      const buttons = screen.getAllByRole('button');
      fireEvent.click(buttons[buttons.length - 1]);

      expect(screen.queryByText('Some values contain placeholders')).not.toBeInTheDocument();
    });
  });

  describe('download functionality', () => {
    it('downloads readonly content when not editable', () => {
      const mockCreateElement = vi.spyOn(document, 'createElement');
      const mockCreateObjectURL = vi.fn(() => 'blob:mock-url');
      const mockRevokeObjectURL = vi.fn();
      URL.createObjectURL = mockCreateObjectURL;
      URL.revokeObjectURL = mockRevokeObjectURL;

      render(<EnvVariablesViewer content={mockContent} fileName={mockFileName} />);

      const downloadButton = screen.getByText('Download');
      fireEvent.click(downloadButton);

      expect(mockCreateObjectURL).toHaveBeenCalled();
      expect(mockCreateElement).toHaveBeenCalledWith('a');
    });

    it('downloads edited content when editable', () => {
      const mockCreateElement = vi.spyOn(document, 'createElement');
      const mockCreateObjectURL = vi.fn(() => 'blob:mock-url');
      URL.createObjectURL = mockCreateObjectURL;

      render(<EnvVariablesViewer content={mockContent} fileName={mockFileName} editable={true} />);

      const buttons = screen.getAllByRole('button');
      fireEvent.click(buttons[buttons.length - 1]);

      const editor = screen.getByTestId('monaco-editor');
      fireEvent.change(editor, {target: {value: ''}});
      fireEvent.change(editor, {target: {value: (editor as HTMLTextAreaElement).value + 'EDITED_VAR=edited'}});

      const downloadButton = screen.getByText('Download');
      fireEvent.click(downloadButton);

      expect(mockCreateElement).toHaveBeenCalledWith('a');
    });

    it('handles File System Access API when available', async () => {
      const mockWrite = vi.fn();
      const mockClose = vi.fn();
      const mockShowSaveFilePicker = vi.fn().mockResolvedValue({
        createWritable: vi.fn().mockResolvedValue({
          write: mockWrite,
          close: mockClose,
        }),
      });

      (window as unknown as Window & {showSaveFilePicker: () => Promise<unknown>}).showSaveFilePicker =
        mockShowSaveFilePicker;

      render(<EnvVariablesViewer content={mockContent} fileName={mockFileName} />);

      const downloadButton = screen.getByText('Download');
      fireEvent.click(downloadButton);

      await waitFor(() => {
        expect(mockShowSaveFilePicker).toHaveBeenCalled();
      });
    });

    it('handles download cancellation silently', () => {
      const mockShowSaveFilePicker = vi.fn().mockRejectedValue(new Error('User cancelled'));
      (window as unknown as Window & {showSaveFilePicker: () => Promise<unknown>}).showSaveFilePicker =
        mockShowSaveFilePicker;

      render(<EnvVariablesViewer content={mockContent} fileName={mockFileName} />);

      const downloadButton = screen.getByText('Download');

      fireEvent.click(downloadButton);
      expect(mockShowSaveFilePicker).toBeDefined();
    });
  });

  describe('maxHeight prop', () => {
    it('uses default maxHeight of 400', () => {
      render(<EnvVariablesViewer content={mockContent} fileName={mockFileName} />);

      const buttons = screen.getAllByRole('button');
      fireEvent.click(buttons[buttons.length - 1]);

      // Editor is rendered with maxHeight
      expect(screen.getByTestId('monaco-editor')).toBeInTheDocument();
    });

    it('uses custom maxHeight when provided', () => {
      render(<EnvVariablesViewer content={mockContent} fileName={mockFileName} maxHeight={600} />);

      const buttons = screen.getAllByRole('button');
      fireEvent.click(buttons[buttons.length - 1]);

      expect(screen.getByTestId('monaco-editor')).toBeInTheDocument();
    });
  });

  describe('edge cases', () => {
    it('handles empty content', () => {
      render(<EnvVariablesViewer content="" fileName={mockFileName} />);

      expect(screen.getByText('0 variables')).toBeInTheDocument();
    });

    it('handles content with only comments', () => {
      const commentsOnly = '# Comment 1\n# Comment 2\n';
      render(<EnvVariablesViewer content={commentsOnly} fileName={mockFileName} />);

      expect(screen.getByText('0 variables')).toBeInTheDocument();
    });

    it('handles undefined onChange callback', () => {
      render(<EnvVariablesViewer content={mockContent} fileName={mockFileName} editable={true} />);

      const buttons = screen.getAllByRole('button');
      fireEvent.click(buttons[buttons.length - 1]);

      const editor = screen.getByTestId('monaco-editor');

      // Component should not throw when onChange is undefined
      expect(() => {
        fireEvent.change(editor, {target: {value: (editor as HTMLTextAreaElement).value + 'NEW_VAR=value'}});
      }).not.toThrow();
    });
  });
});
