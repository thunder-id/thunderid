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

import {cx} from '@emotion/css';
import {withVendorCSSClassPrefix, bem} from '@thunderid/browser';
import {
  Context,
  forwardRef,
  HTMLAttributes,
  ReactNode,
  Ref,
  RefAttributes,
  ForwardRefExoticComponent,
  createContext,
  useContext,
  FC,
  SVGProps,
} from 'react';
import useStyles from './Alert.styles';
import useTheme from '../../../contexts/Theme/useTheme';
import CircleAlert from '../Icons/CircleAlert';
import CircleCheck from '../Icons/CircleCheck';
import Info from '../Icons/Info';
import TriangleAlert from '../Icons/TriangleAlert';
import Typography from '../Typography/Typography';

export type AlertVariant = 'success' | 'error' | 'warning' | 'info';

export interface AlertProps extends HTMLAttributes<HTMLDivElement> {
  /**
   * Alert content
   */
  children?: ReactNode;
  /**
   * Whether to show the default icon for the variant
   */
  showIcon?: boolean;
  /**
   * The visual variant of the alert that determines color scheme and icon
   */
  variant?: AlertVariant;
}

export interface AlertTitleProps extends HTMLAttributes<HTMLHeadingElement> {
  /**
   * Title content
   */
  children?: ReactNode;
}

export interface AlertDescriptionProps extends HTMLAttributes<HTMLParagraphElement> {
  /**
   * Description content
   */
  children?: ReactNode;
}

const getDefaultIcon = (variant: AlertVariant): FC<SVGProps<SVGSVGElement>> => {
  switch (variant) {
    case 'success':
      return CircleCheck;
    case 'error':
      return CircleAlert;
    case 'warning':
      return TriangleAlert;
    case 'info':
      return Info;
    default:
      return Info;
  }
};

const AlertVariantContext: Context<AlertVariant> = createContext<AlertVariant>('info');

export const useAlertVariant = (): AlertVariant => useContext(AlertVariantContext);

/**
 * Alert component that displays important information with different severity levels.
 *
 * @example
 * ```tsx
 * <Alert variant="success" showIcon>
 *   <Alert.Title>Success! Your changes have been saved</Alert.Title>
 *   <Alert.Description>
 *     This is an alert with icon, title and description.
 *   </Alert.Description>
 * </Alert>
 * ```
 */
const Alert: ForwardRefExoticComponent<AlertProps & RefAttributes<HTMLDivElement>> = forwardRef<
  HTMLDivElement,
  AlertProps
>(({variant = 'info', showIcon = true, children, className, style, ...rest}: AlertProps, ref: Ref<HTMLDivElement>) => {
  const {theme, colorScheme}: ReturnType<typeof useTheme> = useTheme();
  const styles: ReturnType<typeof useStyles> = useStyles(theme, colorScheme, variant);
  const IconComponent: FC<SVGProps<SVGSVGElement>> = getDefaultIcon(variant);

  return (
    <AlertVariantContext.Provider value={variant}>
      <div
        ref={ref}
        role="alert"
        style={style}
        className={cx(
          withVendorCSSClassPrefix(bem('alert')),
          styles['alert'],
          styles['variant'],
          withVendorCSSClassPrefix(bem('alert', null, variant)),
          className,
        )}
        {...rest}
      >
        {showIcon && (
          <div className={cx(withVendorCSSClassPrefix(bem('alert', 'icon')), styles['icon'])}>
            <IconComponent />
          </div>
        )}
        <div className={cx(withVendorCSSClassPrefix(bem('alert', 'content')), styles['content'])}>{children}</div>
      </div>
    </AlertVariantContext.Provider>
  );
});

/**
 * Alert title component.
 */
const AlertTitle: FC<AlertTitleProps> = ({children, className, style, ...rest}: AlertTitleProps) => {
  const {theme, colorScheme}: ReturnType<typeof useTheme> = useTheme();
  const variant: AlertVariant = useAlertVariant();
  const styles: ReturnType<typeof useStyles> = useStyles(theme, colorScheme, variant);

  const {color, ...filteredRest} = rest;

  return (
    <Typography
      component="h3"
      variant="h6"
      fontWeight={600}
      style={style}
      className={cx(withVendorCSSClassPrefix(bem('alert', 'title')), styles['title'], className)}
      {...filteredRest}
    >
      {children}
    </Typography>
  );
};

/**
 * Alert description component.
 */
const AlertDescription: FC<AlertDescriptionProps> = ({children, className, style, ...rest}: AlertDescriptionProps) => {
  const {theme, colorScheme}: ReturnType<typeof useTheme> = useTheme();
  const variant: AlertVariant = useAlertVariant();
  const styles: ReturnType<typeof useStyles> = useStyles(theme, colorScheme, variant);

  const {color, ...filteredRest} = rest;

  return (
    <Typography
      component="p"
      variant="body2"
      style={style}
      className={cx(withVendorCSSClassPrefix(bem('alert', 'description')), styles['description'], className)}
      {...filteredRest}
    >
      {children}
    </Typography>
  );
};

Alert.displayName = 'Alert';
AlertTitle.displayName = 'Alert.Title';
AlertDescription.displayName = 'Alert.Description';

(Alert as any).Title = AlertTitle;
(Alert as any).Description = AlertDescription;

export interface AlertComponent extends ForwardRefExoticComponent<AlertProps & RefAttributes<HTMLDivElement>> {
  Description: typeof AlertDescription;
  Title: typeof AlertTitle;
}

export default Alert as AlertComponent;
export {Alert, AlertTitle, AlertDescription};
