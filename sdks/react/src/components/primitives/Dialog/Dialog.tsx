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
import {
  useFloating,
  useClick,
  useDismiss,
  useRole,
  useInteractions,
  useMergeRefs,
  FloatingFocusManager,
  FloatingOverlay,
  FloatingPortal,
  useId,
  UseFloatingReturn,
  UseInteractionsReturn,
} from '@floating-ui/react';
import {withVendorCSSClassPrefix, bem} from '@thunderid/browser';
import {
  type ButtonHTMLAttributes,
  type Context,
  type Dispatch,
  type FC,
  type ForwardRefExoticComponent,
  type HTMLProps,
  type JSX,
  type MouseEvent,
  type ReactNode,
  type Ref,
  type RefAttributes,
  type SetStateAction,
  cloneElement,
  createContext,
  forwardRef,
  isValidElement,
  useContext,
  useLayoutEffect,
  useMemo,
  useState,
} from 'react';
import useStyles from './Dialog.styles';
import useTheme from '../../../contexts/Theme/useTheme';
import Button from '../Button/Button';
import {X} from '../Icons';

interface DialogOptions {
  initialOpen?: boolean;
  onOpenChange?: (open: boolean) => void;
  open?: boolean;
}

interface DialogHookReturn extends UseFloatingReturn, UseInteractionsReturn {
  descriptionId: string | undefined;
  labelId: string | undefined;
  open: boolean;
  setDescriptionId: Dispatch<SetStateAction<string | undefined>>;
  setLabelId: Dispatch<SetStateAction<string | undefined>>;
  setOpen: (open: boolean) => void;
}

export function useDialog({
  initialOpen = false,
  open: controlledOpen,
  onOpenChange: setControlledOpen,
}: DialogOptions = {}): DialogHookReturn {
  const [uncontrolledOpen, setUncontrolledOpen] = useState(initialOpen);
  const [labelId, setLabelId] = useState<string | undefined>();
  const [descriptionId, setDescriptionId] = useState<string | undefined>();

  const open: boolean = controlledOpen ?? uncontrolledOpen;
  const setOpen: (openVal: boolean) => void = setControlledOpen ?? setUncontrolledOpen;

  const data: UseFloatingReturn = useFloating({
    onOpenChange: setOpen,
    open,
  });

  const {context} = data;

  const click: ReturnType<typeof useClick> = useClick(context, {
    enabled: controlledOpen == null,
  });
  const dismiss: ReturnType<typeof useDismiss> = useDismiss(context, {outsidePressEvent: 'mousedown'});
  const role: ReturnType<typeof useRole> = useRole(context);

  const interactions: UseInteractionsReturn = useInteractions([click, dismiss, role]);

  return useMemo(
    () => ({
      open,
      setOpen,
      ...interactions,
      ...data,
      descriptionId,
      labelId,
      setDescriptionId,
      setLabelId,
    }),
    [open, setOpen, interactions, data, labelId, descriptionId],
  );
}

type DialogContextType =
  | (DialogHookReturn & {
      setDescriptionId: Dispatch<SetStateAction<string | undefined>>;
      setLabelId: Dispatch<SetStateAction<string | undefined>>;
    })
  | null;

const DialogContext: Context<DialogContextType> = createContext<DialogContextType>(null);

export const useDialogContext = (): DialogHookReturn => {
  const context: DialogContextType = useContext(DialogContext);
  if (context == null) {
    throw new Error('Dialog components must be wrapped in <Dialog />');
  }
  return context;
};

// Dialog Components (Modal)
export function Dialog({children, ...options}: {children: ReactNode} & DialogOptions): JSX.Element {
  const dialog: DialogHookReturn = useDialog(options);
  return <DialogContext.Provider value={dialog}>{children}</DialogContext.Provider>;
}

interface DialogTriggerProps {
  asChild?: boolean;
  children: ReactNode;
}

export const DialogTrigger: ForwardRefExoticComponent<
  HTMLProps<HTMLElement> & DialogTriggerProps & RefAttributes<HTMLElement>
> = forwardRef<HTMLElement, HTMLProps<HTMLElement> & DialogTriggerProps>(
  ({children, asChild = false, ...props}: HTMLProps<HTMLElement> & DialogTriggerProps, propRef: Ref<HTMLElement>) => {
    const context: DialogHookReturn = useDialogContext();
    const childrenRef: Ref<HTMLElement> = (children as any).ref;
    const ref: ReturnType<typeof useMergeRefs> = useMergeRefs([context.refs.setReference, propRef, childrenRef]);

    if (asChild && isValidElement(children)) {
      return cloneElement(
        children,
        context.getReferenceProps({
          ref,
          ...props,
          ...(children.props as any),
          'data-state': context.open ? 'open' : 'closed',
        }),
      );
    }

    return (
      <button ref={ref} data-state={context.open ? 'open' : 'closed'} {...context.getReferenceProps(props)}>
        {children}
      </button>
    );
  },
);

export const DialogContent: ForwardRefExoticComponent<HTMLProps<HTMLDivElement> & RefAttributes<HTMLDivElement>> =
  forwardRef<HTMLDivElement, HTMLProps<HTMLDivElement>>(
    (props: HTMLProps<HTMLDivElement>, propRef: Ref<HTMLDivElement>) => {
      const {context: floatingContext, ...context} = useDialogContext();
      const {theme, colorScheme}: ReturnType<typeof useTheme> = useTheme();
      const styles: Record<string, string> = useStyles(theme, colorScheme);
      const ref: ReturnType<typeof useMergeRefs> = useMergeRefs([context.refs.setFloating, propRef]);

      if (!floatingContext.open) return null;

      return (
        <FloatingPortal>
          <FloatingOverlay
            className={cx(withVendorCSSClassPrefix(bem('dialog', 'overlay')), styles['overlay'])}
            lockScroll
          >
            <FloatingFocusManager context={floatingContext} initialFocus={-1}>
              <div
                ref={ref}
                className={cx(withVendorCSSClassPrefix(bem('dialog', 'content')), styles['content'], props.className)}
                aria-labelledby={context.labelId}
                aria-describedby={context.descriptionId}
                {...context.getFloatingProps(props)}
              >
                {props.children}
              </div>
            </FloatingFocusManager>
          </FloatingOverlay>
        </FloatingPortal>
      );
    },
  );

export const DialogHeading: ForwardRefExoticComponent<
  HTMLProps<HTMLHeadingElement> & RefAttributes<HTMLHeadingElement>
> = forwardRef<HTMLHeadingElement, HTMLProps<HTMLHeadingElement>>(
  ({children, ...props}: HTMLProps<HTMLHeadingElement>, ref: Ref<HTMLHeadingElement>) => {
    const context: DialogHookReturn = useDialogContext();
    const {theme, colorScheme}: ReturnType<typeof useTheme> = useTheme();
    const styles: Record<string, string> = useStyles(theme, colorScheme);
    const id: string = useId();

    useLayoutEffect((): (() => void) => {
      context.setLabelId(id);
      return (): void => {
        context.setLabelId(undefined);
      };
    }, [id, context.setLabelId]);

    return (
      <div className={cx(withVendorCSSClassPrefix(bem('dialog', 'header')), styles['header'])}>
        <h2
          {...props}
          ref={ref}
          id={id}
          className={cx(withVendorCSSClassPrefix(bem('dialog', 'title')), styles['headerTitle'])}
        >
          {children}
        </h2>
        <Button
          color="tertiary"
          variant="icon"
          size="small"
          shape="round"
          onClick={(): void => {
            context.setOpen(false);
          }}
          aria-label="Close"
        >
          <X width={16} height={16} />
        </Button>
      </div>
    );
  },
);

export const DialogDescription: ForwardRefExoticComponent<
  HTMLProps<HTMLParagraphElement> & RefAttributes<HTMLParagraphElement>
> = forwardRef<HTMLParagraphElement, HTMLProps<HTMLParagraphElement>>(
  ({children, ...props}: HTMLProps<HTMLParagraphElement>, ref: Ref<HTMLParagraphElement>) => {
    const context: DialogHookReturn = useDialogContext();
    const {theme, colorScheme}: ReturnType<typeof useTheme> = useTheme();
    const styles: Record<string, string> = useStyles(theme, colorScheme);
    const id: string = useId();

    useLayoutEffect((): (() => void) => {
      context.setDescriptionId(id);
      return (): void => {
        context.setDescriptionId(undefined);
      };
    }, [id, context.setDescriptionId]);

    return (
      <p
        {...props}
        ref={ref}
        id={id}
        className={cx(withVendorCSSClassPrefix(bem('dialog', 'description')), styles['description'], props.className)}
      >
        {children}
      </p>
    );
  },
);

interface DialogCloseProps {
  asChild?: boolean;
  children?: ReactNode;
}

export const DialogClose: ForwardRefExoticComponent<
  ButtonHTMLAttributes<HTMLButtonElement> & DialogCloseProps & RefAttributes<HTMLButtonElement>
> = forwardRef<HTMLButtonElement, ButtonHTMLAttributes<HTMLButtonElement> & DialogCloseProps>(
  (
    {children, asChild = false, ...props}: ButtonHTMLAttributes<HTMLButtonElement> & DialogCloseProps,
    propRef: Ref<HTMLButtonElement>,
  ) => {
    const context: DialogHookReturn = useDialogContext();
    const childrenRef: Ref<HTMLButtonElement> = (children as any)?.ref;
    const ref: ReturnType<typeof useMergeRefs> = useMergeRefs([propRef, childrenRef]);

    const handleClick = (event: MouseEvent<HTMLButtonElement>): void => {
      context.setOpen(false);
      props.onClick?.(event);
    };

    if (asChild && isValidElement(children)) {
      return cloneElement(children, {
        ref,
        ...props,
        ...(children.props as any),
        onClick: handleClick,
      });
    }

    return (
      <Button
        {...props}
        ref={ref}
        onClick={handleClick}
        className={cx(withVendorCSSClassPrefix(bem('dialog', 'close')), props.className)}
        variant="text"
      >
        {children}
      </Button>
    );
  },
);

DialogTrigger.displayName = 'DialogTrigger';
DialogContent.displayName = 'DialogContent';
DialogHeading.displayName = 'DialogHeading';
DialogDescription.displayName = 'DialogDescription';
DialogClose.displayName = 'DialogClose';

// Attach subcomponents to Dialog
(Dialog as any).Trigger = DialogTrigger;
(Dialog as any).Content = DialogContent;
(Dialog as any).Heading = DialogHeading;
(Dialog as any).Description = DialogDescription;
(Dialog as any).Close = DialogClose;

export interface DialogComponent extends FC<{children: ReactNode} & DialogOptions> {
  Close: typeof DialogClose;
  Content: typeof DialogContent;
  Description: typeof DialogDescription;
  Heading: typeof DialogHeading;
  Trigger: typeof DialogTrigger;
}

export default Dialog as DialogComponent;
