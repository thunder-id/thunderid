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

import {withVendorCSSClassPrefix} from '@thunderid/browser';
import {type Component, type SetupContext, type VNode, defineComponent, h} from 'vue';

type LogoProps = Readonly<{
  alt: string;
  height: string | number | undefined;
  href: string | undefined;
  src: string | undefined;
  width: string | number | undefined;
}>;

const Logo: Component = defineComponent({
  name: 'Logo',
  props: {
    alt: {default: 'Logo', type: String},
    height: {default: undefined, type: [String, Number]},
    href: {default: undefined, type: String},
    src: {default: undefined, type: String},
    width: {default: undefined, type: [String, Number]},
  },
  setup(props: LogoProps, {attrs}: SetupContext): () => VNode {
    return (): VNode => {
      const img: VNode = h('img', {
        alt: props.alt,
        class: withVendorCSSClassPrefix('logo__image'),
        height: props.height,
        src: props.src,
        width: props.width,
      });

      if (props.href) {
        return h(
          'a',
          {
            class: [withVendorCSSClassPrefix('logo'), (attrs.class as string) || ''].filter(Boolean).join(' '),
            href: props.href,
            style: attrs.style,
          },
          [img],
        );
      }

      return h(
        'div',
        {
          class: [withVendorCSSClassPrefix('logo'), (attrs.class as string) || ''].filter(Boolean).join(' '),
          style: attrs.style,
        },
        [img],
      );
    };
  },
});

export default Logo;
