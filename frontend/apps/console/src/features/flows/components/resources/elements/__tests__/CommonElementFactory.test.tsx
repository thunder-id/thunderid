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

import {render, screen} from '@testing-library/react';
import {describe, it, expect, vi} from 'vitest';
import {BlockTypes, ElementCategories, ElementTypes, type Element} from '../../../../models/elements';
import CommonElementFactory from '../CommonElementFactory';

// Mock all adapter components
vi.mock('../adapters/FormAdapter', () => ({
  default: ({stepId, resource}: {stepId: string; resource: Element}) => (
    <div data-testid="form-adapter" data-step-id={stepId} data-resource-id={resource.id}>
      Form Adapter
    </div>
  ),
}));

vi.mock('../adapters/BlockAdapter', () => ({
  default: ({resource}: {resource: Element}) => (
    <div data-testid="block-adapter" data-resource-id={resource.id}>
      Block Adapter
    </div>
  ),
}));

vi.mock('../adapters/input/CheckboxAdapter', () => ({
  default: ({resource}: {resource: Element}) => (
    <div data-testid="checkbox-adapter" data-resource-id={resource.id}>
      Checkbox Adapter
    </div>
  ),
}));

vi.mock('../adapters/input/PhoneNumberInputAdapter', () => ({
  default: ({resource}: {resource: Element}) => (
    <div data-testid="phone-input-adapter" data-resource-id={resource.id}>
      Phone Input Adapter
    </div>
  ),
}));

vi.mock('../adapters/input/OTPInputAdapter', () => ({
  default: ({resource}: {resource: Element}) => (
    <div data-testid="otp-input-adapter" data-resource-id={resource.id}>
      OTP Input Adapter
    </div>
  ),
}));

vi.mock('../adapters/input/DefaultInputAdapter', () => ({
  default: ({resource}: {resource: Element}) => (
    <div data-testid="default-input-adapter" data-resource-id={resource.id} data-type={resource.type}>
      Default Input Adapter
    </div>
  ),
}));

vi.mock('../adapters/ChoiceAdapter', () => ({
  default: ({resource}: {resource: Element}) => (
    <div data-testid="choice-adapter" data-resource-id={resource.id}>
      Choice Adapter
    </div>
  ),
}));

vi.mock('../adapters/ButtonAdapter', () => ({
  default: ({resource, elementIndex}: {resource: Element; elementIndex?: number}) => (
    <div data-testid="button-adapter" data-resource-id={resource.id} data-element-index={elementIndex}>
      Button Adapter
    </div>
  ),
}));

vi.mock('../adapters/TypographyAdapter', () => ({
  default: ({stepId, resource}: {stepId: string; resource: Element}) => (
    <div data-testid="typography-adapter" data-step-id={stepId} data-resource-id={resource.id}>
      Typography Adapter
    </div>
  ),
}));

vi.mock('../adapters/DividerAdapter', () => ({
  default: ({resource}: {resource: Element}) => (
    <div data-testid="divider-adapter" data-resource-id={resource.id}>
      Divider Adapter
    </div>
  ),
}));

vi.mock('../adapters/RichTextAdapter', () => ({
  default: ({resource}: {resource: Element}) => (
    <div data-testid="rich-text-adapter" data-resource-id={resource.id}>
      Rich Text Adapter
    </div>
  ),
}));

vi.mock('../adapters/ImageAdapter', () => ({
  default: ({resource}: {resource: Element}) => (
    <div data-testid="image-adapter" data-resource-id={resource.id}>
      Image Adapter
    </div>
  ),
}));

vi.mock('../adapters/CaptchaAdapter', () => ({
  default: ({resource}: {resource: Element}) => (
    <div data-testid="captcha-adapter" data-resource-id={resource.id}>
      Captcha Adapter
    </div>
  ),
}));

vi.mock('../adapters/ResendButtonAdapter', () => ({
  default: ({stepId, resource}: {stepId: string; resource: Element}) => (
    <div data-testid="resend-button-adapter" data-step-id={stepId} data-resource-id={resource.id}>
      Resend Button Adapter
    </div>
  ),
}));

vi.mock('../adapters/IconAdapter', () => ({
  default: ({resource}: {resource: Element}) => (
    <div data-testid="icon-adapter" data-resource-id={resource.id}>
      Icon Adapter
    </div>
  ),
}));

vi.mock('../adapters/StackAdapter', () => ({
  default: ({stepId, resource}: {stepId: string; resource: Element}) => (
    <div data-testid="stack-adapter" data-step-id={stepId} data-resource-id={resource.id}>
      Stack Adapter
    </div>
  ),
}));

vi.mock('../adapters/TimerAdapter', () => ({
  default: ({resource}: {resource: Element}) => (
    <div data-testid="timer-adapter" data-resource-id={resource.id}>
      Timer Adapter
    </div>
  ),
}));

vi.mock('../adapters/CustomAdapter', () => ({
  default: ({resource}: {resource: Element}) => (
    <div data-testid="custom-adapter" data-resource-id={resource.id}>
      Custom Adapter
    </div>
  ),
}));

vi.mock('../adapters/DynamicInputPlaceholderAdapter', () => ({
  default: ({resource}: {resource: Element}) => (
    <div data-testid="dynamic-input-placeholder-adapter" data-resource-id={resource.id}>
      Dynamic Input Placeholder Adapter
    </div>
  ),
}));

describe('CommonElementFactory', () => {
  const createMockElement = (overrides: Partial<Element> = {}): Element =>
    ({
      id: 'element-1',
      type: ElementTypes.TextInput,
      category: ElementCategories.Field,
      config: {},
      ...overrides,
    }) as Element;

  describe('Form Block', () => {
    it('should render FormAdapter for Form block with BLOCK category', () => {
      const formElement = createMockElement({
        type: BlockTypes.Form,
        category: ElementCategories.Block,
      });

      render(<CommonElementFactory stepId="step-1" resource={formElement} />);

      expect(screen.getByTestId('form-adapter')).toBeInTheDocument();
      expect(screen.getByTestId('form-adapter')).toHaveAttribute('data-step-id', 'step-1');
      expect(screen.getByTestId('form-adapter')).toHaveAttribute('data-resource-id', 'element-1');
    });

    it('should render BlockAdapter for Form block with non-BLOCK category', () => {
      const actionBlock = createMockElement({
        type: BlockTypes.Form,
        category: ElementCategories.Action,
      });

      render(<CommonElementFactory stepId="step-1" resource={actionBlock} />);

      expect(screen.getByTestId('block-adapter')).toBeInTheDocument();
    });

    it('should pass availableElements and onAddElementToForm to FormAdapter', () => {
      const formElement = createMockElement({
        type: BlockTypes.Form,
        category: ElementCategories.Block,
      });

      const availableElements = [createMockElement({id: 'available-1'})];
      const onAddElementToForm = vi.fn();

      render(
        <CommonElementFactory
          stepId="step-1"
          resource={formElement}
          availableElements={availableElements}
          onAddElementToForm={onAddElementToForm}
        />,
      );

      expect(screen.getByTestId('form-adapter')).toBeInTheDocument();
    });
  });

  describe('Checkbox Element', () => {
    it('should render CheckboxAdapter for Checkbox type', () => {
      const checkboxElement = createMockElement({
        type: ElementTypes.Checkbox,
      });

      render(<CommonElementFactory stepId="step-1" resource={checkboxElement} />);

      expect(screen.getByTestId('checkbox-adapter')).toBeInTheDocument();
    });
  });

  describe('Phone Input Element', () => {
    it('should render PhoneNumberInputAdapter for PhoneInput type', () => {
      const phoneElement = createMockElement({
        type: ElementTypes.PhoneInput,
      });

      render(<CommonElementFactory stepId="step-1" resource={phoneElement} />);

      expect(screen.getByTestId('phone-input-adapter')).toBeInTheDocument();
    });
  });

  describe('OTP Input Element', () => {
    it('should render OTPInputAdapter for OtpInput type', () => {
      const otpElement = createMockElement({
        type: ElementTypes.OtpInput,
      });

      render(<CommonElementFactory stepId="step-1" resource={otpElement} />);

      expect(screen.getByTestId('otp-input-adapter')).toBeInTheDocument();
    });
  });

  describe('Default Input Elements', () => {
    it('should render DefaultInputAdapter for TextInput type', () => {
      const textInputElement = createMockElement({
        type: ElementTypes.TextInput,
      });

      render(<CommonElementFactory stepId="step-1" resource={textInputElement} />);

      expect(screen.getByTestId('default-input-adapter')).toBeInTheDocument();
      expect(screen.getByTestId('default-input-adapter')).toHaveAttribute('data-type', ElementTypes.TextInput);
    });

    it('should render DefaultInputAdapter for PasswordInput type', () => {
      const passwordElement = createMockElement({
        type: ElementTypes.PasswordInput,
      });

      render(<CommonElementFactory stepId="step-1" resource={passwordElement} />);

      expect(screen.getByTestId('default-input-adapter')).toBeInTheDocument();
      expect(screen.getByTestId('default-input-adapter')).toHaveAttribute('data-type', ElementTypes.PasswordInput);
    });

    it('should render DefaultInputAdapter for EmailInput type', () => {
      const emailElement = createMockElement({
        type: ElementTypes.EmailInput,
      });

      render(<CommonElementFactory stepId="step-1" resource={emailElement} />);

      expect(screen.getByTestId('default-input-adapter')).toBeInTheDocument();
      expect(screen.getByTestId('default-input-adapter')).toHaveAttribute('data-type', ElementTypes.EmailInput);
    });

    it('should render DefaultInputAdapter for NumberInput type', () => {
      const numberElement = createMockElement({
        type: ElementTypes.NumberInput,
      });

      render(<CommonElementFactory stepId="step-1" resource={numberElement} />);

      expect(screen.getByTestId('default-input-adapter')).toBeInTheDocument();
      expect(screen.getByTestId('default-input-adapter')).toHaveAttribute('data-type', ElementTypes.NumberInput);
    });

    it('should render DefaultInputAdapter for DateInput type', () => {
      const dateElement = createMockElement({
        type: ElementTypes.DateInput,
      });

      render(<CommonElementFactory stepId="step-1" resource={dateElement} />);

      expect(screen.getByTestId('default-input-adapter')).toBeInTheDocument();
      expect(screen.getByTestId('default-input-adapter')).toHaveAttribute('data-type', ElementTypes.DateInput);
    });
  });

  describe('Dropdown Element', () => {
    it('should render ChoiceAdapter for Dropdown type', () => {
      const dropdownElement = createMockElement({
        type: ElementTypes.Dropdown,
      });

      render(<CommonElementFactory stepId="step-1" resource={dropdownElement} />);

      expect(screen.getByTestId('choice-adapter')).toBeInTheDocument();
    });
  });

  describe('Action Element', () => {
    it('should render ButtonAdapter for Action type', () => {
      const actionElement = createMockElement({
        type: ElementTypes.Action,
      });

      render(<CommonElementFactory stepId="step-1" resource={actionElement} />);

      expect(screen.getByTestId('button-adapter')).toBeInTheDocument();
    });

    it('should pass elementIndex to ButtonAdapter', () => {
      const actionElement = createMockElement({
        type: ElementTypes.Action,
      });

      render(<CommonElementFactory stepId="step-1" resource={actionElement} elementIndex={5} />);

      expect(screen.getByTestId('button-adapter')).toHaveAttribute('data-element-index', '5');
    });
  });

  describe('Text Element', () => {
    it('should render TypographyAdapter for Text type', () => {
      const textElement = createMockElement({
        type: ElementTypes.Text,
      });

      render(<CommonElementFactory stepId="step-1" resource={textElement} />);

      expect(screen.getByTestId('typography-adapter')).toBeInTheDocument();
      expect(screen.getByTestId('typography-adapter')).toHaveAttribute('data-step-id', 'step-1');
    });
  });

  describe('RichText Element', () => {
    it('should render RichTextAdapter for RichText type', () => {
      const richTextElement = createMockElement({
        type: ElementTypes.RichText,
      });

      render(<CommonElementFactory stepId="step-1" resource={richTextElement} />);

      expect(screen.getByTestId('rich-text-adapter')).toBeInTheDocument();
    });
  });

  describe('Divider Element', () => {
    it('should render DividerAdapter for Divider type', () => {
      const dividerElement = createMockElement({
        type: ElementTypes.Divider,
      });

      render(<CommonElementFactory stepId="step-1" resource={dividerElement} />);

      expect(screen.getByTestId('divider-adapter')).toBeInTheDocument();
    });
  });

  describe('Image Element', () => {
    it('should render ImageAdapter for Image type', () => {
      const imageElement = createMockElement({
        type: ElementTypes.Image,
      });

      render(<CommonElementFactory stepId="step-1" resource={imageElement} />);

      expect(screen.getByTestId('image-adapter')).toBeInTheDocument();
    });
  });

  describe('Captcha Element', () => {
    it('should render CaptchaAdapter for Captcha type', () => {
      const captchaElement = createMockElement({
        type: ElementTypes.Captcha,
      });

      render(<CommonElementFactory stepId="step-1" resource={captchaElement} />);

      expect(screen.getByTestId('captcha-adapter')).toBeInTheDocument();
    });
  });

  describe('Resend Element', () => {
    it('should render ResendButtonAdapter for Resend type', () => {
      const resendElement = createMockElement({
        type: ElementTypes.Resend,
      });

      render(<CommonElementFactory stepId="step-1" resource={resendElement} />);

      expect(screen.getByTestId('resend-button-adapter')).toBeInTheDocument();
      expect(screen.getByTestId('resend-button-adapter')).toHaveAttribute('data-step-id', 'step-1');
    });
  });

  describe('Icon Element', () => {
    it('should render IconAdapter for Icon type', () => {
      const iconElement = createMockElement({
        type: ElementTypes.Icon,
      });

      render(<CommonElementFactory stepId="step-1" resource={iconElement} />);

      expect(screen.getByTestId('icon-adapter')).toBeInTheDocument();
      expect(screen.getByTestId('icon-adapter')).toHaveAttribute('data-resource-id', 'element-1');
    });
  });

  describe('Stack Element', () => {
    it('should render StackAdapter for Stack type', () => {
      const stackElement = createMockElement({
        type: ElementTypes.Stack,
      });

      render(<CommonElementFactory stepId="step-1" resource={stackElement} />);

      expect(screen.getByTestId('stack-adapter')).toBeInTheDocument();
      expect(screen.getByTestId('stack-adapter')).toHaveAttribute('data-step-id', 'step-1');
      expect(screen.getByTestId('stack-adapter')).toHaveAttribute('data-resource-id', 'element-1');
    });
  });

  describe('Timer Element', () => {
    it('should render TimerAdapter for Timer type', () => {
      const timerElement = createMockElement({
        type: ElementTypes.Timer,
      });

      render(<CommonElementFactory stepId="step-1" resource={timerElement} />);

      expect(screen.getByTestId('timer-adapter')).toBeInTheDocument();
      expect(screen.getByTestId('timer-adapter')).toHaveAttribute('data-resource-id', 'element-1');
    });
  });

  describe('Custom Element', () => {
    it('should render CustomAdapter for Custom type', () => {
      const customElement = createMockElement({
        type: ElementTypes.Custom,
      });

      render(<CommonElementFactory stepId="step-1" resource={customElement} />);

      expect(screen.getByTestId('custom-adapter')).toBeInTheDocument();
      expect(screen.getByTestId('custom-adapter')).toHaveAttribute('data-resource-id', 'element-1');
    });
  });

  describe('Dynamic Input Placeholder Element', () => {
    it('should render DynamicInputPlaceholderAdapter for DynamicInputPlaceholder type', () => {
      const placeholderElement = createMockElement({
        type: ElementTypes.DynamicInputPlaceholder,
      });

      render(<CommonElementFactory stepId="step-1" resource={placeholderElement} />);

      expect(screen.getByTestId('dynamic-input-placeholder-adapter')).toBeInTheDocument();
      expect(screen.getByTestId('dynamic-input-placeholder-adapter')).toHaveAttribute('data-resource-id', 'element-1');
    });
  });

  describe('Unknown Element Type', () => {
    it('should return null for unknown element type', () => {
      const unknownElement = createMockElement({
        type: 'UNKNOWN_TYPE' as (typeof ElementTypes)[keyof typeof ElementTypes],
      });

      const {container} = render(<CommonElementFactory stepId="step-1" resource={unknownElement} />);

      expect(container.firstChild).toBeNull();
    });
  });

  describe('Default Props', () => {
    it('should work with undefined elementIndex', () => {
      const actionElement = createMockElement({
        type: ElementTypes.Action,
      });

      render(<CommonElementFactory stepId="step-1" resource={actionElement} />);

      expect(screen.getByTestId('button-adapter')).toBeInTheDocument();
    });

    it('should work with undefined availableElements', () => {
      const formElement = createMockElement({
        type: BlockTypes.Form,
        category: ElementCategories.Block,
      });

      render(<CommonElementFactory stepId="step-1" resource={formElement} />);

      expect(screen.getByTestId('form-adapter')).toBeInTheDocument();
    });

    it('should work with undefined onAddElementToForm', () => {
      const formElement = createMockElement({
        type: BlockTypes.Form,
        category: ElementCategories.Block,
      });

      render(<CommonElementFactory stepId="step-1" resource={formElement} />);

      expect(screen.getByTestId('form-adapter')).toBeInTheDocument();
    });
  });
});
