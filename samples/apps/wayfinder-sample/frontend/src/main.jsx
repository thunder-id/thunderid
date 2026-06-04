/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com). All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { ThunderIDProvider } from "@thunderid/react";
import { BrowserRouter } from "react-router-dom";
import App from "./App.jsx";
import { NativeAuthProvider } from "./auth/NativeAuthContext.jsx";
import { AUTH_CONFIG, SCOPES } from "./auth/config.js";
import { nativeVerboseAuthExtensions } from "./pages/NativeVerboseAuthPage.jsx";
import "./styles.css";

const clientId = import.meta.env.VITE_THUNDER_CLIENT_ID;
const appId = import.meta.env.VITE_THUNDER_APP_ID;
const baseUrl = import.meta.env.VITE_THUNDER_BASE_URL;
const thunderidReady = Boolean(clientId && baseUrl);
const afterSignInUrl = AUTH_CONFIG.isRedirectBased ? window.location.origin : "";

createRoot(document.getElementById("root")).render(
  <StrictMode>
    <BrowserRouter>
      {thunderidReady ? (
        <ThunderIDProvider
          clientId={clientId}
          applicationId={appId}
          baseUrl={baseUrl}
          afterSignInUrl={afterSignInUrl}
          afterSignOutUrl={window.location.origin}
          scopes={SCOPES}
          extensions={nativeVerboseAuthExtensions}
          discovery={{ wellKnown: { enabled: true } }}
        >
          <NativeAuthProvider>
            <App authReady />
          </NativeAuthProvider>
        </ThunderIDProvider>
      ) : (
        <App authReady={false} />
      )}
    </BrowserRouter>
  </StrictMode>
);
