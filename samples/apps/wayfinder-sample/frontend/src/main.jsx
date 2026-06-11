/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { ThunderIDProvider } from "@thunderid/react";
import { BrowserRouter } from "react-router-dom";
import App from "./App.jsx";
import "./styles.css";

const clientId = import.meta.env.VITE_THUNDER_CLIENT_ID;
const baseUrl = import.meta.env.VITE_THUNDER_BASE_URL;
const thunderidReady = Boolean(clientId && baseUrl);

const SCOPES = ["openid", "profile", "email", "ou", "agent:access", "booking:read", "booking:create", "booking:cancel"];

createRoot(document.getElementById("root")).render(
  <StrictMode>
    <BrowserRouter>
      {thunderidReady ? (
        <ThunderIDProvider
          clientId={clientId}
          baseUrl={baseUrl}
          afterSignInUrl={window.location.origin}
          afterSignOutUrl={window.location.origin}
          scopes={SCOPES}
          discovery={{ wellKnown: { enabled: true } }}
        >
          <App authReady />
        </ThunderIDProvider>
      ) : (
        <App authReady={false} />
      )}
    </BrowserRouter>
  </StrictMode>
);
