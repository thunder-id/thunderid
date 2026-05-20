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
import {describe, expect, it, vi} from 'vitest';
import TemplateVariableDisplay from '../TemplateVariableDisplay';

const mockT = (key: string) => key;

vi.mock('react-i18next', () => ({
  useTranslation: () => ({t: mockT}),
}));

describe('TemplateVariableDisplay', () => {
  describe('template variable display', () => {
    it('displays template variable with success color when value exists', () => {
      const {container} = render(
        <TemplateVariableDisplay text="{{.CONSOLE_CLIENT_ID}}" envData="CONSOLE_CLIENT_ID=abc123" />,
      );

      const chip = container.querySelector('[class*="MuiChip-colorSuccess"]');
      expect(chip).toBeInTheDocument();
      expect(screen.getByText('{{.CONSOLE_CLIENT_ID}}')).toBeInTheDocument();
      expect(screen.getByText('abc123')).toBeInTheDocument();
    });

    it('displays template variable with warning color when value is empty', () => {
      const {container} = render(
        <TemplateVariableDisplay text="{{.CONSOLE_CLIENT_ID}}" envData="CONSOLE_CLIENT_ID=" />,
      );

      const chip = container.querySelector('[class*="MuiChip-colorWarning"]');
      expect(chip).toBeInTheDocument();
      expect(screen.getByText('templateVariable.valueMissing')).toBeInTheDocument();
    });

    it('displays template variable with error color when value is missing', () => {
      const {container} = render(<TemplateVariableDisplay text="{{.CONSOLE_CLIENT_ID}}" envData="OTHER_VAR=value" />);

      const chip = container.querySelector('[class*="MuiChip-colorError"]');
      expect(chip).toBeInTheDocument();
      expect(screen.getByText('templateVariable.valueMissing')).toBeInTheDocument();
    });

    it('displays template variable with warning color when value is only whitespace', () => {
      const {container} = render(
        <TemplateVariableDisplay text="{{.CONSOLE_CLIENT_ID}}" envData="CONSOLE_CLIENT_ID=   " />,
      );

      const chip = container.querySelector('[class*="MuiChip-colorWarning"]');
      expect(chip).toBeInTheDocument();
    });
  });

  describe('env data parsing', () => {
    it('parses single environment variable', () => {
      render(<TemplateVariableDisplay text="{{.API_KEY}}" envData="API_KEY=secret123" />);

      expect(screen.getByText('secret123')).toBeInTheDocument();
    });

    it('parses multiple environment variables', () => {
      const envData = 'VAR1=value1\nVAR2=value2\nVAR3=value3';
      render(<TemplateVariableDisplay text="{{.VAR2}}" envData={envData} />);

      expect(screen.getByText('value2')).toBeInTheDocument();
    });

    it('ignores comment lines', () => {
      const envData = '# This is a comment\nAPI_KEY=secret\n# Another comment';
      render(<TemplateVariableDisplay text="{{.API_KEY}}" envData={envData} />);

      expect(screen.getByText('secret')).toBeInTheDocument();
    });

    it('ignores empty lines', () => {
      const envData = 'API_KEY=secret\n\n\nOTHER=value';
      render(<TemplateVariableDisplay text="{{.API_KEY}}" envData={envData} />);

      expect(screen.getByText('secret')).toBeInTheDocument();
    });

    it('handles values with equals signs', () => {
      render(
        <TemplateVariableDisplay text="{{.CONNECTION_STRING}}" envData="CONNECTION_STRING=key=value;other=data" />,
      );

      expect(screen.getByText('key=value;other=data')).toBeInTheDocument();
    });

    it('trims whitespace from keys and values', () => {
      render(<TemplateVariableDisplay text="{{.API_KEY}}" envData="  API_KEY  =  secret123  " />);

      expect(screen.getByText('secret123')).toBeInTheDocument();
    });

    it('handles null envData', () => {
      const {container} = render(<TemplateVariableDisplay text="{{.API_KEY}}" envData={null} />);

      const chip = container.querySelector('[class*="MuiChip-colorError"]');
      expect(chip).toBeInTheDocument();
    });

    it('handles undefined envData', () => {
      const {container} = render(<TemplateVariableDisplay text="{{.API_KEY}}" envData={undefined} />);

      const chip = container.querySelector('[class*="MuiChip-colorError"]');
      expect(chip).toBeInTheDocument();
    });

    it('handles empty envData', () => {
      const {container} = render(<TemplateVariableDisplay text="{{.API_KEY}}" envData="" />);

      const chip = container.querySelector('[class*="MuiChip-colorError"]');
      expect(chip).toBeInTheDocument();
    });
  });

  describe('template variable extraction', () => {
    it('extracts variable name with uppercase letters', () => {
      render(<TemplateVariableDisplay text="{{.CONSOLE_CLIENT_ID}}" envData="CONSOLE_CLIENT_ID=value" />);

      expect(screen.getByText('value')).toBeInTheDocument();
    });

    it('extracts variable name with underscores', () => {
      render(<TemplateVariableDisplay text="{{.MY_API_KEY}}" envData="MY_API_KEY=secret" />);

      expect(screen.getByText('secret')).toBeInTheDocument();
    });

    it('extracts variable name with numbers', () => {
      render(<TemplateVariableDisplay text="{{.KEY123}}" envData="KEY123=value" />);

      expect(screen.getByText('value')).toBeInTheDocument();
    });

    it('does not match invalid variable names starting with number', () => {
      const {container} = render(<TemplateVariableDisplay text="{{.123KEY}}" envData="123KEY=value" />);

      expect(screen.queryByText('value')).not.toBeInTheDocument();
      expect(container.textContent).toContain('{{.123KEY}}');
    });

    it('does not match variables without dot prefix', () => {
      const {container} = render(
        <TemplateVariableDisplay text="{{CONSOLE_CLIENT_ID}}" envData="CONSOLE_CLIENT_ID=value" />,
      );

      expect(screen.queryByText('value')).not.toBeInTheDocument();
      expect(container.textContent).toContain('{{CONSOLE_CLIENT_ID}}');
    });
  });

  describe('non-template text', () => {
    it('displays regular text as-is', () => {
      render(<TemplateVariableDisplay text="https://example.com" />);

      expect(screen.getByText('https://example.com')).toBeInTheDocument();
    });

    it('displays partial template syntax as regular text', () => {
      render(<TemplateVariableDisplay text="{{INCOMPLETE" />);

      expect(screen.getByText('{{INCOMPLETE')).toBeInTheDocument();
    });

    it('displays text with template-like pattern but invalid format', () => {
      render(<TemplateVariableDisplay text="{{.lowercase_var}}" />);

      expect(screen.getByText('{{.lowercase_var}}')).toBeInTheDocument();
    });
  });

  describe('label prop', () => {
    it('displays label when provided', () => {
      render(<TemplateVariableDisplay text="{{.API_KEY}}" envData="API_KEY=secret" label="Redirect URI" />);

      expect(screen.getByText('Redirect URI:')).toBeInTheDocument();
      expect(screen.getByText('secret')).toBeInTheDocument();
    });

    it('does not display label when not provided', () => {
      const {container} = render(<TemplateVariableDisplay text="{{.API_KEY}}" envData="API_KEY=secret" />);

      expect(container.textContent).not.toContain(':');
    });

    it('displays label with non-template text', () => {
      render(<TemplateVariableDisplay text="https://example.com" label="URL" />);

      expect(screen.getByText('URL:')).toBeInTheDocument();
      expect(screen.getByText('https://example.com')).toBeInTheDocument();
    });
  });

  describe('chip styling', () => {
    it('displays chip with monospace font', () => {
      const {container} = render(<TemplateVariableDisplay text="{{.API_KEY}}" envData="API_KEY=value" />);

      const chip = container.querySelector('[class*="MuiChip-root"]');
      expect(chip).toBeInTheDocument();
    });

    it('displays value with monospace font and background', () => {
      render(<TemplateVariableDisplay text="{{.API_KEY}}" envData="API_KEY=secret123" />);

      const valueElement = screen.getByText('secret123');
      expect(valueElement).toBeInTheDocument();
    });
  });

  describe('edge cases', () => {
    it('handles extremely long values', () => {
      const longValue = 'a'.repeat(1000);
      render(<TemplateVariableDisplay text="{{.LONG_VAR}}" envData={`LONG_VAR=${longValue}`} />);

      expect(screen.getByText(longValue)).toBeInTheDocument();
    });

    it('handles special characters in values', () => {
      render(<TemplateVariableDisplay text="{{.SPECIAL}}" envData={`SPECIAL=!@#$%^&*()[]{}|;':",.<>?/`} />);

      expect(screen.getByText(`!@#$%^&*()[]{}|;':",.<>?/`)).toBeInTheDocument();
    });

    it('handles unicode characters in values', () => {
      render(<TemplateVariableDisplay text="{{.UNICODE}}" envData="UNICODE=你好世界🌍" />);

      expect(screen.getByText('你好世界🌍')).toBeInTheDocument();
    });

    it('handles multiline env data', () => {
      const envData = 'VAR1=value1\r\nVAR2=value2\rVAR3=value3\n';
      render(<TemplateVariableDisplay text="{{.VAR2}}" envData={envData} />);

      expect(screen.getByText('value2')).toBeInTheDocument();
    });
  });
});
