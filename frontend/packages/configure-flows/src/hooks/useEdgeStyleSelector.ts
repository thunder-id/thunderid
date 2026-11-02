// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useState, useCallback} from 'react';

/**
 * Hook to manage edge style selector state.
 * Returns handlers and state for the edge style menu.
 */
function useEdgeStyleSelector(): {
  anchorEl: HTMLElement | null;
  handleClick: (event: React.MouseEvent<HTMLElement>) => void;
  handleClose: () => void;
} {
  const [anchorEl, setAnchorEl] = useState<null | HTMLElement>(null);

  const handleClick = useCallback((event: React.MouseEvent<HTMLElement>) => {
    setAnchorEl(event.currentTarget);
  }, []);

  const handleClose = useCallback(() => {
    setAnchorEl(null);
  }, []);

  return {anchorEl, handleClick, handleClose};
}

export default useEdgeStyleSelector;
