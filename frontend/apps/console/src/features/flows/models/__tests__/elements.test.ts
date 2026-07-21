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

import {describe, it, expect} from 'vitest';
import {
  ElementCategories,
  ElementTypes,
  BlockTypes,
  InputVariants,
  ButtonVariants,
  ButtonTypes,
  TypographyVariants,
  DividerVariants,
  ActionEventTypes,
} from '../elements';

describe('elements models', () => {
  describe('ElementCategories', () => {
    it('should have Action category', () => {
      expect(ElementCategories.Action).toBe('ACTION');
    });

    it('should have Block category', () => {
      expect(ElementCategories.Block).toBe('BLOCK');
    });

    it('should have Display category', () => {
      expect(ElementCategories.Display).toBe('DISPLAY');
    });

    it('should have Field category', () => {
      expect(ElementCategories.Field).toBe('FIELD');
    });

    it('should have Miscellaneous category', () => {
      expect(ElementCategories.Miscellaneous).toBe('MISCELLANEOUS');
    });

    it('should have exactly 5 categories', () => {
      expect(Object.keys(ElementCategories)).toHaveLength(5);
    });
  });

  describe('ElementTypes', () => {
    it('should have all input types', () => {
      expect(ElementTypes.TextInput).toBe('TEXT_INPUT');
      expect(ElementTypes.PasswordInput).toBe('PASSWORD_INPUT');
      expect(ElementTypes.EmailInput).toBe('EMAIL_INPUT');
      expect(ElementTypes.PhoneInput).toBe('PHONE_INPUT');
      expect(ElementTypes.NumberInput).toBe('NUMBER_INPUT');
      expect(ElementTypes.DateInput).toBe('DATE_INPUT');
      expect(ElementTypes.OtpInput).toBe('OTP_INPUT');
      expect(ElementTypes.Checkbox).toBe('CHECKBOX');
      expect(ElementTypes.Dropdown).toBe('DROPDOWN');
    });

    it('should have display types', () => {
      expect(ElementTypes.Action).toBe('ACTION');
      expect(ElementTypes.Captcha).toBe('CAPTCHA');
      expect(ElementTypes.Divider).toBe('DIVIDER');
      expect(ElementTypes.Icon).toBe('ICON');
      expect(ElementTypes.Image).toBe('IMAGE');
      expect(ElementTypes.RichText).toBe('RICH_TEXT');
      expect(ElementTypes.Stack).toBe('STACK');
      expect(ElementTypes.Text).toBe('TEXT');
      expect(ElementTypes.DynamicInputPlaceholder).toBe('DYNAMIC_INPUT_PLACEHOLDER');
      expect(ElementTypes.Resend).toBe('RESEND');
      expect(ElementTypes.Timer).toBe('TIMER');
    });

    it('should have consent types', () => {
      expect(ElementTypes.Consent).toBe('CONSENT');
      expect(ElementTypes.ConsentInput).toBe('CONSENT_INPUT');
    });

    it('should have Custom type', () => {
      expect(ElementTypes.Custom).toBe('CUSTOM');
    });

    it('should have exactly 24 element types', () => {
      expect(Object.keys(ElementTypes)).toHaveLength(24);
    });
  });

  describe('BlockTypes', () => {
    it('should have Form block type', () => {
      expect(BlockTypes.Form).toBe('BLOCK');
    });

    it('should have exactly 1 block type', () => {
      expect(Object.keys(BlockTypes)).toHaveLength(1);
    });
  });

  describe('InputVariants', () => {
    it('should have all input variants', () => {
      expect(InputVariants.Text).toBe('TEXT');
      expect(InputVariants.Password).toBe('PASSWORD');
      expect(InputVariants.Email).toBe('EMAIL');
      expect(InputVariants.Telephone).toBe('TELEPHONE');
      expect(InputVariants.Number).toBe('NUMBER');
      expect(InputVariants.Checkbox).toBe('CHECKBOX');
      expect(InputVariants.OTP).toBe('OTP');
    });

    it('should have exactly 7 input variants', () => {
      expect(Object.keys(InputVariants)).toHaveLength(7);
    });
  });

  describe('ButtonVariants', () => {
    it('should have Primary variant', () => {
      expect(ButtonVariants.Primary).toBe('PRIMARY');
    });

    it('should have Secondary variant', () => {
      expect(ButtonVariants.Secondary).toBe('SECONDARY');
    });

    it('should have Outlined variant', () => {
      expect(ButtonVariants.Outlined).toBe('OUTLINED');
    });

    it('should have Text variant', () => {
      expect(ButtonVariants.Text).toBe('TEXT');
    });

    it('should have exactly 4 button variants', () => {
      expect(Object.keys(ButtonVariants)).toHaveLength(4);
    });
  });

  describe('ButtonTypes', () => {
    it('should have Submit type', () => {
      expect(ButtonTypes.Submit).toBe('submit');
    });

    it('should have Button type', () => {
      expect(ButtonTypes.Button).toBe('button');
    });

    it('should have exactly 2 button types', () => {
      expect(Object.keys(ButtonTypes)).toHaveLength(2);
    });
  });

  describe('TypographyVariants', () => {
    it('should have all heading variants', () => {
      expect(TypographyVariants.H1).toBe('HEADING_1');
      expect(TypographyVariants.H2).toBe('HEADING_2');
      expect(TypographyVariants.H3).toBe('HEADING_3');
      expect(TypographyVariants.H4).toBe('HEADING_4');
      expect(TypographyVariants.H5).toBe('HEADING_5');
      expect(TypographyVariants.H6).toBe('HEADING_6');
    });

    it('should have body variants', () => {
      expect(TypographyVariants.Body1).toBe('BODY_1');
      expect(TypographyVariants.Body2).toBe('BODY_2');
    });

    it('should have exactly 8 typography variants', () => {
      expect(Object.keys(TypographyVariants)).toHaveLength(8);
    });
  });

  describe('DividerVariants', () => {
    it('should have Horizontal variant', () => {
      expect(DividerVariants.Horizontal).toBe('HORIZONTAL');
    });

    it('should have Vertical variant', () => {
      expect(DividerVariants.Vertical).toBe('VERTICAL');
    });

    it('should have exactly 2 divider variants', () => {
      expect(Object.keys(DividerVariants)).toHaveLength(2);
    });
  });

  describe('ActionEventTypes', () => {
    it('should have Trigger event type', () => {
      expect(ActionEventTypes.Trigger).toBe('TRIGGER');
    });

    it('should have Submit event type', () => {
      expect(ActionEventTypes.Submit).toBe('SUBMIT');
    });

    it('should have Navigate event type', () => {
      expect(ActionEventTypes.Navigate).toBe('NAVIGATE');
    });

    it('should have Cancel event type', () => {
      expect(ActionEventTypes.Cancel).toBe('CANCEL');
    });

    it('should have Reset event type', () => {
      expect(ActionEventTypes.Reset).toBe('RESET');
    });

    it('should have Back event type', () => {
      expect(ActionEventTypes.Back).toBe('BACK');
    });

    it('should have exactly 6 action event types', () => {
      expect(Object.keys(ActionEventTypes)).toHaveLength(6);
    });
  });
});
