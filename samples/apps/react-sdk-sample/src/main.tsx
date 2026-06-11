/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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
import "./index.css";
import App from "./App.tsx";
import { ThunderIDProvider } from "@thunderid/react";
import { ConfigurationError } from "./ConfigurationError.tsx";
import config from "./config.tsx";

const baseUrl = config.baseUrl;
const clientId = config.clientId;

// Validate required configuration
const missingConfig: string[] = [];
if (!baseUrl) {
    missingConfig.push("baseUrl");
}
if (!clientId) {
    missingConfig.push("clientId");
}

if (missingConfig.length > 0) {
    console.error(
        "⚠️ Missing required configuration:",
        missingConfig.join(", "),
    );
    console.error(
        "Please configure these values in public/runtime.json. See the documentation for reference.",
    );
}

createRoot(document.getElementById("root")!).render(
    <StrictMode>
        {missingConfig.length > 0 ? (
            <ConfigurationError missingConfig={missingConfig} />
        ) : (
            <ThunderIDProvider
                baseUrl={baseUrl}
                clientId={clientId}
                scopes={config.scopes}
            >
                <App />
            </ThunderIDProvider>
        )}
    </StrictMode>,
);
