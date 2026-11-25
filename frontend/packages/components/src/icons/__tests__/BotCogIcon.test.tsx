// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render} from '@testing-library/react';
import {UserRoundCog} from '@wso2/oxygen-ui-icons-react';
import {describe, expect, it} from 'vitest';
import BotCogIcon from '../BotCogIcon';

function renderSvg(element: React.ReactElement): SVGSVGElement {
  const {container} = render(element);
  const svg = container.querySelector('svg');
  if (!svg) {
    throw new Error('Expected the icon to render an <svg> element');
  }
  return svg;
}

// The cog badge is every circle and path a Lucide cog icon draws around the circle centered on (18, 18).
function cogBadgeOf(svg: SVGSVGElement): string[] {
  const circle = svg.querySelector('circle[cx="18"][cy="18"]');
  const teeth = Array.from(svg.querySelectorAll('path'))
    .map((path) => path.getAttribute('d') ?? '')
    .filter((d) => /^m(1[4-9]|2[0-1])\./.test(d));
  return [circle?.outerHTML ?? '', ...teeth.sort()];
}

describe('BotCogIcon', () => {
  it('should be a Lucide icon, taking the props the other Oxygen icons take', () => {
    const svg = renderSvg(<BotCogIcon size={16} strokeWidth={1.5} />);

    expect(svg).toHaveClass('lucide', 'lucide-bot-cog');
    expect(svg).toHaveAttribute('viewBox', '0 0 24 24');
    expect(svg).toHaveAttribute('width', '16');
    expect(svg).toHaveAttribute('stroke-width', '1.5');
  });

  it('should carry the cog badge of UserRoundCog, unchanged', () => {
    const badge = cogBadgeOf(renderSvg(<BotCogIcon />));

    expect(badge).toHaveLength(9);
    expect(badge).toEqual(cogBadgeOf(renderSvg(<UserRoundCog />)));
  });
});
