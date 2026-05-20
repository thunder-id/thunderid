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

import {ExtendedAuthorizeRequestUrlParams, OIDCRequestConstants} from '@thunderid/javascript';
import {
  CHECK_SESSION_SIGNED_IN,
  CHECK_SESSION_SIGNED_OUT,
  INITIALIZED_SILENT_SIGN_IN,
  OP_IFRAME,
  PROMPT_NONE_IFRAME,
  RP_IFRAME,
  SET_SESSION_STATE_FROM_IFRAME,
  SILENT_SIGN_IN_STATE,
  STATE,
  STATE_QUERY,
} from '../constants/SPAConstants';
import SPAUtils from './SPAUtils';

interface AuthorizationInfo {
  code: string;
  sessionState: string;
  state: string;
}

interface Message<T> {
  type: string;
  data?: T;
}

/**
 * Interface for the session management helper returned by {@link createSessionManagementHelper}.
 */
export interface SessionManagementHelperInterface {
  /** Starts session polling and sets up the RP iframe. */
  initialize(
    clientId: string,
    checkSessionEndpoint: string,
    getSessionState: () => Promise<string>,
    interval: number,
    sessionRefreshInterval: number,
    redirectURL: string,
    getSignInUrl: (params?: ExtendedAuthorizeRequestUrlParams) => Promise<string>,
  ): void;
  /**
   * Processes a prompt-none response when the page is loaded inside the prompt-none iframe.
   * Returns `true` if the current page load was a prompt-none response.
   */
  receivePromptNoneResponse(setSessionState?: (sessionState: string | null) => Promise<void>): Promise<boolean>;
  /** Clears the session check and refresh intervals. */
  reset(): void;
}

/**
 * Factory that creates an OIDC Session Management helper using an RP iframe and polling.
 * Appends a hidden RP iframe to the document and returns the helper interface.
 *
 * @param signOut - Returns the sign-out URL for the current session.
 * @param setSessionState - Stores a new session state from a prompt-none response.
 * @returns A `SessionManagementHelperInterface` instance.
 */
export const createSessionManagementHelper = async (
  signOut: () => Promise<string>,
  setSessionState: (sessionState: string) => void,
): Promise<SessionManagementHelperInterface> => {
  let _clientID: string;
  let _checkSessionEndpoint: string;
  let _sessionState: () => Promise<string>;
  let _interval: number;
  let _redirectURL: string;
  let _sessionRefreshInterval: number;
  let _sessionRefreshIntervalTimeout: number;
  let _checkSessionIntervalTimeout: number;
  let _getSignInUrl: (params?: ExtendedAuthorizeRequestUrlParams) => Promise<string>;

  const initialize = (
    clientId: string,
    checkSessionEndpoint: string,
    getSessionState: () => Promise<string>,
    interval: number,
    sessionRefreshInterval: number,
    redirectURL: string,
    getSignInUrl: (params?: ExtendedAuthorizeRequestUrlParams) => Promise<string>,
  ): void => {
    _clientID = clientId;
    _checkSessionEndpoint = checkSessionEndpoint;
    _sessionState = getSessionState;
    _interval = interval;
    _redirectURL = redirectURL;
    _sessionRefreshInterval = sessionRefreshInterval;
    _getSignInUrl = getSignInUrl;

    if (_interval > -1) {
      initiateCheckSession();
    }

    if (_sessionRefreshInterval > -1) {
      _sessionRefreshIntervalTimeout = setInterval(() => {
        sendPromptNoneRequest();
      }, _sessionRefreshInterval * 1000) as unknown as number;
    }
  };

  const initiateCheckSession = async (): Promise<void> => {
    if (!_checkSessionEndpoint || !_clientID || !_redirectURL) {
      return;
    }

    async function checkSession(): Promise<void> {
      const sessionState = await _sessionState();
      if (Boolean(_clientID) && Boolean(sessionState)) {
        const message = `${_clientID} ${sessionState}`;
        const rpIFrame = document.getElementById(RP_IFRAME) as HTMLIFrameElement;
        const opIframe: HTMLIFrameElement = rpIFrame?.contentDocument?.getElementById(OP_IFRAME) as HTMLIFrameElement;
        const win: Window | null = opIframe.contentWindow;
        win?.postMessage(message, _checkSessionEndpoint);
      }
    }

    const rpIFrame = document.getElementById(RP_IFRAME) as HTMLIFrameElement;
    const opIframe: HTMLIFrameElement = rpIFrame?.contentDocument?.getElementById(OP_IFRAME) as HTMLIFrameElement;
    opIframe.src = _checkSessionEndpoint + '?client_id=' + _clientID + '&redirect_uri=' + _redirectURL;

    _checkSessionIntervalTimeout = setInterval(checkSession, _interval * 1000) as unknown as number;

    listenToResponseFromOPIFrame();
  };

  const reset = (): void => {
    clearInterval(_checkSessionIntervalTimeout);
    clearInterval(_sessionRefreshIntervalTimeout);
  };

  const listenToResponseFromOPIFrame = (): void => {
    async function receiveMessage(e: MessageEvent) {
      const targetOrigin = _checkSessionEndpoint;

      if (!targetOrigin || targetOrigin?.indexOf(e.origin) < 0 || e?.data?.type === SET_SESSION_STATE_FROM_IFRAME) {
        return;
      }

      if (e.data === 'unchanged') {
        // session state has not changed
      } else if (e.data === 'error') {
        window.location.href = await signOut();
      } else if (e.data === 'changed') {
        sendPromptNoneRequest();
      }
    }

    window?.addEventListener('message', receiveMessage, false);
  };

  const sendPromptNoneRequest = async () => {
    const rpIFrame = document.getElementById(RP_IFRAME) as HTMLIFrameElement;
    const promptNoneIFrame: HTMLIFrameElement = rpIFrame?.contentDocument?.getElementById(
      PROMPT_NONE_IFRAME,
    ) as HTMLIFrameElement;

    if (SPAUtils.canSendPromptNoneRequest()) {
      SPAUtils.setPromptNoneRequestSent(true);

      const receiveMessageListener = (e: MessageEvent<Message<string>>) => {
        if (e?.data?.type === SET_SESSION_STATE_FROM_IFRAME) {
          setSessionState(e?.data?.data ?? '');
          window?.removeEventListener('message', receiveMessageListener);
        }
      };

      window?.addEventListener('message', receiveMessageListener);

      const promptNoneURL: string = await _getSignInUrl({
        prompt: 'none',
        response_mode: 'query',
        state: STATE,
      });

      promptNoneIFrame.src = promptNoneURL;
    }
  };

  const receivePromptNoneResponse = async (
    setSessionStateFn?: (sessionState: string | null) => Promise<void>,
  ): Promise<boolean> => {
    const state = new URL(window.location.href).searchParams.get(STATE_QUERY);
    const sessionState = new URL(window.location.href).searchParams.get(OIDCRequestConstants.Params.SESSION_STATE);
    const parent = window.parent.parent;

    if (state !== null && (state.includes(STATE) || state.includes(SILENT_SIGN_IN_STATE))) {
      const code = new URL(window.location.href).searchParams.get('code');

      if (code !== null && code.length !== 0) {
        if (state.includes(SILENT_SIGN_IN_STATE)) {
          const message: Message<AuthorizationInfo> = {
            data: {
              code,
              sessionState: sessionState ?? '',
              state,
            },
            type: CHECK_SESSION_SIGNED_IN,
          };

          sessionStorage.setItem(INITIALIZED_SILENT_SIGN_IN, 'false');
          parent.postMessage(message, parent.origin);
          SPAUtils.setPromptNoneRequestSent(false);

          window.location.href = 'about:blank';

          await SPAUtils.waitTillPageRedirect();

          return true;
        }

        const newSessionState = new URL(window.location.href).searchParams.get('session_state');

        setSessionStateFn && (await setSessionStateFn(newSessionState));

        SPAUtils.setPromptNoneRequestSent(false);

        window.location.href = 'about:blank';

        await SPAUtils.waitTillPageRedirect();

        return true;
      } else {
        if (state.includes(SILENT_SIGN_IN_STATE)) {
          const message: Message<null> = {
            type: CHECK_SESSION_SIGNED_OUT,
          };

          window.parent.parent.postMessage(message, parent.origin);
          SPAUtils.setPromptNoneRequestSent(false);

          window.location.href = 'about:blank';

          await SPAUtils.waitTillPageRedirect();

          return true;
        }

        SPAUtils.setPromptNoneRequestSent(false);

        const signOutURL = await signOut();
        parent.location.href = signOutURL;
        window.location.href = 'about:blank';

        await SPAUtils.waitTillPageRedirect();

        return true;
      }
    }

    return false;
  };

  let rpIFrame = document.createElement('iframe');
  rpIFrame.setAttribute('id', RP_IFRAME);
  rpIFrame.style.display = 'none';

  let rpIframeLoaded = false;
  rpIFrame.onload = () => {
    rpIFrame = document.getElementById(RP_IFRAME) as HTMLIFrameElement;

    const rpDoc = rpIFrame?.contentDocument;

    const opIFrame = rpDoc?.createElement('iframe');
    if (opIFrame) {
      opIFrame.setAttribute('id', OP_IFRAME);
      opIFrame.style.display = 'none';
    }

    const promptNoneIFrame = rpDoc?.createElement('iframe');
    if (promptNoneIFrame) {
      promptNoneIFrame.setAttribute('id', PROMPT_NONE_IFRAME);
      promptNoneIFrame.style.display = 'none';
    }

    opIFrame && rpIFrame?.contentDocument?.body?.appendChild(opIFrame);
    promptNoneIFrame && rpIFrame?.contentDocument?.body?.appendChild(promptNoneIFrame);

    rpIframeLoaded = true;
  };

  document?.body?.appendChild(rpIFrame);

  const sleep = (): Promise<void> => new Promise((resolve) => setTimeout(resolve, 1));
  while (!rpIframeLoaded) {
    await sleep();
  }

  return {
    initialize,
    receivePromptNoneResponse,
    reset,
  };
};

export default createSessionManagementHelper;
