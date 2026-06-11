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

import {render, screen} from '@thunderid/test-utils';
import {afterEach, describe, expect, it, vi} from 'vitest';

vi.mock('@wso2/oxygen-ui-icons-react', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@wso2/oxygen-ui-icons-react')>();
  return {
    ...actual,
    Download: () => <span data-testid="icon-download" />,
  };
});

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
  }),
}));

const {mockUseWayfinderReleases} = vi.hoisted(() => ({
  mockUseWayfinderReleases: vi.fn(),
}));

vi.mock('../../api/useWayfinderReleases', () => ({
  default: (...args: unknown[]): unknown => mockUseWayfinderReleases(...args),
}));

import WayfinderSampleDownload from '../WayfinderSampleDownload';

const mockAsset = {
  name: 'sample-app-wayfinder-1.0.0.zip',
  downloadUrl: 'https://example.com/sample-app-wayfinder-1.0.0.zip',
  sizeLabel: '10 MB',
};

const mockReleasesData = {
  latestRelease: {
    tagName: 'v1.0.0',
    assets: [mockAsset],
  },
  releases: [],
};

describe('WayfinderSampleDownload', () => {
  afterEach(() => {
    vi.clearAllMocks();
  });

  it('returns null when isError is true', () => {
    mockUseWayfinderReleases.mockReturnValue({data: undefined, isError: true});
    const {container} = render(<WayfinderSampleDownload releasesUrl="https://example.com/releases.json" />);
    expect(container).toBeEmptyDOMElement();
  });

  it('returns null when assets array is empty', () => {
    mockUseWayfinderReleases.mockReturnValue({
      data: {latestRelease: {tagName: 'v1.0.0', assets: []}, releases: []},
      isError: false,
    });
    const {container} = render(<WayfinderSampleDownload releasesUrl="https://example.com/releases.json" />);
    expect(container).toBeEmptyDOMElement();
  });

  it('returns null when data is not yet loaded', () => {
    mockUseWayfinderReleases.mockReturnValue({data: undefined, isError: false});
    const {container} = render(<WayfinderSampleDownload releasesUrl="https://example.com/releases.json" />);
    expect(container).toBeEmptyDOMElement();
  });

  it('returns null when no asset matches the expected filename pattern', () => {
    mockUseWayfinderReleases.mockReturnValue({
      data: {
        latestRelease: {
          tagName: 'v1.0.0',
          assets: [{name: 'README.md', downloadUrl: 'https://example.com/README.md', sizeLabel: '1 KB'}],
        },
        releases: [],
      },
      isError: false,
    });
    const {container} = render(<WayfinderSampleDownload releasesUrl="https://example.com/releases.json" />);
    expect(container).toBeEmptyDOMElement();
  });

  it('shows download button when a matching asset is found', async () => {
    mockUseWayfinderReleases.mockReturnValue({data: mockReleasesData, isError: false});
    render(<WayfinderSampleDownload releasesUrl="https://example.com/releases.json" />);

    expect(await screen.findByText('common:welcome.wayfinderSampleDownload.downloadButton')).toBeInTheDocument();
  });

  it('download button links to the asset URL', async () => {
    mockUseWayfinderReleases.mockReturnValue({data: mockReleasesData, isError: false});
    render(<WayfinderSampleDownload releasesUrl="https://example.com/releases.json" />);

    const button = await screen.findByRole('link', {name: /downloadButton/});
    expect(button).toHaveAttribute('href', mockAsset.downloadUrl);
  });

  it('shows the asset filename', async () => {
    mockUseWayfinderReleases.mockReturnValue({data: mockReleasesData, isError: false});
    render(<WayfinderSampleDownload releasesUrl="https://example.com/releases.json" />);

    expect(await screen.findByText('sample-app-wayfinder-1.0.0.zip')).toBeInTheDocument();
  });

  it('shows the size chip when sizeLabel is present', async () => {
    mockUseWayfinderReleases.mockReturnValue({data: mockReleasesData, isError: false});
    render(<WayfinderSampleDownload releasesUrl="https://example.com/releases.json" />);

    expect(await screen.findByText('10 MB')).toBeInTheDocument();
  });
});
