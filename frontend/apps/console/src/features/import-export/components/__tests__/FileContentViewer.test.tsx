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

import {render, screen, userEvent, waitFor} from '@thunderid/test-utils';
import {describe, expect, it, vi} from 'vitest';
import FileContentViewer from '../FileContentViewer';

const mockT = (key: string, params?: Record<string, unknown>) => {
  if (key === 'fileViewer.download' && params?.fileName !== undefined && typeof params.fileName === 'string') {
    return `Download ${params.fileName}`;
  }
  return key;
};

vi.mock('react-i18next', () => ({
  useTranslation: () => ({t: mockT}),
}));

vi.mock('@monaco-editor/react', () => ({
  default: ({value, language}: {value: string; language: string}) => (
    <div data-testid="monaco-editor" data-language={language}>
      {value}
    </div>
  ),
}));

vi.mock('@wso2/oxygen-ui', async () => {
  const actual = await vi.importActual('@wso2/oxygen-ui');
  return {
    ...actual,
    useColorScheme: () => ({mode: 'light', systemMode: 'light'}),
  };
});

describe('FileContentViewer', () => {
  const mockYamlContent = 'name: test\nversion: 1.0\n';
  const mockFileName = 'config.yml';
  const mockTitle = 'Configuration File';

  describe('rendering', () => {
    it('renders component with title', () => {
      render(<FileContentViewer content={mockYamlContent} fileName={mockFileName} title={mockTitle} />);

      expect(screen.getByText(mockTitle)).toBeInTheDocument();
    });

    it('renders subtitle when provided', () => {
      const subtitle = 'Application configuration';
      render(
        <FileContentViewer content={mockYamlContent} fileName={mockFileName} title={mockTitle} subtitle={subtitle} />,
      );

      expect(screen.getByText(subtitle)).toBeInTheDocument();
    });

    it('does not render subtitle when null', () => {
      const {container} = render(
        <FileContentViewer content={mockYamlContent} fileName={mockFileName} title={mockTitle} subtitle={null} />,
      );

      expect(container.querySelector('[class*="caption"]')).not.toBeInTheDocument();
    });

    it('does not render subtitle when undefined', () => {
      render(<FileContentViewer content={mockYamlContent} fileName={mockFileName} title={mockTitle} />);

      // Only title should be present
      expect(screen.getByText(mockTitle)).toBeInTheDocument();
    });

    it('renders download button by default', () => {
      render(<FileContentViewer content={mockYamlContent} fileName={mockFileName} title={mockTitle} />);

      expect(screen.getByText(`Download ${mockFileName}`)).toBeInTheDocument();
    });

    it('hides download button when showDownload is false', () => {
      render(
        <FileContentViewer content={mockYamlContent} fileName={mockFileName} title={mockTitle} showDownload={false} />,
      );

      expect(screen.queryByText(`Download ${mockFileName}`)).not.toBeInTheDocument();
    });

    it('renders custom icon when provided', () => {
      const CustomIcon = () => <svg data-testid="custom-icon" />;
      render(
        <FileContentViewer content={mockYamlContent} fileName={mockFileName} title={mockTitle} icon={<CustomIcon />} />,
      );

      expect(screen.getByTestId('custom-icon')).toBeInTheDocument();
    });
  });

  describe('expand/collapse behavior', () => {
    it('starts collapsed by default', () => {
      render(<FileContentViewer content={mockYamlContent} fileName={mockFileName} title={mockTitle} />);

      expect(screen.queryByTestId('monaco-editor')).not.toBeVisible();
    });

    it('expands when icon button clicked', async () => {
      render(<FileContentViewer content={mockYamlContent} fileName={mockFileName} title={mockTitle} />);

      const buttons = screen.getAllByRole('button');
      const expandButton = buttons[buttons.length - 1];

      await userEvent.click(expandButton);

      expect(screen.getByTestId('monaco-editor')).toBeVisible();
    });

    it('collapses when icon button clicked again', async () => {
      render(<FileContentViewer content={mockYamlContent} fileName={mockFileName} title={mockTitle} />);

      const buttons = screen.getAllByRole('button');
      const expandButton = buttons[buttons.length - 1];

      await userEvent.click(expandButton);
      expect(screen.getByTestId('monaco-editor')).toBeVisible();

      await userEvent.click(expandButton);
      await waitFor(() => {
        expect(screen.queryByTestId('monaco-editor')).not.toBeVisible();
      });
    });

    it('displays content when expanded', async () => {
      render(<FileContentViewer content={mockYamlContent} fileName={mockFileName} title={mockTitle} />);

      const buttons = screen.getAllByRole('button');
      await userEvent.click(buttons[buttons.length - 1]);

      const editor = screen.getByTestId('monaco-editor');
      expect(editor).toBeVisible();
    });
  });

  describe('language detection', () => {
    it('detects yaml language from .yml extension', async () => {
      render(<FileContentViewer content={mockYamlContent} fileName="config.yml" title={mockTitle} />);

      const buttons = screen.getAllByRole('button');
      await userEvent.click(buttons[buttons.length - 1]);

      const editor = screen.getByTestId('monaco-editor');
      expect(editor).toHaveAttribute('data-language', 'yaml');
    });

    it('detects yaml language from .yaml extension', async () => {
      render(<FileContentViewer content={mockYamlContent} fileName="config.yaml" title={mockTitle} />);

      const buttons = screen.getAllByRole('button');
      await userEvent.click(buttons[buttons.length - 1]);

      const editor = screen.getByTestId('monaco-editor');
      expect(editor).toHaveAttribute('data-language', 'yaml');
    });

    it('uses plaintext for unknown extensions', async () => {
      render(<FileContentViewer content="some content" fileName="file.txt" title={mockTitle} />);

      const buttons = screen.getAllByRole('button');
      await userEvent.click(buttons[buttons.length - 1]);

      const editor = screen.getByTestId('monaco-editor');
      expect(editor).toHaveAttribute('data-language', 'plaintext');
    });

    it('uses plaintext when no extension', async () => {
      render(<FileContentViewer content="some content" fileName="README" title={mockTitle} />);

      const buttons = screen.getAllByRole('button');
      await userEvent.click(buttons[buttons.length - 1]);

      const editor = screen.getByTestId('monaco-editor');
      expect(editor).toHaveAttribute('data-language', 'plaintext');
    });
  });

  describe('download functionality', () => {
    it('triggers download with correct filename', async () => {
      const mockCreateElement = vi.spyOn(document, 'createElement');
      const mockCreateObjectURL = vi.fn(() => 'blob:mock-url');
      const mockRevokeObjectURL = vi.fn();
      URL.createObjectURL = mockCreateObjectURL;
      URL.revokeObjectURL = mockRevokeObjectURL;

      render(<FileContentViewer content={mockYamlContent} fileName={mockFileName} title={mockTitle} />);

      const downloadButton = screen.getByText(`Download ${mockFileName}`);
      await userEvent.click(downloadButton);

      expect(mockCreateElement).toHaveBeenCalledWith('a');
      expect(mockCreateObjectURL).toHaveBeenCalled();
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

      render(<FileContentViewer content={mockYamlContent} fileName={mockFileName} title={mockTitle} />);

      const downloadButton = screen.getByText(`Download ${mockFileName}`);
      await userEvent.click(downloadButton);

      await waitFor(() => {
        expect(mockShowSaveFilePicker).toHaveBeenCalled();
      });
    });

    it('uses yaml MIME type for .yml files', async () => {
      const mockShowSaveFilePicker = vi.fn().mockResolvedValue({
        createWritable: vi.fn().mockResolvedValue({
          write: vi.fn(),
          close: vi.fn(),
        }),
      });

      (
        window as unknown as Window & {
          showSaveFilePicker: (options?: {
            suggestedName?: string;
            types?: {description: string; accept: Record<string, string[]>}[];
          }) => Promise<unknown>;
        }
      ).showSaveFilePicker = mockShowSaveFilePicker;

      render(<FileContentViewer content={mockYamlContent} fileName="config.yml" title={mockTitle} />);

      const downloadButton = screen.getByText('Download config.yml');
      await userEvent.click(downloadButton);

      await waitFor(() => {
        expect(mockShowSaveFilePicker).toHaveBeenCalledWith(
          expect.objectContaining({
            suggestedName: 'config.yml',
            types: expect.arrayContaining([
              expect.objectContaining({
                accept: expect.objectContaining({
                  'text/yaml': expect.any(Array) as string[],
                }) as Record<string, string[]>,
              }) as {accept: Record<string, string[]>},
            ]) as {accept: Record<string, string[]>}[],
          }) as Record<string, unknown>,
        );
      });
    });

    it('uses plain text MIME type for non-yaml files', async () => {
      const mockWrite = vi.fn();
      const mockClose = vi.fn();
      const mockCreateWritable = vi.fn().mockResolvedValue({
        write: mockWrite,
        close: mockClose,
      });
      const mockShowSaveFilePicker = vi.fn().mockResolvedValue({
        createWritable: mockCreateWritable,
      });

      (
        window as unknown as Window & {
          showSaveFilePicker: (options?: {
            suggestedName?: string;
            types?: {description: string; accept: Record<string, string[]>}[];
          }) => Promise<unknown>;
        }
      ).showSaveFilePicker = mockShowSaveFilePicker;

      render(<FileContentViewer content="content" fileName="file.txt" title={mockTitle} />);

      const downloadButton = screen.getByText('Download file.txt');
      await userEvent.click(downloadButton);

      await waitFor(() => {
        expect(mockShowSaveFilePicker).toHaveBeenCalledWith(
          expect.objectContaining({
            types: expect.arrayContaining([
              expect.objectContaining({
                accept: expect.objectContaining({
                  'text/plain': expect.any(Array) as string[],
                }) as Record<string, string[]>,
              }) as {accept: Record<string, string[]>},
            ]) as {accept: Record<string, string[]>}[],
          }) as Record<string, unknown>,
        );
      });
    });

    it('handles download cancellation silently', async () => {
      const mockShowSaveFilePicker = vi.fn().mockRejectedValue(new Error('User cancelled'));
      (window as unknown as Window & {showSaveFilePicker: () => Promise<unknown>}).showSaveFilePicker =
        mockShowSaveFilePicker;

      render(<FileContentViewer content={mockYamlContent} fileName={mockFileName} title={mockTitle} />);

      const downloadButton = screen.getByText(`Download ${mockFileName}`);

      await userEvent.click(downloadButton);
      expect(mockShowSaveFilePicker).toBeDefined();
    });
  });

  describe('maxHeight prop', () => {
    it('uses default maxHeight of 400', async () => {
      render(<FileContentViewer content={mockYamlContent} fileName={mockFileName} title={mockTitle} />);

      const buttons = screen.getAllByRole('button');
      await userEvent.click(buttons[buttons.length - 1]);

      expect(screen.getByTestId('monaco-editor')).toBeVisible();
    });

    it('uses custom maxHeight when provided', async () => {
      render(<FileContentViewer content={mockYamlContent} fileName={mockFileName} title={mockTitle} maxHeight={600} />);

      const buttons = screen.getAllByRole('button');
      await userEvent.click(buttons[buttons.length - 1]);

      expect(screen.getByTestId('monaco-editor')).toBeVisible();
    });
  });

  describe('icon customization', () => {
    it('uses custom icon background color', () => {
      const {container} = render(
        <FileContentViewer
          content={mockYamlContent}
          fileName={mockFileName}
          title={mockTitle}
          iconBgColor="error.lighter"
        />,
      );

      const iconBox = container.querySelector('[class*="MuiBox-root"]');
      expect(iconBox).toBeInTheDocument();
    });

    it('uses custom icon color', () => {
      const {container} = render(
        <FileContentViewer
          content={mockYamlContent}
          fileName={mockFileName}
          title={mockTitle}
          iconColor="error.main"
        />,
      );

      const iconBox = container.querySelector('[class*="MuiBox-root"]');
      expect(iconBox).toBeInTheDocument();
    });

    it('uses default colors when not provided', () => {
      const {container} = render(
        <FileContentViewer content={mockYamlContent} fileName={mockFileName} title={mockTitle} />,
      );

      const iconBox = container.querySelector('[class*="MuiBox-root"]');
      expect(iconBox).toBeInTheDocument();
    });
  });

  describe('edge cases', () => {
    it('handles empty content', async () => {
      render(<FileContentViewer content="" fileName={mockFileName} title={mockTitle} />);

      const buttons = screen.getAllByRole('button');
      await userEvent.click(buttons[buttons.length - 1]);

      const editor = screen.getByTestId('monaco-editor');
      expect(editor).toHaveTextContent('');
    });

    it('handles very long content', async () => {
      const longContent = 'line\n'.repeat(10000);
      render(<FileContentViewer content={longContent} fileName={mockFileName} title={mockTitle} />);

      const buttons = screen.getAllByRole('button');
      await userEvent.click(buttons[buttons.length - 1]);

      expect(screen.getByTestId('monaco-editor')).toBeVisible();
    });

    it('handles special characters in content', async () => {
      const specialContent = '!@#$%^&*()[]{}|;\':",.<>?/';
      render(<FileContentViewer content={specialContent} fileName={mockFileName} title={mockTitle} />);

      const buttons = screen.getAllByRole('button');
      await userEvent.click(buttons[buttons.length - 1]);

      expect(screen.getByText(specialContent)).toBeInTheDocument();
    });

    it('handles unicode content', async () => {
      const unicodeContent = '你好世界 🌍 مرحبا بالعالم';
      render(<FileContentViewer content={unicodeContent} fileName={mockFileName} title={mockTitle} />);

      const buttons = screen.getAllByRole('button');
      await userEvent.click(buttons[buttons.length - 1]);

      expect(screen.getByText(unicodeContent)).toBeInTheDocument();
    });
  });
});
