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

import userEvent from '@testing-library/user-event';
import {render, screen} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, vi} from 'vitest';
import ConfigureDesign, {type ConfigureDesignProps} from '../ConfigureDesign';

// Mock the Packages
vi.mock('@thunderid/components', () => ({
  LogoPicker: vi.fn(({value, onChange}: {value: string; onChange: (value: string) => void}) => (
    <button type="button" data-testid="logo-picker" onClick={() => onChange('emoji:🚀')}>
      {value}
    </button>
  )),
}));
vi.mock('@thunderid/react', () => ({
  buildAvatarSpec: vi.fn(() => 'avatar:shape=rounded,variant=anonymous_entity,content=briefcase,colors=0'),
  pickAnonymousEntityName: vi.fn(() => 'briefcase'),
}));
vi.mock('@thunderid/design');

const {useGetThemes, useGetTheme} = await import('@thunderid/design');

describe('ConfigureDesign', () => {
  const mockOnLogoSelect = vi.fn();
  const mockOnThemeSelect = vi.fn();

  const mockDefaultEntityLogo = 'avatar:shape=rounded,variant=anonymous_entity,content=briefcase,colors=0';

  const defaultProps: ConfigureDesignProps = {
    appLogo: null,
    onLogoSelect: mockOnLogoSelect,
    onThemeSelect: mockOnThemeSelect,
  };

  beforeEach(() => {
    vi.clearAllMocks();

    vi.mocked(useGetThemes).mockReturnValue({
      data: undefined,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useGetThemes>);

    vi.mocked(useGetTheme).mockReturnValue({
      data: undefined,
      isLoading: false,
      error: null,
    } as ReturnType<typeof useGetTheme>);
  });

  const renderComponent = (props: Partial<ConfigureDesignProps> = {}) =>
    render(<ConfigureDesign {...defaultProps} {...props} />);

  it('should render the component with title', () => {
    renderComponent();

    expect(screen.getByRole('heading', {level: 1})).toBeInTheDocument();
  });

  it('should render subtitle', () => {
    renderComponent();

    expect(screen.getByText('Customize the appearance of your application')).toBeInTheDocument();
  });

  it('should render logo section title', () => {
    renderComponent();

    expect(screen.getByRole('heading', {name: 'Application Logo'})).toBeInTheDocument();
  });

  it('should render the LogoPicker with the current logo value', () => {
    renderComponent({appLogo: 'emoji:🐼'});

    expect(screen.getByTestId('logo-picker')).toHaveTextContent('emoji:🐼');
  });

  it('should auto-select a default entity avatar when appLogo is null', () => {
    renderComponent();

    expect(mockOnLogoSelect).toHaveBeenCalledWith(mockDefaultEntityLogo);
  });

  it('should not auto-select when appLogo is already set', () => {
    renderComponent({appLogo: 'emoji:🐼'});

    expect(mockOnLogoSelect).not.toHaveBeenCalled();
  });

  it('should call onLogoSelect when the LogoPicker fires onChange', async () => {
    const user = userEvent.setup();
    renderComponent({appLogo: 'emoji:🐼'});

    await user.click(screen.getByTestId('logo-picker'));

    expect(mockOnLogoSelect).toHaveBeenCalledWith('emoji:🚀');
  });

  it('should handle null appLogo prop without errors', () => {
    renderComponent({appLogo: null});

    expect(screen.getByRole('heading', {level: 1})).toBeInTheDocument();
  });

  describe('onReadyChange callback', () => {
    it('should call onReadyChange with true on mount', () => {
      const mockOnReadyChange = vi.fn();
      renderComponent({onReadyChange: mockOnReadyChange});

      expect(mockOnReadyChange).toHaveBeenCalledWith(true);
    });
  });

  describe('Theme selection', () => {
    const mockThemeDetails = {
      id: 'theme-1',
      displayName: 'Corporate Blue',
      theme: {
        colorSchemes: {
          light: {
            colors: {
              primary: {
                main: '#123456',
              },
            },
          },
        },
      },
    };

    const mockThemesList = [
      {id: 'theme-1', displayName: 'Corporate Blue'},
      {id: 'theme-2', displayName: 'Sunset Orange'},
    ];

    it('should render theme cards when themes are available', () => {
      vi.mocked(useGetThemes).mockReturnValue({
        data: {themes: mockThemesList},
        isLoading: false,
        error: null,
      } as ReturnType<typeof useGetThemes>);

      vi.mocked(useGetTheme).mockReturnValue({
        data: mockThemeDetails,
        isLoading: false,
        error: null,
      } as unknown as ReturnType<typeof useGetTheme>);

      renderComponent();

      expect(screen.getByText('Corporate Blue')).toBeInTheDocument();
      expect(screen.getByText('Sunset Orange')).toBeInTheDocument();
    });

    it('should render a card for each theme', () => {
      vi.mocked(useGetThemes).mockReturnValue({
        data: {themes: mockThemesList},
        isLoading: false,
        error: null,
      } as ReturnType<typeof useGetThemes>);

      vi.mocked(useGetTheme).mockReturnValue({
        data: mockThemeDetails,
        isLoading: false,
        error: null,
      } as unknown as ReturnType<typeof useGetTheme>);

      renderComponent();

      expect(screen.getByTestId('theme-card-theme-1')).toBeInTheDocument();
      expect(screen.getByTestId('theme-card-theme-2')).toBeInTheDocument();
    });

    it('should call onThemeSelect with theme details when theme is loaded', () => {
      vi.mocked(useGetThemes).mockReturnValue({
        data: {themes: mockThemesList},
        isLoading: false,
        error: null,
      } as ReturnType<typeof useGetThemes>);

      vi.mocked(useGetTheme).mockReturnValue({
        data: mockThemeDetails,
        isLoading: false,
        error: null,
      } as unknown as ReturnType<typeof useGetTheme>);

      renderComponent();

      expect(mockOnThemeSelect).toHaveBeenCalledWith('theme-1', mockThemeDetails.theme);
    });

    it('should show empty state when no themes are configured', () => {
      vi.mocked(useGetThemes).mockReturnValue({
        data: {themes: []},
        isLoading: false,
        error: null,
      } as unknown as ReturnType<typeof useGetThemes>);

      renderComponent();

      expect(screen.getByText('No themes configured')).toBeInTheDocument();
      expect(screen.getByText('You can configure themes later from the Design settings.')).toBeInTheDocument();
    });

    it('should select a different theme when clicking its card', async () => {
      const user = userEvent.setup();
      const mockOnThemeSelectLocal = vi.fn();

      vi.mocked(useGetThemes).mockReturnValue({
        data: {themes: mockThemesList},
        isLoading: false,
        error: null,
      } as ReturnType<typeof useGetThemes>);

      vi.mocked(useGetTheme).mockReturnValue({
        data: mockThemeDetails,
        isLoading: false,
        error: null,
      } as unknown as ReturnType<typeof useGetTheme>);

      renderComponent({onThemeSelect: mockOnThemeSelectLocal});

      const secondThemeCard = screen.getByTestId('theme-card-theme-2');
      await user.click(secondThemeCard);

      expect(mockOnThemeSelectLocal).toHaveBeenCalledWith('theme-1', mockThemeDetails.theme);
    });
  });
});
