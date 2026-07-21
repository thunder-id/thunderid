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

import {render, screen, userEvent, waitFor, fireEvent} from '@thunderid/test-utils';
import {afterEach, describe, expect, it, vi} from 'vitest';

const mockNavigate = vi.fn();

vi.mock('react-i18next', () => ({
  useTranslation: () => ({t: (key: string) => key}),
}));

vi.mock('react-router', async () => {
  const actual = await vi.importActual<typeof import('react-router')>('react-router');
  return {...actual, useNavigate: () => mockNavigate};
});

vi.mock('@thunderid/logger/react', () => ({
  useLogger: () => ({error: vi.fn(), warn: vi.fn(), info: vi.fn(), debug: vi.fn()}),
}));

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: () => ({
      config: {
        brand: {
          product_name: 'ThunderID',
        },
      },
    }),
  };
});

vi.mock('@wso2/oxygen-ui-icons-react', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@wso2/oxygen-ui-icons-react')>();
  return {
    ...actual,
    ChevronRight: () => <span data-testid="icon-chevron-right" />,
    Upload: () => <span data-testid="icon-upload" />,
    X: () => <span data-testid="icon-x" />,
  };
});

import ImportConfigurationUploadPage from '../ImportConfigurationUploadPage';

afterEach(() => {
  vi.clearAllMocks();
});

describe('ImportConfigurationUploadPage', () => {
  it('renders without crashing', () => {
    const {container} = render(<ImportConfigurationUploadPage />);
    expect(container).toBeInTheDocument();
  });

  it('renders upload title', () => {
    render(<ImportConfigurationUploadPage />);
    expect(screen.getByText('upload.title')).toBeInTheDocument();
  });

  it('renders the file drop area', () => {
    render(<ImportConfigurationUploadPage />);
    expect(screen.getByText('upload.dropConfig')).toBeInTheDocument();
  });

  it('renders the env file drop area', () => {
    render(<ImportConfigurationUploadPage />);
    expect(screen.getByText('upload.env.title')).toBeInTheDocument();
  });

  it('renders cancel and continue buttons', () => {
    render(<ImportConfigurationUploadPage />);
    expect(screen.getByRole('button', {name: 'common:actions.cancel'})).toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'common:actions.continue'})).toBeInTheDocument();
  });

  it('continue button is disabled when no files selected', () => {
    render(<ImportConfigurationUploadPage />);
    expect(screen.getByRole('button', {name: 'common:actions.continue'})).toBeDisabled();
  });

  it('navigates to /home on cancel', async () => {
    const user = userEvent.setup();
    render(<ImportConfigurationUploadPage />);

    await user.click(screen.getByRole('button', {name: 'common:actions.cancel'}));

    expect(mockNavigate).toHaveBeenCalledWith('/home');
  });

  it('navigates to /home on close', async () => {
    const user = userEvent.setup();
    render(<ImportConfigurationUploadPage />);

    await user.click(screen.getByRole('button', {name: 'common:actions.close'}));

    expect(mockNavigate).toHaveBeenCalledWith('/home');
  });

  it('shows error when non-yaml file is selected', () => {
    render(<ImportConfigurationUploadPage />);

    const input = document.getElementById('file-upload') as HTMLInputElement;
    const file = new File(['content'], 'config.txt', {type: 'text/plain'});
    fireEvent.change(input, {target: {files: [file]}});

    expect(screen.getByRole('alert')).toBeInTheDocument();
    expect(screen.getByText('upload.errors.uploadYaml')).toBeInTheDocument();
  });

  it('shows file name after valid yaml file is selected', async () => {
    const user = userEvent.setup();
    render(<ImportConfigurationUploadPage />);

    const input = document.getElementById('file-upload') as HTMLInputElement;
    const file = new File(['key: value'], 'config.yaml', {type: 'text/yaml'});
    await user.upload(input, file);

    expect(screen.getByText('config.yaml')).toBeInTheDocument();
  });

  it('shows error when non-env file is selected for env input', () => {
    render(<ImportConfigurationUploadPage />);

    const input = document.getElementById('env-file-upload') as HTMLInputElement;
    const file = new File(['KEY=VALUE'], 'secrets.txt', {type: 'text/plain'});
    fireEvent.change(input, {target: {files: [file]}});

    expect(screen.getByRole('alert')).toBeInTheDocument();
    expect(screen.getByText('upload.errors.uploadEnv')).toBeInTheDocument();
  });

  it('shows env file name after valid .env file is selected', async () => {
    const user = userEvent.setup();
    render(<ImportConfigurationUploadPage />);

    const input = document.getElementById('env-file-upload') as HTMLInputElement;
    const file = new File(['KEY=VALUE'], '.env', {type: 'text/plain'});
    await user.upload(input, file);

    expect(screen.getByText('.env')).toBeInTheDocument();
  });

  it('continue button becomes enabled after both files are selected', async () => {
    const user = userEvent.setup();
    render(<ImportConfigurationUploadPage />);

    const yamlFile = new File(['key: value'], 'config.yaml', {type: 'text/yaml'});
    await user.upload(document.getElementById('file-upload') as HTMLInputElement, yamlFile);

    const envFile = new File(['KEY=VALUE'], '.env', {type: 'text/plain'});
    await user.upload(document.getElementById('env-file-upload') as HTMLInputElement, envFile);

    expect(screen.getByRole('button', {name: 'common:actions.continue'})).not.toBeDisabled();
  });

  it('continue button is enabled with only a YAML file selected (env file is optional)', async () => {
    const user = userEvent.setup();
    render(<ImportConfigurationUploadPage />);

    const yamlFile = new File(['key: value'], 'config.yaml', {type: 'text/yaml'});
    await user.upload(document.getElementById('file-upload') as HTMLInputElement, yamlFile);

    expect(screen.getByRole('button', {name: 'common:actions.continue'})).not.toBeDisabled();
  });

  it('navigates to validate page with envFile and envData absent when only YAML is provided', async () => {
    const user = userEvent.setup();
    render(<ImportConfigurationUploadPage />);

    const yamlContent = '---\n# resource_type: application\nname: test-app\n';
    const yamlFile = new File([yamlContent], 'config.yaml', {type: 'text/yaml'});
    Object.defineProperty(yamlFile, 'text', {value: () => Promise.resolve(yamlContent)});

    await user.upload(document.getElementById('file-upload') as HTMLInputElement, yamlFile);
    await user.click(screen.getByRole('button', {name: 'common:actions.continue'}));

    await waitFor(
      () => {
        expect(mockNavigate).toHaveBeenCalledWith(
          '/welcome/import-configuration/validate',
          expect.objectContaining({
            state: expect.objectContaining({
              method: 'file',
              envFile: null,
              envData: null,
            }) as Record<string, unknown>,
          }),
        );
      },
      {timeout: 5000},
    );
  });

  it('navigates to validate page after both valid files are provided', async () => {
    const user = userEvent.setup();
    render(<ImportConfigurationUploadPage />);

    const yamlContent = '---\n# resource_type: application\nname: test-app\n';
    const envContent = 'KEY=VALUE';
    const yamlFile = new File([yamlContent], 'config.yaml', {type: 'text/yaml'});
    const envFile = new File([envContent], '.env', {type: 'text/plain'});

    // jsdom does not implement File.prototype.text(); provide it for the async handler
    Object.defineProperty(yamlFile, 'text', {value: () => Promise.resolve(yamlContent)});
    Object.defineProperty(envFile, 'text', {value: () => Promise.resolve(envContent)});

    await user.upload(document.getElementById('file-upload') as HTMLInputElement, yamlFile);
    await user.upload(document.getElementById('env-file-upload') as HTMLInputElement, envFile);
    await user.click(screen.getByRole('button', {name: 'common:actions.continue'}));

    await waitFor(
      () => {
        expect(mockNavigate).toHaveBeenCalledWith(
          '/welcome/import-configuration/validate',
          expect.objectContaining({state: expect.objectContaining({method: 'file'}) as Record<string, unknown>}),
        );
      },
      {timeout: 5000},
    );
  });

  it('accepts a server_config resource without flagging it as unknown', async () => {
    const user = userEvent.setup();
    render(<ImportConfigurationUploadPage />);

    const yamlContent =
      '---\n# resource_type: server_config\nname: cors\nvalue:\n  allowedOrigins:\n    - https://example.com\n';
    const yamlFile = new File([yamlContent], 'config.yaml', {type: 'text/yaml'});
    Object.defineProperty(yamlFile, 'text', {value: () => Promise.resolve(yamlContent)});

    await user.upload(document.getElementById('file-upload') as HTMLInputElement, yamlFile);
    await user.click(screen.getByRole('button', {name: 'common:actions.continue'}));

    await waitFor(
      () => {
        expect(mockNavigate).toHaveBeenCalledWith(
          '/welcome/import-configuration/validate',
          expect.objectContaining({
            state: expect.objectContaining({
              method: 'file',
              parseErrors: [],
              parseStats: {successCount: 1, failCount: 0},
            }) as Record<string, unknown>,
          }),
        );
      },
      {timeout: 5000},
    );
  });

  it('shows error when file.text() throws during continue', async () => {
    const user = userEvent.setup();
    render(<ImportConfigurationUploadPage />);

    const yamlFile = new File(['key: value'], 'config.yaml', {type: 'text/yaml'});
    Object.defineProperty(yamlFile, 'text', {value: () => Promise.reject(new Error('read error'))});

    await user.upload(document.getElementById('file-upload') as HTMLInputElement, yamlFile);
    await user.click(screen.getByRole('button', {name: 'common:actions.continue'}));

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument();
    });
  });

  it('accepts .yml file format (not just .yaml)', async () => {
    const user = userEvent.setup();
    render(<ImportConfigurationUploadPage />);

    const input = document.getElementById('file-upload') as HTMLInputElement;
    const file = new File(['key: value'], 'config.yml', {type: 'text/yaml'});
    await user.upload(input, file);

    expect(screen.getByText('config.yml')).toBeInTheDocument();
  });

  it('handles drag and drop events on config file area', () => {
    render(<ImportConfigurationUploadPage />);

    const dropZone = screen.getByText('upload.dropConfig').closest('div')?.parentElement;
    expect(dropZone).toBeInTheDocument();

    // Simulate drag enter
    fireEvent.dragEnter(dropZone!, {
      dataTransfer: {files: []},
    });

    // Simulate drag over
    fireEvent.dragOver(dropZone!, {
      dataTransfer: {files: []},
    });

    // Simulate drag leave
    fireEvent.dragLeave(dropZone!, {
      dataTransfer: {files: []},
    });
  });

  it('handles drag and drop events on env file area', () => {
    render(<ImportConfigurationUploadPage />);

    const envLabel = screen.getByText('upload.env.title');
    const dropZone = envLabel.closest('div')?.nextElementSibling;
    expect(dropZone).toBeInTheDocument();

    // Simulate drag enter
    fireEvent.dragEnter(dropZone!, {
      dataTransfer: {files: []},
    });

    // Simulate drag over
    fireEvent.dragOver(dropZone!, {
      dataTransfer: {files: []},
    });

    // Simulate drag leave
    fireEvent.dragLeave(dropZone!, {
      dataTransfer: {files: []},
    });
  });

  it('accepts .yml file format via drag and drop', () => {
    render(<ImportConfigurationUploadPage />);

    const dropZone = screen.getByText('upload.dropConfig').closest('div')?.parentElement;
    const ymlFile = new File(['key: value'], 'config.yml', {type: 'text/yaml'});

    fireEvent.drop(dropZone!, {
      dataTransfer: {files: [ymlFile]},
    });

    expect(screen.getByText('config.yml')).toBeInTheDocument();
  });
});
