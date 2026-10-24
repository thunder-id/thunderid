// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, IconButton, Stack, Tooltip, Typography} from '@wso2/oxygen-ui';
import {Expand, Maximize, Minus, Plus, Shrink} from '@wso2/oxygen-ui-icons-react';
import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type JSX,
  type MouseEvent,
  type PointerEvent as ReactPointerEvent,
} from 'react';
import {useTranslation} from 'react-i18next';
import ResourceTile from './ResourceTile';
import {
  edgePath,
  LANE_BY_ID,
  LANES,
  laneTop,
  neighbours,
  STAGE,
  type Graph,
  type GraphEdge,
  type GraphNode,
  type Label,
  type Lane,
  type OverflowNode,
} from '../../explorer/model';

const ZOOM = {min: 0.4, max: 1.6, step: 0.1, initial: 0.8} as const;

export interface ExplorerCanvasProps {
  graph: Graph;
  selected: string | null;
  onSelect: (key: string | null) => void;
  /** Turns a model label into text. */
  labelFor: (label: Label) => string;
  /** Draws the lines between lanes as curves rather than straight. */
  curved?: boolean;
  /** The keys of the resources the gateway holds back. */
  heldBack?: ReadonlySet<string>;
  /** Whether the explorer fills the screen; with onToggleFullscreen, a button switches it. */
  fullscreen?: boolean;
  onToggleFullscreen?: () => void;
}

interface Tip {
  x: number;
  y: number;
  pair: string;
  label: string;
}

const clampZoom = (zoom: number): number => Math.max(ZOOM.min, Math.min(ZOOM.max, Math.round(zoom * 100) / 100));

/**
 * The map: lanes, the resources in them, and the lines between them. Hovering a line says what it
 * means; hovering or selecting a resource lights its lines and fades everything unrelated. The
 * stage is scaled by the zoom, which the bar at the bottom right sets.
 */
export default function ExplorerCanvas({
  graph,
  selected,
  onSelect,
  labelFor,
  curved = false,
  heldBack = undefined,
  fullscreen = false,
  onToggleFullscreen = undefined,
}: ExplorerCanvasProps): JSX.Element {
  const {t} = useTranslation('deploy');
  const canvasRef = useRef<HTMLDivElement>(null);
  const [hoverNode, setHoverNode] = useState<string | null>(null);
  const [hoverEdge, setHoverEdge] = useState<number | null>(null);
  const [tip, setTip] = useState<Tip | null>(null);
  const [zoom, setZoom] = useState<number>(ZOOM.initial);
  // The canvas pans rather than scrolls: a drag on it, or the wheel, moves the whole drawing.
  const [pan, setPan] = useState<{x: number; y: number}>({x: 0, y: 0});
  const drag = useRef<{startX: number; startY: number; panX: number; panY: number; moved: boolean} | null>(null);
  const [dragging, setDragging] = useState<boolean>(false);
  const byKey = useMemo(() => new Map(graph.nodes.map((node: GraphNode) => [node.key, node])), [graph.nodes]);
  const focus = selected ?? hoverNode;
  const related = useMemo(() => (focus ? neighbours(graph, focus) : null), [graph, focus]);

  const showTip = useCallback(
    (index: number, event: MouseEvent): void => {
      const canvas = canvasRef.current;
      const edge = graph.edges[index];
      if (!canvas || !edge) return;
      const from = byKey.get(edge.from);
      const to = byKey.get(edge.to);
      if (!from || !to) return;
      const rect = canvas.getBoundingClientRect();
      setHoverEdge(index);
      setTip({
        x: event.clientX - rect.left,
        y: event.clientY - rect.top,
        pair: `${from.name} → ${to.name}`,
        label: labelFor(edge.label),
      });
    },
    [graph.edges, byKey, labelFor],
  );
  const hideTip = (): void => {
    setHoverEdge(null);
    setTip(null);
  };
  const fit = (): void => {
    const canvas = canvasRef.current;
    if (canvas) setZoom(clampZoom((canvas.clientWidth - 48) / graph.width));
    setPan({x: 0, y: 0});
  };
  const startDrag = (event: ReactPointerEvent<HTMLDivElement>): void => {
    // A press on a button, a tile or a field is that control's, not the start of a drag: capturing
    // the pointer here would take its click away.
    if (event.button !== 0 || (event.target as HTMLElement).closest('button, a, input, textarea, select')) return;
    drag.current = {startX: event.clientX, startY: event.clientY, panX: pan.x, panY: pan.y, moved: false};
    setDragging(true);
    try {
      event.currentTarget.setPointerCapture(event.pointerId);
    } catch {
      // A pointer that is not active, as a synthetic event's is not, cannot be captured; the drag
      // still works while the pointer stays over the canvas.
    }
  };
  const moveDrag = (event: ReactPointerEvent<HTMLDivElement>): void => {
    const current = drag.current;
    if (!current) return;
    const dx = event.clientX - current.startX;
    const dy = event.clientY - current.startY;
    if (Math.abs(dx) > 3 || Math.abs(dy) > 3) current.moved = true;
    if (current.moved) setPan({x: current.panX + dx, y: current.panY + dy});
  };
  const endDrag = (event: ReactPointerEvent<HTMLDivElement>): void => {
    if (drag.current && event.currentTarget.hasPointerCapture(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId);
    }
    // A drag that moved the drawing is not a click on it, so it leaves the selection alone.
    const moved = drag.current?.moved ?? false;
    drag.current = null;
    setDragging(false);
    if (moved) event.currentTarget.dataset['dragged'] = 'true';
  };
  // The wheel pans too, as it would scroll, and zooms with the control or command key held. The
  // listener is attached by hand because React attaches wheel listeners as passive, which cannot
  // stop the page from scrolling instead.
  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return undefined;
    const onWheel = (event: WheelEvent): void => {
      event.preventDefault();
      if (event.ctrlKey || event.metaKey) {
        setZoom((current: number) => clampZoom(current - Math.sign(event.deltaY) * ZOOM.step));
        return;
      }
      setPan((current) => ({x: current.x - event.deltaX, y: current.y - event.deltaY}));
    };
    canvas.addEventListener('wheel', onWheel, {passive: false});
    return () => canvas.removeEventListener('wheel', onWheel);
  }, []);

  return (
    <Box
      ref={canvasRef}
      data-testid="explorer-canvas"
      onPointerDown={startDrag}
      onPointerMove={moveDrag}
      onPointerUp={endDrag}
      onPointerCancel={endDrag}
      onClick={(event) => {
        if (event.currentTarget.dataset['dragged'] === 'true') {
          delete event.currentTarget.dataset['dragged'];
          return;
        }
        onSelect(null);
      }}
      sx={{
        position: 'relative',
        flex: 1,
        minWidth: 0,
        overflow: 'hidden',
        cursor: dragging ? 'grabbing' : 'grab',
        touchAction: 'none',
        bgcolor: 'background.default',
        backgroundImage: 'radial-gradient(var(--oxygen-palette-divider) 1px, transparent 1px)',
        backgroundSize: '18px 18px',
      }}
    >
      <Box
        sx={{
          width: graph.width * zoom + 48,
          height: graph.height * zoom + 48,
          p: 3,
          boxSizing: 'border-box',
          transform: `translate(${String(pan.x)}px, ${String(pan.y)}px)`,
        }}
      >
        <Box
          sx={{
            position: 'relative',
            width: graph.width,
            height: graph.height,
            transform: `scale(${zoom})`,
            transformOrigin: '0 0',
          }}
        >
          {LANES.map((lane: Lane, index: number) => (
            <Box
              key={lane.id}
              data-testid={`explorer-lane-${lane.id}`}
              sx={{
                position: 'absolute',
                left: 0,
                top: laneTop(index),
                width: graph.width,
                height: STAGE.laneHeight,
                bgcolor: 'action.hover',
                border: 1,
                borderColor: 'divider',
                borderRadius: '8px',
                boxSizing: 'border-box',
              }}
            >
              <Stack direction="row" alignItems="center" spacing={1} sx={{position: 'absolute', left: 14, top: 12}}>
                <Box sx={{width: 8, height: 8, borderRadius: 0.5, bgcolor: lane.color}} />
                <Typography
                  variant="caption"
                  sx={{
                    fontSize: 11,
                    fontWeight: 600,
                    letterSpacing: '0.12em',
                    textTransform: 'uppercase',
                    color: 'text.secondary',
                  }}
                >
                  {t(lane.title[0].replace(/^deploy:/, ''), lane.title[1])}
                </Typography>
              </Stack>
            </Box>
          ))}
          <svg
            width={graph.width}
            height={graph.height}
            style={{position: 'absolute', left: 0, top: 0, pointerEvents: 'none', zIndex: 1}}
            aria-hidden
          >
            {graph.edges.map((edge: GraphEdge, index: number) => {
              const from = byKey.get(edge.from);
              const to = byKey.get(edge.to);
              if (!from || !to) return null;
              const path = edgePath(from, to, curved);
              const on = Boolean(focus && (edge.from === focus || edge.to === focus));
              const hovered = hoverEdge === index;
              const dim = Boolean(focus && !on && !hovered);
              return (
                <g key={`${edge.from}>${edge.to}>${edge.label.key}`}>
                  <path
                    d={path}
                    fill="none"
                    stroke={
                      on || hovered ? 'var(--oxygen-palette-primary-main)' : 'var(--oxygen-palette-text-disabled)'
                    }
                    strokeWidth={on || hovered ? 2 : 1.2}
                    opacity={dim ? 0.07 : 1}
                    style={{transition: 'opacity 150ms, stroke 150ms'}}
                  />
                  <path
                    d={path}
                    fill="none"
                    stroke="transparent"
                    strokeWidth={12}
                    style={{pointerEvents: 'stroke', cursor: 'help'}}
                    data-testid={`explorer-edge-${index}`}
                    onMouseEnter={(event) => showTip(index, event)}
                    onMouseMove={(event) => showTip(index, event)}
                    onMouseLeave={hideTip}
                  />
                </g>
              );
            })}
          </svg>
          {graph.nodes.map((node: GraphNode) => (
            <ResourceTile
              key={node.key}
              node={node}
              selected={selected === node.key}
              dimmed={Boolean(related && !related.has(node.key))}
              onSelect={onSelect}
              onHover={setHoverNode}
              heldBack={heldBack?.has(node.key)}
            />
          ))}
          {graph.overflow.map((entry: OverflowNode) => {
            const lane = LANE_BY_ID[entry.lane];
            return (
              <Box
                key={entry.lane}
                sx={{
                  position: 'absolute',
                  left: entry.x - 28,
                  top: entry.y - STAGE.tileSize / 2,
                  width: STAGE.tileSize,
                  height: STAGE.tileSize,
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  borderRadius: '8px',
                  border: `2px dashed ${lane.color}`,
                  color: lane.color,
                  fontSize: 13,
                  fontWeight: 600,
                  zIndex: 2,
                }}
              >
                +{entry.count}
              </Box>
            );
          })}
        </Box>
      </Box>
      {tip && (
        <Box
          role="tooltip"
          sx={{
            position: 'absolute',
            left: tip.x + 14,
            top: tip.y + 14,
            maxWidth: 280,
            px: 1.5,
            py: 1,
            bgcolor: 'background.paper',
            border: 1,
            borderColor: 'primary.main',
            borderRadius: 2,
            boxShadow: 3,
            pointerEvents: 'none',
            zIndex: 5,
          }}
        >
          <Typography variant="caption" color="text.secondary" sx={{display: 'block', fontSize: 11}}>
            {tip.pair}
          </Typography>
          <Typography variant="body2">{tip.label}</Typography>
        </Box>
      )}
      <Stack
        direction="row"
        alignItems="center"
        spacing={0.5}
        onClick={(event) => event.stopPropagation()}
        sx={{
          position: 'absolute',
          bottom: 12,
          right: 16,
          px: 0.5,
          bgcolor: 'background.paper',
          border: 1,
          borderColor: 'divider',
          boxShadow: 1,
          borderRadius: 100,
          color: 'text.primary',
        }}
      >
        {onToggleFullscreen && (
          <Tooltip
            title={
              fullscreen
                ? t('explorer.fullscreen.exit', 'Exit full screen')
                : t('explorer.fullscreen.enter', 'Full screen')
            }
          >
            <IconButton
              size="small"
              color="inherit"
              aria-label={
                fullscreen
                  ? t('explorer.fullscreen.exit', 'Exit full screen')
                  : t('explorer.fullscreen.enter', 'Full screen')
              }
              aria-pressed={fullscreen}
              onClick={onToggleFullscreen}
            >
              {fullscreen ? <Shrink size={14} /> : <Expand size={14} />}
            </IconButton>
          </Tooltip>
        )}
        <Tooltip title={t('explorer.zoom.fit', 'Fit to width')}>
          <IconButton size="small" color="inherit" aria-label={t('explorer.zoom.fit', 'Fit to width')} onClick={fit}>
            <Maximize size={14} />
          </IconButton>
        </Tooltip>
        <Tooltip title={t('explorer.zoom.out', 'Zoom out')}>
          <IconButton
            size="small"
            color="inherit"
            aria-label={t('explorer.zoom.out', 'Zoom out')}
            onClick={() => setZoom(clampZoom(zoom - ZOOM.step))}
          >
            <Minus size={14} />
          </IconButton>
        </Tooltip>
        <Typography
          variant="caption"
          sx={{minWidth: 36, textAlign: 'center', fontFamily: 'monospace'}}
          data-testid="explorer-zoom"
        >
          {Math.round(zoom * 100)}%
        </Typography>
        <Tooltip title={t('explorer.zoom.in', 'Zoom in')}>
          <IconButton
            size="small"
            color="inherit"
            aria-label={t('explorer.zoom.in', 'Zoom in')}
            onClick={() => setZoom(clampZoom(zoom + ZOOM.step))}
          >
            <Plus size={14} />
          </IconButton>
        </Tooltip>
      </Stack>
    </Box>
  );
}
