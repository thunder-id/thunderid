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

import {cx} from '@emotion/css';
import {type ConsentPurposeDataV2 as ConsentPurposeData, withVendorCSSClassPrefix, bem} from '@thunderid/browser';
import {type ChangeEvent, FC, ReactNode} from 'react';
import useStyles from './ConsentCheckboxList.styles';
import useTheme from '../../contexts/Theme/useTheme';
import Divider from '../primitives/Divider/Divider';
import Toggle from '../primitives/Toggle/Toggle';
import Typography from '../primitives/Typography/Typography';

/**
 * Computes the form value key for tracking an optional attribute's consent state.
 *
 * @param purposeId - The ID of the consent purpose.
 * @param attrName - The name of the attribute.
 * @returns A stable form key string.
 */
export const getConsentOptionalKey = (purposeId: string, attrName: string): string =>
  `__consent_opt__${purposeId}__${attrName}`;

/**
 * ConsentInputVariant defines whether the ConsentCheckboxList is rendering essential (read-only) or
 * optional (toggleable) attributes.
 */
export type ConsentInputVariant = 'ESSENTIAL' | 'OPTIONAL';

/**
 * Render props exposed by ConsentCheckboxList when using the render-prop pattern.
 */
export interface ConsentCheckboxListRenderProps {
  /** The list of attribute names to render. */
  attributes: string[];
  /**
   * Call this when an optional attribute checkbox is toggled.
   * No-op for ESSENTIAL attributes (they cannot be changed).
   */
  handleChange: (attrName: string, checked: boolean) => void;
  /**
   * Returns the current checked state for the given attribute name.
   * Always returns true for ESSENTIAL attributes.
   */
  isChecked: (attrName: string) => boolean;
  /** Whether the list is rendering essential or optional attributes. */
  variant: ConsentInputVariant;
}

/**
 * Props for the ConsentCheckboxList component.
 */
export interface ConsentCheckboxListProps {
  /**
   * Render-props callback. When provided, the default checkbox list UI is replaced
   * with whatever JSX the callback returns.
   *
   * @example
   * ```tsx
   * <ConsentCheckboxList variant="OPTIONAL" purpose={purpose} formValues={formValues} onInputChange={onChange}>
   *   {({ attributes, isChecked, handleChange }) => (
   *     <ul>
   *       {attributes.map(attr => (
   *         <li key={attr}>
   *           <Checkbox checked={isChecked(attr)} onChange={e => handleChange(attr, e.target.checked)} />
   *           {attr}
   *         </li>
   *       ))}
   *     </ul>
   *   )}
   * </ConsentCheckboxList>
   * ```
   */
  children?: (props: ConsentCheckboxListRenderProps) => ReactNode;
  /** Current form values - used to read optional checkbox state */
  formValues: Record<string, string>;
  /** Callback invoked when an optional attribute checkbox is toggled */
  onInputChange: (name: string, value: string) => void;
  /** The consent purpose data containing attribute lists */
  purpose: ConsentPurposeData;
  /** Whether to render essential (disabled) or optional (toggleable) attributes */
  variant: ConsentInputVariant;
}

/**
 * Renders a list of consent attribute checkboxes.
 *
 * - ESSENTIAL variant: renders read-only checked checkboxes for required attributes.
 * - OPTIONAL variant: renders toggleable checkboxes for optional attributes.
 *   Opt-in is the default when no prior form value exists.
 */
const ConsentCheckboxList: FC<ConsentCheckboxListProps> = ({
  variant,
  purpose,
  formValues,
  onInputChange,
  children,
}: ConsentCheckboxListProps) => {
  const {theme, colorScheme}: ReturnType<typeof useTheme> = useTheme();
  const styles: Record<string, string> = useStyles(theme, colorScheme);

  const attributes: string[] = (variant === 'ESSENTIAL' ? purpose.essential : purpose.optional).map(
    (e): string => e.name,
  );

  if (!attributes || attributes.length === 0) {
    return null;
  }

  const isEssential: boolean = variant === 'ESSENTIAL';

  const isChecked = (attrName: string): boolean => {
    if (isEssential) {
      return true;
    }
    const key: string = getConsentOptionalKey(purpose.purposeId, attrName);
    // Default to opted-in (true) when there's no explicit form value
    return formValues[key] !== 'false';
  };

  const handleChange = (attrName: string, checked: boolean): void => {
    const key: string = getConsentOptionalKey(purpose.purposeId, attrName);
    onInputChange(key, checked ? 'true' : 'false');
  };

  if (children) {
    return <>{children({attributes, handleChange, isChecked, variant})}</>;
  }

  return (
    <div className={cx(withVendorCSSClassPrefix(bem('consent-checkbox-list')), styles['listContainer'])}>
      {attributes.map((attr: string) => {
        const inputId = `consent_${isEssential ? 'ess' : 'opt'}_${purpose.purposeId}_${attr}`;
        const checked: boolean = isChecked(attr);

        return (
          <div
            key={attr}
            className={cx(withVendorCSSClassPrefix(bem('consent-checkbox-list', 'item')), styles['listItem'])}
          >
            <div className={cx(withVendorCSSClassPrefix(bem('consent-checkbox-list', 'row')), styles['listRow'])}>
              <div
                className={cx(
                  withVendorCSSClassPrefix(bem('consent-checkbox-list', 'label-container')),
                  styles['labelContainer'],
                )}
              >
                <div
                  className={cx(withVendorCSSClassPrefix(bem('consent-checkbox-list', 'bullet')), styles['bullet'])}
                />
                <Typography
                  variant="body2"
                  className={cx(
                    withVendorCSSClassPrefix(bem('consent-checkbox-list', 'typography')),
                    styles['typography'],
                  )}
                >
                  {attr}
                </Typography>
              </div>
              <Toggle
                id={inputId}
                checked={checked}
                disabled={isEssential}
                onChange={
                  isEssential
                    ? undefined
                    : (e: ChangeEvent<HTMLInputElement>): void => handleChange(attr, e.target.checked)
                }
              />
            </div>
            <Divider
              className={cx(withVendorCSSClassPrefix(bem('consent-checkbox-list', 'divider')), styles['divider'])}
            />
          </div>
        );
      })}
    </div>
  );
};

export default ConsentCheckboxList;
