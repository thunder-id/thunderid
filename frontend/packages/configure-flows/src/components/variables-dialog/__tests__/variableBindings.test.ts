// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import {rewriteBindings} from '../variableBindings';

describe('rewriteBindings', () => {
  it('points the bindings to a variable at the new identifier', () => {
    expect(rewriteBindings({a: '{{var(old)}}', b: ' {{ var( old ) }} ', c: 'plain'}, 'old', 'fresh')).toEqual({
      a: '{{var(fresh)}}',
      b: '{{var(fresh)}}',
      c: 'plain',
    });
  });

  it('takes the bindings off when there is no new identifier', () => {
    expect(rewriteBindings({a: '{{var(old)}}', c: 'plain'}, 'old', null)).toEqual({c: 'plain'});
  });

  it('leaves other variables, partial placeholders and non-strings alone', () => {
    expect(rewriteBindings({a: '{{var(other)}}', b: 'x {{var(old)}}', c: 3}, 'old', 'fresh')).toBeUndefined();
  });

  it('reads a placeholder only when its identifier is one the server accepts', () => {
    const longest = 'a'.repeat(64);

    expect(rewriteBindings({a: `{{var(${longest})}}`}, longest, 'fresh')).toEqual({a: '{{var(fresh)}}'});
    expect(rewriteBindings({a: `{{var(${longest}b)}}`}, `${longest}b`, 'fresh')).toBeUndefined();
  });

  it('reports nothing to change when there are no properties', () => {
    expect(rewriteBindings(undefined, 'old', 'fresh')).toBeUndefined();
  });
});
