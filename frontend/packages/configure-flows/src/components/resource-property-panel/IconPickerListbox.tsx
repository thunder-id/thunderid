// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useVirtualizer} from '@tanstack/react-virtual';
import {Box, useMediaQuery, useTheme} from '@wso2/oxygen-ui';
import {
  Children,
  cloneElement,
  forwardRef,
  isValidElement,
  useEffect,
  useImperativeHandle,
  useRef,
  type CSSProperties,
  type HTMLAttributes,
  type ReactElement,
} from 'react';

type OptionElement = ReactElement<HTMLAttributes<HTMLLIElement> & {style?: CSSProperties}>;

/**
 * The listbox of the icon picker: it renders only the options in view, so that the picker can list
 * every icon. It follows the virtualization the MUI Autocomplete documentation describes, with the
 * options rendered by the Autocomplete's renderOption.
 *
 * Every option has the height of an Autocomplete option, which depends on the breakpoint only, so
 * the options are laid out without measuring them. They are placed with `top` rather than a
 * transform, so that their offsetTop is their position in the list: the Autocomplete scrolls an
 * option highlighted with the keyboard into view by it.
 */
const IconPickerListbox = forwardRef<HTMLDivElement, HTMLAttributes<HTMLElement>>(function IconPickerListbox(
  {children, ...listboxProps},
  ref,
) {
  const scrollRef = useRef<HTMLDivElement>(null);
  useImperativeHandle(ref, () => scrollRef.current!, []);

  // The heights MUI gives Autocomplete options.
  const theme = useTheme();
  const optionHeight = useMediaQuery(theme.breakpoints.up('sm'), {noSsr: true}) ? 36 : 48;

  const options = Children.toArray(children).filter(isValidElement) as OptionElement[];
  const selectedIndex = options.findIndex((option) => option.props['aria-selected'] === true);

  const virtualizer = useVirtualizer({
    count: options.length,
    getScrollElement: () => scrollRef.current,
    estimateSize: () => optionHeight,
    overscan: 5,
  });

  // When the list opens, or a search lists other options, show the selected icon if it is listed,
  // and the top of the list otherwise: the Autocomplete cannot scroll to an option that is not
  // rendered. Not when the selection changes while the list is open.
  useEffect(() => {
    if (selectedIndex >= 0) {
      virtualizer.scrollToIndex(selectedIndex, {align: 'center'});
    } else {
      virtualizer.scrollToOffset(0);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [options.length, virtualizer]);

  // The Autocomplete gives the listbox its role, id and handlers; they go on the scrolling element,
  // which the documentation requires to carry the listbox role.
  return (
    <div ref={scrollRef} {...listboxProps}>
      <Box
        component="ul"
        sx={{
          height: virtualizer.getTotalSize(),
          m: 0,
          p: 0,
          position: 'relative',
          // Qualified with the option class to win over the Autocomplete's own option styles.
          '& > li.MuiAutocomplete-option': {
            boxSizing: 'border-box',
            height: optionHeight,
            left: 0,
            minHeight: 0,
            position: 'absolute',
            width: '100%',
          },
        }}
      >
        {virtualizer.getVirtualItems().map((item) =>
          cloneElement(options[item.index], {
            key: item.key,
            // Each option's offset is computed per option and changes on every scroll, so it is set
            // inline: in sx, every offset would add a CSS class of its own.
            style: {...options[item.index].props.style, top: item.start},
          }),
        )}
      </Box>
    </div>
  );
});

export default IconPickerListbox;
