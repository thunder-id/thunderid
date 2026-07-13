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

import {render, screen} from '@thunderid/test-utils';
import {describe, it, expect} from 'vitest';
import {IdentityProviderTypes} from '../../models/identity-provider';
import getConnectionIcon from '../getConnectionIcon';

describe('getConnectionIcon', () => {
  describe('Supported Provider Types', () => {
    it('should return Google icon for GOOGLE type', () => {
      const icon = getConnectionIcon(IdentityProviderTypes.GOOGLE);

      expect(icon).not.toBeNull();
      expect(icon?.type).toBeDefined();
    });

    it('should render Google icon correctly', () => {
      const icon = getConnectionIcon(IdentityProviderTypes.GOOGLE);

      const {container} = render(<div>{icon}</div>);
      const svgElement = container.querySelector('svg');

      expect(svgElement).toBeInTheDocument();
    });

    it('should return GitHub icon for GITHUB type', () => {
      const icon = getConnectionIcon(IdentityProviderTypes.GITHUB);

      expect(icon).not.toBeNull();
      expect(icon?.type).toBeDefined();
    });

    it('should render GitHub icon correctly', () => {
      const icon = getConnectionIcon(IdentityProviderTypes.GITHUB);

      const {container} = render(<div>{icon}</div>);
      const svgElement = container.querySelector('svg');

      expect(svgElement).toBeInTheDocument();
    });

    it('should return different icons for different provider types', () => {
      const googleIcon = getConnectionIcon(IdentityProviderTypes.GOOGLE);
      const githubIcon = getConnectionIcon(IdentityProviderTypes.GITHUB);

      expect(googleIcon).not.toBeNull();
      expect(githubIcon).not.toBeNull();
      expect(googleIcon?.type).not.toBe(githubIcon?.type);
    });
  });

  describe('Unsupported Provider Types', () => {
    it('should return null for OIDC type', () => {
      const icon = getConnectionIcon(IdentityProviderTypes.OIDC);

      expect(icon).toBeNull();
    });

    it('should return null for OAUTH type', () => {
      const icon = getConnectionIcon(IdentityProviderTypes.OAUTH);

      expect(icon).toBeNull();
    });

    it('should return null for unknown provider type', () => {
      const icon = getConnectionIcon('UNKNOWN_PROVIDER');

      expect(icon).toBeNull();
    });

    it('should return null for empty string', () => {
      const icon = getConnectionIcon('');

      expect(icon).toBeNull();
    });

    it('should return null for undefined type', () => {
      const icon = getConnectionIcon(undefined as unknown as string);

      expect(icon).toBeNull();
    });

    it('should return null for null type', () => {
      const icon = getConnectionIcon(null as unknown as string);

      expect(icon).toBeNull();
    });
  });

  describe('Case Sensitivity', () => {
    it('should be case-sensitive for GOOGLE type', () => {
      const icon = getConnectionIcon('google');

      expect(icon).toBeNull();
    });

    it('should be case-sensitive for GITHUB type', () => {
      const icon = getConnectionIcon('github');

      expect(icon).toBeNull();
    });

    it('should match exact case for GOOGLE', () => {
      const upperCaseIcon = getConnectionIcon('GOOGLE');
      const mixedCaseIcon = getConnectionIcon('Google');

      expect(upperCaseIcon).not.toBeNull();
      expect(mixedCaseIcon).toBeNull();
    });

    it('should match exact case for GITHUB', () => {
      const upperCaseIcon = getConnectionIcon('GITHUB');
      const mixedCaseIcon = getConnectionIcon('GitHub');

      expect(upperCaseIcon).not.toBeNull();
      expect(mixedCaseIcon).toBeNull();
    });
  });

  describe('Icon Rendering', () => {
    it('should render Google icon as a valid React element', () => {
      const icon = getConnectionIcon(IdentityProviderTypes.GOOGLE);

      expect(icon).toBeTruthy();
      expect(typeof icon).toBe('object');
    });

    it('should render GitHub icon as a valid React element', () => {
      const icon = getConnectionIcon(IdentityProviderTypes.GITHUB);

      expect(icon).toBeTruthy();
      expect(typeof icon).toBe('object');
    });

    it('should render Google icon in a container without errors', () => {
      const icon = getConnectionIcon(IdentityProviderTypes.GOOGLE);

      expect(() => render(<div data-testid="icon-container">{icon}</div>)).not.toThrow();

      expect(screen.getByTestId('icon-container')).toBeInTheDocument();
    });

    it('should render GitHub icon in a container without errors', () => {
      const icon = getConnectionIcon(IdentityProviderTypes.GITHUB);

      expect(() => render(<div data-testid="icon-container">{icon}</div>)).not.toThrow();

      expect(screen.getByTestId('icon-container')).toBeInTheDocument();
    });

    it('should not render anything when icon is null', () => {
      const icon = getConnectionIcon('UNSUPPORTED');

      render(<div data-testid="icon-container">{icon}</div>);
      const iconContainer = screen.getByTestId('icon-container');

      expect(icon).toBeNull();
      expect(iconContainer).toBeEmptyDOMElement();
    });
  });

  describe('All Provider Types Coverage', () => {
    it('should handle all defined IdentityProviderTypes', () => {
      const results = Object.values(IdentityProviderTypes).map((type) => ({
        type,
        icon: getConnectionIcon(type),
      }));

      // GOOGLE and GITHUB should have icons
      const googleResult = results.find((r) => r.type === IdentityProviderTypes.GOOGLE);
      const githubResult = results.find((r) => r.type === IdentityProviderTypes.GITHUB);

      expect(googleResult?.icon).not.toBeNull();
      expect(githubResult?.icon).not.toBeNull();

      // OIDC and OAUTH should return null
      const oidcResult = results.find((r) => r.type === IdentityProviderTypes.OIDC);
      const oauthResult = results.find((r) => r.type === IdentityProviderTypes.OAUTH);

      expect(oidcResult?.icon).toBeNull();
      expect(oauthResult?.icon).toBeNull();
    });

    it('should return consistent results for the same provider type', () => {
      const firstCall = getConnectionIcon(IdentityProviderTypes.GOOGLE);
      const secondCall = getConnectionIcon(IdentityProviderTypes.GOOGLE);

      // Both should be non-null
      expect(firstCall).not.toBeNull();
      expect(secondCall).not.toBeNull();

      // Should have the same type
      expect(firstCall?.type).toBe(secondCall?.type);
    });
  });

  describe('Edge Cases', () => {
    it('should handle whitespace in provider type', () => {
      const icon = getConnectionIcon(' GOOGLE ');

      expect(icon).toBeNull();
    });

    it('should handle numeric input', () => {
      const icon = getConnectionIcon('123' as string);

      expect(icon).toBeNull();
    });

    it('should handle special characters', () => {
      const icon = getConnectionIcon('GOOGLE@#$%');

      expect(icon).toBeNull();
    });

    it('should handle very long strings', () => {
      const longString = 'A'.repeat(1000);
      const icon = getConnectionIcon(longString);

      expect(icon).toBeNull();
    });
  });

  describe('Type Safety', () => {
    it('should accept string type parameter', () => {
      const stringType = 'GOOGLE';
      const icon = getConnectionIcon(stringType);

      expect(icon).not.toBeNull();
    });

    it('should work with IdentityProviderType enum values', () => {
      const enumType = IdentityProviderTypes.GOOGLE;
      const icon = getConnectionIcon(enumType);

      expect(icon).not.toBeNull();
    });
  });

  describe('Return Value Validation', () => {
    it('should return JSX.Element for supported types', () => {
      const googleIcon = getConnectionIcon(IdentityProviderTypes.GOOGLE);

      expect(googleIcon).not.toBeNull();
      expect(googleIcon).toHaveProperty('type');
      expect(googleIcon).toHaveProperty('props');
    });

    it('should return null (not undefined) for unsupported types', () => {
      const icon = getConnectionIcon('UNSUPPORTED');

      expect(icon).toBeNull();
      expect(icon).not.toBeUndefined();
    });

    it('should return exact null value for multiple unsupported types', () => {
      const icon1 = getConnectionIcon('UNKNOWN1');
      const icon2 = getConnectionIcon('UNKNOWN2');

      expect(icon1).toBeNull();
      expect(icon2).toBeNull();
      expect(icon1).toBe(icon2); // Both are null
    });
  });

  describe('Integration with React', () => {
    it('should be renderable in a React component tree', () => {
      function GoogleIconComponent() {
        const icon = getConnectionIcon(IdentityProviderTypes.GOOGLE);
        return <div data-testid="google-wrapper">{icon}</div>;
      }

      render(<GoogleIconComponent />);

      expect(screen.getByTestId('google-wrapper')).toBeInTheDocument();
    });

    it('should be conditionally renderable based on provider type', () => {
      function ConditionalIcon({type}: {type: string}) {
        const icon = getConnectionIcon(type);
        return <div data-testid="conditional-wrapper">{icon ?? <span>No icon</span>}</div>;
      }

      const {rerender} = render(<ConditionalIcon type={IdentityProviderTypes.GOOGLE} />);
      expect(screen.getByTestId('conditional-wrapper').querySelector('svg')).toBeInTheDocument();

      rerender(<ConditionalIcon type="UNSUPPORTED" />);
      expect(screen.getByText('No icon')).toBeInTheDocument();
    });

    it('should handle multiple icons in the same component', () => {
      function MultiIconComponent() {
        const googleIcon = getConnectionIcon(IdentityProviderTypes.GOOGLE);
        const githubIcon = getConnectionIcon(IdentityProviderTypes.GITHUB);

        return (
          <div data-testid="multi-icon-wrapper">
            <div data-testid="google-icon">{googleIcon}</div>
            <div data-testid="github-icon">{githubIcon}</div>
          </div>
        );
      }

      render(<MultiIconComponent />);

      expect(screen.getByTestId('google-icon').querySelector('svg')).toBeInTheDocument();
      expect(screen.getByTestId('github-icon').querySelector('svg')).toBeInTheDocument();
    });
  });
});
