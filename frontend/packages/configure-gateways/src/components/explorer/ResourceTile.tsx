// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, ButtonBase, Typography, alpha} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {LANE_BY_ID, STAGE, type GraphNode, type Lane} from '../../explorer/model';
import LaneIcon from '../LaneIcon';

export interface TileProps {
  lane: Lane;
  /** The tile's size in pixels; a pill is wider. */
  size?: number;
  selected?: boolean;
  icon?: string;
  letter?: string;
  badge?: string;
}

/** The shape of a resource: a rounded tile, a wider pill for a flow, a circle for a connection. */
export function Tile({
  lane,
  size = STAGE.tileSize,
  selected = false,
  icon = undefined,
  letter = undefined,
  badge = undefined,
}: TileProps): JSX.Element {
  return (
    <Box
      sx={{
        position: 'relative',
        width: lane.shape === 'pill' ? size * 1.45 : size,
        height: size,
        flexShrink: 0,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        color: lane.color,
        bgcolor: selected ? alpha(lane.color, 0.12) : 'background.paper',
        border: `2px solid ${alpha(lane.color, selected ? 1 : 0.63)}`,
        borderRadius: lane.shape === 'circle' ? '50%' : '8px',
        boxShadow: selected ? `0 0 0 3px ${alpha(lane.color, 0.16)}` : undefined,
        transition: 'box-shadow 150ms, background-color 150ms, transform 120ms',
        fontWeight: 700,
        fontSize: size * 0.4,
      }}
    >
      {letter ?? <LaneIcon name={icon ?? lane.icon} size={Math.round(size * 0.43)} />}
      {badge && (
        <Box
          component="span"
          sx={{
            position: 'absolute',
            bottom: -8,
            left: '50%',
            transform: 'translateX(-50%)',
            bgcolor: lane.color,
            color: 'common.black',
            fontSize: 9,
            fontWeight: 700,
            letterSpacing: '0.06em',
            px: 0.75,
            borderRadius: 1,
            lineHeight: 1.6,
          }}
        >
          {badge}
        </Box>
      )}
    </Box>
  );
}

export interface ResourceTileProps {
  node: GraphNode;
  selected: boolean;
  dimmed: boolean;
  onSelect: (key: string) => void;
  onHover: (key: string | null) => void;
  /** The gateway holds back this resource's change, so it runs it as it was before. */
  heldBack?: boolean;
}

/** One resource on the map: its tile and its name, placed where the model put it. */
export default function ResourceTile({
  node,
  selected,
  dimmed,
  onSelect,
  onHover,
  heldBack = false,
}: ResourceTileProps): JSX.Element {
  const {t} = useTranslation('deploy');
  const lane = LANE_BY_ID[node.lane];
  return (
    <ButtonBase
      data-testid={`explorer-node-${node.key}`}
      aria-label={node.name}
      aria-pressed={selected}
      // No ripple: it would spread over icon and label as a block. A press shrinks the tile a little
      // instead, and the selection shows on the tile itself.
      disableRipple
      onClick={(event) => {
        event.stopPropagation();
        onSelect(node.key);
      }}
      onMouseEnter={() => onHover(node.key)}
      onMouseLeave={() => onHover(null)}
      onFocus={() => onHover(node.key)}
      onBlur={() => onHover(null)}
      sx={{
        position: 'absolute',
        left: node.x - 75,
        top: node.y - STAGE.tileSize / 2,
        width: 150,
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        gap: 1.25,
        opacity: dimmed ? 0.2 : 1,
        transition: 'opacity 150ms',
        zIndex: 2,
        borderRadius: 2,
        bgcolor: 'transparent',
        '&:hover, &.Mui-focusVisible, &[aria-pressed="true"]': {bgcolor: 'transparent'},
        '&:active > :first-of-type': {transform: 'scale(0.94)'},
      }}
    >
      <Tile lane={lane} selected={selected} icon={node.icon} letter={node.letter} badge={node.badge} />
      <Typography variant="caption" sx={{textAlign: 'center', lineHeight: 1.3, px: 0.5}}>
        {node.name}
      </Typography>
      {heldBack && (
        <Typography
          variant="caption"
          color="text.secondary"
          data-testid="explorer-node-held-back"
          sx={{mt: -1, fontSize: 10, px: 0.75, borderRadius: 1, bgcolor: 'action.selected'}}
        >
          {t('explorer.heldBack', 'Held back')}
        </Typography>
      )}
    </ButtonBase>
  );
}
