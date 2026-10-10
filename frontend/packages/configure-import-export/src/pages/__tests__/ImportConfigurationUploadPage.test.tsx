// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, userEvent, waitFor, fireEvent} from '@thunderid/test-utils';
import {afterEach, describe, expect, it, vi} from 'vitest';

const mockNavigate = vi.fn();
let mockPathname = '/import-configuration';

vi.mock('react-router', async () => {
  const actual = await vi.importActual<typeof import('react-router')>('react-router');
  return {...actual, useNavigate: () => mockNavigate, useLocation: () => ({pathname: mockPathname})};
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
  mockPathname = '/import-configuration';
});

describe('ImportConfigurationUploadPage', () => {
  it('renders without crashing', () => {
    const {container} = render(<ImportConfigurationUploadPage />);
    expect(container).toBeInTheDocument();
  });

  it('renders upload title', () => {
    render(<ImportConfigurationUploadPage />);
    // "Import Configuration" is used both as the page title and the (non-clickable) trailing
    // breadcrumb crumb; disambiguate via heading level (h2 title vs h5 breadcrumb item).
    expect(screen.getByRole('heading', {level: 2, name: 'Import Configuration'})).toBeInTheDocument();
  });

  it('renders the file drop area', () => {
    render(<ImportConfigurationUploadPage />);
    expect(screen.getByText('Drop your configuration file here')).toBeInTheDocument();
  });

  it('renders the env file drop area', () => {
    render(<ImportConfigurationUploadPage />);
    expect(screen.getByText('Environment Variables')).toBeInTheDocument();
  });

  it('renders cancel and continue buttons', () => {
    render(<ImportConfigurationUploadPage />);
    expect(screen.getByRole('button', {name: 'Cancel'})).toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'Continue'})).toBeInTheDocument();
  });

  it('continue button is disabled when no files selected', () => {
    render(<ImportConfigurationUploadPage />);
    expect(screen.getByRole('button', {name: 'Continue'})).toBeDisabled();
  });

  it('navigates to /home on cancel', async () => {
    const user = userEvent.setup();
    render(<ImportConfigurationUploadPage />);

    await user.click(screen.getByRole('button', {name: 'Cancel'}));

    expect(mockNavigate).toHaveBeenCalledWith('/home');
  });

  it('navigates to /home on close', async () => {
    const user = userEvent.setup();
    render(<ImportConfigurationUploadPage />);

    await user.click(screen.getByRole('button', {name: 'Close'}));

    expect(mockNavigate).toHaveBeenCalledWith('/home');
  });

  it('navigates to /import-export when the default breadcrumb is clicked outside the welcome flow', async () => {
    const user = userEvent.setup();
    render(<ImportConfigurationUploadPage />);

    await user.click(screen.getByText('Import / Export'));

    expect(mockNavigate).toHaveBeenCalledWith('/import-export');
  });

  it('navigates to /welcome when the welcome breadcrumb is clicked from the welcome flow', async () => {
    mockPathname = '/welcome/import-configuration';
    const user = userEvent.setup();
    render(<ImportConfigurationUploadPage />);

    await user.click(screen.getByText('Welcome'));

    expect(mockNavigate).toHaveBeenCalledWith('/welcome');
  });

  it('shows error when non-yaml file is selected', () => {
    render(<ImportConfigurationUploadPage />);

    const input = document.getElementById('file-upload') as HTMLInputElement;
    const file = new File(['content'], 'config.txt', {type: 'text/plain'});
    fireEvent.change(input, {target: {files: [file]}});

    expect(screen.getByRole('alert')).toBeInTheDocument();
    expect(screen.getByText('Please upload a YAML file (thunderid-config.yml)')).toBeInTheDocument();
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
    expect(screen.getByText('Please upload an .env file')).toBeInTheDocument();
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

    expect(screen.getByRole('button', {name: 'Continue'})).not.toBeDisabled();
  });

  it('continue button is enabled with only a YAML file selected (env file is optional)', async () => {
    const user = userEvent.setup();
    render(<ImportConfigurationUploadPage />);

    const yamlFile = new File(['key: value'], 'config.yaml', {type: 'text/yaml'});
    await user.upload(document.getElementById('file-upload') as HTMLInputElement, yamlFile);

    expect(screen.getByRole('button', {name: 'Continue'})).not.toBeDisabled();
  });

  it('navigates to validate page with envFile and envData absent when only YAML is provided', async () => {
    const user = userEvent.setup();
    render(<ImportConfigurationUploadPage />);

    const yamlContent = 'resource_type: application\nname: test-app\n';
    const yamlFile = new File([yamlContent], 'config.yaml', {type: 'text/yaml'});
    Object.defineProperty(yamlFile, 'text', {value: () => Promise.resolve(yamlContent)});

    await user.upload(document.getElementById('file-upload') as HTMLInputElement, yamlFile);
    await user.click(screen.getByRole('button', {name: 'Continue'}));

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

    const yamlContent = 'resource_type: application\nname: test-app\n';
    const envContent = 'KEY=VALUE';
    const yamlFile = new File([yamlContent], 'config.yaml', {type: 'text/yaml'});
    const envFile = new File([envContent], '.env', {type: 'text/plain'});

    // jsdom does not implement File.prototype.text(); provide it for the async handler
    Object.defineProperty(yamlFile, 'text', {value: () => Promise.resolve(yamlContent)});
    Object.defineProperty(envFile, 'text', {value: () => Promise.resolve(envContent)});

    await user.upload(document.getElementById('file-upload') as HTMLInputElement, yamlFile);
    await user.upload(document.getElementById('env-file-upload') as HTMLInputElement, envFile);
    await user.click(screen.getByRole('button', {name: 'Continue'}));

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
      'resource_type: server_config\nname: cors\nvalue:\n  allowedOrigins:\n    - https://example.com\n';
    const yamlFile = new File([yamlContent], 'config.yaml', {type: 'text/yaml'});
    Object.defineProperty(yamlFile, 'text', {value: () => Promise.resolve(yamlContent)});

    await user.upload(document.getElementById('file-upload') as HTMLInputElement, yamlFile);
    await user.click(screen.getByRole('button', {name: 'Continue'}));

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

  it('detects resource types from the resource_type YAML field (not comments)', async () => {
    const user = userEvent.setup();
    render(<ImportConfigurationUploadPage />);

    const yamlContent =
      'resource_type: application\n' +
      'id: claims-demo-m2m-app\n' +
      'name: Claims Demo M2M Application\n' +
      'ouHandle: default\n' +
      '---\n' +
      'resource_type: agent\n' +
      'id: claims-demo-agent\n' +
      'ouHandle: default\n' +
      'type: default\n' +
      'attributes:\n' +
      '  name: Claims Demo Agent\n';
    const yamlFile = new File([yamlContent], 'config.yaml', {type: 'text/yaml'});
    Object.defineProperty(yamlFile, 'text', {value: () => Promise.resolve(yamlContent)});

    await user.upload(document.getElementById('file-upload') as HTMLInputElement, yamlFile);
    await user.click(screen.getByRole('button', {name: 'Continue'}));

    await waitFor(
      () => {
        expect(mockNavigate).toHaveBeenCalledWith(
          '/welcome/import-configuration/validate',
          expect.objectContaining({
            state: expect.objectContaining({
              method: 'file',
              parseErrors: [],
              parseStats: {successCount: 2, failCount: 0},
            }) as Record<string, unknown>,
          }),
        );
      },
      {timeout: 5000},
    );

    const {state} = (mockNavigate.mock.calls[0] as [string, {state: {configData: Record<string, unknown[]>}}])[1];
    expect(Object.keys(state.configData)).toEqual(['application', 'agent']);
    expect(state.configData.application).toHaveLength(1);
    expect(state.configData.agent).toHaveLength(1);
  });

  it('shows error when file.text() throws during continue', async () => {
    const user = userEvent.setup();
    render(<ImportConfigurationUploadPage />);

    const yamlFile = new File(['key: value'], 'config.yaml', {type: 'text/yaml'});
    Object.defineProperty(yamlFile, 'text', {value: () => Promise.reject(new Error('read error'))});

    await user.upload(document.getElementById('file-upload') as HTMLInputElement, yamlFile);
    await user.click(screen.getByRole('button', {name: 'Continue'}));

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

    const dropZone = screen.getByText('Drop your configuration file here').closest('div')?.parentElement;
    expect(dropZone).toBeInTheDocument();

    const dataTransfer = new DataTransfer();

    // Simulate drag enter
    fireEvent.dragEnter(dropZone!, {
      dataTransfer,
    });

    // Simulate drag over
    fireEvent.dragOver(dropZone!, {
      dataTransfer,
    });

    // Simulate drag leave
    fireEvent.dragLeave(dropZone!, {
      dataTransfer,
    });
  });

  it('handles drag and drop events on env file area', () => {
    render(<ImportConfigurationUploadPage />);

    const envLabel = screen.getByText('Environment Variables');
    const dropZone = envLabel.closest('div')?.nextElementSibling;
    expect(dropZone).toBeInTheDocument();

    const dataTransfer = new DataTransfer();

    // Simulate drag enter
    fireEvent.dragEnter(dropZone!, {
      dataTransfer,
    });

    // Simulate drag over
    fireEvent.dragOver(dropZone!, {
      dataTransfer,
    });

    // Simulate drag leave
    fireEvent.dragLeave(dropZone!, {
      dataTransfer,
    });
  });

  it('accepts .yml file format via drag and drop', () => {
    render(<ImportConfigurationUploadPage />);

    const dropZone = screen.getByText('Drop your configuration file here').closest('div')?.parentElement;
    const ymlFile = new File(['key: value'], 'config.yml', {type: 'text/yaml'});

    // Real browsers only populate `dataTransfer.files` for events that originate from a genuine
    // OS-level drag gesture; a `DataTransfer` built and dispatched from script reports an empty
    // file list on read (unlike jsdom). Dispatch a plain `Event` with a stubbed `dataTransfer`
    // instead, since the component only reads `dataTransfer.files` off the event.
    const event = new Event('drop', {bubbles: true, cancelable: true});
    Object.defineProperty(event, 'dataTransfer', {value: {files: [ymlFile]}});
    fireEvent(dropZone!, event);

    expect(screen.getByText('config.yml')).toBeInTheDocument();
  });

  it('shows an error when a non-yaml file is dropped onto the config drop zone', () => {
    render(<ImportConfigurationUploadPage />);

    const dropZone = screen.getByText('Drop your configuration file here').closest('div')?.parentElement;
    const txtFile = new File(['not yaml'], 'notes.txt', {type: 'text/plain'});

    const event = new Event('drop', {bubbles: true, cancelable: true});
    Object.defineProperty(event, 'dataTransfer', {value: {files: [txtFile]}});
    fireEvent(dropZone!, event);

    expect(screen.getByText('Please upload a YAML file (thunderid-config.yml)')).toBeInTheDocument();
  });

  it('handles drag and drop events on the env drop zone', () => {
    render(<ImportConfigurationUploadPage />);

    const dropZone = document.getElementById('env-file-upload')!.parentElement;
    const dataTransfer = new DataTransfer();

    fireEvent.dragEnter(dropZone!, {dataTransfer});
    fireEvent.dragOver(dropZone!, {dataTransfer});
    fireEvent.dragLeave(dropZone!, {dataTransfer});
  });

  it('accepts a valid .env file dropped onto the env drop zone', () => {
    render(<ImportConfigurationUploadPage />);

    const dropZone = document.getElementById('env-file-upload')!.parentElement;
    const envFile = new File(['KEY=VALUE'], '.env', {type: 'text/plain'});

    const event = new Event('drop', {bubbles: true, cancelable: true});
    Object.defineProperty(event, 'dataTransfer', {value: {files: [envFile]}});
    fireEvent(dropZone!, event);

    expect(screen.getByText('.env')).toBeInTheDocument();
  });

  it('shows an error when a non-env file is dropped onto the env drop zone', () => {
    render(<ImportConfigurationUploadPage />);

    const dropZone = document.getElementById('env-file-upload')!.parentElement;
    const txtFile = new File(['not env'], 'secrets.txt', {type: 'text/plain'});

    const event = new Event('drop', {bubbles: true, cancelable: true});
    Object.defineProperty(event, 'dataTransfer', {value: {files: [txtFile]}});
    fireEvent(dropZone!, event);

    expect(screen.getByText('Please upload an .env file')).toBeInTheDocument();
  });

  it('skips an empty first section produced when the file starts with a document separator', async () => {
    const user = userEvent.setup();
    render(<ImportConfigurationUploadPage />);

    const yamlContent = '---\nresource_type: application\nname: leading-separator-app\n';
    const yamlFile = new File([yamlContent], 'config.yaml', {type: 'text/yaml'});
    Object.defineProperty(yamlFile, 'text', {value: () => Promise.resolve(yamlContent)});

    await user.upload(document.getElementById('file-upload') as HTMLInputElement, yamlFile);
    await user.click(screen.getByRole('button', {name: 'Continue'}));

    await waitFor(
      () => {
        expect(mockNavigate).toHaveBeenCalledWith(
          '/welcome/import-configuration/validate',
          expect.objectContaining({
            state: expect.objectContaining({
              method: 'file',
              parseStats: {successCount: 1, failCount: 0},
            }) as Record<string, unknown>,
          }),
        );
      },
      {timeout: 5000},
    );
  });

  it('extracts the original file name from a leading File: comment', async () => {
    const user = userEvent.setup();
    render(<ImportConfigurationUploadPage />);

    const yamlContent = '# File: applications/my-app.yaml\nresource_type: application\nname: My App\n';
    const yamlFile = new File([yamlContent], 'config.yaml', {type: 'text/yaml'});
    Object.defineProperty(yamlFile, 'text', {value: () => Promise.resolve(yamlContent)});

    await user.upload(document.getElementById('file-upload') as HTMLInputElement, yamlFile);
    await user.click(screen.getByRole('button', {name: 'Continue'}));

    await waitFor(
      () => {
        expect(mockNavigate).toHaveBeenCalledWith(
          '/welcome/import-configuration/validate',
          expect.objectContaining({
            state: expect.objectContaining({parseStats: {successCount: 1, failCount: 0}}) as Record<string, unknown>,
          }),
        );
      },
      {timeout: 5000},
    );

    const [, {state}] = mockNavigate.mock.calls[0] as [
      string,
      {state: {configData: Record<string, {_metadata?: {originalFileName?: string}}[]>}},
    ];
    expect(state.configData.application[0]._metadata?.originalFileName).toBe('applications/my-app.yaml');
  });

  it('records a parse error and warns when a section is not valid YAML', async () => {
    const user = userEvent.setup();
    render(<ImportConfigurationUploadPage />);

    const yamlContent = 'resource_type: application\nname: [unterminated\n';
    const yamlFile = new File([yamlContent], 'config.yaml', {type: 'text/yaml'});
    Object.defineProperty(yamlFile, 'text', {value: () => Promise.resolve(yamlContent)});

    await user.upload(document.getElementById('file-upload') as HTMLInputElement, yamlFile);
    await user.click(screen.getByRole('button', {name: 'Continue'}));

    await waitFor(
      () => {
        expect(mockNavigate).toHaveBeenCalledWith(
          '/welcome/import-configuration/validate',
          expect.objectContaining({
            state: expect.objectContaining({
              parseStats: {successCount: 0, failCount: 1},
            }) as Record<string, unknown>,
          }),
        );
      },
      {timeout: 5000},
    );
  });

  it('strips unknown resource types and records a parse error for each', async () => {
    const user = userEvent.setup();
    render(<ImportConfigurationUploadPage />);

    const yamlContent = 'resource_type: not_a_real_type\nname: bogus\n';
    const yamlFile = new File([yamlContent], 'config.yaml', {type: 'text/yaml'});
    Object.defineProperty(yamlFile, 'text', {value: () => Promise.resolve(yamlContent)});

    await user.upload(document.getElementById('file-upload') as HTMLInputElement, yamlFile);
    await user.click(screen.getByRole('button', {name: 'Continue'}));

    await waitFor(
      () => {
        expect(mockNavigate).toHaveBeenCalledWith(
          '/welcome/import-configuration/validate',
          expect.objectContaining({
            state: expect.objectContaining({
              parseStats: {successCount: 1, failCount: 1},
            }) as Record<string, unknown>,
          }),
        );
      },
      {timeout: 5000},
    );

    const [, {state}] = mockNavigate.mock.calls[0] as [
      string,
      {state: {configData: Record<string, unknown>; parseErrors: {resourceType: string; error: string}[]}},
    ];
    expect(state.configData.not_a_real_type).toBeUndefined();
    expect(state.parseErrors).toHaveLength(1);
    expect(state.parseErrors[0].resourceType).toBe('not_a_real_type');
  });
});
