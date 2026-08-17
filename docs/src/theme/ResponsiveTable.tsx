// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import React, {useCallback, useEffect, useMemo, useRef, useState} from 'react';
import {Box} from '@wso2/oxygen-ui';

export interface ResponsiveTableProps extends React.TableHTMLAttributes<HTMLTableElement> {
  children?: React.ReactNode;
  className?: string;
}

function extractNodeText(node: React.ReactNode): string {
  if (typeof node === 'string' || typeof node === 'number') {
    return String(node);
  }
  if (!node || typeof node !== 'object') {
    return '';
  }
  if (Array.isArray(node)) {
    return node.map(extractNodeText).join(' ');
  }
  if ('props' in node && node.props && typeof node.props === 'object' && 'children' in node.props) {
    return extractNodeText((node.props as {children?: React.ReactNode}).children);
  }
  return '';
}

function checkHasDescriptionColumn(children: React.ReactNode): boolean {
  let found = false;
  React.Children.forEach(children, (child) => {
    if (!React.isValidElement(child)) return;
    const childElement = child as React.ReactElement<{children?: React.ReactNode}>;
    if (childElement.type === 'thead') {
      React.Children.forEach(childElement.props.children, (tr) => {
        if (!React.isValidElement(tr)) return;
        const trElement = tr as React.ReactElement<{children?: React.ReactNode}>;
        React.Children.forEach(trElement.props.children, (th) => {
          if (!React.isValidElement(th)) return;
          const thText = extractNodeText(th);
          if (/description/i.test(thText)) {
            found = true;
          }
        });
      });
    }
  });
  return found;
}

export default function ResponsiveTable({children, className, ...props}: ResponsiveTableProps): React.JSX.Element {
  const containerRef = useRef<HTMLDivElement>(null);
  const tableRef = useRef<HTMLTableElement>(null);
  const [canScrollLeft, setCanScrollLeft] = useState(false);
  const [canScrollRight, setCanScrollRight] = useState(false);
  const [hasOverflow, setHasOverflow] = useState(false);
  const [domHasDescription, setDomHasDescription] = useState(false);

  const reactHasDescription = useMemo(() => checkHasDescriptionColumn(children), [children]);
  const hasDescription = reactHasDescription || domHasDescription;

  const updateScrollState = useCallback(() => {
    const el = containerRef.current;
    if (!el) return;

    const {scrollLeft, scrollWidth, clientWidth} = el;
    const maxScroll = scrollWidth - clientWidth;
    const overflow = maxScroll > 1;

    setHasOverflow(overflow);
    setCanScrollLeft(overflow && scrollLeft > 1);
    setCanScrollRight(overflow && scrollLeft < maxScroll - 1);
  }, []);

  useEffect(() => {
    const el = containerRef.current;
    const tableEl = tableRef.current;
    if (!el) return;

    if (tableEl && !reactHasDescription) {
      const headers = Array.from(tableEl.querySelectorAll('thead th'));
      const foundDesc = headers.some((th) => /description/i.test(th.textContent || ''));
      if (foundDesc) {
        setDomHasDescription(true);
      }
    }

    updateScrollState();

    let resizeObserver: ResizeObserver | null = null;
    if (typeof ResizeObserver !== 'undefined') {
      resizeObserver = new ResizeObserver(() => {
        updateScrollState();
      });
      resizeObserver.observe(el);
      if (tableEl) {
        resizeObserver.observe(tableEl);
      }
    }

    const handleScroll = () => {
      updateScrollState();
    };

    el.addEventListener('scroll', handleScroll, {passive: true});
    window.addEventListener('resize', updateScrollState, {passive: true});

    return () => {
      if (resizeObserver) {
        resizeObserver.disconnect();
      }
      el.removeEventListener('scroll', handleScroll);
      window.removeEventListener('resize', updateScrollState);
    };
  }, [updateScrollState, reactHasDescription]);

  return (
    <Box
      sx={{
        position: 'relative',
        mb: 'var(--ifm-spacing-vertical, 1rem)',
        maxWidth: '100%',
        '&::before, &::after': {
          content: '""',
          position: 'absolute',
          top: 0,
          bottom: 0,
          width: 24,
          zIndex: 2,
          pointerEvents: 'none',
          transition: 'opacity 0.2s ease-in-out',
        },
        '@media (prefers-reduced-motion: reduce)': {
          '&::before, &::after': {
            transition: 'none',
          },
        },
        '&::before': {
          left: 0,
          background: 'linear-gradient(to right, var(--ifm-background-color, #ffffff), transparent)',
          opacity: canScrollLeft ? 1 : 0,
        },
        '&::after': {
          right: 0,
          background: 'linear-gradient(to left, var(--ifm-background-color, #ffffff), transparent)',
          opacity: canScrollRight ? 1 : 0,
        },
      }}
    >
      <Box
        ref={containerRef}
        tabIndex={hasOverflow ? 0 : undefined}
        role={hasOverflow ? 'region' : undefined}
        aria-label={hasOverflow ? 'Scrollable table' : undefined}
        sx={{
          overflowX: 'auto',
          maxWidth: '100%',
          WebkitOverflowScrolling: 'touch',
          '&:focus-visible': {
            outline: '2px solid var(--ifm-color-primary, #0070f3)',
            outlineOffset: '2px',
            borderRadius: 'var(--ifm-global-radius, 6px)',
          },
          '& table': {
            display: 'table',
            width: '100%',
            borderCollapse: 'collapse',
            margin: 0,
          },
          '& th, & td': {
            verticalAlign: 'top',
            overflowWrap: 'break-word',
            wordBreak: 'break-word',
          },
          '& td code, & th code': {
            whiteSpace: 'pre-wrap',
            wordBreak: 'break-word',
            overflowWrap: 'anywhere',
          },
          ...(hasDescription && {
            '& th:last-child, & td:last-child': {
              width: '100%',
              minWidth: {xs: '180px', sm: '200px'},
            },
            '& th:not(:last-child), & td:not(:last-child)': {
              minWidth: {xs: '100px', sm: '120px'},
            },
          }),
        }}
      >
        <table ref={tableRef} className={className} {...props}>
          {children}
        </table>
      </Box>
    </Box>
  );
}
