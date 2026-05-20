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
  HTMLAttributes,
  forwardRef,
  ReactNode,
  ForwardRefExoticComponent,
  RefAttributes,
  Ref,
  ElementType,
  FC,
} from 'react';
import useStyles from './Card.styles';
import useTheme from '../../../contexts/Theme/useTheme';
import Typography from '../Typography/Typography';
import {TypographyVariant} from '../Typography/Typography.styles';

export type CardVariant = 'default' | 'outlined' | 'elevated';

export interface CardProps extends HTMLAttributes<HTMLDivElement> {
  /**
   * Card content
   */
  children?: ReactNode;
  /**
   * Whether the card should be clickable (shows hover effects)
   */
  clickable?: boolean;
  /**
   * The visual variant of the card
   */
  variant?: CardVariant;
}

export interface CardHeaderProps extends HTMLAttributes<HTMLDivElement> {
  /**
   * Header content
   */
  children?: ReactNode;
}

export interface CardTitleProps extends HTMLAttributes<HTMLHeadingElement> {
  /**
   * Title content
   */
  children?: ReactNode;
  /**
   * The heading level to use
   */
  level?: 1 | 2 | 3 | 4 | 5 | 6;
}

export interface CardDescriptionProps extends HTMLAttributes<HTMLParagraphElement> {
  /**
   * Description content
   */
  children?: ReactNode;
}

export interface CardActionProps extends HTMLAttributes<HTMLDivElement> {
  /**
   * Action content
   */
  children?: ReactNode;
}

export interface CardContentProps extends HTMLAttributes<HTMLDivElement> {
  /**
   * Content
   */
  children?: ReactNode;
}

export interface CardFooterProps extends HTMLAttributes<HTMLDivElement> {
  /**
   * Footer content
   */
  children?: ReactNode;
}

/**
 * Card component that provides a flexible container for content.
 *
 * @example
 * ```tsx
 * <Card variant="elevated" clickable>
 *   <Card.Header>
 *     <Card.Title>Card Title</Card.Title>
 *     <Card.Description>Card Description</Card.Description>
 *     <Card.Action>
 *       <Button variant="link">Action</Button>
 *     </Card.Action>
 *   </Card.Header>
 *   <Card.Content>
 *     <p>Card content goes here</p>
 *   </Card.Content>
 *   <Card.Footer>
 *     <Button>Cancel</Button>
 *     <Button variant="outline">Submit</Button>
 *   </Card.Footer>
 * </Card>
 * ```
 */
const Card: ForwardRefExoticComponent<CardProps & RefAttributes<HTMLDivElement>> = forwardRef<
  HTMLDivElement,
  CardProps
>(
  (
    {variant = 'default', clickable = false, children, className, style, ...rest}: CardProps,
    ref: Ref<HTMLDivElement>,
  ) => {
    const {theme, colorScheme}: ReturnType<typeof useTheme> = useTheme();
    const styles: Record<string, string> = useStyles(theme, colorScheme, variant, clickable);

    return (
      <div
        ref={ref}
        style={style}
        className={cx(
          withVendorCSSClassPrefix(bem('card')),
          styles['card'],
          styles['variant'],
          styles['clickable'],
          withVendorCSSClassPrefix(bem('card', null, variant)),
          {
            [withVendorCSSClassPrefix(bem('card', null, 'clickable'))]: clickable,
          },
          className,
        )}
        {...rest}
      >
        {children}
      </div>
    );
  },
);

/**
 * Card header component that contains the title, description, and optional actions.
 */
const CardHeader: ForwardRefExoticComponent<CardHeaderProps & RefAttributes<HTMLDivElement>> = forwardRef<
  HTMLDivElement,
  CardHeaderProps
>(({children, className, style, ...rest}: CardHeaderProps, ref: Ref<HTMLDivElement>) => {
  const {theme, colorScheme}: ReturnType<typeof useTheme> = useTheme();
  const styles: Record<string, string> = useStyles(theme, colorScheme, 'default', false);

  return (
    <div
      ref={ref}
      style={style}
      className={cx(withVendorCSSClassPrefix(bem('card', 'header')), styles['header'], className)}
      {...rest}
    >
      {children}
    </div>
  );
});

/**
 * Card title component.
 */
const CardTitle: FC<CardTitleProps> = ({children, level = 3, className, style, ...rest}: CardTitleProps) => {
  const {theme, colorScheme}: ReturnType<typeof useTheme> = useTheme();
  const styles: Record<string, string> = useStyles(theme, colorScheme, 'default', false);

  const getVariantFromLevel = (lvl: number): string => {
    switch (lvl) {
      case 1:
        return 'h1';
      case 2:
        return 'h2';
      case 3:
        return 'h3';
      case 4:
        return 'h4';
      case 5:
        return 'h5';
      case 6:
        return 'h6';
      default:
        return 'h3';
    }
  };

  const getComponentFromLevel = (lvl: number): string => {
    switch (lvl) {
      case 1:
        return 'h1';
      case 2:
        return 'h2';
      case 3:
        return 'h3';
      case 4:
        return 'h4';
      case 5:
        return 'h5';
      case 6:
        return 'h6';
      default:
        return 'h3';
    }
  };

  const {color, ...filteredRest} = rest;

  return (
    <Typography
      component={getComponentFromLevel(level) as ElementType}
      variant={getVariantFromLevel(level) as TypographyVariant}
      style={style}
      className={cx(withVendorCSSClassPrefix(bem('card', 'title')), styles['title'], className)}
      fontWeight={600}
      {...filteredRest}
    >
      {children}
    </Typography>
  );
};

/**
 * Card description component.
 */
const CardDescription: FC<CardDescriptionProps> = ({children, className, style, ...rest}: CardDescriptionProps) => {
  const {theme, colorScheme}: ReturnType<typeof useTheme> = useTheme();
  const styles: Record<string, string> = useStyles(theme, colorScheme, 'default', false);

  const {color, ...filteredRest} = rest;

  return (
    <Typography
      component="p"
      variant="body2"
      color="textSecondary"
      style={style}
      className={cx(withVendorCSSClassPrefix(bem('card', 'description')), styles['description'], className)}
      {...filteredRest}
    >
      {children}
    </Typography>
  );
};

/**
 * Card action component for action elements in the header.
 */
const CardAction: ForwardRefExoticComponent<CardActionProps & RefAttributes<HTMLDivElement>> = forwardRef<
  HTMLDivElement,
  CardActionProps
>(({children, className, style, ...rest}: CardActionProps, ref: Ref<HTMLDivElement>) => {
  const {theme, colorScheme}: ReturnType<typeof useTheme> = useTheme();
  const styles: Record<string, string> = useStyles(theme, colorScheme, 'default', false);

  return (
    <div
      ref={ref}
      style={style}
      className={cx(withVendorCSSClassPrefix(bem('card', 'action')), styles['action'], className)}
      {...rest}
    >
      {children}
    </div>
  );
});

/**
 * Card content component that contains the main content of the card.
 */
const CardContent: ForwardRefExoticComponent<CardContentProps & RefAttributes<HTMLDivElement>> = forwardRef<
  HTMLDivElement,
  CardContentProps
>(({children, className, style, ...rest}: CardContentProps, ref: Ref<HTMLDivElement>) => {
  const {theme, colorScheme}: ReturnType<typeof useTheme> = useTheme();
  const styles: Record<string, string> = useStyles(theme, colorScheme, 'default', false);

  return (
    <div
      ref={ref}
      style={style}
      className={cx(withVendorCSSClassPrefix(bem('card', 'content')), styles['content'], className)}
      {...rest}
    >
      {children}
    </div>
  );
});

/**
 * Card footer component that contains footer actions or additional information.
 */
const CardFooter: ForwardRefExoticComponent<CardFooterProps & RefAttributes<HTMLDivElement>> = forwardRef<
  HTMLDivElement,
  CardFooterProps
>(({children, className, style, ...rest}: CardFooterProps, ref: Ref<HTMLDivElement>) => {
  const {theme, colorScheme}: ReturnType<typeof useTheme> = useTheme();
  const styles: Record<string, string> = useStyles(theme, colorScheme, 'default', false);

  return (
    <div
      ref={ref}
      style={style}
      className={cx(withVendorCSSClassPrefix(bem('card', 'footer')), styles['footer'], className)}
      {...rest}
    >
      {children}
    </div>
  );
});

Card.displayName = 'Card';
CardHeader.displayName = 'Card.Header';
CardTitle.displayName = 'Card.Title';
CardDescription.displayName = 'Card.Description';
CardAction.displayName = 'Card.Action';
CardContent.displayName = 'Card.Content';
CardFooter.displayName = 'Card.Footer';

(Card as any).Header = CardHeader;
(Card as any).Title = CardTitle;
(Card as any).Description = CardDescription;
(Card as any).Action = CardAction;
(Card as any).Content = CardContent;
(Card as any).Footer = CardFooter;

export interface CardComponent extends ForwardRefExoticComponent<CardProps & RefAttributes<HTMLDivElement>> {
  Action: typeof CardAction;
  Content: typeof CardContent;
  Description: typeof CardDescription;
  Footer: typeof CardFooter;
  Header: typeof CardHeader;
  Title: typeof CardTitle;
}

export default Card as CardComponent;
export {Card, CardHeader, CardTitle, CardDescription, CardAction, CardContent, CardFooter};
