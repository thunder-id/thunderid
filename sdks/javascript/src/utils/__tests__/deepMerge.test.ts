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

import {describe, expect, it} from 'vitest';
import deepMerge from '../deepMerge';

describe('deepMerge', (): void => {
  it('should merge simple objects', (): void => {
    const target: Record<string, unknown> = {a: 1, b: 2};
    const source: Record<string, unknown> = {b: 3, c: 4};
    const result: Record<string, unknown> = deepMerge(target, source as any);

    expect(result).toEqual({a: 1, b: 3, c: 4});
    expect(result).not.toBe(target); // Should create a new object
  });

  it('should merge nested objects recursively', (): void => {
    const target: Record<string, unknown> = {
      a: 1,
      b: {
        x: 1,
        y: 2,
      },
    };
    const source: Record<string, unknown> = {
      b: {
        y: 3,
        z: 4,
      },
      c: 3,
    };
    const result: Record<string, unknown> = deepMerge(target, source as any);

    expect(result).toEqual({
      a: 1,
      b: {x: 1, y: 3, z: 4},
      c: 3,
    });
  });

  it('should handle deeply nested objects', (): void => {
    const target: Record<string, unknown> = {
      theme: {
        colors: {
          primary: 'blue',
          secondary: 'green',
        },
        spacing: {
          small: 8,
        },
      },
    };
    const source: Record<string, unknown> = {
      theme: {
        colors: {
          accent: 'yellow',
          secondary: 'red',
        },
        typography: {
          fontSize: 16,
        },
      },
    };
    const result: Record<string, unknown> = deepMerge(target, source as any);

    expect(result).toEqual({
      theme: {
        colors: {
          accent: 'yellow',
          primary: 'blue',
          secondary: 'red',
        },
        spacing: {
          small: 8,
        },
        typography: {
          fontSize: 16,
        },
      },
    });
  });

  it('should replace arrays entirely instead of merging', (): void => {
    const target: Record<string, unknown> = {arr: [1, 2, 3]};
    const source: Record<string, unknown> = {arr: [4, 5]};
    const result: Record<string, unknown> = deepMerge(target, source);

    expect(result).toEqual({arr: [4, 5]});
  });

  it('should handle multiple sources', (): void => {
    const target: Record<string, unknown> = {a: 1, b: {x: 1}};
    const source1: Record<string, unknown> = {b: {y: 2}, c: 3};
    const source2: Record<string, unknown> = {b: {z: 3}, d: 4};
    const result: Record<string, unknown> = deepMerge(target, source1 as any, source2 as any);

    expect(result).toEqual({
      a: 1,
      b: {x: 1, y: 2, z: 3},
      c: 3,
      d: 4,
    });
  });

  it('should handle undefined and null sources', (): void => {
    const target: Record<string, unknown> = {a: 1, b: 2};
    const result: Record<string, unknown> = deepMerge(target, undefined, null as any, {c: 3} as any);

    expect(result).toEqual({a: 1, b: 2, c: 3});
  });

  it('should handle empty objects', (): void => {
    const target: Record<string, unknown> = {};
    const source: Record<string, unknown> = {a: 1, b: {x: 2}};
    const result: Record<string, unknown> = deepMerge(target, source);

    expect(result).toEqual({a: 1, b: {x: 2}});
  });

  it('should not modify the original objects', (): void => {
    const target: Record<string, unknown> = {a: 1, b: {x: 1}};
    const source: Record<string, unknown> = {b: {y: 2}, c: 3};
    const originalTarget: Record<string, unknown> = JSON.parse(JSON.stringify(target));
    const originalSource: Record<string, unknown> = JSON.parse(JSON.stringify(source));

    deepMerge(target, source as any);

    expect(target).toEqual(originalTarget);
    expect(source).toEqual(originalSource);
  });

  it('should handle special object types correctly', (): void => {
    const date: Date = new Date('2023-01-01');
    const regex = /test/g;
    const target: Record<string, unknown> = {a: 1};
    const source: Record<string, unknown> = {
      date,
      func: () => 'test',
      regex,
    };
    const result: Record<string, unknown> = deepMerge(target, source as any);

    expect((result as any).date).toBe(date);
    expect((result as any).regex).toBe(regex);
    expect(typeof (result as any).func).toBe('function');
  });

  it('should handle nested special objects', (): void => {
    const target: Record<string, unknown> = {
      config: {
        timeout: 1000,
      },
    };
    const source: Record<string, unknown> = {
      config: {
        date: new Date('2023-01-01'),
        patterns: [/test/g, /example/i],
      },
    };
    const result: Record<string, unknown> = deepMerge(target, source as any);

    expect((result as any).config.timeout).toBe(1000);
    expect((result as any).config.date).toBeInstanceOf(Date);
    expect(Array.isArray((result as any).config.patterns)).toBe(true);
  });

  it('should handle boolean and number values', (): void => {
    const target: Record<string, unknown> = {count: 5, enabled: true};
    const source: Record<string, unknown> = {active: true, count: 10, enabled: false};
    const result: Record<string, unknown> = deepMerge(target, source);

    expect(result).toEqual({active: true, count: 10, enabled: false});
  });

  it('should handle string values', (): void => {
    const target: Record<string, unknown> = {name: 'John', nested: {title: 'Mr.'}};
    const source: Record<string, unknown> = {name: 'Jane', nested: {surname: 'Doe', title: 'Ms.'}};
    const result: Record<string, unknown> = deepMerge(target, source);

    expect(result).toEqual({
      name: 'Jane',
      nested: {surname: 'Doe', title: 'Ms.'},
    });
  });

  it('should throw error for non-object target', (): void => {
    expect(() => deepMerge(null as any)).toThrow('Target must be an object');
    expect(() => deepMerge(undefined as any)).toThrow('Target must be an object');
    expect(() => deepMerge('string' as any)).toThrow('Target must be an object');
    expect(() => deepMerge(123 as any)).toThrow('Target must be an object');
  });

  it('should handle complex real-world scenario', (): void => {
    const defaultConfig: Record<string, unknown> = {
      api: {
        baseUrl: 'https://api.example.com',
        retries: 3,
        timeout: 5000,
      },
      features: {
        analytics: true,
        debug: false,
      },
      ui: {
        components: {
          button: {
            borderRadius: 4,
          },
        },
        theme: {
          colors: {
            primary: '#007bff',
            secondary: '#6c757d',
          },
          spacing: {
            md: 16,
            sm: 8,
            xs: 4,
          },
        },
      },
    };

    const userConfig: Record<string, unknown> = {
      api: {
        baseUrl: 'https://custom-api.example.com',
        headers: {
          'X-Custom': 'value',
        },
      },
      features: {
        debug: true,
        experimental: true,
      },
      ui: {
        components: {
          input: {
            borderWidth: 2,
          },
        },
        theme: {
          colors: {
            primary: '#ff0000',
          },
          spacing: {
            lg: 32,
          },
        },
      },
    };

    const result: Record<string, unknown> = deepMerge(defaultConfig, userConfig as any);

    expect(result).toEqual({
      api: {
        baseUrl: 'https://custom-api.example.com',
        headers: {
          'X-Custom': 'value',
        },
        retries: 3,
        timeout: 5000,
      },
      features: {
        analytics: true,
        debug: true,
        experimental: true,
      },
      ui: {
        components: {
          button: {
            borderRadius: 4,
          },
          input: {
            borderWidth: 2,
          },
        },
        theme: {
          colors: {
            primary: '#ff0000',
            secondary: '#6c757d',
          },
          spacing: {
            lg: 32,
            md: 16,
            sm: 8,
            xs: 4,
          },
        },
      },
    });
  });

  it('should not overwrite with undefined from source', () => {
    const target: Record<string, unknown> = {a: 1, b: 2};
    const source: Record<string, unknown> = {a: undefined, b: undefined};
    const result: Record<string, unknown> = deepMerge(target, source);
    expect(result).toEqual({a: 1, b: 2});
  });

  it('should overwrite with null from source', () => {
    const target: Record<string, unknown> = {a: 1, b: {x: 1}};
    const source: Record<string, unknown> = {a: null, b: null};
    const result: Record<string, unknown> = deepMerge(target, source);
    expect(result).toEqual({a: null, b: null});
  });

  it('should not mutate original nested objects', () => {
    const target: Record<string, unknown> = {a: {x: 1}, b: {y: 2}};
    const source: Record<string, unknown> = {a: {z: 3}};
    const result: Record<string, unknown> = deepMerge(target, source);
    // mutate originals
    (target.a as any).x = 999;
    (source.a as any).z = 777;
    expect(result).toEqual({a: {x: 1, z: 3}, b: {y: 2}});
  });

  it('should handle multiple sources with nested merges', () => {
    const target: Record<string, unknown> = {cfg: {depth: 1, mode: 'a'}, k: 1};
    const s1: Record<string, unknown> = {cfg: {mode: 'b'}, k: 2};
    const s2: Record<string, unknown> = {cfg: {extra: true, mode: 'c'}, k: 3};
    const result: Record<string, unknown> = deepMerge(target, s1, s2);
    expect(result).toEqual({cfg: {depth: 1, extra: true, mode: 'c'}, k: 3});
  });

  it('should replace non-plain with plain (and vice versa) instead of merging', () => {
    const d: Date = new Date('2024-01-01');
    const target: Record<string, unknown> = {a: d, b: {x: 1}, c: /re/g, f: () => 1};
    const source: Record<string, unknown> = {a: {y: 2}, b: new Date('2024-02-02'), c: {z: 3}, f: {k: 1}};
    const result: Record<string, unknown> = deepMerge(target, source as any);
    // a: Date -> plain object (replace)
    expect(result.a).toEqual({y: 2});
    // b: plain -> Date (replace)
    expect(result.b).toBeInstanceOf(Date);
    // c: RegExp -> plain object (replace)
    expect(result.c).toEqual({z: 3});
    // f: function -> plain object (replace)
    expect((result as any).f).toEqual({k: 1});
  });

  it('should replace nested arrays instead of merging them', () => {
    const target: Record<string, unknown> = {cfg: {list: [1, 2, 3], other: 1}};
    const source: Record<string, unknown> = {cfg: {list: ['a']}};
    const result: Record<string, unknown> = deepMerge(target, source);
    expect(result).toEqual({cfg: {list: ['a'], other: 1}});
  });

  it('should not add keys for undefined-only sources', () => {
    const target: Record<string, unknown> = {a: 1};
    const source: Record<string, unknown> = {b: undefined};
    const result: Record<string, unknown> = deepMerge(target, source);
    expect(result).toEqual({a: 1});
    expect('b' in result).toBe(false);
  });
});
