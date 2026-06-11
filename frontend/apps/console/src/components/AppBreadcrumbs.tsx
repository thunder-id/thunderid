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

import {Breadcrumbs, MenuItem, MenuList, Paper, Popper, Typography} from '@wso2/oxygen-ui';
import {ChevronRight} from '@wso2/oxygen-ui-icons-react';
import type {JSX, MouseEvent} from 'react';
import {useEffect, useRef, useState} from 'react';

export interface BreadcrumbItem {
  key: string;
  label: string;
  onClick?: () => void;
}

interface AppBreadcrumbsProps {
  items: BreadcrumbItem[];
  maxItems?: number;
}

export default function AppBreadcrumbs({items, maxItems = 4}: AppBreadcrumbsProps): JSX.Element {
  const [anchorEl, setAnchorEl] = useState<HTMLElement | null>(null);
  const [ellipsisHovered, setEllipsisHovered] = useState(false);
  const ellipsisRef = useRef<HTMLElement | null>(null);
  const popperRef = useRef<HTMLDivElement | null>(null);
  const open = Boolean(anchorEl);

  useEffect(() => {
    if (!open) return undefined;

    const handleDocumentClick = (e: globalThis.MouseEvent) => {
      const target = e.target as Node;
      if (ellipsisRef.current?.contains(target) || popperRef.current?.contains(target)) {
        return;
      }
      setAnchorEl(null);
    };

    document.addEventListener('click', handleDocumentClick);
    return () => document.removeEventListener('click', handleDocumentClick);
  }, [open]);

  const handleEllipsisClick = (e: MouseEvent<HTMLElement>) => {
    ellipsisRef.current = e.currentTarget;
    setAnchorEl(open ? null : e.currentTarget);
  };

  const handleMenuItemClick = (item: BreadcrumbItem) => {
    setAnchorEl(null);
    item.onClick?.();
  };

  const shouldTruncate = items.length > maxItems;
  const visibleItems: BreadcrumbItem[] = shouldTruncate ? [items[0], ...items.slice(items.length - 2)] : items;
  const hiddenItems: BreadcrumbItem[] = shouldTruncate ? items.slice(1, items.length - 2) : [];

  const renderItem = (item: BreadcrumbItem, isLast: boolean) => {
    if (isLast) {
      return (
        <Typography key={item.key} variant="h5" color="text.primary" sx={{whiteSpace: 'nowrap'}}>
          {item.label}
        </Typography>
      );
    }

    return (
      <Typography
        key={item.key}
        variant="h5"
        color="inherit"
        role="button"
        tabIndex={0}
        onClick={item.onClick}
        onKeyDown={(e: React.KeyboardEvent) => {
          if (e.key === 'Enter' || e.key === ' ') {
            e.preventDefault();
            item.onClick?.();
          }
        }}
        sx={{cursor: 'pointer', whiteSpace: 'nowrap', '&:hover': {textDecoration: 'underline'}}}
      >
        {item.label}
      </Typography>
    );
  };

  const breadcrumbChildren: JSX.Element[] = [];

  if (shouldTruncate) {
    breadcrumbChildren.push(renderItem(visibleItems[0], false));

    breadcrumbChildren.push(
      <Typography
        key="__ellipsis__"
        variant="h5"
        color="inherit"
        onClick={handleEllipsisClick}
        onMouseEnter={() => setEllipsisHovered(true)}
        onMouseLeave={() => setEllipsisHovered(false)}
        sx={{
          cursor: 'pointer',
          whiteSpace: 'nowrap',
          userSelect: 'none',
          px: 0.5,
          borderRadius: 1,
          bgcolor: ellipsisHovered || open ? 'action.hover' : 'transparent',
        }}
      >
        ...
      </Typography>,
    );

    breadcrumbChildren.push(renderItem(visibleItems[1], false));
    breadcrumbChildren.push(renderItem(visibleItems[2], true));
  } else {
    items.forEach((item, index) => {
      breadcrumbChildren.push(renderItem(item, index === items.length - 1));
    });
  }

  return (
    <>
      <Breadcrumbs
        separator={<ChevronRight size={16} />}
        aria-label="breadcrumb"
        sx={{'& ol': {flexWrap: 'nowrap', alignItems: 'center'}}}
      >
        {breadcrumbChildren}
      </Breadcrumbs>

      <Popper open={open} anchorEl={anchorEl} placement="bottom-start" sx={{zIndex: 1400}}>
        <Paper ref={popperRef} elevation={3} sx={{minWidth: 160, mt: 0.5}}>
          <MenuList dense>
            {hiddenItems.map((item) => (
              <MenuItem key={item.key} onClick={() => handleMenuItemClick(item)}>
                <Typography variant="body2">{item.label}</Typography>
              </MenuItem>
            ))}
          </MenuList>
        </Paper>
      </Popper>
    </>
  );
}
