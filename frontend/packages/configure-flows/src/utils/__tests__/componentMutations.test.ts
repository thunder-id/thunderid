// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import FlowConstants from '../../constants/FlowConstants';
import {
  BlockTypes,
  ButtonTypes,
  ButtonVariants,
  ElementCategories,
  ElementTypes,
  type Element,
} from '../../models/elements';
import {INPUT_ELEMENT_TYPES, processFormComponents, mutateComponents} from '../componentMutations';

const createMockElement = (overrides: Partial<Element> = {}): Element =>
  ({
    id: 'element-1',
    type: ElementTypes.Action,
    category: ElementCategories.Action,
    version: '1.0.0',
    deprecated: false,
    deletable: true,
    resourceType: 'ELEMENT',
    display: {
      label: 'Element Label',
      image: '',
      showOnResourcePanel: true,
    },
    config: {},
    ...overrides,
  }) as Element;

describe('componentMutations', () => {
  describe('INPUT_ELEMENT_TYPES', () => {
    it('should contain all input element types', () => {
      expect(INPUT_ELEMENT_TYPES.has(ElementTypes.TextInput)).toBe(true);
      expect(INPUT_ELEMENT_TYPES.has(ElementTypes.PasswordInput)).toBe(true);
      expect(INPUT_ELEMENT_TYPES.has(ElementTypes.EmailInput)).toBe(true);
      expect(INPUT_ELEMENT_TYPES.has(ElementTypes.PhoneInput)).toBe(true);
      expect(INPUT_ELEMENT_TYPES.has(ElementTypes.NumberInput)).toBe(true);
      expect(INPUT_ELEMENT_TYPES.has(ElementTypes.DateInput)).toBe(true);
      expect(INPUT_ELEMENT_TYPES.has(ElementTypes.OtpInput)).toBe(true);
      expect(INPUT_ELEMENT_TYPES.has(ElementTypes.Checkbox)).toBe(true);
      expect(INPUT_ELEMENT_TYPES.has(ElementTypes.Dropdown)).toBe(true);
      expect(INPUT_ELEMENT_TYPES.has(ElementTypes.Select)).toBe(true);
    });

    it('should not contain non-input element types', () => {
      expect(INPUT_ELEMENT_TYPES.has(ElementTypes.Action)).toBe(false);
      expect(INPUT_ELEMENT_TYPES.has(ElementTypes.Divider)).toBe(false);
      expect(INPUT_ELEMENT_TYPES.has(ElementTypes.Image)).toBe(false);
      expect(INPUT_ELEMENT_TYPES.has(ElementTypes.Text)).toBe(false);
    });
  });

  describe('processFormComponents', () => {
    it('should return undefined for undefined input', () => {
      const result = processFormComponents(undefined);
      expect(result).toBeUndefined();
    });

    it('should return empty array for empty array input', () => {
      const result = processFormComponents([]);
      expect(result).toEqual([]);
    });

    it('should detect password field and set hasPasswordField', () => {
      const components: Element[] = [
        createMockElement({id: 'password-1', type: ElementTypes.PasswordInput}),
        createMockElement({
          id: 'button-1',
          type: ElementTypes.Action,
          variant: ButtonVariants.Primary,
        }),
      ];

      const result = processFormComponents(components);

      expect(result).toBeDefined();
      // Should assign PASSWORD_PROVISIONING executor
      const button = result?.find((c) => c.id === 'button-1');
      expect((button?.action?.['executor'] as {name: string})?.name).toBe(
        FlowConstants.ExecutorNames.PASSWORD_PROVISIONING,
      );
    });

    it('should detect OTP field and set hasOtpField', () => {
      const components: Element[] = [
        createMockElement({id: 'otp-1', type: ElementTypes.OtpInput}),
        createMockElement({
          id: 'button-1',
          type: ElementTypes.Action,
          variant: ButtonVariants.Primary,
        }),
      ];

      const result = processFormComponents(components);

      expect(result).toBeDefined();
      // Should assign EMAIL_OTP executor
      const button = result?.find((c) => c.id === 'button-1');
      expect((button?.action?.['executor'] as {name: string})?.name).toBe(FlowConstants.ExecutorNames.EMAIL_OTP);
    });

    it('should set PRIMARY buttons to submit type', () => {
      const components: Element[] = [
        createMockElement({
          id: 'button-1',
          type: ElementTypes.Action,
          variant: ButtonVariants.Primary,
          config: {} as Element['config'],
        }),
      ];

      const result = processFormComponents(components);

      expect(result).toBeDefined();
      const button = result?.find((c) => c.id === 'button-1');
      expect((button?.config as {type?: string})?.type).toBe(ButtonTypes.Submit);
    });

    it('should count existing submit buttons', () => {
      const components: Element[] = [
        createMockElement({id: 'password-1', type: ElementTypes.PasswordInput}),
        createMockElement({
          id: 'button-1',
          type: ElementTypes.Action,
          config: {type: ButtonTypes.Submit} as unknown as Element['config'],
        }),
      ];

      const result = processFormComponents(components);

      expect(result).toBeDefined();
      // With one submit button and password field, executor should be assigned
      const button = result?.find((c) => c.id === 'button-1');
      expect((button?.action?.['executor'] as {name: string})?.name).toBe(
        FlowConstants.ExecutorNames.PASSWORD_PROVISIONING,
      );
    });

    it('should not assign executor when multiple submit buttons exist', () => {
      const components: Element[] = [
        createMockElement({id: 'password-1', type: ElementTypes.PasswordInput}),
        createMockElement({
          id: 'button-1',
          type: ElementTypes.Action,
          variant: ButtonVariants.Primary,
        }),
        createMockElement({
          id: 'button-2',
          type: ElementTypes.Action,
          variant: ButtonVariants.Primary,
        }),
      ];

      const result = processFormComponents(components);

      expect(result).toBeDefined();
      // With two submit buttons, no executor should be assigned
      const button1 = result?.find((c) => c.id === 'button-1');
      const button2 = result?.find((c) => c.id === 'button-2');
      expect(button1?.action?.['executor']).toBeUndefined();
      expect(button2?.action?.['executor']).toBeUndefined();
    });

    it('should not assign executor when no password or OTP field exists', () => {
      const components: Element[] = [
        createMockElement({id: 'text-1', type: ElementTypes.TextInput}),
        createMockElement({
          id: 'button-1',
          type: ElementTypes.Action,
          variant: ButtonVariants.Primary,
        }),
      ];

      const result = processFormComponents(components);

      expect(result).toBeDefined();
      const button = result?.find((c) => c.id === 'button-1');
      expect(button?.action?.['executor']).toBeUndefined();
    });

    it('should preserve existing action properties when assigning executor', () => {
      const components: Element[] = [
        createMockElement({id: 'password-1', type: ElementTypes.PasswordInput}),
        createMockElement({
          id: 'button-1',
          type: ElementTypes.Action,
          variant: ButtonVariants.Primary,
          action: {onSuccess: 'next-step', customProp: 'value'},
        }),
      ];

      const result = processFormComponents(components);

      expect(result).toBeDefined();
      const button = result?.find((c) => c.id === 'button-1');
      expect(button?.action?.onSuccess).toBe('next-step');
      expect(button?.action?.['customProp']).toBe('value');
      expect((button?.action?.['executor'] as {name: string})?.name).toBe(
        FlowConstants.ExecutorNames.PASSWORD_PROVISIONING,
      );
    });

    it('should prefer password executor over OTP when both fields exist', () => {
      const components: Element[] = [
        createMockElement({id: 'password-1', type: ElementTypes.PasswordInput}),
        createMockElement({id: 'otp-1', type: ElementTypes.OtpInput}),
        createMockElement({
          id: 'button-1',
          type: ElementTypes.Action,
          variant: ButtonVariants.Primary,
        }),
      ];

      const result = processFormComponents(components);

      expect(result).toBeDefined();
      const button = result?.find((c) => c.id === 'button-1');
      // Password takes precedence
      expect((button?.action?.['executor'] as {name: string})?.name).toBe(
        FlowConstants.ExecutorNames.PASSWORD_PROVISIONING,
      );
    });

    it('should return components unchanged when no special processing needed', () => {
      const components: Element[] = [
        createMockElement({id: 'text-1', type: ElementTypes.Text}),
        createMockElement({id: 'divider-1', type: ElementTypes.Divider}),
      ];

      const result = processFormComponents(components);

      expect(result).toHaveLength(2);
      expect(result?.[0].id).toBe('text-1');
      expect(result?.[1].id).toBe('divider-1');
    });
  });

  describe('mutateComponents', () => {
    it('should filter out non-ELEMENT resources', () => {
      const components: Element[] = [
        createMockElement({id: 'element-1', resourceType: 'ELEMENT'}),
        createMockElement({id: 'step-1', resourceType: 'STEP'}),
        createMockElement({id: 'template-1', resourceType: 'TEMPLATE'}),
      ];

      const result = mutateComponents(components);

      expect(result).toHaveLength(1);
      expect(result[0].id).toBe('element-1');
    });

    it('should keep only the first BLOCK form', () => {
      const components: Element[] = [
        createMockElement({
          id: 'form-1',
          type: BlockTypes.Form,
          category: ElementCategories.Block,
        }),
        createMockElement({
          id: 'form-2',
          type: BlockTypes.Form,
          category: ElementCategories.Block,
        }),
        createMockElement({
          id: 'form-3',
          type: BlockTypes.Form,
          category: ElementCategories.Block,
        }),
      ];

      const result = mutateComponents(components);

      expect(result).toHaveLength(1);
      expect(result[0].id).toBe('form-1');
    });

    it('should keep social login blocks (category: ACTION, type: BLOCK)', () => {
      const components: Element[] = [
        createMockElement({
          id: 'form-1',
          type: BlockTypes.Form,
          category: ElementCategories.Block,
        }),
        createMockElement({
          id: 'social-block-1',
          type: BlockTypes.Form,
          category: ElementCategories.Action,
        }),
        createMockElement({
          id: 'social-block-2',
          type: BlockTypes.Form,
          category: ElementCategories.Action,
        }),
      ];

      const result = mutateComponents(components);

      // Should keep form-1 and both social blocks
      expect(result).toHaveLength(3);
      expect(result.map((c) => c.id)).toContain('form-1');
      expect(result.map((c) => c.id)).toContain('social-block-1');
      expect(result.map((c) => c.id)).toContain('social-block-2');
    });

    it('should process form components', () => {
      const components: Element[] = [
        createMockElement({
          id: 'form-1',
          type: BlockTypes.Form,
          category: ElementCategories.Block,
          components: [
            createMockElement({id: 'password-1', type: ElementTypes.PasswordInput}),
            createMockElement({
              id: 'button-1',
              type: ElementTypes.Action,
              variant: ButtonVariants.Primary,
            }),
          ],
        }),
      ];

      const result = mutateComponents(components);

      expect(result).toHaveLength(1);
      const form = result[0];
      expect(form.components).toBeDefined();
      const button = form.components?.find((c) => c.id === 'button-1');
      expect((button?.action?.['executor'] as {name: string})?.name).toBe(
        FlowConstants.ExecutorNames.PASSWORD_PROVISIONING,
      );
    });

    it('should not modify non-form components', () => {
      const components: Element[] = [
        createMockElement({id: 'text-1', type: ElementTypes.Text}),
        createMockElement({id: 'button-1', type: ElementTypes.Action}),
      ];

      const result = mutateComponents(components);

      expect(result).toHaveLength(2);
      expect(result[0].id).toBe('text-1');
      expect(result[1].id).toBe('button-1');
    });

    it('should clone components to avoid mutation', () => {
      const originalComponents: Element[] = [createMockElement({id: 'element-1'})];

      const result = mutateComponents(originalComponents);

      // Modify result
      result[0].id = 'modified-id';

      // Original should not be affected
      expect(originalComponents[0].id).toBe('element-1');
    });

    it('should handle components without resourceType', () => {
      const components: Element[] = [
        {
          id: 'no-resource-type',
          type: ElementTypes.Text,
          category: ElementCategories.Display,
          version: '1.0.0',
          deprecated: false,
          display: {label: 'Text', image: ''},
          config: {},
        } as Element,
      ];

      const result = mutateComponents(components);

      expect(result).toHaveLength(1);
      expect(result[0].id).toBe('no-resource-type');
    });

    it('should handle empty components array', () => {
      const result = mutateComponents([]);

      expect(result).toEqual([]);
    });
  });
});
