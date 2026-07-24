/**
 * Copyright (c) 2025-2026, WSO2 LLC. (https://www.wso2.com).
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

/**
 * English (US) translations for applications
 * All namespaces organized in a single file for better maintainability
 */
const translations = {
  // ============================================================================
  // Common namespace - Shared translations across all applications
  // ============================================================================
  common: {
    // Actions
    'actions.add': 'Add',
    'actions.edit': 'Edit',
    'actions.delete': 'Delete',
    'actions.cancel': 'Cancel',
    'actions.save': 'Save',
    'actions.create': 'Create',
    'actions.update': 'Update',
    'actions.remove': 'Remove',
    'actions.search': 'Search',
    'actions.filter': 'Filter',
    'actions.reset': 'Reset',
    'actions.submit': 'Submit',
    'actions.close': 'Close',
    'actions.back': 'Back',
    'actions.next': 'Next',
    'actions.previous': 'Previous',
    'actions.confirm': 'Confirm',
    'actions.ok': 'OK',
    'actions.yes': 'Yes',
    'actions.no': 'No',
    'actions.continue': 'Continue',
    'actions.skip': 'Skip',
    'actions.finish': 'Finish',
    'actions.done': 'Done',
    'actions.refresh': 'Refresh',
    'actions.copy': 'Copy',
    'actions.copyId': 'Copy ID',
    'actions.copied': 'Copied!',
    'actions.download': 'Download',
    'actions.upload': 'Upload',
    'actions.export': 'Export',
    'actions.import': 'Import',
    'actions.openActionsMenu': 'Open actions menu',
    'actions.view': 'View',
    'actions.details': 'Details',
    'actions.settings': 'Settings',
    'actions.logout': 'Logout',
    'actions.login': 'Login',

    // Dictionary
    'dictionary.unknown': 'Unknown',

    // Short action words (used as button labels, etc.)
    show: 'Show',
    publish: 'Publish',
    saveDraft: 'Save Draft',
    common: 'Common',
    new: 'New',
    edit: 'Edit',
    delete: 'Delete',
    close: 'Close',
    back: 'Back',
    create: 'Create',
    update: 'Update',
    save: 'Save',
    cancel: 'Cancel',
    or: 'or',

    // Page names (for breadcrumbs and navigation)
    home: 'Home',
    flows: 'Flows',

    // Status messages
    'status.loading': 'Loading...',
    'status.saving': 'Saving...',
    'status.deleting': 'Deleting...',
    'status.success': 'Success',
    'status.error': 'Error',
    'status.warning': 'Warning',
    'status.info': 'Info',
    'status.pending': 'Pending',
    'status.active': 'Active',
    'status.inactive': 'Inactive',
    'status.enabled': 'Enabled',
    'status.disabled': 'Disabled',
    'status.readOnly': 'Read Only',
    'status.completed': 'Completed',
    'status.failed': 'Failed',

    // Form labels
    'edit.general.name.label': 'Name',
    'edit.general.description.label': 'Description',
    'form.email': 'Email',
    'form.password': 'Password',
    'form.username': 'Username',
    'form.required': 'Required',
    'form.optional': 'Optional',
    'form.requiredField': 'This field is required',
    'form.invalidEmail': 'Invalid email address',
    'form.invalidFormat': 'Invalid format',
    'form.searchPlaceholder': 'Search...',

    // Messages
    'messages.confirmDelete': 'Are you sure you want to delete this item?',
    'messages.deleteSuccess': 'Item deleted successfully',
    'messages.deleteError': 'Failed to delete item',
    'messages.saveSuccess': 'Saved successfully',
    'messages.saveError': 'Failed to save',
    'messages.updateSuccess': 'Updated successfully',
    'messages.updateError': 'Failed to update',
    'messages.createSuccess': 'Created successfully',
    'messages.createError': 'Failed to create',
    'messages.noData': 'No data available',
    'messages.noResults': 'No results found',
    'messages.somethingWentWrong': 'Something went wrong',
    'messages.tryAgain': 'Please try again',

    // Navigation
    'navigation.home': 'Home',
    'navigation.dashboard': 'Dashboard',
    'navigation.profile': 'Profile',
    'navigation.help': 'Help',
    'navigation.documentation': 'Documentation',

    // User menu
    'userMenu.profile': 'Profile',
    'userMenu.myAccount': 'My account',
    'userMenu.addAnotherAccount': 'Add another account',
    'userMenu.settings': 'Settings',
    'userMenu.welcome': 'Welcome',
    'userMenu.signOut': 'Sign Out',

    // Welcome screen
    'welcome.header': 'Welcome',
    'welcome.dismissed': 'Welcome window can be reopened through the user dropdown menu.',
    'welcome.sections.start': 'Start',
    'welcome.sections.recent': 'Recent',
    'welcome.sections.tryoutProduct': 'Try Sample',
    'welcome.tryoutProduct.securingApplication': 'Secured Web Application',
    'welcome.tryoutProduct.securingApplicationDesc': 'Tryout user journeys of a secured web application',
    'welcome.tryoutProduct.aiAgents': 'Secured AI Agent',
    'welcome.tryoutProduct.aiAgentsDesc': 'Tryout identity security patterns for AI agents and tools',
    'welcome.tryoutProduct.mcp': 'Secured MCP Server',
    'welcome.tryoutProduct.mcpDesc': 'Tryout authorizing MCP client to a secured MCP server',
    'welcome.start.newProject': 'New',
    'welcome.start.newProjectDesc': 'Onboard your first resource to start',
    'welcome.start.openImport': 'Import',
    'welcome.start.openImportDesc': 'Import an existing {{productName}} configuration file',
    'welcome.start.startSamples': 'Start with samples',
    'welcome.start.connectTo': 'Connect to \u2026',
    'welcome.noRecentItems': 'No recent items found',
    'welcome.hero.titlePrefix': 'Welcome to',
    'welcome.hero.subtitle': 'Secure your digital resources and manage user identities and access',
    'welcome.walkthrough.getStartedDesigner': 'Get started',
    'welcome.walkthrough.getStartedDesignerDesc': 'Learn how to design and customize your identity experience',
    'welcome.walkthrough.learnFundamentals': 'Learn the Fundamentals',
    'welcome.walkthrough.learnFundamentalsDesc': 'Understand core concepts and architecture',
    'welcome.createProject.breadcrumb': 'New',
    'welcome.createProject.title': 'How {{productName}} Works!',
    'welcome.createProject.subtitle':
      'Onboard your digital resources, configure security, verify the setup, and run {{productName}} with your configuration in production deployment.',
    'welcome.createProject.cards.configure.title': 'Configure',
    'welcome.createProject.cards.configure.description':
      'Set up authentication flows, choose sign-in methods, and customize the user experience.',
    'welcome.createProject.cards.verify.title': 'Verify',
    'welcome.createProject.cards.verify.description': 'Test your configuration to ensure everything works as expected.',
    'welcome.createProject.cards.runServer.title': 'Run Server',
    'welcome.createProject.cards.runServer.description':
      '{{productName}} will run in immutable mode with the attached configurations.',
    'welcome.createProject.actions.getStarted': 'Get Started',
    'welcome.wayfinderSampleSetup.title': 'Setup Wayfinder Sample',
    'welcome.wayfinderSampleSetup.oneTimeSetup': 'One-time setup',
    'welcome.wayfinderSampleSetup.setupComplete': 'Already set up — you can skip this section',
    'welcome.wayfinderSampleSetup.steps.getSample.title': 'Get the Wayfinder Sample',
    'welcome.wayfinderSampleSetup.steps.getSample.description':
      'Download the latest Wayfinder sample distribution and extract the archive. It ships with the <strong>web frontend</strong>, <strong>AI agent service</strong>, <strong>MCP</strong>, <strong>SMTP mail server</strong> and <strong>Wayfinder server</strong>.',
    'welcome.wayfinderSampleSetup.steps.configure.title': 'Configure Wayfinder Sample in {{productName}}',
    'welcome.wayfinderSampleSetup.steps.configure.description':
      'Apply the Wayfinder sample configurations to {{productName}}. Skip this step if already done.',
    'welcome.wayfinderSampleSetup.steps.run.title': 'Run the Sample',
    'welcome.wayfinderSampleSetup.steps.run.description':
      'Start all Wayfinder services from the extracted sample directory.',
    'welcome.wayfinderSampleDownload.downloadButton': 'Download',

    'welcome.wayfinderFolderImport.actions.selectFolder': 'Select Wayfinder Sample Folder',
    'welcome.wayfinderFolderImport.actions.change': 'Change',
    'welcome.wayfinderFolderImport.actions.importConfig': 'Configure in {{productName}}',
    'welcome.wayfinderFolderImport.actions.reconfigure': 'Reconfigure',
    'welcome.wayfinderFolderImport.actions.reSelectFolder': 'Re-select Folder',
    'welcome.wayfinderFolderImport.status.importing': 'Importing configuration…',
    'welcome.wayfinderFolderImport.status.alreadyDone': 'Wayfinder sample already configured in {{productName}}',
    'welcome.wayfinderFolderImport.status.lastImported': 'Last configured on {{date}} — you can skip this step.',
    'welcome.wayfinderFolderImport.status.success': 'Wayfinder sample configured in {{productName}} successfully',
    'welcome.wayfinderFolderImport.status.resourcesImported_one': '{{count}} resource imported',
    'welcome.wayfinderFolderImport.status.resourcesImported_other': '{{count}} resources imported',
    'welcome.wayfinderFolderImport.status.envNotFound': 'thunderid-config/*.env — not found',
    'welcome.wayfinderFolderImport.errors.cannotReadFolder': 'Could not read the selected folder.',
    'welcome.wayfinderFolderImport.errors.importFailed': 'Import failed. Please try again.',
    'welcome.wayfinderFolderImport.errors.partialFailure': 'Import completed with {{count}} failed resource(s).',

    'welcome.tryout.breadcrumb': 'Try It Out',
    'welcome.tryout.title': 'Try It Out',
    'welcome.tryout.actions.readDocs': 'Read Documentation for more guides and details',
    'welcome.tryout.importConfigs.title': 'Configure Wayfinder Sample in {{productName}}',
    'welcome.tryout.importConfigs.description': 'Apply the Wayfinder sample configurations to {{productName}}.',

    'welcome.getStarted.breadcrumb': 'Get started',
    'welcome.getStarted.actions.skipToConsole': 'Skip to console',
    'welcome.getStarted.title': 'Get Started',
    'welcome.getStarted.subtitle': 'Onboard your first resource and start securing it with {{productName}}.',
    'welcome.getStarted.options.onboardApp.title': 'Web Application',
    'welcome.getStarted.options.onboardApp.description':
      'Register your application in {{productName}} and integrate authentication step by step.',
    'welcome.getStarted.options.onboardApp.action': 'Add Application',
    'welcome.getStarted.options.secureAiAgent.title': 'Secure an AI Agent',
    'welcome.getStarted.options.secureAiAgent.description':
      'Protect your AI agents with token-based access control and scope enforcement.',
    'welcome.getStarted.options.secureMcp.title': 'Secure MCP Server / Client',
    'welcome.getStarted.options.secureMcp.description':
      'Authorize MCP clients to access your MCP server with fine-grained permissions.',
    'welcome.getStarted.options.comingSoon': 'Coming Soon',
    'welcome.getStarted.options.skip.title': 'Skip for now',
    'welcome.getStarted.options.skip.description':
      'Head straight to the console and explore {{productName}} on your own.',
    'welcome.getStarted.options.skip.action': 'Go to Console',

    'welcome.applicationTryout.breadcrumb': 'Tryout Secured Web Application',
    'welcome.applicationTryout.overline': 'Secured Web Application',
    'welcome.applicationTryout.title': 'Secure Your Application',
    'welcome.applicationTryout.subtitle':
      'Run web application use cases against the Wayfinder sample, a fictional travel-booking application.',
    'welcome.applicationTryout.steps.getSample.title': 'Get the Wayfinder Sample',
    'welcome.applicationTryout.steps.getSample.description':
      'Download the latest Wayfinder sample distribution and extract the archive. It ships with the web frontend, AI agent service, MCP, SMTP mail server and Wayfinder server.',
    'welcome.applicationTryout.steps.getSample.action': 'Download Sample',
    'welcome.applicationTryout.steps.importConfigs.title': 'Configure Wayfinder Sample in {{productName}}',
    'welcome.applicationTryout.steps.importConfigs.description':
      'Apply the Wayfinder sample configurations to {{productName}}.',
    'welcome.applicationTryout.steps.runSample.title': 'Run the Sample',
    'welcome.applicationTryout.steps.runSample.description':
      'Start all Wayfinder services from the extracted sample directory.',
    'welcome.applicationTryout.steps.runSample.action': 'See Run Instructions',
    'welcome.applicationTryout.steps.login.title': 'Sign-In to the App',
    'welcome.applicationTryout.steps.login.description':
      'Open the Wayfinder sample app and sign in with the demo credentials below.',

    'welcome.applicationTryout.scenarios.title': 'Try the following user journeys',
    'welcome.applicationTryout.scenarios.tabs.login': 'Sign-In',
    'welcome.applicationTryout.scenarios.tabs.signup': 'Self Sign-Up',
    'welcome.applicationTryout.scenarios.tabs.profile': 'View Profile',
    'welcome.applicationTryout.scenarios.tabs.recovery': 'Account Recovery',
    'welcome.applicationTryout.scenarios.tabs.onboard': 'Staff Sign-Up',
    'welcome.applicationTryout.scenarios.tabs.mfa': 'Multi-Factor Authentication',
    'welcome.applicationTryout.scenarios.tabs.social': 'Social Login',

    'welcome.applicationTryout.scenarios.login.description':
      'Sign in with the test user account to explore {{productName}} Sign in experience.',
    'welcome.applicationTryout.scenarios.login.step1': 'Open the Wayfinder app at <a>http://localhost:5173</a>.',
    'welcome.applicationTryout.scenarios.login.step2': 'Click Sign in and use the credentials below.',

    'welcome.applicationTryout.scenarios.signup.description':
      'Register a new customer account and see {{productName}} assign the Traveler role automatically on completion.',
    'welcome.applicationTryout.scenarios.signup.step1': 'Open <a>http://localhost:5173</a> and click Sign in.',
    'welcome.applicationTryout.scenarios.signup.step2': 'On the {{productName}} page, click Sign up.',
    'welcome.applicationTryout.scenarios.signup.step3': 'Fill in below sample details and click Continue.',
    'welcome.applicationTryout.scenarios.signup.sampleFields.username': 'Username',
    'welcome.applicationTryout.scenarios.signup.sampleFields.password': 'Password',
    'welcome.applicationTryout.scenarios.signup.sampleFields.email': 'Email',
    'welcome.applicationTryout.scenarios.signup.sampleFields.givenName': 'First name',
    'welcome.applicationTryout.scenarios.signup.sampleFields.familyName': 'Last name',
    'welcome.applicationTryout.scenarios.signup.sampleFields.mobileNumber': 'Mobile number',
    'welcome.applicationTryout.scenarios.signup.step4':
      'Fill in the registration form using these sample details and click Submit.',
    'welcome.applicationTryout.scenarios.signup.step5':
      '{{productName}} will create a Customer user and assign the Traveler role. The browser shows a confirmation screen with a link to redirect back to the Wayfinder app.',
    'welcome.applicationTryout.scenarios.profile.description':
      'Explore the self-service profile page - view account details, edit attributes, and change your password.',
    'welcome.applicationTryout.scenarios.profile.step1': 'Sign in as John at <a>http://localhost:5173</a>.',
    'welcome.applicationTryout.scenarios.profile.step2':
      'Click the username in the top-right corner and select Profile.',
    'welcome.applicationTryout.scenarios.profile.step3':
      'View account details, edit profile attributes, or change your password. The page calls {{productName}} directly with your session token.',

    'welcome.applicationTryout.scenarios.recovery.description':
      'Walk through the password recovery flow - John forgets his password and resets it via email.',
    'welcome.applicationTryout.scenarios.recovery.step1': 'Open <a>http://localhost:5173</a> and click Sign in.',
    'welcome.applicationTryout.scenarios.recovery.step2': 'On the {{productName}} sign-in page, click Forgot password?',
    'welcome.applicationTryout.scenarios.recovery.step3': 'Enter <code>john.doe</code> as the username and submit.',
    'welcome.applicationTryout.scenarios.recovery.step4':
      "{{productName}} sends a recovery email to John's registered address. Open it from the inbox at <mail>http://localhost:8788</mail>.",
    'welcome.applicationTryout.scenarios.recovery.step5': 'Click the reset link in the email and set a new password.',
    'welcome.applicationTryout.scenarios.recovery.step6': 'Sign in again with the new credentials.',

    'welcome.applicationTryout.scenarios.onboard.description':
      'Invite and onboard two new staff members entirely from the {{productName}} Console: Sam Rivera (Support) and Maya Patel (DestinationsAdmin). The admin picks the staff role and sends the invitation, and the matching role is attached automatically when the invitee completes their profile.',
    'welcome.applicationTryout.scenarios.onboard.smtpNote':
      "Before trying this flow, set flow.user_onboarding_flow_handle to wayfinder-onboarding-flow in {{productName}}'s deployment.yaml and restart the server.",
    'welcome.applicationTryout.scenarios.onboard.step1': 'Sign in to the {{productName}} Console as your admin user.',
    'welcome.applicationTryout.scenarios.onboard.step2': 'Navigate to Users and select Add User.',
    'welcome.applicationTryout.scenarios.onboard.step3': 'Select Staff as the user type.',
    'welcome.applicationTryout.scenarios.onboard.step4':
      "Pick Support as the role, enter Sam Rivera's email (sam.rivera@example.com), and click Send invitation. An invite link is emailed to Sam.",
    'welcome.applicationTryout.scenarios.onboard.step5':
      "Open Sam's invitation email from the inbox at <mail>http://localhost:8788</mail> and open the link. The browser opens a Complete Your Profile page.",
    'welcome.applicationTryout.scenarios.onboard.step6':
      "Fill in the additional attributes and submit. Sam's account is now active with the Support role attached.",
    'welcome.applicationTryout.scenarios.onboard.step7':
      'Repeat the flow for Maya Patel (email maya.patel@example.com), picking DestinationsAdmin as the role.',

    'welcome.aiAgentsTryout.breadcrumb': 'Tryout Secured AI Agent',
    'welcome.aiAgentsTryout.overline': 'Secured AI Agent',
    'welcome.aiAgentsTryout.subtitle':
      'Run AI agent use cases against the Wayfinder sample, a travel-booking app with a built-in AI chat assistant.',
    'welcome.aiAgentsTryout.steps.getSample.title': 'Get the Wayfinder Sample',
    'welcome.aiAgentsTryout.steps.getSample.description':
      'Download the latest Wayfinder sample distribution and extract the archive. It ships with the web frontend, AI agent service, MCP, SMTP mail server and Wayfinder server.',
    'welcome.aiAgentsTryout.steps.importConfigs.title': 'Configure Wayfinder Sample in {{productName}}',
    'welcome.aiAgentsTryout.steps.importConfigs.description':
      'Apply the Wayfinder sample configurations to {{productName}}.',
    'welcome.aiAgentsTryout.steps.configureSample.title': 'Configure the AI Provider Details in Wayfinder Sample',
    'welcome.aiAgentsTryout.steps.configureSample.description':
      'Copy each .env.example to .env in backend/, ai-agent/, and frontend/. Fill in your LLM API key in ai-agent/.env — an Anthropic key from console.anthropic.com or a Gemini key from aistudio.google.com.',
    'welcome.aiAgentsTryout.steps.runSample.title': 'Run the Sample',
    'welcome.aiAgentsTryout.steps.runSample.description':
      'Start all Wayfinder services from the extracted sample directory.',
    'welcome.aiAgentsTryout.steps.login.title': 'Sign-In to the App',
    'welcome.aiAgentsTryout.steps.login.description':
      'Open the Wayfinder sample app and sign in with the demo credentials below.',

    'welcome.aiAgentsTryout.apiKeySetup.getKey.title': 'Get an LLM API key',
    'welcome.aiAgentsTryout.apiKeySetup.getKey.description':
      'The AI agent needs access to an LLM. Obtain a free API key from one of the providers below.',
    'welcome.aiAgentsTryout.apiKeySetup.setKey.title': 'Add the key to ai-agent/.env',
    'welcome.aiAgentsTryout.apiKeySetup.setKey.description':
      'Open ai-agent/.env and set the key for the provider you chose.',

    'welcome.aiAgentsTryout.scenarios.title': 'Try the following AI agents security patterns',
    'welcome.aiAgentsTryout.scenarios.apiKeyNote':
      'The AI agent requires an LLM API key. Edit ai-agent/.env in the sample directory and set ANTHROPIC_API_KEY or GOOGLE_API_KEY before starting the services.',
    'welcome.aiAgentsTryout.scenarios.tabs.protect': 'Protect the Agent',
    'welcome.aiAgentsTryout.scenarios.tabs.browse': 'Browse with Agent',
    'welcome.aiAgentsTryout.scenarios.tabs.book': 'Book on Behalf',

    'welcome.aiAgentsTryout.scenarios.protect.description':
      'See scope-based access control in action - John can use the AI concierge, but Jane cannot.',
    'welcome.aiAgentsTryout.scenarios.protect.step1': 'Open <a>http://localhost:5173</a> and sign in as John Doe.',
    'welcome.aiAgentsTryout.scenarios.protect.step2':
      "Open the chat widget (bottom-right corner) and send any message. The concierge responds — John's token carries the <code>agent:access</code> scope.",
    'welcome.aiAgentsTryout.scenarios.protect.step3': 'Sign out and sign in as Jane Smith.',
    'welcome.aiAgentsTryout.scenarios.protect.step4':
      'Open the chat. Since Jane does not have the Wayfinder Chat User role. Chat agent will not be accessible and the widget will show an error message instead.',
    'welcome.aiAgentsTryout.scenarios.protect.johnLabel': 'John Have access to chat with Wayfinder chat agent',
    'welcome.aiAgentsTryout.scenarios.protect.janeLabel': 'Jane does not have access to chat with Wayfinder chat agent',

    'welcome.aiAgentsTryout.scenarios.browse.description':
      'Watch the agent use its own Machine-to-Machine (M2M) token to call read-only tools - no user consent popup required.',
    'welcome.aiAgentsTryout.scenarios.browse.step1':
      'Sign in as John at <a>http://localhost:5173</a> and open the chat widget.',
    'welcome.aiAgentsTryout.scenarios.browse.step2': 'Ask a browsing question in the chat:',
    'welcome.aiAgentsTryout.scenarios.browse.step3':
      'The agent calls the Wayfinder MCP server with its own M2M token (client_credentials grant). No popup appears.',
    'welcome.aiAgentsTryout.scenarios.browse.step4':
      'You can also try asking for flight deals — the agent calls the recommend_bookings tool, which requires the <code>booking:recommend</code> scope — granted to the Wayfinder Concierge via its Recommender role.',
    'welcome.aiAgentsTryout.scenarios.browse.step4Prompt': 'Suggest a few flight deals.',

    'welcome.aiAgentsTryout.scenarios.book.description':
      'Trigger the on-behalf-of consent flow - the agent pauses, asks for your permission, and only proceeds after you approve.',
    'welcome.aiAgentsTryout.scenarios.book.step1':
      'Sign in as John at <a>http://localhost:5173</a> and open the chat widget.',
    'welcome.aiAgentsTryout.scenarios.book.step2': 'Ask the agent to book something, for example:',
    'welcome.aiAgentsTryout.scenarios.book.step2Prompt': 'Book flight 2',
    'welcome.aiAgentsTryout.scenarios.book.step3':
      'The agent returns a consent request. A popup opens - sign in as John and select which booking permissions to grant (<code>booking:read</code>, <code>booking:create</code>, <code>booking:cancel</code>).',
    'welcome.aiAgentsTryout.scenarios.book.step4':
      "Click Authorize. The agent will retries the action using a user's context token. And you should see the booking confirmation in the chat window shortly after.",
    'welcome.aiAgentsTryout.scenarios.book.step5':
      'To see the rejection path, repeat the flow but deny <code>booking:create</code> in the consent screen. The agent returns a 403.',

    'welcome.mcpTryout.breadcrumb': 'Tryout Secured MCP Server',
    'welcome.mcpTryout.overline': 'Secured MCP Server',
    'welcome.mcpTryout.subtitle':
      'Connect an external MCP client to the Wayfinder MCP server, signed in through {{productName}}.',
    'welcome.mcpTryout.steps.prerequisite.title': 'Complete AI Agents Setup',
    'welcome.mcpTryout.steps.prerequisite.description':
      'This tryout extends the AI Agents environment. Complete the Securing AI Agents tryout setup first — the same bundle seeds the EXTERNAL-MCP-CLIENT application used here.',
    'welcome.mcpTryout.steps.importConfigs.title': 'Configure Wayfinder Sample in {{productName}}',
    'welcome.mcpTryout.steps.importConfigs.description':
      'Apply the Wayfinder sample configurations to {{productName}}.',
    'welcome.mcpTryout.steps.verifyApp.title': 'Verify the Application',
    'welcome.mcpTryout.steps.verifyApp.description':
      'In the {{productName}} Console, open Applications and confirm EXTERNAL-MCP-CLIENT is listed.',
    'welcome.mcpTryout.steps.verifyApp.action': 'Open Applications',
    'welcome.mcpTryout.steps.installInspector.title': 'Launch MCP Inspector',
    'welcome.mcpTryout.steps.installInspector.description':
      'Launch MCP Inspector locally — a browser-based reference UI for MCP servers with built-in OAuth support. Open a new terminal in the sample app directory and run:',
    'welcome.mcpTryout.steps.allowCors.title': 'Allow Inspector in CORS',
    'welcome.mcpTryout.steps.allowCors.description':
      "Add Inspector's origin to {{productName}}'s CORS allow-list in repository/conf/deployment.yaml, then restart {{productName}}.",

    'welcome.mcpTryout.scenarios.title': 'Try out the following test scenarios',
    'welcome.mcpTryout.scenarios.tabs.connect': 'Connect & Sign In',
    'welcome.mcpTryout.scenarios.tabs.permissions': 'Test Permissions',

    'welcome.mcpTryout.scenarios.connect.description':
      'Point MCP Inspector at the Wayfinder MCP server, authenticate through {{productName}}, and grant booking permissions at the consent screen.',
    'welcome.mcpTryout.scenarios.connect.step1':
      'Open <a>http://localhost:6274</a> in your browser (Inspector should already be running from the setup step above).',
    'welcome.mcpTryout.scenarios.connect.step2': 'Fill in the form with the details below.',
    'welcome.mcpTryout.scenarios.connect.step3': 'Expand "Authentication" and fill in the details below.',
    'welcome.mcpTryout.scenarios.connect.step4':
      'Click Connect. You are redirected to {{productName}} Sign in page — Sign in as John.',
    'welcome.mcpTryout.scenarios.connect.step5':
      'At the consent screen select the booking permissions to grant (<code>booking:read</code>, <code>booking:create</code>, <code>booking:cancel</code>) and confirm.',
    'welcome.mcpTryout.scenarios.connect.connectionLabel': 'Connection details',
    'welcome.mcpTryout.scenarios.connect.fields.transport': 'Transport Type',
    'welcome.mcpTryout.scenarios.connect.fields.serverUrl': 'URL',
    'welcome.mcpTryout.scenarios.connect.fields.connectionType': 'Connection Type',
    'welcome.mcpTryout.scenarios.connect.fields.clientId': 'Client ID',
    'welcome.mcpTryout.scenarios.connect.fields.clientSecret': 'Client Secret',
    'welcome.mcpTryout.scenarios.connect.fields.redirectUrl': 'Redirect URL',

    'welcome.mcpTryout.scenarios.permissions.description':
      'Call MCP tools and observe how {{productName}} enforces the scopes you granted at consent. Reconnect with different permissions to see the difference.',
    'welcome.mcpTryout.scenarios.permissions.step0':
      'At the consent screen select the booking permissions to grant (<code>booking:read</code>, <code>booking:create</code>, <code>booking:cancel</code>) and confirm.',
    'welcome.mcpTryout.scenarios.permissions.step1':
      'In the Tools tab, call create_booking. Set type=flight, itemId=flight-cmb-sin-01, travelers=1. The call succeeds if you granted <code>booking:create</code>.',
    'welcome.mcpTryout.scenarios.permissions.step2':
      'Call delete_all_bookings. If <code>booking:cancel</code> was not granted you get: "Insufficient scope for tool delete_all_bookings. Required: <code>booking:cancel</code>".',
    'welcome.mcpTryout.scenarios.permissions.step3':
      'To narrow or expand permissions, disconnect from Inspector and reconnect.',
    'welcome.mcpTryout.scenarios.permissions.step4':
      'Re-authenticate as john.doe and toggle different scopes at the consent screen, then retry the previously denied tool call — it now succeeds.',

    'welcome.setupComplete.breadcrumb': 'Complete',
    'welcome.setupComplete.title': "You're all set!",
    'welcome.setupComplete.subtitle':
      'The essential configuration is done. Start exploring the designer, build your project, onboard applications, and export the config when ready to run {{productName}}.',
    'welcome.setupComplete.exploreDashboard': 'Start Exploring',
    'welcome.setupComplete.nextSteps.onboardApp.title': 'Onboard an Application',
    'welcome.setupComplete.nextSteps.onboardApp.description':
      'Register your first application to start integrating sign-in and identity flows.',
    'welcome.setupComplete.nextSteps.onboardApp.action': 'Add Application',
    'welcome.setupComplete.nextSteps.exportConfig.title': 'Export Configuration',
    'welcome.setupComplete.nextSteps.exportConfig.description':
      'Export your {{productName}} configuration to deploy and run {{productName}}.',
    'welcome.setupComplete.nextSteps.exportConfig.action': 'Export Config',

    // Header
    'header.notifications': 'Coming soon',
    'header.openNotifications': 'Open notifications',

    // Data table - MUI DataGrid locale text
    // Root
    'dataTable.noRowsLabel': 'No rows',
    'dataTable.noResultsOverlayLabel': 'No results found.',
    'dataTable.noColumnsOverlayLabel': 'No columns',
    'dataTable.noColumnsOverlayManageColumns': 'Manage columns',

    // Density selector toolbar button text
    'dataTable.toolbarDensity': 'Density',
    'dataTable.toolbarDensityLabel': 'Density',
    'dataTable.toolbarDensityCompact': 'Compact',
    'dataTable.toolbarDensityStandard': 'Standard',
    'dataTable.toolbarDensityComfortable': 'Comfortable',

    // Columns selector toolbar button text
    'dataTable.toolbarColumns': 'Columns',
    'dataTable.toolbarColumnsLabel': 'Select columns',

    // Filters toolbar button text
    'dataTable.toolbarFilters': 'Filters',
    'dataTable.toolbarFiltersLabel': 'Show filters',
    'dataTable.toolbarFiltersTooltipHide': 'Hide filters',
    'dataTable.toolbarFiltersTooltipShow': 'Show filters',
    'dataTable.toolbarFiltersTooltipActive': (count: number) =>
      count !== 1 ? `${count} active filters` : `${count} active filter`,

    // Quick filter toolbar field
    'dataTable.toolbarQuickFilterPlaceholder': 'Search…',
    'dataTable.toolbarQuickFilterLabel': 'Search',
    'dataTable.toolbarQuickFilterDeleteIconLabel': 'Clear',

    // Export selector toolbar button text
    'dataTable.toolbarExport': 'Export',
    'dataTable.toolbarExportLabel': 'Export',
    'dataTable.toolbarExportCSV': 'Download as CSV',
    'dataTable.toolbarExportPrint': 'Print',

    // Columns management text
    'dataTable.columnsManagementSearchTitle': 'Search',
    'dataTable.columnsManagementNoColumns': 'No columns',
    'dataTable.columnsManagementShowHideAllText': 'Show/Hide All',
    'dataTable.columnsManagementReset': 'Reset',

    // Filter panel text
    'dataTable.filterPanelAddFilter': 'Add filter',
    'dataTable.filterPanelRemoveAll': 'Remove all',
    'dataTable.filterPanelDeleteIconLabel': 'Delete',
    'dataTable.filterPanelLogicOperator': 'Logic operator',
    'dataTable.filterPanelOperator': 'Operator',
    'dataTable.filterPanelOperatorAnd': 'And',
    'dataTable.filterPanelOperatorOr': 'Or',
    'dataTable.filterPanelColumns': 'Columns',
    'dataTable.filterPanelInputLabel': 'Value',
    'dataTable.filterPanelInputPlaceholder': 'Filter value',

    // Filter operators text
    'dataTable.filterOperatorContains': 'contains',
    'dataTable.filterOperatorDoesNotContain': 'does not contain',
    'dataTable.filterOperatorEquals': 'equals',
    'dataTable.filterOperatorDoesNotEqual': 'does not equal',
    'dataTable.filterOperatorStartsWith': 'starts with',
    'dataTable.filterOperatorEndsWith': 'ends with',
    'dataTable.filterOperatorIs': 'is',
    'dataTable.filterOperatorNot': 'is not',
    'dataTable.filterOperatorAfter': 'is after',
    'dataTable.filterOperatorOnOrAfter': 'is on or after',
    'dataTable.filterOperatorBefore': 'is before',
    'dataTable.filterOperatorOnOrBefore': 'is on or before',
    'dataTable.filterOperatorIsEmpty': 'is empty',
    'dataTable.filterOperatorIsNotEmpty': 'is not empty',
    'dataTable.filterOperatorIsAnyOf': 'is any of',

    // Filter values text
    'dataTable.filterValueAny': 'any',
    'dataTable.filterValueTrue': 'true',
    'dataTable.filterValueFalse': 'false',

    // Column menu text
    'dataTable.columnMenuLabel': 'Menu',
    'dataTable.columnMenuShowColumns': 'Show columns',
    'dataTable.columnMenuManageColumns': 'Manage columns',
    'dataTable.columnMenuFilter': 'Filter',
    'dataTable.columnMenuHideColumn': 'Hide column',
    'dataTable.columnMenuUnsort': 'Unsort',
    'dataTable.columnMenuSortAsc': 'Sort by ASC',
    'dataTable.columnMenuSortDesc': 'Sort by DESC',

    // Column header text
    'dataTable.columnHeaderFiltersTooltipActive': (count: number) =>
      count !== 1 ? `${count} active filters` : `${count} active filter`,
    'dataTable.columnHeaderFiltersLabel': 'Show filters',
    'dataTable.columnHeaderSortIconLabel': 'Sort',

    // Rows selected footer text
    'dataTable.footerRowSelected': (count: number) =>
      count !== 1 ? `${count.toLocaleString()} rows selected` : `${count.toLocaleString()} row selected`,

    // Total row amount footer text
    'dataTable.footerTotalRows': 'Total Rows:',

    // Total visible row amount footer text
    'dataTable.footerTotalVisibleRows': (visibleCount: number, totalCount: number) =>
      `${visibleCount.toLocaleString()} of ${totalCount.toLocaleString()}`,

    // Checkbox selection text
    'dataTable.checkboxSelectionHeaderName': 'Checkbox selection',
    'dataTable.checkboxSelectionSelectAllRows': 'Select all rows',
    'dataTable.checkboxSelectionUnselectAllRows': 'Unselect all rows',
    'dataTable.checkboxSelectionSelectRow': 'Select row',
    'dataTable.checkboxSelectionUnselectRow': 'Unselect row',

    // Boolean cell text
    'dataTable.booleanCellTrueLabel': 'yes',
    'dataTable.booleanCellFalseLabel': 'no',

    // Actions cell more text
    'dataTable.actionsCellMore': 'more',

    // Column pinning text
    'dataTable.pinToLeft': 'Pin to left',
    'dataTable.pinToRight': 'Pin to right',
    'dataTable.unpin': 'Unpin',

    // Tree Data
    'dataTable.treeDataGroupingHeaderName': 'Group',
    'dataTable.treeDataExpand': 'see children',
    'dataTable.treeDataCollapse': 'hide children',

    // Grouping columns
    'dataTable.groupingColumnHeaderName': 'Group',
    'dataTable.groupColumn': (name: string) => `Group by ${name}`,
    'dataTable.unGroupColumn': (name: string) => `Stop grouping by ${name}`,

    // Master/detail
    'dataTable.detailPanelToggle': 'Detail panel toggle',
    'dataTable.expandDetailPanel': 'Expand',
    'dataTable.collapseDetailPanel': 'Collapse',

    // Pagination
    'dataTable.paginationRowsPerPage': 'Rows per page:',
    'dataTable.paginationDisplayedRows': ({from, to, count}: {from: number; to: number; count: number}) =>
      `${from}–${to} of ${count !== -1 ? count : `more than ${to}`}`,

    // Row reordering text
    'dataTable.rowReorderingHeaderName': 'Row reordering',

    // Aggregation
    'dataTable.aggregationMenuItemHeader': 'Aggregation',
    'dataTable.aggregationFunctionLabelSum': 'sum',
    'dataTable.aggregationFunctionLabelAvg': 'avg',
    'dataTable.aggregationFunctionLabelMin': 'min',
    'dataTable.aggregationFunctionLabelMax': 'max',
    'dataTable.aggregationFunctionLabelSize': 'size',
  },

  // ============================================================================
  // Navigation namespace - Navigation related translations
  // ============================================================================
  navigation: {
    'categories.identities': 'Identities',
    'categories.resources': 'Resources',
    'categories.configure': 'Configure',
    'categories.customize': 'Customize',
    'categories.system': 'System',
    'pages.importConfiguration': 'Import Configuration',
    'pages.openProject': 'Import Configuration',
    'pages.importExport': 'Import / Export',
    'pages.home': 'Home',
    'pages.users': 'Users',
    'pages.userTypes': 'User Types',
    'pages.agentTypes': 'Agent Types',
    'pages.organizationUnits': 'Organization Units',
    'pages.groups': 'Groups',
    'pages.roles': 'Roles',
    'pages.connections': 'Connections',
    'pages.applications': 'Applications',
    'pages.apis': 'APIs',
    'pages.verifiablePresentations': 'Verifiable Presentations',
    'pages.verifiableCredentials': 'Verifiable Credentials',
    'pages.credentials': 'Templates',
    'pages.presentations': 'Presentations',
    'pages.dashboard': 'Dashboard',
    'pages.flows': 'Flows',
    'pages.design': 'Design',
    'pages.translations': 'Translations',
    'pages.settings': 'Settings',
    'breadcrumb.console': 'Console',
  },

  // ============================================================================
  // Users namespace - User management feature translations
  // ============================================================================
  users: {
    // Listing page
    title: 'User Management',
    subtitle: 'Manage users, roles, and permissions across your organization',
    addUser: 'Add User',
    inviteUser: 'Invite User',
    inviteUserDescription: 'Send an invite link to a new user to complete their registration',
    inviteLinkGenerated: 'Invite Link Generated!',
    inviteLinkDescription: 'Share this link with the user to complete their registration.',
    inviteLink: 'Invite Link',
    addAnother: 'Add Another User',
    inviteAnother: 'Invite Another User',
    'invite.steps.userdetails': 'User Details',
    'invite.steps.invitelink': 'Invite Link',
    editUser: 'Edit User',
    deleteUser: 'Delete User',
    userDetails: 'User Details',
    given_name: 'First Name',
    family_name: 'Last Name',
    email: 'Email Address',
    username: 'Username',
    role: 'Role',
    status: 'Status',
    createdAt: 'Created At',
    lastLogin: 'Last Login',
    'listing.columns.name': 'Name',
    'listing.columns.userId': 'User ID',
    'listing.columns.actions': 'Actions',
    noUsers: 'No users found',
    searchUsers: 'Search users...',
    confirmDeleteUser: 'Are you sure you want to delete this user?',
    userCreatedSuccess: 'User created successfully',
    userUpdatedSuccess: 'User updated successfully',
    userDeletedSuccess: 'User deleted successfully',
    'errors.failed.title': 'Error',
    'errors.failed.description': 'An error occurred. Please try again.',

    // Edit page
    'manageUser.title': 'Manage User',
    'manageUser.subtitle': 'View and manage user information',
    'manageUser.back': 'Back to Users',
    'manageUser.sections.quickCopy.title': 'Quick Copy',
    'manageUser.sections.quickCopy.description': 'Copy user identifiers for use in your application.',
    'manageUser.sections.quickCopy.userId': 'User ID',
    'manageUser.sections.quickCopy.copyUserId': 'Copy User ID',

    // Create page
    'createUser.title': 'Create User',
    'createUser.subtitle': 'Add a new user to your organization',

    // Create wizard steps
    'createWizard.steps.userType': 'User Type',
    'createWizard.steps.organizationUnit': 'Organization Unit',
    'createWizard.steps.userDetails': 'User Details',
    'createWizard.selectUserType.title': 'Select a user type',
    'createWizard.selectUserType.subtitle': 'Choose a user type (schema) for the new user.',
    'createWizard.selectUserType.fieldLabel': 'User Type',
    'createWizard.selectUserType.placeholder': 'Select a user type',
    'createWizard.selectOrganizationUnit.title': 'Select an organization unit',
    'createWizard.selectOrganizationUnit.subtitle': 'Choose which organization unit this user should belong to.',
    'createWizard.selectOrganizationUnit.fieldLabel': 'Organization Unit',
    'createWizard.userDetails.title': 'Enter user details',
    'createWizard.userDetails.subtitle': 'Fill in the required information for the new user.',
    'createWizard.validationErrors.userTypeRequired': 'Please select a user type before proceeding.',
    'createWizard.validationErrors.ouIdMissing': 'Organization unit ID is missing for the selected user type.',
    'createWizard.errors.noOuAccess':
      'You do not have permission to access the organization units for the selected user type, and no organization unit could be resolved.',
    'createWizard.errors.childOuProbeFailed': 'Unable to retrieve organization units for the selected user type.',
    'create.success': 'User created successfully.',
    'create.error': 'Failed to create user. Please try again.',
    'update.success': 'User updated successfully.',
    'update.error': 'Failed to update user. Please try again.',
    'delete.title': 'Delete User',
    'delete.message': 'Are you sure you want to delete this user? This action cannot be undone.',
    'delete.disclaimer': 'All associated data will be permanently removed.',
    'delete.success': 'User deleted successfully.',
    'delete.error': 'Failed to delete user. Please try again.',
    'delete.usages.loading': 'Checking affected resources…',
    'delete.usages.none': 'No agents currently list this user as their owner.',
    'delete.usages.title': 'The following agents list this user as their owner:',
    'delete.usages.more': '+{{count}} more',
    'delete.blocking.title': 'This user cannot be deleted until the following agents are reassigned or removed:',

    // Credentials section on edit page
    'manageUser.sections.credentials.title': 'Credentials',
    'manageUser.sections.credentials.description': 'Update credential values such as passwords for this user.',
    'manageUser.sections.credentials.info':
      'Credential values are write-only and cannot be viewed. You can set new values below.',

    // Update credentials
    'updateCredentials.button': 'Update Credentials',
    'updateCredentials.hint': 'Fill in only the credentials you want to update. Empty fields will be skipped.',
    'updateCredentials.success': 'Credentials updated successfully.',
    'updateCredentials.error': 'Failed to update credentials. Please try again.',
  },

  // ============================================================================
  // User Types namespace - User types feature translations
  // ============================================================================
  userTypes: {
    // Listing page
    title: 'User Types',
    subtitle: 'Define and manage user types with custom schemas',
    addUserType: 'Add User Type',
    createUserType: 'Create User Type',
    editUserType: 'Edit User Type',
    deleteUserType: 'Delete User Type',
    userTypeDetails: 'User Type Details',
    typeName: 'Type Name',
    typeNamePlaceholder: 'e.g., Employee, Customer, Partner',
    organizationUnit: 'Organization Unit',
    ouSelectPlaceholder: 'Select an organization unit',
    allowSelfRegistration: 'Allow Self Registration',
    description: 'Description',
    createDescription: 'Define a new user type schema for your organization',
    permissions: 'Permissions',
    schemaProperties: 'Schema Properties',
    propertyName: 'Property Name',
    propertyNamePlaceholder: 'e.g., email, age, address',
    propertyType: 'Type',
    addProperty: 'Add Property',
    'attributes.libraryTitle': 'Available Properties',
    'attributes.searchPlaceholder': 'Search properties',
    'attributes.allAdded': 'All available properties have been added.',
    'attributes.noResults': 'No properties match your search.',
    newAttribute: 'New property',
    credential: 'Credential',
    unique: 'Unique',
    removeProperty: 'Remove property',
    regexPattern: 'Regular Expression Pattern (Optional)',
    regexPlaceholder: 'e.g., ^[a-zA-Z0-9]+$',
    enumValues: 'Allowed Values (Enum) - Optional',
    enumPlaceholder: 'Add value and press Enter',
    'tooltips.required': 'Users must provide a value for this field',
    'tooltips.unique': 'Each user must have a distinct value for this field',
    'tooltips.credential': 'Values will be hashed and not returned in API responses',
    credentialHint: 'This field will be treated as a secret. Values will be hashed and cannot be retrieved.',
    'types.string': 'String',
    'types.number': 'Number',
    'types.boolean': 'Boolean',
    'types.enum': 'Enum',
    'types.array': 'Array',
    'types.object': 'Object',
    'validationErrors.nameRequired': 'Please enter a user type name',
    'validationErrors.ouIdRequired': 'Please provide an organization unit ID',
    'validationErrors.propertiesRequired': 'Please add at least one property',
    'validationErrors.duplicateProperties': 'Duplicate property names found: {{duplicates}}',
    'errors.organizationUnitsFailedTitle': 'Failed to load organization units',
    noUserTypes: 'No user types found',
    'listing.columns.name': 'Name',
    'listing.columns.id': 'User Type ID',
    'listing.columns.organizationUnit': 'Organization Unit',
    'listing.columns.allowSelfRegistration': 'Self Registration',
    'listing.columns.actions': 'Actions',
    noOrganizationUnits: 'No organization units available',
    confirmDeleteUserType: 'Are you sure you want to delete this user type?',

    // Edit page
    'manageUserType.title': 'Manage User Type',
    'manageUserType.subtitle': 'View and manage user type information',

    // Edit page (new UI)
    'edit.back': 'Back to User Types',
    'edit.editName': 'Edit user type name',
    'edit.copyId': 'Copy user type ID',
    'edit.tabs.general': 'General',
    'edit.tabs.schema': 'Schema',
    'edit.unsavedChanges': 'You have unsaved changes',
    'edit.saveError': 'Failed to save user type',
    'edit.loadError': 'Failed to load user type information',
    'edit.notFound': 'User type not found',
    'edit.general.organizationUnit.title': 'Organization Unit',
    'edit.general.organizationUnit.description': 'The organization unit this user type belongs to.',
    'edit.general.selfRegistration.title': 'Self Registration',
    'edit.general.selfRegistration.description': 'Allow users to self-register with this user type.',
    'edit.general.selfRegistration.enabledHint': 'Users can register themselves as this user type.',
    'edit.general.displayAttribute.title': 'Display Attribute',
    'edit.general.displayAttribute.description': 'The attribute used to display user identity.',
    'edit.general.dangerZone.title': 'Danger Zone',
    'edit.general.dangerZone.description': 'Irreversible actions for this user type.',
    'edit.general.dangerZone.deleteUserType': 'Delete User Type',
    'edit.general.dangerZone.deleteUserTypeDescription':
      'Permanently delete this user type and all associated schema definitions. This action cannot be undone.',

    // Create page
    'createUserType.title': 'Create User Type',
    'createUserType.subtitle': 'Add a new user type to your organization',

    // Create wizard steps
    'createWizard.steps.name': 'Create a User Type',
    'createWizard.steps.general': 'General',
    'createWizard.steps.properties': 'Properties',
    'createWizard.name.title': "Let's name your user type",
    'createWizard.name.fieldLabel': 'User Type Name',
    'createWizard.name.placeholder': 'Enter your user type name',
    'createWizard.name.suggestions.label': 'In a hurry? Pick a random name:',
    'createWizard.general.title': 'Configure general settings',
    'createWizard.general.subtitle': 'Choose an organization unit and set registration preferences',
    'createWizard.properties.title': 'Define your schema properties',
    'createWizard.properties.subtitle': 'Add the fields that make up this user type',
    'create.success': 'User type created successfully.',
    'create.error': 'Failed to create user type. Please try again.',
    'update.success': 'User type updated successfully.',
    'update.error': 'Failed to update user type. Please try again.',
    'delete.title': 'Delete User Type',
    'delete.message':
      'Are you sure you want to delete this user type? This action cannot be undone and may affect existing users of this type.',
    'delete.disclaimer': 'All associated schema definitions will be permanently removed.',
    'delete.success': 'User type deleted successfully.',
    'delete.error': 'Failed to delete user type. Please try again.',
    'removeCredentialDialog.title': 'Remove Credential Flag',
    'removeCredentialDialog.description':
      'Removing the credential flag will cause this field to no longer be hashed or protected. Existing hashed values may become inaccessible. Are you sure you want to proceed?',
    'removeCredentialDialog.descriptionNew':
      'Removing the credential flag will cause this field to no longer be hashed or protected. Are you sure you want to proceed?',
    'removeCredentialDialog.confirm': 'Remove Credential',
  },

  // ============================================================================
  // Agent Types namespace - Agent types feature translations
  // ============================================================================
  agentTypes: {
    // Listing page
    title: 'Agent Types',
    subtitle: 'Define and manage agent types with custom schemas',
    addAgentType: 'Add Agent Type',
    createAgentType: 'Create Agent Type',
    editAgentType: 'Edit Agent Type',
    deleteAgentType: 'Delete Agent Type',
    agentTypeDetails: 'Agent Type Details',
    typeName: 'Type Name',
    typeNamePlaceholder: 'e.g., Worker, Assistant, Tool',
    organizationUnit: 'Organization Unit',
    ouSelectPlaceholder: 'Select an organization unit',
    description: 'Description',
    createDescription: 'Define a new agent type schema for your organization',
    permissions: 'Permissions',
    schemaProperties: 'Schema Properties',
    propertyName: 'Property Name',
    propertyNamePlaceholder: 'e.g., model, environment, team',
    propertyType: 'Type',
    addProperty: 'Add Property',
    credential: 'Credential',
    unique: 'Unique',
    removeProperty: 'Remove property',
    regexPattern: 'Regular Expression Pattern (Optional)',
    regexPlaceholder: 'e.g., ^[a-zA-Z0-9]+$',
    enumValues: 'Allowed Values (Enum) - Optional',
    enumPlaceholder: 'Add value and press Enter',
    'tooltips.required': 'Agents must provide a value for this field',
    'tooltips.unique': 'Each agent must have a distinct value for this field',
    'tooltips.credential': 'Values will be hashed and not returned in API responses',
    credentialHint: 'This field will be treated as a secret. Values will be hashed and cannot be retrieved.',
    'types.string': 'String',
    'types.number': 'Number',
    'types.boolean': 'Boolean',
    'types.enum': 'Enum',
    'types.array': 'Array',
    'types.object': 'Object',
    'validationErrors.nameRequired': 'Please enter an agent type name',
    'validationErrors.ouIdRequired': 'Please provide an organization unit ID',
    'validationErrors.propertiesRequired': 'Please add at least one property',
    'validationErrors.duplicateProperties': 'Duplicate property names found: {{duplicates}}',
    'errors.organizationUnitsFailedTitle': 'Failed to load organization units',
    noAgentTypes: 'No agent types found',
    'listing.columns.name': 'Name',
    'listing.columns.id': 'Agent Type ID',
    'listing.columns.organizationUnit': 'Organization Unit',
    'listing.columns.actions': 'Actions',
    noOrganizationUnits: 'No organization units available',
    confirmDeleteAgentType: 'Are you sure you want to delete this agent type?',

    // Edit page
    'manageAgentType.title': 'Manage Agent Type',
    'manageAgentType.subtitle': 'View and manage agent type information',

    // Edit page (new UI)
    'edit.back': 'Back to Agents',
    'edit.title': 'Agent Schema',
    'edit.editName': 'Edit agent type name',
    'edit.copyId': 'Copy agent type ID',
    'edit.tabs.general': 'General',
    'edit.tabs.schema': 'Schema',
    'edit.unsavedChanges': 'You have unsaved changes',
    'edit.saveError': 'Failed to save agent type',
    'edit.loadError': 'Failed to load agent type information',
    'edit.notFound': 'Agent type not found',
    'edit.general.organizationUnit.title': 'Organization Unit',
    'edit.general.organizationUnit.description': 'The organization unit this agent type belongs to.',
    'edit.general.displayAttribute.title': 'Display Attribute',
    'edit.general.displayAttribute.description': 'The attribute used to display agent identity.',
    'edit.general.dangerZone.title': 'Danger Zone',
    'edit.general.dangerZone.description': 'Irreversible actions for this agent type.',
    'edit.general.dangerZone.deleteAgentType': 'Delete Agent Type',
    'edit.general.dangerZone.deleteAgentTypeDescription':
      'Permanently delete this agent type and all associated schema definitions. This action cannot be undone.',

    // Create page
    'createAgentType.title': 'Create Agent Type',
    'createAgentType.subtitle': 'Add a new agent type to your organization',

    // Create wizard steps
    'createWizard.steps.name': 'Create an Agent Type',
    'createWizard.steps.general': 'General',
    'createWizard.steps.properties': 'Properties',
    'createWizard.name.title': "Let's name your agent type",
    'createWizard.name.fieldLabel': 'Agent Type Name',
    'createWizard.name.placeholder': 'Enter your agent type name',
    'createWizard.name.suggestions.label': 'In a hurry? Pick a random name:',
    'createWizard.general.title': 'Configure general settings',
    'createWizard.general.subtitle': 'Choose an organization unit and set registration preferences',
    'createWizard.properties.title': 'Define your schema properties',
    'createWizard.properties.subtitle': 'Add the fields that make up this agent type',
    'create.success': 'Agent type created successfully.',
    'create.error': 'Failed to create agent type. Please try again.',
    'update.success': 'Agent type updated successfully.',
    'update.error': 'Failed to update agent type. Please try again.',
    'delete.disclaimer': 'All associated schema definitions will be permanently removed.',
    'delete.success': 'Agent type deleted successfully.',
    'delete.error': 'Failed to delete agent type. Please try again.',
    'removeCredentialDialog.title': 'Remove Credential Flag',
    'removeCredentialDialog.description':
      'Removing the credential flag will cause this field to no longer be hashed or protected. Existing hashed values may become inaccessible. Are you sure you want to proceed?',
    'removeCredentialDialog.confirm': 'Remove Credential',
  },

  // ============================================================================
  // Agents namespace - Agent management feature translations
  // ============================================================================
  agents: {
    // Listing page
    'listing.title': 'Agents',
    'listing.subtitle': 'Manage service identities and machine clients',
    'listing.addAgent': 'Add Agent',
    'listing.schema': 'Schema',
    'listing.search.placeholder': 'Search agents',
    'listing.loadError': 'Failed to load agents',
    'listing.columns.name': 'Name',
    'listing.columns.agentId': 'Agent ID',
    'listing.columns.organizationUnit': 'Organization Unit',
    'listing.columns.actions': 'Actions',

    // Create wizard
    'createWizard.createAgent': 'Create agent',
    'createWizard.errors.createFailed': 'Failed to create agent. Please try again.',
    'createWizard.errors.ouRequired': 'Organization unit is required',
    'createWizard.errors.schemaRequired': 'Schema is required',
    'createWizard.steps.name': 'Name',
    'createWizard.steps.organizationUnit': 'Organization unit',
    'createWizard.steps.profile': 'Profile',
    'createWizard.steps.owner': 'Owner',
    'createWizard.name.title': "What's this agent called?",
    'createWizard.name.fieldLabel': 'Agent name',
    'createWizard.name.placeholder': 'e.g. Billing Service',
    'createWizard.name.suggestions.label': 'Need inspiration? Pick one:',
    'createWizard.agentDetails.title': 'Agent attributes',
    'createWizard.agentDetails.subtitle': 'Provide values for the attributes defined by the agent schema.',
    'createWizard.owner.title': 'Owner',
    'createWizard.owner.subtitle': 'Choose the user that owns this agent.',
    'createWizard.owner.userLabel': 'Owner',
    'createWizard.owner.userPlaceholder': 'Select a user',
    'createWizard.owner.helperText':
      'Defaults to you if left unchanged. You can assign ownership to another user instead.',

    // Client secret (creation)
    'clientSecret.saveTitle': 'Save your client secret',
    'clientSecret.saveSubtitle': "This secret won't be shown again. Copy it and store it somewhere safe.",
    'clientSecret.agentNameLabel': 'Agent name',
    'clientSecret.clientIdLabel': 'Client ID',
    'clientSecret.clientSecretLabel': 'Client Secret',
    'clientSecret.copySecret': 'Copy client secret',
    'clientSecret.copied': 'Copied',
    'clientSecret.securityReminder.title': "You won't be able to see this secret again",
    'clientSecret.securityReminder.description':
      'Store the client secret somewhere safe. If you lose it, you will need to regenerate it from the agent settings.',

    // Delete dialog
    'delete.title': 'Delete agent',
    'delete.message': 'Are you sure you want to delete this agent? This action cannot be undone.',
    'delete.disclaimer': 'Deleting this agent will revoke all its credentials and access tokens.',
    'delete.error': 'Failed to delete agent. Please try again.',

    // Regenerate client secret
    'regenerateSecret.dialog.title': 'Regenerate client secret?',
    'regenerateSecret.dialog.message':
      'A new client secret will be generated for this agent. Any service using the current client secret will stop working immediately.',
    'regenerateSecret.dialog.disclaimer':
      'This action cannot be undone. The current client secret will be invalidated as soon as you confirm.',
    'regenerateSecret.dialog.confirmButton': 'Regenerate',
    'regenerateSecret.dialog.regenerating': 'Regenerating…',
    'regenerateSecret.dialog.error': 'Failed to regenerate client secret',
    'regenerateSecret.success.title': 'Client secret regenerated',
    'regenerateSecret.success.subtitle':
      "Copy your new client secret and store it somewhere safe. It won't be shown again.",
    'regenerateSecret.success.secretLabel': 'New Client Secret',
    'regenerateSecret.success.copySecret': 'Copy client secret',
    'regenerateSecret.success.copied': 'Copied',
    'regenerateSecret.success.securityReminder.title': "You won't be able to see this secret again",
    'regenerateSecret.success.securityReminder.description':
      "Store the new client secret somewhere safe. If you lose it, you'll need to regenerate it again.",

    // Edit page (header)
    'edit.page.error': 'Failed to load agent',
    'edit.page.notFound': 'Agent not found',
    'edit.page.back': 'Back to agents',
    'edit.page.description.empty': 'No description',
    'edit.page.description.placeholder': 'Add a description',
    'edit.page.tabs.general': 'General',
    'edit.page.tabs.attributes': 'Attributes',
    'edit.page.tabs.credentials': 'Credentials',
    'edit.page.tabs.access': 'Access',
    'edit.page.tabs.flows': 'Flows',
    'edit.page.tabs.tokens': 'Tokens',
    'edit.page.tabs.advanced': 'Advanced',
    'edit.page.unsavedChanges': 'You have unsaved changes',
    'edit.page.unsavedChangesInvalid': 'Before saving, {{issues}}.',
    'edit.page.validation.missingRedirectUri': 'add a redirect URI',
    'edit.page.validation.missingAllowedUserType': 'select at least one allowed user type',
    'edit.page.validation.missingCertificate': 'add a certificate',
    'edit.page.validation.tokenSettings': 'fix the token settings',
    'edit.page.reset': 'Reset',
    'edit.page.save': 'Save',
    'edit.page.saving': 'Saving…',
    'update.success': 'Agent updated successfully.',
    'update.error': 'Failed to update agent. Please try again.',

    // Edit page - Attributes tab
    'edit.attributes.title': 'Attributes',
    'edit.attributes.description': 'View and manage agent attribute values.',
    'edit.attributes.empty': 'No attributes available.',
    'edit.attributes.noEditable': 'No editable attributes available.',
    'edit.attributes.noSchema': 'No schema available for editing',

    // Edit page - General tab
    'edit.general.sections.quickCopy.title': 'Identifier',
    'edit.general.sections.quickCopy.description': 'The unique identifier for this agent.',
    'edit.general.labels.agentId': 'Agent ID',
    'edit.general.labels.ownerId': 'Owner ID',
    'edit.general.agentId.hint': 'Unique identifier for this agent',
    'edit.general.clientId.hint': 'OAuth2 client identifier used by this agent to obtain tokens',
    'edit.general.owner.hint': 'Identifier of the user that owns this agent',
    'edit.general.owner.empty': 'No owner assigned',
    'edit.general.sections.owner.title': 'Owner',
    'edit.general.sections.owner.description': 'The user accountable for this agent.',
    'edit.general.sections.owner.label': 'Owner',
    'edit.general.sections.owner.summaryDescription':
      'The user who is accountable for this agent, shown in audit records and used as the contact point for questions about what this agent does. Assigning an owner does not give that user any special access to the agent. Manage this from the Advanced tab.',
    'edit.general.sections.attributes.title': 'Attributes',
    'edit.general.sections.attributes.description':
      "A preview of this agent's attribute values. Manage them from the Attributes tab.",
    'edit.general.sections.organizationUnit.title': 'Organization Unit',
    'edit.general.sections.organizationUnit.description': 'The organization unit this agent belongs to.',
    'edit.general.sections.dangerZone.title': 'Danger Zone',
    'edit.general.sections.dangerZone.description': 'Actions here are permanent. Make sure before you proceed.',
    'edit.general.dangerZone.deleteAgent.title': 'Delete Agent',
    'edit.general.dangerZone.deleteAgent.description':
      'Permanently deletes this agent and immediately invalidates any tokens it has issued. This action cannot be undone.',
    'edit.general.dangerZone.deleteAgent.button': 'Delete Agent',

    // Edit page - Credentials tab
    'edit.credentials.clientId.title': 'Client ID',
    'edit.credentials.clientId.description': 'The public identifier this agent uses to authenticate as a client.',
    'edit.credentials.clientSecret.title': 'Client Secret',
    'edit.credentials.clientSecret.description': 'The secret this agent uses to authenticate as a client.',
    'edit.credentials.clientSecret.clientIdLabel': 'Client ID',
    'edit.credentials.clientSecret.regenerateHint':
      'Client secret was shown once at creation. Regenerate to issue a new one.',
    'edit.credentials.clientSecret.regenerateButton': 'Regenerate secret',
    'edit.credentials.tokenEndpointAuthMethod.title': 'Token Endpoint Auth Method',
    'edit.credentials.tokenEndpointAuthMethod.description':
      'Defines how this agent authenticates when requesting tokens.',
    'edit.credentials.tokenEndpointAuthMethod.placeholder': 'Select an auth method',
    'edit.credentials.tokenEndpointAuthMethod.hint':
      'How this agent proves its identity when it calls the token endpoint.',
    'edit.credentials.tokenEndpointAuthMethod.lockedHint': 'Set to "none" because this agent is a public client.',
    'edit.credentials.certificate.title': 'Certificate',
    'edit.credentials.certificate.description':
      'Used to verify signed requests from this agent when it authenticates with private_key_jwt.',
    'edit.credentials.certificate.sourceLabel': 'Public key source',
    'edit.credentials.certificate.type.none': 'None',
    'edit.credentials.certificate.type.jwks': 'JWKS (JSON)',
    'edit.credentials.certificate.type.jwksUri': 'JWKS URI',
    'edit.credentials.certificate.placeholder.jwks': '{ "keys": [ ... ] }',
    'edit.credentials.certificate.placeholder.jwksUri': 'https://example.com/.well-known/jwks.json',
    'edit.credentials.certificate.hint.jwks': 'The JSON Web Key Set to verify signed requests from this agent against.',
    'edit.credentials.certificate.hint.jwksUri': 'The URL to verify signed requests from this agent against.',
    'edit.credentials.certificate.error.required':
      'This agent needs a certificate before it can use private_key_jwt authentication.',
    'edit.credentials.certificate.error.valueRequired': 'This field cannot be empty.',

    // Edit page - Access tab
    'edit.access.groups.title': 'Groups',
    'edit.access.groups.description':
      'Groups this agent belongs to. Manage membership from the <manageLink>Groups page</manageLink>.',
    'edit.access.groups.label': 'Groups',
    'edit.access.groups.empty': 'This agent does not belong to any groups.',
    'edit.access.groups.error': 'Failed to load groups for this agent.',
    'edit.access.roles.title': 'Roles',
    'edit.access.roles.description':
      'Roles assigned to this agent, directly or through its groups. Manage assignments from the <manageLink>Roles page</manageLink>.',
    'edit.access.roles.label': 'Roles',
    'edit.access.roles.empty': 'This agent does not have any roles assigned.',
    'edit.access.roles.error': 'Failed to load roles for this agent.',

    // Edit page - Flows tab
    'edit.flows.allowedUserTypes.title': 'Allowed User Types',
    'edit.flows.allowedUserTypes.description':
      'Restrict which user types can authenticate or register through this agent.',
    'edit.flows.allowedUserTypes.label': 'User Types',
    'edit.flows.allowedUserTypes.placeholder': 'Select or add user types',
    'edit.flows.allowedUserTypes.hint': 'Only these user types can authenticate or register through this agent.',
    'edit.flows.allowedUserTypes.required': 'Select at least one user type that can sign in through this agent.',
    'edit.flows.delegationToggle.label': 'Delegated mode',
    'edit.flows.delegationLock.message':
      'These settings are frozen for this agent. Turn on Delegated mode above to unlock and start using them.',

    // Edit page - Advanced tab
    'edit.advanced.redirectUris.title': 'Authorized redirect URIs',
    'edit.advanced.redirectUris.description': 'For use with requests from a web server',
    'edit.advanced.redirectUris.empty': 'No redirect URIs configured.',
    'edit.advanced.redirectUris.addUri': 'Add URI',
    'edit.advanced.redirectUris.error.empty': 'URI cannot be empty',
    'edit.advanced.redirectUris.error.invalid': 'Enter a valid URL',
    'edit.advanced.redirectUris.required': 'The Authorization Code grant requires at least one valid redirect URI.',
    'edit.advanced.oauthAccess.title': 'OAuth Configuration',
    'edit.advanced.oauthAccess.description': 'The grants and redirect URIs this agent is authorized to use.',
    'edit.advanced.oauthAccess.grantTypes.label': 'Grant Types',
    'edit.advanced.oauthAccess.grantTypes.hint':
      'The greyed-out grants unlock once you turn on Delegated mode in the Flows tab.',
    'edit.advanced.security.title': 'Security',
    'edit.advanced.security.description':
      'Controls how this agent protects the authorization code exchange when a user signs in.',
    'edit.advanced.security.pkce.label': 'Require PKCE',
    'edit.advanced.security.pkce.forced':
      'This agent is set up as a public client, so PKCE is required and cannot be turned off.',
    'edit.advanced.security.pkce.on': 'authorization_code is on for this agent, so PKCE is required automatically.',
    'edit.advanced.security.pkce.notApplicable':
      'PKCE only applies to the <code>authorization_code</code> grant. Turn that on to enable this setting.',
    'edit.advanced.security.par.label': 'Require Pushed Authorization Requests',
    'edit.advanced.security.par.hint':
      'Require this agent to push its authorization request to the PAR endpoint before redirecting a user to sign in.',

    // Edit page - Tokens tab
    'edit.tokens.tabs.user': 'User',
    'edit.tokens.tabs.agent': 'Agent',
    'edit.tokens.delegationLock.message':
      'These settings are frozen for this agent. Turn on Delegated mode in the Flows tab to unlock and start using them.',
    'edit.tokens.agent.attributes.title': 'Access Token Attributes',
    'edit.tokens.agent.attributes.description':
      'Attributes included in the access token this agent receives for its own requests (client_credentials grant).',
    'edit.tokens.agent.attributes.label': 'Add or Remove Attributes',
    'edit.tokens.agent.attributes.hint': "Click on this agent's attributes to add them to its access token.",
    'edit.tokens.agent.attributes.empty':
      'No attributes available. Configure attributes for this agent in the Attributes tab.',
    'edit.tokens.agent.validity.title': 'Token Validity',
    'edit.tokens.agent.validity.label': 'Token Validity',
    'edit.tokens.agent.validity.description': 'How long this access token remains valid before expiration.',
    'edit.tokens.agent.validity.hint': 'Token validity period in seconds (e.g., 3600 for 1 hour).',
    'edit.tokens.agent.validity.error': 'Enter a validity period of at least 1 second.',

    // Backend error code translations (per agent service error envelope).
    'errors.AGT-1001': 'The request body is malformed or contains invalid data.',
    'errors.AGT-1002': 'One or more redirect URIs are not valid.',
    'errors.AGT-1003': 'One or more grant types are not supported.',
    'errors.AGT-1004': 'The agent with the specified id does not exist.',
    'errors.AGT-1005': 'The specified organization unit does not exist.',
    'errors.AGT-1008': 'The agent ID is required.',
    'errors.AGT-1009': 'The agent name must be provided and non-empty.',
    'errors.AGT-1010': 'The agent type must be provided.',
    'errors.AGT-1011': 'The limit parameter must be between 1 and 100.',
    'errors.AGT-1012': 'The offset parameter must be a non-negative integer.',
    'errors.AGT-1013': 'An agent with the same name already exists.',
    'errors.AGT-1014': 'An agent with the same unique attribute value already exists.',
    'errors.AGT-1015': 'The provided attributes failed schema validation.',
    'errors.AGT-1016': 'The provided credential is invalid.',
    'errors.AGT-1020': 'The filter format is invalid.',
    'errors.AGT-1021': 'The provided OAuth configuration is invalid.',
    'errors.AGT-1022': 'The provided token endpoint authentication method is not supported.',
    'errors.AGT-1023': 'The public client configuration is invalid.',
    'errors.AGT-1024': 'The provided certificate type is not supported.',
    'errors.AGT-1025': 'The provided certificate value is invalid or missing.',
    'errors.AGT-1026': 'The JWKS URI must be a publicly reachable HTTPS URL.',
    'errors.AGT-1027': 'Declaratively managed agents cannot be modified via the API.',
    'errors.AGT-1028': 'The provided authentication flow ID is invalid.',
    'errors.AGT-1029': 'The provided registration flow ID is invalid.',
    'errors.AGT-1030': 'An error occurred while retrieving the flow definition.',
    'errors.AGT-1031': 'One or more specified allowed user types are invalid.',
    'errors.AGT-1032': 'The specified theme does not exist.',
    'errors.AGT-1033': 'The specified layout does not exist.',
    'errors.AGT-1034': 'One or more provided response types are invalid.',
    'errors.AGT-1035': 'Failed to sync agent attribute changes with the consent service.',
    'errors.AGT-1036': 'A certificate operation failed due to invalid input.',
    'errors.AGT-1037': 'An entity with the same client ID already exists.',
    'errors.AGT-1038': 'An entity may have at most one inbound auth config per protocol.',
    'errors.AGT-1039': 'The specified owner does not match any known user, application, or agent.',
    'errors.AGT-1040': 'One or more user attributes are not valid for the configured allowed user types.',
  },

  // ============================================================================
  // Organization Units namespace - Organization unit management feature translations
  // ============================================================================
  organizationUnits: {
    // Tree picker (shared component)
    'treePicker.empty': 'No organization units available',
    'treePicker.error': 'Failed to load organization unit data',

    // Listing page
    'listing.title': 'Organization Units (OU)',
    'listing.subtitle': 'Manage organization units and hierarchies',
    'listing.addRootOrganizationUnit': 'Add Root Organization Unit',
    'listing.error.title': 'Failed to load organization units',
    'listing.error.unknown': 'An unknown error occurred',
    'listing.treeView.empty': 'No organization units found',
    'listing.treeView.noChildren': 'No child organization units',
    'listing.treeView.loadError': 'Failed to load child organization units',
    'listing.treeView.addChild': 'Add Child OU',
    'listing.treeView.addChildOrganizationUnit': 'Add Child Organization Unit',
    'listing.treeView.loadMore': 'Load more',
    'listing.columns.name': 'Name',
    'listing.columns.handle': 'Handle',
    'listing.columns.description': 'Description',

    // Create page
    'create.title': 'Create Organization Unit',
    'create.heading': "Let's set up your organization unit",
    'create.subtitle': 'Define a new organization unit',
    'create.error': 'Failed to create organization unit. Please try again.',
    'create.suggestions.label': 'In a hurry? Pick a random name:',

    'delete.dialog.title': 'Delete Organization Unit',
    'delete.dialog.message': 'Are you sure you want to delete this organization unit? This action cannot be undone.',
    'delete.dialog.disclaimer':
      'Warning: All associated data, configurations, and user assignments will be permanently removed.',
    'delete.dialog.error': 'Failed to delete organization unit. Please try again.',

    /* -------------------- Edit page -------------------- */
    // Common
    'edit.page.error': 'Failed to load organization unit',
    'edit.page.notFound': 'Organization unit not found',
    'edit.page.logoUpdate.label': 'Update Logo',
    'edit.page.copyOuId': 'Copy Organization Unit ID',
    'edit.page.back': 'Back to Organization Units',
    'edit.page.backToOU': 'Back to {{name}}',
    'edit.page.description.empty': 'No description',
    'edit.page.description.placeholder': 'Enter a description...',
    'edit.page.tabs.general': 'General',
    'edit.page.tabs.childOUs': 'Child OUs',
    'edit.page.tabs.users': 'Users',
    'edit.page.tabs.groups': 'Groups',
    'edit.page.tabs.customization': 'Customization',
    'edit.page.tabs.advanced': 'Advanced Settings',
    'edit.actions.unsavedChanges.label': 'You have unsaved changes',
    'edit.actions.reset.label': 'Reset',
    'edit.actions.save.label': 'Save Changes',
    'edit.actions.saving.label': 'Saving...',

    // General section
    'edit.general.sections.quickCopy.title': 'Quick Copy',
    'edit.general.sections.quickCopy.description': 'Copy organization unit identifiers for quick reference.',
    'edit.general.sections.parentOUSettings.title': 'Parent Organization Unit',
    'edit.general.sections.parentOUSettings.description': 'The parent organization unit in the hierarchy.',
    'edit.general.sections.dangerZone.title': 'Danger Zone',
    'edit.general.sections.dangerZone.description': 'Actions in this section are irreversible. Proceed with caution.',
    'edit.general.sections.dangerZone.deleteOU.title': 'Delete Organization Unit',
    'edit.general.sections.dangerZone.deleteOU.description':
      'Deleting this organization unit is permanent and cannot be undone.',
    'edit.general.ou.id.label': 'Organization Unit ID',
    'edit.general.ou.parent.label': 'Parent Organization Unit',
    'edit.general.ou.noParent.label': 'Root Organization Unit',
    'edit.general.dangerZone.delete.button.label': 'Delete Organization Unit',
    // Form fields
    'edit.general.handle.label': 'Handle',
    'edit.general.handle.placeholder': 'e.g., engineering, sales, hr',
    'edit.general.handle.hint': 'A unique identifier for this organization unit',
    'edit.general.handle.validations.required': 'Handle is required',
    'edit.general.handle.validations.format': 'Handle must be lowercase alphanumeric with hyphens only',
    'edit.general.name.label': 'Name',
    'edit.general.name.placeholder': 'e.g., Engineering Department',
    'edit.general.name.validations.required': 'Name is required',
    'edit.general.description.label': 'Description',
    'edit.general.description.placeholder': 'Enter a description for this organization unit',
    'edit.general.parent.label': 'Parent Organization Unit',
    'edit.general.parent.hint': 'The parent organization unit for this new unit',
    'edit.general.dangerZone.delete.title': 'Delete Organization Unit',
    'edit.general.dangerZone.delete.message':
      'Are you sure you want to delete this organization unit? This action cannot be undone.',
    'edit.general.dangerZone.delete.error': 'Failed to delete organization unit. Please try again.',
    'edit.general.dangerZone.delete.success': 'Organization unit deleted successfully.',

    // Child OUs Section
    'edit.childOUs.sections.manage.title': 'Child Organization Units',
    'edit.childOUs.sections.manage.description': 'View and manage child organization units under this OU',

    // Users Section
    'edit.users.sections.manage.title': 'Users',
    'edit.users.sections.manage.description': 'View users belonging to this organization unit',
    'edit.users.sections.manage.listing.columns.id': 'User ID',
    'edit.users.sections.manage.listing.columns.type': 'User Type',
    'edit.users.sections.manage.listing.columns.name': 'Display Name',

    // Groups Section
    'edit.groups.sections.manage.title': 'Groups',
    'edit.groups.sections.manage.description': 'View groups belonging to this organization unit',
    'edit.groups.sections.manage.listing.columns.name': 'Group Name',
    'edit.groups.sections.manage.listing.columns.id': 'Group ID',

    // Customization tab
    'edit.customization.sections.appearance': 'Appearance',
    'edit.customization.sections.appearance.description': 'Customize the look and feel of this organization unit.',
    'edit.customization.labels.theme': 'Theme',
    'edit.customization.theme.placeholder': 'Select a theme',
    'edit.customization.theme.hint': 'The theme applied to this organization unit.',
    'create.success': 'Organization unit created successfully.',
    'update.success': 'Organization unit updated successfully.',
    'update.error': 'Failed to update organization unit. Please try again.',
    'delete.success': 'Organization unit deleted successfully.',
    'delete.error': 'Failed to delete organization unit. Please try again.',
  },

  // ============================================================================
  // Groups namespace - Group management feature translations
  // ============================================================================
  groups: {
    // List page
    'listing.title': 'Groups',
    'listing.subtitle': 'Manage groups and their members across organization units',
    'listing.addGroup': 'Add Group',
    'listing.error': 'Failed to load groups',
    'listing.search.placeholder': 'Search groups...',
    'listing.columns.name': 'Name',
    'listing.columns.description': 'Description',
    'listing.columns.organizationUnit': 'Organization Unit',
    'listing.columns.actions': 'Actions',

    // Create page
    'create.title': 'Create Group',
    'create.heading': 'Create a new group',
    'create.error': 'Failed to create group. Please try again.',
    'create.form.name.label': 'Group Name',
    'create.form.name.placeholder': 'Enter group name',
    'create.form.name.required': 'Group name is required',
    'create.form.description.label': 'Description',
    'create.form.description.placeholder': 'Enter group description',
    'create.form.organizationUnit.label': 'Organization Unit',
    'create.form.organizationUnit.placeholder': 'Select an organization unit',
    'create.form.organizationUnit.required': 'Organization unit is required',

    // Create wizard
    'createWizard.steps.name': 'Create a Group',
    'createWizard.steps.organizationUnit': 'Organization Unit',
    'createWizard.name.title': "Let's give a name to your group",
    'createWizard.name.suggestions.label': 'In a hurry? Pick a random name:',
    'createWizard.organizationUnit.title': 'Select an organization unit',
    'createWizard.organizationUnit.subtitle': 'Choose the organization unit this group will belong to.',
    'createWizard.createGroup': 'Create Group',

    // Edit page
    'edit.page.back': 'Back to Groups',
    'edit.page.error': 'Failed to load group',
    'edit.page.notFound': 'Group not found',
    'edit.page.description.placeholder': 'Add a description...',
    'edit.page.description.empty': 'No description',
    'edit.page.header.groupId': 'ID',
    'edit.page.header.ouId': 'Organization Unit',
    'edit.page.header.editName': 'Edit group name',
    'edit.page.header.editDescription': 'Edit description',
    'edit.page.tabs.general': 'General',
    'edit.page.tabs.members': 'Members',
    'edit.page.unsavedChanges': 'You have unsaved changes',
    'edit.page.reset': 'Reset',
    'edit.page.save': 'Save Changes',
    'edit.page.saving': 'Saving...',

    // General settings
    'edit.general.sections.quickCopy.title': 'Quick Copy',
    'edit.general.sections.quickCopy.description': 'Copy group identifiers for quick reference.',
    'edit.general.sections.quickCopy.groupId': 'Group ID',
    'edit.general.sections.quickCopy.copyGroupId': 'Copy Group ID',
    'edit.general.sections.quickCopy.copyOrganizationUnitId': 'Copy organization unit ID',
    'edit.general.sections.organizationUnit.title': 'Organization Unit',
    'edit.general.sections.organizationUnit.description': 'The organization unit this group belongs to.',
    'edit.general.sections.organizationUnit.handleLabel': 'Handle',
    'edit.general.sections.organizationUnit.idLabel': 'ID',
    'edit.general.sections.quickCopy.copyOrganizationUnitHandle': 'Copy organization unit handle',
    'edit.general.sections.dangerZone.title': 'Danger Zone',
    'edit.general.sections.dangerZone.description': 'Actions in this section are irreversible. Proceed with caution.',
    'edit.general.sections.dangerZone.deleteGroup': 'Delete this group',
    'edit.general.sections.dangerZone.deleteGroupDescription': 'Deleting this group is permanent and cannot be undone.',

    // Members settings
    'edit.members.sections.manage.title': 'Members',
    'edit.members.sections.manage.description': 'Manage the members of this group',
    'edit.members.sections.manage.listing.columns.name': 'Name',
    'edit.members.sections.manage.listing.columns.id': 'Member ID',
    'edit.members.sections.manage.listing.columns.type': 'Type',
    'edit.members.sections.manage.addMember': 'Add Member',
    'edit.members.sections.manage.noMembers': 'No members in this group',

    // Add member dialog
    'addMember.title': 'Add Member',
    'addMember.tabs.users': 'Users',
    'addMember.tabs.apps': 'Apps',
    'addMember.tabs.agents': 'Agents',
    'addMember.search.placeholder': 'Search users...',
    'addMember.noResults': 'No users found',
    'addMember.noResultsApps': 'No apps found',
    'addMember.noResultsAgents': 'No agents found',
    'addMember.add': 'Add Selected',
    'addMember.columns.displayName': 'Display Name',
    'addMember.columns.userType': 'User Type',
    'addMember.columns.userId': 'User ID',
    'addMember.error': 'Failed to add member. Please try again.',
    'addMember.fetchError': 'Failed to load users. Please try again.',
    'addMember.fetchAppsError': 'Failed to load apps. Please try again.',
    'addMember.fetchAgentsError': 'Failed to load agents. Please try again.',
    'removeMember.error': 'Failed to remove member. Please try again.',

    // Delete dialog
    'delete.title': 'Delete Group',
    'delete.message': 'Are you sure you want to delete this group?',
    'delete.disclaimer': 'This action cannot be undone. All group associations will be permanently removed.',
    'delete.error': 'Failed to delete group. Please try again.',
    'create.success': 'Group created successfully.',
    'update.success': 'Group updated successfully.',
    'update.error': 'Failed to update group. Please try again.',
    'delete.success': 'Group deleted successfully.',
    'addMember.success': 'Member added successfully.',
    'removeMember.success': 'Member removed successfully.',
  },

  // ============================================================================
  // Roles namespace - Role management feature translations
  // ============================================================================
  roles: {
    // List page
    'listing.title': 'Roles',
    'listing.subtitle': 'Manage roles and their permissions across organization units',
    'listing.addRole': 'Add Role',
    'listing.error': 'Failed to load roles',
    'listing.search.placeholder': 'Search roles...',
    'listing.columns.name': 'Name',
    'listing.columns.description': 'Description',
    'listing.columns.organizationUnit': 'Organization Unit',
    'listing.columns.actions': 'Actions',

    // Create page
    'create.title': 'Create Role',
    'create.error': 'Failed to create role. Please try again.',
    'create.form.name.label': 'Role Name',
    'create.form.name.placeholder': 'Enter role name',
    'create.form.name.required': 'Role name is required',
    'create.form.description.label': 'Description',
    'create.form.description.placeholder': 'Enter role description',
    'create.form.organizationUnit.label': 'Organization Unit',
    'create.form.organizationUnit.required': 'Organization unit is required',

    // Create wizard
    'createWizard.steps.basicInfo': 'Create a Role',
    'createWizard.steps.organizationUnit': 'Organization Unit',
    'createWizard.steps.permissions': 'Permissions',
    'createWizard.basicInfo.title': "Let's give a name to your role",
    'createWizard.basicInfo.suggestions.label': 'In a hurry? Pick a random name:',
    'createWizard.organizationUnit.title': 'Select an organization unit',
    'createWizard.organizationUnit.subtitle': 'Choose the organization unit this role will belong to.',
    'createWizard.permissions.title': 'Assign permissions (optional)',
    'createWizard.permissions.subtitle':
      'Choose what this role grants. You can skip this step and add permissions later.',
    'createWizard.permissions.scopes.label': 'Selected scopes',

    // Edit page
    'edit.page.back': 'Back to Roles',
    'edit.page.error': 'Failed to load role',
    'edit.page.notFound': 'Role not found',
    'edit.page.description.placeholder': 'Add a description...',
    'edit.page.description.empty': 'No description',
    'edit.page.editName': 'Edit role name',
    'edit.page.editDescription': 'Edit role description',
    'edit.page.settingsTabs': 'Role settings tabs',
    'edit.page.tabs.general': 'General',
    'edit.page.tabs.permissions': 'Permissions',
    'edit.page.tabs.assignments': 'Assignments',
    'edit.page.unsavedChanges': 'You have unsaved changes',
    'edit.page.reset': 'Reset',
    'edit.page.save': 'Save Changes',
    'edit.page.saving': 'Saving...',
    'edit.page.saveError': 'Failed to save role. Please try again.',

    // General settings
    'edit.general.sections.quickCopy.copyRoleId': 'Copy Role ID',
    'edit.general.sections.organizationUnit.title': 'Organization Unit',
    'edit.general.sections.organizationUnit.description': 'The organization unit this role belongs to.',
    'edit.general.sections.organizationUnit.copyId': 'Copy Organization Unit ID',
    'edit.general.sections.dangerZone.title': 'Danger Zone',
    'edit.general.sections.dangerZone.description': 'Actions in this section are irreversible. Proceed with caution.',
    'edit.general.sections.dangerZone.deleteRole': 'Delete this role',
    'edit.general.sections.dangerZone.deleteRoleDescription': 'Deleting this role is permanent and cannot be undone.',

    // Permissions settings
    'edit.permissions.title': 'Permissions',
    'edit.permissions.description': 'Select the permissions this role grants, grouped by resource server',
    'edit.permissions.scopes.title': 'Selected scopes',
    'edit.permissions.scopes.description':
      'The OAuth scopes granted by these permissions. Copy them for use in your application.',

    // Assignments settings
    'edit.assignments.sections.manage.title': 'Assigned Users, Groups, Apps & Agents',
    'edit.assignments.sections.manage.description': 'Manage users, groups, apps, and agents assigned to this role',
    'edit.assignments.sections.manage.tabs.users': 'Users',
    'edit.assignments.sections.manage.tabs.groups': 'Groups',
    'edit.assignments.sections.manage.tabs.apps': 'Apps',
    'edit.assignments.sections.manage.tabs.agents': 'Agents',
    'edit.assignments.sections.manage.listing.columns.name': 'Name',
    'edit.assignments.sections.manage.listing.columns.id': 'ID',
    'edit.assignments.sections.manage.listing.columns.type': 'Type',
    'edit.assignments.sections.manage.addAssignment': 'Add',

    // Add assignment dialog
    'assignments.dialog.title': 'Add Assignment',
    'assignments.dialog.tabs.users': 'Users',
    'assignments.dialog.tabs.groups': 'Groups',
    'assignments.dialog.tabs.apps': 'Apps',
    'assignments.dialog.tabs.agents': 'Agents',
    'assignments.dialog.columns.displayName': 'Display Name',
    'assignments.dialog.columns.name': 'Name',
    'assignments.dialog.columns.description': 'Description',
    'assignments.dialog.columns.userType': 'User Type',
    'assignments.dialog.add': 'Add Selected',
    'assignments.dialog.fetchError': 'Failed to load data. Please try again.',
    'assignments.add.error': 'Failed to add assignment. Please try again.',
    'assignments.remove.error': 'Failed to remove assignment. Please try again.',

    // Delete dialog
    'delete.title': 'Delete Role',
    'delete.message': 'Are you sure you want to delete this role?',
    'delete.disclaimer':
      'This action cannot be undone. All role assignments and permissions will be permanently removed.',
    'delete.error': 'Failed to delete role. Please try again.',

    // Success / error toasts
    'create.success': 'Role created successfully.',
    'update.success': 'Role updated successfully.',
    'update.error': 'Failed to update role. Please try again.',
    'delete.success': 'Role deleted successfully.',
    'assignments.add.success': 'Assignment added successfully.',
    'assignments.remove.success': 'Assignment removed successfully.',
  },

  // ============================================================================
  // Connections namespace - Connections feature translations
  // ============================================================================
  connections: {
    // Listing page
    'listing.title': 'Connections',
    'listing.subtitle': 'Configure the external services ThunderID connects to.',
    'listing.search.placeholder': 'Search connections',
    'listing.showingCount': 'Showing {{count}} connections',
    'listing.loading': 'Loading connections...',
    'listing.clearFilters': 'Clear filters',
    'listing.empty.title': 'No connections match your filters',
    'listing.empty.description':
      'Try a different search term, or clear the active filters to see all available connections.',

    // Filters / categories
    'categories.all': 'All',
    'categories.social-login': 'Social Login',
    'categories.enterprise': 'Enterprise',
    'categories.sms': 'SMS',
    'categories.email': 'Email',
    'categories.identity-verification': 'Identity Verification',
    'categories.crm': 'CRM',
    'categories.data-store': 'Data store',
    'categories.trusted-idp': 'Trusted Token Issuer',
    'categories.custom': 'Custom',

    // Card
    'card.configured': 'Configured',
    'card.notConfigured': 'Not configured',
    'card.comingSoon': 'Coming soon',
    'card.addCustom.title': 'Add custom connection',
    'card.addCustom.description': "Set up a connection that isn't in the catalog.",

    // Vendor descriptions
    'vendor.google.description': 'Let users sign in with their Google account.',
    'vendor.github.description': 'Let users sign in with their GitHub account.',
    'vendor.oidc.description': 'Connect any OpenID Connect identity provider.',
    'vendor.oauth.description': 'Connect any OAuth 2.0 identity provider.',
    'vendor.twilio.description': 'Send SMS one-time passcodes via Twilio.',
    'vendor.vonage.description': 'Deliver SMS and email passcodes through Vonage.',
    'vendor.custom-sms.description': 'Route SMS through your own HTTP gateway.',
    'vendor.trustedIdp.description': 'Trusted token issuer for token exchange and ID-JAG.',

    // Add custom connection wizard
    'wizard.title': 'Add custom connection',
    'wizard.steps.type': 'Connection type',
    'wizard.steps.name': 'Name',
    'wizard.steps.configure': 'Configure',
    'wizard.type.heading': 'What kind of connection do you want to add?',
    'wizard.type.subheading':
      "Custom connections aren't in the vendor catalog. Pick the type of integration you want to wire up.",
    'wizard.type.oidc.label': 'OpenID Connect Provider',
    'wizard.type.oidc.description': 'Connect any OpenID Connect identity provider.',
    'wizard.type.oidc.tag': 'Login provider · Enterprise',
    'wizard.type.oauth.label': 'OAuth 2.0 Provider',
    'wizard.type.oauth.description': 'Connect any OAuth 2.0 identity provider.',
    'wizard.type.oauth.tag': 'Login provider · Enterprise',
    'wizard.type.sms.label': 'SMS gateway',
    'wizard.type.sms.description': 'Route SMS through your own HTTP gateway.',
    'wizard.type.sms.tag': 'Message sender · SMS',
    'wizard.type.trustedIdp.label': 'Trusted Token Issuer',
    'wizard.type.trustedIdp.description':
      "Trust an external IdP's identity assertions and exchange them for access tokens.",
    'wizard.type.trustedIdp.tag': 'Token exchange · ID-JAG',
    'wizard.name.title': "Let's give a name to your connection",
    'wizard.name.fieldLabel': 'Connection name',
    'wizard.name.placeholder': 'Enter your connection name',
    'wizard.name.suggestions.label': 'In a hurry? Pick a random name:',
    'wizard.configure.heading': 'Configure your connection',
    'wizard.configure.subheading':
      'Enter the credentials and endpoints for your custom connection. Secrets are stored write-only.',
    'wizard.configure.redirectHint':
      'Register the redirect URI below with your identity provider as an allowed callback URL, then enter the credentials and endpoints it gives you.',

    // Branded configure wizard
    'configure.heading': 'Configure your {{vendor}} connection',
    'configure.subheading': 'Enter the credentials and endpoints for this connection. Secrets are stored write-only.',
    'configure.hint.google':
      'Create an OAuth client for your app in the Google Cloud Console under APIs and Services, Credentials. Register the redirect URI below as an authorized redirect URI, then enter the client ID and client secret Google gives you.',
    'configure.hint.github':
      'Create an OAuth app in GitHub under Settings, Developer settings, OAuth Apps. Set the authorization callback URL to the redirect URI below, then enter the client ID and client secret GitHub gives you.',

    // Connection detail / edit page
    'detail.backToConnections': 'Back to Connections',
    'detail.tabs.general': 'General',
    'detail.tabs.attributeMapping': 'Attribute Configuration',
    'detail.quickCopy.title': 'Quick copy',
    'detail.quickCopy.description': 'Copy connection identifiers for use in your integration.',
    'detail.connectionId': 'Connection ID',
    'detail.connectionId.hint': 'Unique identifier for this connection.',
    'detail.credentials.title': 'Credentials',
    'detail.credentials.description': 'Credentials and endpoints for this connection. Secrets are stored write-only.',
    'detail.dangerZone.title': 'Danger zone',
    'detail.dangerZone.description': 'Actions in this section are irreversible. Proceed with caution.',
    'detail.dangerZone.delete.title': 'Delete connection',
    'detail.dangerZone.delete.description':
      'Permanently delete this connection and all associated data. Any application relying on it will stop accepting logins through this provider. This action cannot be undone.',
    'detail.saveBar.unsaved': 'You have unsaved changes',
    'detail.saveBar.save': 'Save changes',
    'detail.saveBar.saving': 'Saving...',
    'detail.saveBar.discard': 'Discard',

    // Per-vendor configure / edit form
    'form.chrome.configure': 'Configure connection',
    'form.configureTitle': 'Configure {{vendor}}',
    'form.fields.name.label': 'Connection name',
    'form.fields.name.hint': 'Friendly name used to identify this connection in ThunderID.',
    'form.fields.name.placeholder': 'e.g. {{example}}',
    'form.fields.clientId.label': 'Client ID',
    'form.fields.clientId.hint': 'OAuth2 client identifier used for authentication.',
    'form.fields.clientSecret.label': 'Client secret',
    'form.fields.clientSecret.hint': 'OAuth2 client secret issued by your identity provider.',
    'form.fields.redirectUri.label': 'Redirect URI',
    'form.fields.redirectUri.help': 'Add this exact URI to your {{vendor}} OAuth client.',
    'form.fields.redirectUri.hint':
      'Register this exact URL as an authorized redirect URI in your provider. It must match your login gate’s callback URL.',
    'form.fields.scopes.label': 'Scopes',
    'form.fields.scopes.hint':
      'Space-separated scopes to request during sign-in. Defaults to <code>openid email profile</code> if not set.',
    'form.fields.scopes.placeholder': 'openid email profile',
    'form.fields.authorizationEndpoint.label': 'Authorization endpoint',
    'form.fields.authorizationEndpoint.hint': 'Authorization endpoint used to start the OAuth2 sign-in flow.',
    'form.fields.tokenEndpoint.label': 'Token endpoint',
    'form.fields.tokenEndpoint.hint': 'Token endpoint used to exchange the authorization code for tokens.',
    'form.fields.userInfoEndpoint.label': 'UserInfo endpoint',
    'form.fields.userInfoEndpoint.hint': 'Endpoint used to fetch additional profile claims for the signed-in user.',
    'form.fields.jwksEndpoint.label': 'JWKS endpoint',
    'form.fields.jwksEndpoint.hint': 'Endpoint that exposes signing keys for verifying identity tokens.',
    'form.fields.logoutEndpoint.label': 'Logout endpoint',
    'form.fields.issuer.label': 'Issuer',
    'form.fields.issuer.hint': 'Issuer identifier expected in tokens from this provider.',
    'form.fields.tokenExchangeEnabled.label': 'Enable token exchange',
    'form.fields.tokenExchangeEnabled.hint':
      "Let backend services exchange this provider's tokens for ThunderID access tokens.",
    'form.fields.trustedTokenAudience.label': 'Trusted token audience',
    'form.fields.trustedTokenAudience.hint': 'Accepted audience value for external tokens during token exchange.',
    'form.fields.accountSid.label': 'Account SID',
    'form.fields.accountSid.hint': 'Twilio Account SID, starting with <code>AC</code> followed by 32 hex characters.',
    'form.fields.authToken.label': 'Auth token',
    'form.fields.authToken.hint': 'Twilio auth token used to authenticate API requests.',
    'form.fields.apiKey.label': 'API key',
    'form.fields.apiKey.hint': 'Vonage API key from your Vonage dashboard.',
    'form.fields.apiSecret.label': 'API secret',
    'form.fields.apiSecret.hint': 'Vonage API secret used to authenticate API requests.',
    'form.fields.senderId.label': 'Sender ID',
    'form.fields.senderId.hint': 'Phone number or alphanumeric sender ID messages are sent from.',
    'form.sections.federation': 'Federation',
    'form.optional': 'Optional',
    'form.secret.update': 'Update',
    'form.secret.keepHelp': 'Leave unchanged to keep the stored secret.',
    'form.copy': 'Copy',
    'form.copied': 'Copied to clipboard',
    'form.actions.create': 'Create connection',
    'form.actions.save': 'Save changes',
    'form.actions.delete': 'Delete connection',

    // Attribute mapping (authentication providers)
    'attributeMapping.userType.label': 'Default User Type',
    'attributeMapping.userType.placeholder': 'Select a user type',
    'attributeMapping.userTypeRequired': 'Select a default user type.',
    'attributeMapping.add': 'Add Mapping',
    'attributeMapping.externalAttribute.label': 'External Attribute',
    'attributeMapping.externalAttribute.placeholder': 'e.g. given_name',
    'attributeMapping.localAttribute.label': 'Local Attribute',
    'attributeMapping.localAttribute.placeholder': 'e.g. firstName',
    // Section 1 — user type resolution
    'attributeMapping.resolution.title': 'User type resolution',
    'attributeMapping.resolution.description':
      'Select which local user type an external identity resolves to, choosing the attribute-mapping profile applied to it.',
    'attributeMapping.resolution.dynamic.label': 'Resolve user type from an attribute',
    'attributeMapping.resolution.externalAttribute.placeholder': 'e.g. user_type',
    'attributeMapping.resolution.externalAttribute.helper': 'The attribute whose value decides the user type.',
    'attributeMapping.resolution.valueMapping.title': 'Value Mapping',
    'attributeMapping.resolution.valueMapping.enable': 'Enable value mapping',
    'attributeMapping.resolution.valueMapping.hint': 'Map each attribute value to a user type.',
    'attributeMapping.resolution.valueMapping.externalValue': 'External Value',
    'attributeMapping.resolution.valueMapping.localUserType': 'Local User Type',
    'attributeMapping.resolution.valueMapping.valuePlaceholder': 'e.g. employee',
    'attributeMapping.resolution.addValue': 'Add Value',
    'attributeMapping.resolution.default.helperFallback':
      "Used when the attribute is missing or its value isn't mapped.",
    // Section 2 — attribute mappings by user type
    'attributeMapping.mappings.title': 'Attribute Mappings',
    'attributeMapping.mappings.description':
      'Map the attributes this provider returns onto your local user schema. Define a separate mapping set for each user type.',
    'attributeMapping.mappings.userType': 'User Type',
    'attributeMapping.mappings.userTypeRequired': 'Select a user type for this mapping set.',
    'attributeMapping.mappings.addUserType': 'Add User Type',
    'attributeMapping.mappings.remove': 'Remove',
    // Section 3 — account linking
    'attributeMapping.linking.title': 'Account Linking',
    'attributeMapping.linking.description': 'The attributes used to find the associated local user.',
    'attributeMapping.linking.label': 'External Attribute',
    'attributeMapping.linking.labelCombo': 'External Attributes',
    'attributeMapping.linking.placeholder': 'e.g. email',
    'attributeMapping.linking.addAttribute': 'Add Attribute',
    'attributeMapping.linking.and': 'AND',

    // Delete dialog
    'delete.title': 'Delete connection',
    'delete.message': 'Are you sure you want to delete “{{name}}”? This action cannot be undone.',
    'delete.usages.loading': 'Checking affected resources…',
    'delete.usages.more': '+{{count}} more',
    'delete.blocking.title': 'This connection cannot be deleted until the following resources are updated or removed:',

    // Toasts
    'create.success': 'Connection created successfully.',
    'create.error': 'Failed to create connection.',
    'update.success': 'Connection updated successfully.',
    'update.error': 'Failed to update connection.',
    'delete.success': 'Connection deleted successfully.',
    'delete.error': 'Failed to delete connection.',

    // Errors / validation
    'error.duplicateName': 'A connection with this name already exists.',
    'error.loadFailed': 'Failed to load connection.',
    'validation.required': 'This field is required.',
    'validation.url': 'Enter a valid URL.',
    'validation.accountSid': 'Enter a valid Account SID: “AC” followed by 32 hexadecimal characters.',
  },

  // ============================================================================
  // Trusted issuers namespace - Trusted issuer (trust-only OIDC connection) feature translations
  // ============================================================================
  trustedIssuers: {
    // Validation
    'validation.required': 'This field is required.',
    'validation.url': 'Enter a valid https:// URL.',

    // Create form
    'create.title': 'Add trusted issuer',
    'create.subtitle':
      'Register an external identity provider whose identity assertions ThunderID can exchange for access tokens.',
    'create.duplicateName': 'A trusted issuer with this name already exists.',
    'create.submit': 'Add trusted issuer',
    'create.form.name.label': 'Name',
    'create.form.issuer.label': 'Issuer URI',
    'create.form.issuer.hint': "The issuer URI from the external IdP's OpenID Connect discovery document.",
    'create.form.jwksEndpoint.label': 'JWKS endpoint',
    'create.form.jwksEndpoint.hint':
      'The JWKS endpoint used to validate the signature of incoming identity assertions.',
    'create.form.tokenExchangeEnabled.label': 'Enable token exchange',
    'create.form.tokenExchangeEnabled.hint': 'Exchange subject tokens from this issuer for access tokens.',
    'create.form.idJagEnabled.label': 'Enable Identity Assertion JWT Authorization Grant (ID-JAG)',
    'create.form.idJagEnabled.hint':
      'Accept and exchange signed identity assertions from this issuer for access tokens.',

    // Detail page
    'detail.back': 'Back to connections',
    'detail.loadError': 'Failed to load trusted issuer.',
    'detail.duplicateName': 'A trusted issuer with this name already exists.',
    'detail.general.title': 'General',
    'detail.general.description': 'Core identity of this trusted issuer.',
    'detail.tokenExchange.title': 'Token Exchange',
    'detail.tokenExchange.description': 'Exchange subject tokens from this issuer for access tokens.',
    'detail.tokenExchange.audience.label': 'Trusted token audience',
    'detail.tokenExchange.audience.hint':
      "An additional audience value {{productName}} will accept in subject tokens from this issuer. Tokens whose audience is {{productName}}'s own issuer URL are always accepted.",
    'detail.consumption.title': 'Identity Assertion JWT Authorization Grant (ID-JAG)',
    'detail.idJag.description': 'Accept and exchange signed identity assertions from this issuer for access tokens.',
    'detail.idJag.enabledNote': 'Identity assertions from this issuer are accepted via the ID-JAG protocol.',
    'detail.dangerZone.title': 'Danger zone',
    'detail.dangerZone.delete.title': 'Delete trusted issuer',
    'detail.dangerZone.delete.description':
      'Applications relying on assertions from this issuer will stop receiving tokens. This cannot be undone.',
    'detail.saveBar.unsaved': 'You have unsaved changes',
    'detail.saveBar.discard': 'Discard',
    'detail.saveBar.save': 'Save changes',

    // Delete dialog
    'delete.title': 'Delete trusted issuer',
    'delete.message':
      'Delete "{{name}}"? Applications relying on assertions from this issuer will stop receiving tokens. This cannot be undone.',

    // Toasts
    'create.success': 'Trusted issuer created successfully.',
    'create.error': 'Failed to create trusted issuer. Please try again.',
    'update.success': 'Trusted issuer updated successfully.',
    'update.error': 'Failed to update trusted issuer. Please try again.',
    'delete.success': 'Trusted issuer deleted successfully.',
    'delete.error': 'Failed to delete trusted issuer. Please try again.',
  },

  // ============================================================================
  // Authentication namespace - Authentication feature translations
  // ============================================================================
  auth: {
    signIn: 'Sign In',
    signUp: 'Sign Up',
    signOut: 'Sign Out',
    forgotPassword: 'Forgot Password?',
    resetPassword: 'Reset Password',
    changePassword: 'Change Password',
    rememberMe: 'Remember Me',
    welcomeBack: 'Welcome Back',
    createAccount: 'Create Account',
    alreadyHaveAccount: 'Already have an account?',
    dontHaveAccount: "Don't have an account?",
    enterEmail: 'Enter your email',
    enterPassword: 'Enter your password',
    confirmPassword: 'Confirm Password',
    passwordMismatch: 'Passwords do not match',
    invalidCredentials: 'Invalid credentials',
    accountLocked: 'Account is locked',
    sessionExpired: 'Session expired. Please sign in again.',
    signInSuccess: 'Signed in successfully',
    signUpSuccess: 'Account created successfully',
    passwordResetSent: 'Password reset link sent to your email',
    passwordResetSuccess: 'Password reset successfully',
  },

  // ============================================================================
  // MFA namespace - Multi-factor authentication feature translations
  // ============================================================================
  mfa: {
    title: 'Multi-Factor Authentication',
    setupMfa: 'Set Up MFA',
    enableMfa: 'Enable MFA',
    disableMfa: 'Disable MFA',
    verificationCode: 'Verification Code',
    enterCode: 'Enter verification code',
    sendCode: 'Send Code',
    resendCode: 'Resend Code',
    invalidCode: 'Invalid verification code',
    codeExpired: 'Verification code expired',
    scanQrCode: 'Scan this QR code with your authenticator app',
    backupCodes: 'Backup Codes',
    saveBackupCodes: 'Save these backup codes in a secure place',
  },

  // ============================================================================
  // Social namespace - Social login feature translations
  // ============================================================================
  social: {
    continueWith: 'Continue with',
    signInWith: 'Sign in with',
    google: 'Google',
    facebook: 'Facebook',
    github: 'GitHub',
    microsoft: 'Microsoft',
  },

  // ============================================================================
  // Consent namespace - Consent feature translations
  // ============================================================================
  consent: {
    title: 'Consent Required',
    message: 'The application is requesting access to your information',
    requestedPermissions: 'Requested Permissions',
    allow: 'Allow',
    deny: 'Deny',
    learnMore: 'Learn More',
  },

  // ============================================================================
  // Errors namespace - Error messages translations
  // ============================================================================
  errors: {
    authenticationFailed: 'Authentication failed',
    unauthorizedAccess: 'Unauthorized access',
    accessDenied: 'Access denied',
    invalidRequest: 'Invalid request',
    serverError: 'Server error occurred',
    networkError: 'Network error. Please check your connection.',
    redirectFailed: 'Redirect failed',
    'page.defaultTitle': "Oops, that didn't work",
    'page.defaultDescription': "We're sorry, we ran into a problem. Please try again!",
    'page.invalidRequest.title': 'Oh no, we ran into a problem!',
    'page.invalidRequest.description': 'The request is invalid. Please check and try again.',
  },

  // ============================================================================
  // Applications - Applications feature translations
  // ============================================================================
  applications: {
    'listing.title': 'Applications',
    'listing.subtitle': 'Manage your applications and services',
    'listing.addApplication': 'Add Application',
    'listing.columns.name': 'Name',
    'listing.columns.clientId': 'Client ID',
    'listing.columns.actions': 'Actions',
    'listing.columns.template': 'Type',
    'listing.search.placeholder': 'Search ..',
    'delete.title': 'Delete Application',
    'delete.message': 'Are you sure you want to delete this application? This action cannot be undone.',
    'delete.disclaimer': 'Warning: All associated data, configurations, and access tokens will be permanently removed.',
    'regenerateSecret.dialog.title': 'Regenerate Client Secret',
    'regenerateSecret.dialog.message':
      'Are you sure you want to regenerate the client secret for this application? This will immediately invalidate the current client secret and generate a new one.',
    'regenerateSecret.dialog.disclaimer':
      'Warning: Regenerating the client secret will invalidate the current secret and the application may stop working until the new client secret is updated in its configuration.',
    'regenerateSecret.dialog.confirmButton': 'Regenerate',
    'regenerateSecret.dialog.regenerating': 'Regenerating...',
    'regenerateSecret.dialog.error': 'Failed to regenerate client secret. Please try again.',
    'regenerateSecret.success.title': 'Save Your New Client Secret',
    'regenerateSecret.success.subtitle': "This is the only time you'll see this secret. Store it somewhere safe.",
    'regenerateSecret.success.secretLabel': 'New Client Secret',
    'regenerateSecret.success.copyButton': 'Copy to clipboard',
    'regenerateSecret.success.toggleVisibility': 'Toggle secret visibility',
    'regenerateSecret.success.copySecret': 'Copy Secret',
    'regenerateSecret.success.copied': 'Copied to clipboard',
    'regenerateSecret.success.securityReminder.title': 'Security Reminder',
    'regenerateSecret.success.securityReminder.description':
      'Never share your client secret publicly or store it in version control. If you believe your secret has been compromised, regenerate it immediately.',
    'regenerateFlowSecret.dialog.title': 'Regenerate Flow Secret',
    'regenerateFlowSecret.dialog.message':
      'Are you sure you want to regenerate the Flow Secret for this application? This will immediately invalidate the current Flow Secret and generate a new one.',
    'regenerateFlowSecret.dialog.disclaimer':
      'Warning: Regenerating the Flow Secret invalidates the current secret. Server-side flow initiation will fail until the new Flow Secret is deployed.',
    'regenerateFlowSecret.dialog.confirmButton': 'Regenerate',
    'regenerateFlowSecret.dialog.regenerating': 'Regenerating...',
    'regenerateFlowSecret.dialog.error': 'Failed to regenerate Flow Secret. Please try again.',
    'regenerateFlowSecret.success.title': 'Save Your New Flow Secret',
    'regenerateFlowSecret.success.subtitle':
      "This is the only time you'll see this Flow Secret. Store it somewhere safe.",
    'regenerateFlowSecret.success.secretLabel': 'New Flow Secret',
    'regenerateFlowSecret.success.copySecret': 'Copy Flow Secret',
    'regenerateFlowSecret.success.copied': 'Copied to clipboard',
    'regenerateFlowSecret.success.securityReminder.title': 'Security Reminder',
    'regenerateFlowSecret.success.securityReminder.description':
      'Never share your Flow Secret publicly or store it in version control. If you believe your Flow Secret has been compromised, regenerate it immediately.',
    'regenerateFlowSecret.snackbar.success': 'Flow Secret regenerated successfully.',
    'onboarding.preview.title': 'Preview',
    'onboarding.preview.signin': 'Sign In',
    'onboarding.preview.username': 'Username',
    'onboarding.preview.usernamePlaceholder': 'Enter your Username',
    'onboarding.preview.password': 'Password',
    'onboarding.preview.passwordPlaceholder': 'Enter your Password',
    'onboarding.preview.signInButton': 'Sign In',
    'onboarding.preview.passkeySignIn': 'Sign in with Passkey',
    'onboarding.preview.mobileNumber': 'Mobile Number',
    'onboarding.preview.mobileNumberPlaceholder': 'Enter your mobile number',
    'onboarding.preview.sendOtpButton': 'Send OTP',
    'onboarding.preview.dividerText': 'or',
    'onboarding.preview.continueWith': 'Continue with {{providerName}}',
    'onboarding.steps.name': 'Create an Application',
    'onboarding.steps.organizationUnit': 'Organization Unit',
    'onboarding.steps.design': 'Design',
    'onboarding.steps.options': 'Sign In Options',
    'onboarding.steps.experience': 'Sign-In Experience',
    'onboarding.steps.stack': 'Technology Stack',
    'onboarding.steps.configure': 'Configuration',
    'onboarding.steps.walletConfigure': 'Connect Your Wallet',
    'onboarding.steps.clientType': 'Client type',
    'onboarding.steps.quickTest': 'Quick Test',
    'onboarding.steps.export': 'Integration Setup',
    'onboarding.steps.complete': 'Setup Complete',
    'onboarding.steps.summary': 'Summary',
    'onboarding.organizationUnit.title': 'Select an organization unit',
    'onboarding.organizationUnit.subtitle': 'Choose the organization unit this application will belong to.',
    'onboarding.organizationUnit.fieldLabel': 'Organization Unit',
    'onboarding.configure.name.title': "Let's give a name to your application",
    'onboarding.configure.name.fieldLabel': 'Application Name',
    'onboarding.configure.name.placeholder': 'Enter your application name',
    'onboarding.configure.name.suggestions.label': 'In a hurry? Pick a random name:',
    'onboarding.mcp.clientType.title': 'Client type',
    'onboarding.mcp.clientType.subtitle': 'How will this client obtain tokens?',
    'onboarding.mcp.clientType.userDelegated.title': 'On behalf of a user',
    'onboarding.mcp.clientType.userDelegated.description':
      'A client in a host app (IDE, desktop app, or chat client) that acts on behalf of a signed-in user. Uses Authorization Code with PKCE.',
    'onboarding.mcp.clientType.m2m.title': 'On its own behalf',
    'onboarding.mcp.clientType.m2m.description':
      'A client that authenticates with its own credentials without user interaction. Uses Client Credentials.',
    'onboarding.mcp.clientType.preview.label': 'What you get',
    'onboarding.mcp.clientType.preview.nextUserDelegated': 'Add your redirect URIs below.',
    'onboarding.mcp.clientType.preview.nextM2m': 'Next: your client ID and secret are generated.',
    'onboarding.mcp.connection.title': 'Add a redirect URI',
    'onboarding.mcp.connection.subtitle': 'Where should users be sent after they authorize this client?',
    'onboarding.mcp.connection.redirectUris.label': 'Redirect URIs',
    'onboarding.mcp.connection.redirectUris.hint':
      'Each URI must be a loopback address (http://localhost or http://127.0.0.1) or use HTTPS. At least one is required.',
    'onboarding.mcp.connection.redirectUris.addUri': 'Add redirect URI',
    'onboarding.mcp.connection.redirectUris.remove': 'Remove redirect URI',
    'onboarding.mcp.connection.redirectUris.error.empty': 'Enter a redirect URI.',
    'onboarding.mcp.connection.redirectUris.error.invalid': 'Enter a valid loopback (http://127.0.0.1) or HTTPS URI.',
    'onboarding.mcp.connection.inspectorHint': 'Testing with MCP Inspector? Use {{uri}}',
    'onboarding.mcp.connection.inspectorHint.copyAriaLabel': 'Copy MCP Inspector callback URI',
    'onboarding.mcp.oauthProfile.label': 'OAuth profile',
    'onboarding.mcp.oauthProfile.authCodePkce': 'Authorization Code + PKCE (required)',
    'onboarding.mcp.oauthProfile.publicClient': 'Public client',
    'onboarding.mcp.oauthProfile.refreshTokens': 'Refresh tokens',
    'onboarding.mcp.oauthProfile.clientCredentials': 'Client Credentials',
    'onboarding.mcp.oauthProfile.confidentialClient': 'Confidential client',
    'onboarding.mcp.oauthProfile.clientSecretIssued': 'Client secret issued',
    'onboarding.mcp.complete.title': 'Your MCP client is ready',
    'onboarding.mcp.complete.subtitle.userDelegated':
      'Use these pre-registered credentials and endpoints to connect your client.',
    'onboarding.mcp.complete.subtitle.m2m': "Save your client secret now — it's shown only once.",
    'onboarding.mcp.complete.credentials.title': 'Pre-registered client credentials',
    'onboarding.mcp.complete.endpoints.title': 'Endpoints',
    'onboarding.mcp.complete.endpoints.issuer': 'Issuer',
    'onboarding.mcp.complete.endpoints.asMetadata': 'Authorization server metadata',
    'onboarding.mcp.complete.endpoints.oidcDiscovery': 'OpenID Connect discovery',
    'onboarding.mcp.complete.endpoints.authorize': 'Authorization endpoint',
    'onboarding.mcp.complete.endpoints.token': 'Token endpoint',
    'onboarding.mcp.complete.redirectUris.title': 'Registered redirect URIs',
    'onboarding.mcp.complete.m2m.secretPurpose': 'Used to authenticate at the token endpoint.',
    'onboarding.mcp.complete.m2m.warning.title': 'Save your client secret now',
    'onboarding.mcp.complete.m2m.warning.body':
      "This secret is shown only once. Store it securely — you'll need to regenerate it if it's lost.",
    'onboarding.mcp.complete.m2m.tokenHint':
      "Request tokens with grant_type=client_credentials and include the target MCP server's resource parameter so the token is audience-scoped.",
    'onboarding.mcp.complete.goToApp': 'Go to MCP client',
    'onboarding.mcp.complete.copySecret': 'Copy secret',
    'onboarding.mcp.complete.copied': 'Copied',
    'onboarding.configure.design.title': 'Design Your Application',
    'onboarding.configure.design.subtitle': 'Customize the appearance of your application',
    'onboarding.configure.design.logo.title': 'Application Logo',
    'onboarding.configure.design.logo.shuffle': 'Shuffle',
    'onboarding.configure.design.logo.chooseLogo': 'Choose logo or emoji',
    'onboarding.configure.design.theme.title': 'Theme',
    'onboarding.configure.design.theme.emptyState': 'No themes configured',
    'onboarding.configure.design.theme.noDescription': 'No description',
    'onboarding.configure.design.theme.emptyStateHint': 'You can configure themes later from the Design settings.',
    'onboarding.configure.SignInOptions.title': 'Sign In Options',
    'onboarding.configure.SignInOptions.subtitle': 'Choose how users will sign-in to your application',
    'onboarding.configure.SignInOptions.usernamePassword': 'Username & Password',
    'onboarding.configure.SignInOptions.google': 'Google',
    'onboarding.configure.SignInOptions.github': 'GitHub',
    'onboarding.configure.SignInOptions.passkey': 'Passkey',
    'onboarding.configure.SignInOptions.notConfigured': 'Not configured',
    'onboarding.configure.SignInOptions.noFlowFound':
      'No flow found for the selected sign-in options. Please try a different combination.',
    'onboarding.configure.SignInOptions.noSelectionWarning':
      'At least one sign-in option is required. Please select at least one authentication method.',
    'onboarding.configure.SignInOptions.noIntegrations':
      'No social sign-in integrations available. Please configure an integration first.',
    'onboarding.configure.SignInOptions.hint':
      'You can always change these settings later in the application settings.',
    'onboarding.configure.SignInOptions.preConfiguredFlows.selectFlow': 'Select already configured flow',
    'onboarding.configure.SignInOptions.preConfiguredFlows.searchFlows': 'Search flows...',
    'onboarding.configure.SignInOptions.smsOtp': 'SMS OTP',
    'onboarding.configure.SignInOptions.loading': 'Loading...',
    'onboarding.configure.SignInOptions.error': 'Failed to load authentication methods: {{error}}',
    'onboarding.configure.experience.title': 'Sign-In Experience',
    'onboarding.configure.experience.subtitle': 'Select how and who can access your application',
    'onboarding.configure.experience.subtitleWithoutUserTypes': 'Select how users access your application',
    'onboarding.configure.experience.access.userTypes.title': 'User Access',
    'onboarding.configure.experience.access.userTypes.subtitle': 'Select which user types can access this application',
    'onboarding.configure.experience.approach.title': 'Sign-In Approach',
    'onboarding.configure.experience.approach.subtitle': 'Select how users will access this application',
    'onboarding.configure.approach.inbuilt.title': 'Redirect to {{product}} sign-in/sign-up handling pages',
    'onboarding.configure.approach.inbuilt.description':
      'Users will be redirected to system-hosted sign-in and sign-up pages, which can be customized and branded using the Flow Designer and easily integrated with SDKs in just a few steps.',
    'onboarding.configure.approach.native.title': 'Embedded sign-in/sign-up components in your app',
    'onboarding.configure.approach.native.description':
      'Users will sign in or sign up through your app using the UI components or APIs provided by {{product}}. You can customize and brand the flows using the designer or through code.',
    'onboarding.configure.stack.title': 'Choose a type',
    'onboarding.configure.stack.subtitle': 'Select the type that best matches your application.',
    'onboarding.templateSelect.subtitle':
      'Pick the technology that best matches your application, selecting one starts the setup.',
    'onboarding.templateSelect.searchPlaceholder': 'Search types by name',
    'onboarding.templateSelect.count_one': 'Showing {{count}} type',
    'onboarding.templateSelect.count_other': 'Showing {{count}} types',
    'onboarding.configure.stack.categoriesLabel': 'Categories',
    'onboarding.configure.stack.comingSoon': 'Coming Soon',
    'onboarding.configure.stack.category.all': 'All',
    'onboarding.configure.stack.category.web': 'Web',
    'onboarding.configure.stack.category.backend': 'Backend',
    'onboarding.configure.stack.category.mobile': 'Mobile',
    'onboarding.configure.stack.category.ai': 'AI',
    'onboarding.configure.stack.technology.title': 'Technology',
    'onboarding.configure.stack.technology.subtitle': 'What technology are you using to build your application?',
    'onboarding.configure.stack.technology.express.title': 'Express',
    'onboarding.configure.stack.technology.express.description': 'Server-side Node.js application built with Express',
    'onboarding.configure.stack.technology.react.title': 'React',
    'onboarding.configure.stack.technology.react.description': 'Single-page application built with React',
    'onboarding.configure.stack.technology.nextjs.title': 'Next.js',
    'onboarding.configure.stack.technology.nextjs.description': 'Full-stack React framework with server-side rendering',
    'onboarding.configure.stack.technology.angular.title': 'Angular',
    'onboarding.configure.stack.technology.angular.description': 'Single-page application built with Angular',
    'onboarding.configure.stack.technology.vue.title': 'Vue',
    'onboarding.configure.stack.technology.vue.description': 'Single-page application built with Vue.js',
    'onboarding.configure.stack.technology.ios.title': 'iOS',
    'onboarding.configure.stack.technology.ios.description': 'Native iOS application (Swift or Objective-C)',
    'onboarding.configure.stack.technology.android.title': 'Android',
    'onboarding.configure.stack.technology.android.description': 'Native Android application (Kotlin or Java)',
    'onboarding.configure.stack.technology.springboot.title': 'Spring Boot',
    'onboarding.configure.stack.technology.springboot.description': 'Java backend application with Spring Boot',
    'onboarding.configure.stack.technology.nodejs.title': 'Node.js',
    'onboarding.configure.stack.technology.nodejs.description': 'Backend service built with Node.js',
    'onboarding.configure.stack.technology.nuxt.title': 'Nuxt',
    'onboarding.configure.stack.technology.nuxt.description': 'Full-stack Vue framework with server-side rendering',
    'onboarding.configure.stack.technology.vanillaJs.title': 'JavaScript',
    'onboarding.configure.stack.technology.vanillaJs.description': 'Browser application built with vanilla JavaScript',
    'onboarding.configure.stack.technology.mcpClient.title': 'MCP Client',
    'onboarding.configure.stack.technology.mcpClient.description':
      'Register an MCP client to connect to MCP servers with OAuth 2.1 authorization.',
    'onboarding.configure.stack.platform.title': 'Application Type',
    'onboarding.configure.stack.platform.subtitle': 'This helps us configure the right settings for your app',
    'onboarding.configure.stack.dividerLabel': 'OR',
    'onboarding.configure.stack.platform.browser.title': 'Browser App',
    'onboarding.configure.stack.platform.browser.description': 'Single-page apps running in browsers',
    'onboarding.configure.stack.platform.full_stack.title': 'Full-Stack App',
    'onboarding.configure.stack.platform.full_stack.description': 'Apps with both server and client code',
    'onboarding.configure.stack.platform.mobile.title': 'Mobile App',
    'onboarding.configure.stack.platform.mobile.description': 'Native or hybrid mobile applications',
    'onboarding.configure.stack.platform.backend.title': 'Backend Service',
    'onboarding.configure.stack.platform.backend.description': 'Server-to-server APIs and services',
    'onboarding.configure.stack.platform.wallet.title': 'Digital Wallet',
    'onboarding.configure.stack.platform.wallet.description': 'OpenID4VCI wallet that requests verifiable credentials',
    'onboarding.configure.stack.platform.custom.title': 'Custom',
    'onboarding.configure.stack.platform.custom.description':
      'Fully customizable application with all configuration options available',
    'onboarding.configure.details.wallet.vendor.label': 'Wallet',
    'onboarding.configure.details.wallet.vendor.custom': 'Custom',
    'onboarding.configure.details.wallet.clientId.label': 'Client ID',
    'onboarding.configure.details.wallet.clientId.placeholder': 'The wallet’s client ID',
    'onboarding.configure.details.wallet.clientId.helperText':
      'Leave empty to auto-generate. Known wallets pre-fill their fixed client ID.',
    'onboarding.configure.details.wallet.prefilled.helperText': 'Pre-filled for the selected wallet and not editable.',
    'onboarding.configure.details.title': 'Configuration',
    'onboarding.configure.details.description': 'Configure where your application is hosted and callback settings',
    'onboarding.configure.details.wallet.title': 'Connect Your Wallet',
    'onboarding.configure.details.wallet.description':
      'Pick a known wallet, or choose Custom to add your own client ID.',
    'onboarding.configure.details.wallet.duplicate.known':
      'A {{vendor}} wallet application already exists — each wallet can be connected only once.',
    'onboarding.configure.details.wallet.duplicate.custom':
      'An application with this client ID already exists. Enter a different client ID.',
    'onboarding.configure.details.hostingUrl.label': 'Where is your application hosted?',
    'onboarding.configure.details.hostingUrl.placeholder': 'https://myapp.example.com',
    'onboarding.configure.details.hostingUrl.helperText': 'The URL where users will access your application',
    'onboarding.configure.details.hostingUrl.error.required': 'Application hosting URL is required',
    'onboarding.configure.details.hostingUrl.error.invalid':
      'Please enter a valid URL (must start with http:// or https://)',
    'onboarding.configure.details.callbackUrl.label': 'After Sign-in URL',
    'onboarding.configure.details.callbackUrl.placeholder': 'https://myapp.example.com/callback',
    'onboarding.configure.details.callbackUrl.helperText': 'The URL where users will be redirected after signing in',
    'onboarding.configure.details.callbackUrl.error.required': 'After sign-in URL is required',
    'onboarding.configure.details.callbackUrl.error.invalid':
      'Please enter a valid URL (must start with http:// or https://)',
    'onboarding.configure.details.callbackUrl.info':
      'The URL is where users will be redirected after sign-in. For most applications, using the same URL as your application access URL is recommended.',
    'onboarding.configure.details.sameAsHosting': 'Same as the application URL',
    'onboarding.configure.details.callbackMode.same': 'Same as Application Access URL',
    'onboarding.configure.details.callbackMode.custom': 'Custom URL',
    'onboarding.configure.details.mobile.title': 'Mobile Application Configuration',
    'onboarding.configure.details.mobile.description':
      'Configure the deep link or universal link for your mobile application',
    'onboarding.configure.details.mobile.info':
      'Deep links (e.g., myapp://callback) or universal links (e.g., https://example.com/callback) are used to redirect users back to your mobile app after authentication.',
    'onboarding.configure.details.deeplink.label': 'Deep Link / Universal Link',
    'onboarding.configure.details.deeplink.placeholder': 'myapp://callback or https://example.com/callback',
    'onboarding.configure.details.deeplink.helperText': 'The custom URL scheme or universal link for your mobile app',
    'onboarding.configure.details.passkey.title': 'Passkey Settings',

    'onboarding.configure.passkey.title': 'Passkey Configuration',
    'onboarding.configure.passkey.description': 'Configure the Relying Party details for Passkey authentication.',
    'onboarding.configure.details.relyingPartyId.label': 'Relying Party ID',
    'onboarding.configure.details.relyingPartyId.placeholder': 'e.g., example.com',
    'onboarding.configure.details.relyingPartyId.helperText': 'The domain where the WebAuthn credential is valid',
    'onboarding.configure.details.relyingPartyName.label': 'Relying Party Name',
    'onboarding.configure.details.relyingPartyName.placeholder': 'e.g., My Application',
    'onboarding.configure.details.relyingPartyName.helperText': 'A user-friendly name for the Relying Party',
    'onboarding.configure.details.noConfigRequired.title': 'No Additional Configuration Needed',
    'onboarding.configure.details.noConfigRequired.description':
      'Your application is ready to go! You can proceed to the next step.',
    'onboarding.configure.details.userTypes.description': 'Select which user types can access this application',
    'onboarding.configure.details.userTypes.error': 'Please select at least one user type',
    'onboarding.configure.setup.title': 'Application Setup',
    'onboarding.configure.setup.subtitle': 'Select the technology stack for your application',
    'onboarding.configure.setup.platform.label': 'What technology are you using?',
    'onboarding.configure.setup.platform.browser.title': 'Browser',
    'onboarding.configure.setup.platform.browser.description': 'Single page apps (React, Vue, Angular)',
    'onboarding.configure.setup.platform.full_stack.title': 'Server + Browser',
    'onboarding.configure.setup.platform.full_stack.description': 'Full-stack apps (Next.js, Remix)',
    'onboarding.configure.setup.platform.mobile.title': 'Mobile Device',
    'onboarding.configure.setup.platform.mobile.description': 'iOS, Android, React Native',
    'onboarding.configure.setup.platform.desktop.title': 'Desktop',
    'onboarding.configure.setup.platform.desktop.description': 'Electron, Tauri apps',
    'onboarding.configure.setup.platform.backend.title': 'Backend Service',
    'onboarding.configure.setup.platform.backend.description': 'Server-to-server APIs',
    'onboarding.configure.setup.info':
      'Your {{platform}} configuration is set up automatically. You can customize these settings later.',
    'onboarding.configure.oauth.title': 'Configure OAuth',
    'onboarding.configure.oauth.subtitle': 'Configure OAuth2/OIDC settings for your application (optional)',
    'onboarding.configure.oauth.optional': 'This step is optional.',
    'onboarding.configure.oauth.hostedGuidance':
      'OAuth configuration is recommended for redirect-based authentication. Configure OAuth settings to enable secure authentication flows.',
    'onboarding.configure.oauth.nativeGuidance':
      'OAuth configuration is optional for custom sign-in UI. You can skip this step and use the Flow API for authentication instead.',
    'onboarding.configure.oauth.publicClient.label': 'Public Client',
    'onboarding.configure.oauth.pkce.label': 'Enable PKCE',
    'onboarding.configure.oauth.redirectURIs.fieldLabel': 'Redirect URIs',
    'onboarding.configure.oauth.redirectURIs.placeholder': 'https://localhost:3000/callback',
    'onboarding.configure.oauth.redirectURIs.addButton': 'Add',
    'onboarding.configure.oauth.redirectURIs.errors.empty': 'Please enter a redirect URI',
    'onboarding.configure.oauth.redirectURIs.errors.invalid':
      'Please enter a valid URL (must start with http:// or https://)',
    'onboarding.configure.oauth.redirectURIs.errors.duplicate': 'This redirect URI has already been added',
    'onboarding.configure.oauth.grantTypes.label': 'Grant Types',
    'onboarding.configure.oauth.grantTypes.authorizationCode': 'Authorization Code',
    'onboarding.configure.oauth.grantTypes.refreshToken': 'Refresh Token',
    'onboarding.configure.oauth.grantTypes.clientCredentials': 'Client Credentials',
    'onboarding.configure.oauth.tokenEndpointAuthMethod.label': 'Token Endpoint Authentication Method',
    'onboarding.configure.oauth.tokenEndpointAuthMethod.clientSecretBasic': 'Client Secret Basic',
    'onboarding.configure.oauth.tokenEndpointAuthMethod.clientSecretPost': 'Client Secret Post',
    'onboarding.configure.oauth.tokenEndpointAuthMethod.none': 'None',
    'onboarding.configure.oauth.errors.publicClientRequiresPKCE':
      'Public clients must have PKCE enabled. PKCE is automatically enabled for public clients.',
    'onboarding.configure.oauth.errors.publicClientRequiresNone':
      'Public clients must use "None" as the token endpoint authentication method.',
    'onboarding.configure.oauth.errors.publicClientNoClientCredentials':
      'Public clients cannot use the client_credentials grant type.',
    'onboarding.configure.oauth.errors.authorizationCodeRequiresRedirectURIs':
      'Authorization Code grant type requires at least one redirect URI.',
    'onboarding.configure.oauth.errors.clientCredentialsRequiresAuth':
      'Client Credentials grant type cannot use "None" authentication method.',
    'onboarding.configure.oauth.errors.atLeastOneGrantTypeRequired': 'At least one grant type must be selected.',
    'onboarding.configure.oauth.errors.refreshTokenRequiresAuthorizationCode':
      'Refresh Token grant type requires Authorization Code grant type to be selected.',
    'onboarding.complete.title': 'Application Created Successfully',
    'onboarding.complete.description':
      'Your application has been created. Please save the client secret below as it will only be shown once.',
    'onboarding.complete.warning.title': 'Important: Save Your Client Secret',
    'onboarding.complete.warning.message':
      'This is the only time you will see this client secret. Please copy it now and store it securely. You will not be able to retrieve it later.',
    'onboarding.complete.clientSecret.label': 'Client Secret',
    'onboarding.creating': 'Creating...',
    'onboarding.skipAndCreate': 'Skip & Create',
    'onboarding.createApplication': 'Create Application',
    'quickTest.title': 'Test Your Application',
    'quickTest.subtitle': 'Verify that your sign-in configuration works with a test user before going live.',
    'quickTest.summary.text': '{{appName}} is configured with {{methods}} sign-in.',
    'quickTest.summary.method.usernamePassword': 'username & password',
    'quickTest.summary.method.passkey': 'passkeys',
    'quickTest.summary.method.social': '{{count}} social provider',
    'quickTest.summary.method.social_plural': '{{count}} social providers',
    'quickTest.summary.and': 'and',
    'quickTest.createUser.title': 'Test User',
    'quickTest.createUser.subtitle': 'Create a test user with generated credentials to test your sign-in flow.',
    'quickTest.createUser.usernameLabel': 'Username',
    'quickTest.createUser.passwordLabel': 'Password',
    'quickTest.createUser.creating': 'Creating test user...',
    'quickTest.createUser.success': 'Test user created successfully.',
    'quickTest.createUser.error': 'Failed to create test user. You can still proceed.',
    'quickTest.createUser.regenerate': 'Regenerate',
    'quickTest.createUser.button': 'Create Test User',
    'quickTest.createUser.details.title': 'Created User',
    'quickTest.createUser.details.userId': 'User ID',
    'quickTest.createUser.details.type': 'Type',
    'quickTest.testLogin.title': 'Test Sign-In',
    'quickTest.testLogin.subtitle': 'Sign in with the test user credentials to verify your authentication setup.',
    'quickTest.testLogin.button': 'Test Sign In',
    'quickTest.testLogin.testing': 'Testing...',
    'quickTest.testLogin.error': 'Sign-in test failed.',
    'quickTest.testLogin.successTitle': 'Sign-in successful!',
    'quickTest.testLogin.tokenType': 'Type',
    'quickTest.testLogin.expiresIn': 'Expires in',
    'quickTest.testLogin.scope': 'Scope',
    'quickTest.testLogin.idTokenReceived': 'ID token received.',
    'quickTest.openSampleApp': 'Open Sample App',
    'quickTest.copied': 'Copied!',
    'export.title': 'Organization Export Summary',
    'export.subtitle': 'Review the items to be exported and verify the pre-flight checks before proceeding.',
    'export.table.item': 'Item',
    'export.table.status': 'Status',
    'export.table.dependencies': 'Dependencies',
    'export.table.applications': 'Applications',
    'export.table.flows': 'Flows',
    'export.table.branding': 'Branding',
    'export.table.dependencyCount': '{{count}} items',
    'export.table.noDependencies': 'None',
    'export.table.noItems': 'No items configured',
    'export.status.ready': 'Ready',
    'export.status.warning': 'Warning',
    'export.app.clientId': 'Client ID',
    'export.app.connections': 'Connections',
    'export.app.flows': 'Flows',
    'export.app.branding': 'Branding',
    'export.preflight.title': 'Pre-flight Check',
    'export.preflight.samlWarning': '"Admin Console" SAML Certificate is missing',
    'export.preflight.secretNote': '2 Client Secrets will be encrypted using the target environment key',
    'export.actions.saveAndExit': 'Save and Exit',
    'export.actions.exportConfig': 'Export Config',
    'setupComplete.title': 'All Done!',
    'setupComplete.subtitle': '{{appName}} is ready. Start integrating sign-in into your application.',
    'setupComplete.appNameLabel': 'Application Name',
    'setupComplete.clientIdLabel': 'Client ID',
    'setupComplete.copied': 'Copied!',
    'setupComplete.securityNote':
      'Your client secret was shown on the previous screen. Store it securely — it cannot be retrieved again.',
    'setupComplete.goToDashboard': 'Go to Dashboard',
    'setupComplete.viewApplication': 'View Application',
    'onboarding.summary.title': 'Application Created Successfully!',
    'onboarding.summary.subtitle': 'Your application is ready to use',
    'onboarding.summary.viewAppAriaLabel': 'Click to view application details',
    'onboarding.summary.appDetails': 'Application successfully created',
    'onboarding.summary.guides.subtitle': 'Choose how you want to integrate sign-in to your application',
    'onboarding.summary.guides.divider': 'or',
    'clientSecret.saveTitle': 'Save Your Client Secret',
    'clientSecret.saveSubtitle': "This is the only time you'll see this secret. Store it somewhere safe.",
    'clientSecret.appNameLabel': 'App Name',
    'clientSecret.warning':
      "Make sure to copy your client secret now. You won't be able to see it again for security reasons.",
    'clientSecret.clientIdLabel': 'Client ID',
    'clientSecret.clientSecretLabel': 'Client Secret',
    'clientSecret.purpose': 'Used to authenticate your application at the OAuth 2.0 token endpoint.',
    'clientSecret.copied': 'Copied to clipboard',
    'clientSecret.copySecret': 'Copy Secret',
    'clientSecret.securityReminder.title': 'Security Reminder',
    'clientSecret.securityReminder.description':
      'Your client secret is a confidential key used to authenticate your application. It should be treated with the same level of security as a password. Never expose it in browser console, version control, or logs.',
    'flowSecret.label': 'Flow Secret',
    'flowSecret.purpose':
      'Used to authenticate your server when it starts a sign-in flow directly via the Flow Execution API.',
    'view.title': 'Application Details',
    'view.subtitle': 'View application details and configuration',
    'view.sections.basicInformation': 'Basic Information',
    'view.sections.flowConfiguration': 'Flow Configuration',
    'view.sections.userAttributes': 'User Attributes',
    'view.sections.oauth2Configuration': 'OAuth2 Configuration',
    'view.sections.timestamps': 'Timestamps',
    'view.fields.applicationId': 'Application ID',
    'view.fields.description': 'Description',
    'view.fields.url': 'URL',
    'view.fields.tosUri': 'Terms of Service URI',
    'view.fields.policyUri': 'Privacy Policy URI',
    'view.fields.contacts': 'Contacts',
    'view.fields.authFlowId': 'Authentication Flow ID',
    'view.fields.registrationFlowId': 'Registration Flow ID',
    'view.fields.registrationFlowEnabled': 'Registration Flow Enabled',
    'view.fields.clientId': 'Client ID',
    'view.fields.redirectUris': 'Redirect URIs',
    'view.fields.grantTypes': 'Grant Types',
    'view.fields.responseTypes': 'Response Types',
    'view.fields.scopes': 'Scopes',
    'view.fields.publicClient': 'Public Client',
    'view.fields.pkceRequired': 'PKCE Required',
    'view.fields.createdAt': 'Created At',
    'view.fields.updatedAt': 'Updated At',
    'view.values.yes': 'Yes',
    'view.values.no': 'No',
    'edit.customization.tosUri.hint':
      "URL to your application's Terms of Service. May be displayed to users during consent or user sign-in, sign-up or recovery flows.",
    'edit.customization.policyUri.hint':
      "URL to your application's Privacy Policy. May be displayed to users during consent or user sign-in, sign-up or recovery flows.",
    'edit.advanced.oauth2Config.intro': 'Configure OAuth 2.0 settings for this {{entity}}.',
    'edit.advanced.redirectUris.hint':
      'Allowed callback URLs where users will be redirected after authentication. Must be exact matches for security.',
    'edit.advanced.grantTypes.placeholder': 'Select grant types',
    'edit.advanced.grantTypes.hint':
      'OAuth 2.0 flows this {{entity}} can use (e.g., authorization_code, client_credentials, refresh_token).',
    'edit.advanced.responseTypes.placeholder': 'Select response types',
    'edit.advanced.publicClient.public':
      'This is a public client (SPA, mobile app) that cannot securely store credentials.',
    'edit.advanced.publicClient.confidential':
      'This is a confidential client (server-side app) that can securely store credentials.',
    'edit.advanced.pkce.enabled': 'PKCE is required for authorization code flow, providing additional security.',
    'edit.advanced.pkce.disabled': 'PKCE is not required. Consider enabling for public clients (SPAs, mobile apps).',
    'edit.advanced.pkce.requiresAuthorizationCode': 'PKCE applies only to the authorization code flow.',
    'edit.advanced.pkce.requiredForPublicClient': 'Always required for public clients.',
    'edit.advanced.publicClient.requiresAuthorizationCode':
      'Available only for clients using the authorization code flow.',
    'edit.advanced.publicClient.incompatibleWithClientCredentials': 'Not available for machine-to-machine clients.',
    'edit.advanced.responseTypes.codeRequiredHint': 'Required for the authorization code flow.',
    'edit.advanced.responseTypes.notApplicable': 'Response types apply only to the authorization code flow.',
    'edit.advanced.tokenEndpointAuthMethod.placeholder': 'Select authentication method',
    'edit.advanced.tokenEndpointAuthMethod.hint':
      'Defines how the client authenticates at the token endpoint. Use client_secret_basic or client_secret_post for confidential clients, and none for public clients.',
    'edit.advanced.tokenEndpointAuthMethod.lockedHint': 'Locked to "none" because the client is public.',
    'edit.advanced.lockedByTemplate': 'Set by template',
    'edit.advanced.certificate.intro': 'Configure certificates for client authentication and token encryption.',
    'edit.advanced.certificate.type.none': 'None',
    'edit.advanced.certificate.type.jwks': 'JWKS (Inline JSON Web Key Set)',
    'edit.advanced.certificate.type.jwksUri': 'JWKS URI (URL to JWKS endpoint)',
    'edit.advanced.labels.attestation': 'Platform Attestation',
    'edit.advanced.attestation.intro':
      'Verify the binary identity of a mobile client when it initiates a flow directly. Choose the platform the application is built for.',
    'edit.advanced.attestation.labels.platform': 'Platform',
    'edit.advanced.attestation.platform.none': 'None',
    'edit.advanced.attestation.platform.android': 'Android (Play Integrity)',
    'edit.advanced.attestation.platform.apple': 'iOS (App Attest)',
    'edit.advanced.attestation.labels.packageName': 'Package Name',
    'edit.advanced.attestation.labels.certificateSha256Digests': 'Signing Certificate SHA-256 Digests',
    'edit.advanced.attestation.labels.serviceAccountCredentials': 'Service Account Credentials',
    'edit.advanced.attestation.placeholder.packageName': 'com.example.myapp',
    'edit.advanced.attestation.placeholder.certificateSha256Digest': 'URL-safe base64 SHA-256 digest',
    'edit.advanced.attestation.placeholder.serviceAccountCredentials': 'Paste the Google Cloud service account JSON',
    'edit.advanced.attestation.hint.packageName':
      'The Android application package name that must match the attested app.',
    'edit.advanced.attestation.hint.certificateSha256Digests':
      'Allowed signing certificate digests, in the URL-safe base64 form reported by Play Integrity. The attested app must match one of these.',
    'edit.advanced.attestation.hint.serviceAccountCredentials':
      'Write-only. Used to call the Play Integrity API. Leave blank to keep the existing credentials.',
    'edit.advanced.attestation.addDigest': 'Add Digest',
    'edit.advanced.attestation.labels.teamId': 'Team ID',
    'edit.advanced.attestation.labels.bundleId': 'Bundle ID',
    'edit.advanced.attestation.placeholder.teamId': 'ABCDE12345',
    'edit.advanced.attestation.placeholder.bundleId': 'com.example.myapp',
    'edit.advanced.attestation.hint.teamId': 'The Apple Developer Team ID.',
    'edit.advanced.attestation.hint.bundleId': 'The iOS bundle identifier that must match the attested app.',
    'edit.advanced.attestation.error.appleIncomplete': 'Both Team ID and Bundle ID are required together.',

    /* -------------------- Edit page -------------------- */
    // Common
    'edit.page.error': 'Failed to load application information',
    'edit.page.notFound': 'Application not found',
    'edit.page.back': 'Back to Applications',
    'edit.page.logoUpdate.label': 'Update Logo',
    'edit.page.description.empty': 'No description',
    'edit.page.description.placeholder': 'Add a description',
    'edit.page.tabs.overview': 'Guide',
    'edit.page.tabs.general': 'General',
    'edit.page.tabs.flows': 'Flows',
    'edit.page.tabs.customization': 'Customization',
    'edit.page.tabs.token': 'Token',
    'edit.page.tabs.advanced': 'Advanced Settings',
    'edit.page.unsavedChanges': 'Unsaved changes',
    'edit.page.reset': 'Reset',
    'edit.page.save': 'Save',
    'edit.page.saving': 'Saving...',

    'edit.mcp.connect.sections.identity': 'Connection',
    'edit.mcp.connect.sections.identity.description': 'Client identity and credentials for connecting to MCP servers.',
    'edit.mcp.connect.profileBadge.userDelegated': 'On behalf of a user (Authorization Code + PKCE)',
    'edit.mcp.connect.profileBadge.m2m': 'On its own behalf (Client Credentials)',
    'edit.mcp.connect.sections.endpoints': 'Endpoints',
    'edit.mcp.connect.sections.endpoints.description': 'ThunderID OAuth 2.1 endpoints for this client.',
    'edit.mcp.connect.generateSecret': 'Generate',
    'edit.mcp.connect.clientUri.label': 'Client URI',
    'edit.mcp.connect.clientUri.hint': 'Public homepage of this client (optional).',
    'edit.mcp.connect.clientUri.error.invalid': 'Please enter a valid URL',

    // Overview section
    'edit.overview.noGuides': 'No integration guides available for this application type.',

    // General section
    'edit.general.sections.quickCopy': 'Quick Copy',
    'edit.general.sections.quickCopy.description': 'Copy application identifiers for use in your code.',
    'edit.general.sections.access': 'Access',
    'edit.general.sections.access.description': "Configure who can access this application, where it's hosted, etc.",
    'edit.general.sections.contacts': 'Contacts',
    'edit.general.sections.contacts.description': 'Contact email addresses for {{entity}} administrators.',
    'edit.general.labels.applicationId': 'Application ID',
    'edit.general.labels.clientId': 'Client ID',
    'edit.general.labels.allowedUserTypes': 'Allowed User Types',
    'edit.general.labels.applicationUrl': 'Application URL',
    'edit.general.labels.contacts': 'Contacts',
    'edit.general.applicationId.hint': 'Unique identifier for your application',
    'edit.general.clientId.hint': 'OAuth2 client identifier used for authentication',
    'edit.general.noUserTypes': 'No user types configured',
    'edit.general.contacts.placeholder': 'Type an email and press Enter',
    'edit.general.contacts.hint': 'Type a valid email address and press <0>Enter</0> to add it',
    'edit.general.contacts.error.invalid': 'Please enter a valid email address',
    'edit.general.redirectUris.title': 'Authorized redirect URIs',
    'edit.general.redirectUris.description': 'For use with requests from a web server',
    'edit.general.redirectUris.tooltip': 'OAuth2 redirect URIs where users will be redirected after authentication',
    'edit.general.redirectUris.addUri': 'Add URI',
    'edit.general.redirectUris.error.empty': 'Invalid Redirect: URI must not be empty.',
    'edit.general.redirectUris.error.invalid': 'Invalid Redirect: Please enter a valid URL.',
    'edit.general.postLogoutRedirectUris.title': 'Post-Logout Redirect URIs',
    'edit.general.postLogoutRedirectUris.description':
      'Allowed URIs to redirect to after logout. A post_logout_redirect_uri passed to the logout endpoint must match one of these.',
    'edit.general.postLogoutRedirectUris.addUri': 'Add URI',
    'edit.general.postLogoutRedirectUris.error.invalid': 'Invalid Redirect: Please enter a valid URL.',
    'edit.general.allowedUserTypes.placeholder': 'Select user types',
    'edit.general.allowedUserTypes.hint': 'Users of these types can authenticate with this application',
    'edit.general.applicationUrl.hint': 'The homepage URL of your application',
    'edit.general.sections.dangerZone.title': 'Danger Zone',
    'edit.general.sections.dangerZone.description': 'Actions in this section are irreversible. Proceed with caution.',
    'edit.general.sections.dangerZone.regenerateSecret.title': 'Regenerate Client Secret',
    'edit.general.sections.dangerZone.regenerateSecret.description':
      'Regenerating the client secret will immediately invalidate the current client secret and cannot be undone.',
    'edit.general.sections.dangerZone.regenerateSecret.button': 'Regenerate Client Secret',
    'edit.general.sections.dangerZone.regenerateFlowSecret.title': 'Regenerate Flow Secret',
    'edit.general.sections.dangerZone.regenerateFlowSecret.description':
      'Regenerating the Flow Secret immediately invalidates the current one. Server-side flow initiation will fail until the new secret is deployed.',
    'edit.general.sections.dangerZone.regenerateFlowSecret.button': 'Regenerate Flow Secret',
    'edit.general.sections.dangerZone.deleteApplication.title': 'Delete Application',
    'edit.general.sections.dangerZone.deleteApplication.description':
      'Permanently delete this application and all associated data. This action cannot be undone.',
    'edit.general.sections.dangerZone.deleteApplication.button': 'Delete Application',

    // Flows section
    'edit.flows.labels.authFlow': 'Authentication Flow',
    'edit.flows.labels.authFlow.description': 'Choose the flow that handles user login and authentication.',
    'edit.flows.labels.registrationFlow': 'Registration Flow',
    'edit.flows.labels.registrationFlow.description': 'Choose the flow that handles user sign-up and account creation.',
    'edit.flows.authFlow.placeholder': 'Select an authentication flow',
    'edit.flows.authFlow.hint': 'Select the flow that handles user sign-in for this {{entity}}.',
    'edit.flows.authFlow.alert':
      'To modify the selected flow, <0>open the flow builder</0>. To create a new flow, visit the <1>Flows page</1>.',
    'edit.flows.registrationFlow.placeholder': 'Select a registration flow',
    'edit.flows.registrationFlow.hint': 'Select the flow that handles user registration for this {{entity}}.',
    'edit.flows.registrationFlow.alert':
      'To modify the selected flow, <0>open the flow builder</0>. To create a new flow, visit the <1>Flows page</1>.',
    'edit.flows.labels.recoveryFlow': 'Recovery Flow',
    'edit.flows.labels.recoveryFlow.description': 'Choose the flow that handles password and account recovery.',
    'edit.flows.recoveryFlow.placeholder': 'Select a recovery flow',
    'edit.flows.recoveryFlow.hint': 'Select the flow that handles account recovery for this {{entity}}.',
    'edit.flows.recoveryFlow.alert':
      'To modify the selected flow, <0>open the flow builder</0>. To create a new flow, visit the <1>Flows page</1>.',
    'edit.flows.labels.signOutFlow': 'Sign Out Flow',
    'edit.flows.labels.signOutFlow.description': 'Confirm and terminate the SSO session when people sign out.',
    'edit.flows.signOutFlow.placeholder': 'Select a sign-out flow',
    'edit.flows.signOutFlow.hint': 'Select the flow that runs when a user signs out of this {{entity}}.',
    'edit.flows.signOutFlow.alert':
      'To modify the selected flow, <0>open the flow builder</0>. To create a new flow, visit the <1>Flows page</1>.',
    'edit.flows.editFlow': 'Edit flow',

    // Customization section
    'edit.customization.sections.appearance': 'Appearance',
    'edit.customization.sections.appearance.description': 'Customize the visual appearance of your {{entity}}.',
    'edit.customization.sections.urls': 'URLs',
    'edit.customization.sections.urls.description': 'Configure legal and policy URLs for your {{entity}}.',
    'edit.customization.labels.theme': 'Theme',
    'edit.customization.labels.tosUri': 'Terms of Service URI',
    'edit.customization.labels.policyUri': 'Privacy Policy URI',
    'edit.customization.theme.placeholder': 'Select a Theme',
    'edit.customization.theme.hint':
      'Choose a theme to customize authentication pages. Select the Default Theme (shared across all applications) or pick an app-specific theme.',
    'edit.customization.tosUri.placeholder': 'https://example.com/terms',
    'edit.customization.policyUri.placeholder': 'https://example.com/privacy',

    // Token section
    'edit.token.seconds': 'seconds',
    'edit.token.labels.token_validity': 'Token Validity',
    'edit.token.loading_attributes': 'Loading user attributes...',
    'edit.token.no_user_attributes': 'No user attributes available. Configure allowed user types for this {{entity}}.',
    'edit.token.click_to_add': 'Click to add',
    'edit.token.click_to_remove': 'Click to remove',
    'edit.token.configure_attributes': 'Add or Remove Attributes',
    'edit.token.configure_attributes.hint': 'Click on user attributes to add them to your token.',
    'edit.token.token_preview.title': 'Decoded Payload',
    'edit.token.validity.hint': 'Token validity period in seconds (e.g., 3600 for 1 hour)',
    'edit.token.validity.error': 'Validity period must be at least 1 second',
    'edit.token.token_profile_card.title': 'Token Attributes & Response',
    'edit.token.token_profile_card.description':
      'Configure the response types and user attributes included in your tokens and user info responses',
    'edit.token.tabs.access_token': 'Access Token',
    'edit.token.tabs.id_token': 'ID Token',
    'edit.token.tabs.refresh_token': 'Refresh Token',
    'edit.token.tabs.user_info_endpoint': 'User Info Endpoint',
    'edit.token.token_validation.title': 'Token Validity',
    'edit.token.token_validation.description': 'Configure how long tokens remain valid before expiration',
    'edit.token.inherit_from_id_token': 'Use same attributes as ID Token',
    'edit.token.user_info.inherit_info': 'User Info response will use the same attributes as the ID Token',
    'edit.token.scopes_card.title': 'Scopes & User Attribute Mappings',
    'edit.token.scopes_card.description': 'Configure the OAuth2 scopes and the user attributes exposed for each scope',
    'edit.token.scopes.title': 'Available Scopes',
    'edit.token.scopes.hint': 'Toggle the OAuth2 scopes available to this {{entity}}.',
    'edit.token.scopes.active_label': 'Active',
    'edit.token.scopes.suggested_label': 'Suggested',
    'edit.token.scopes.custom_label': 'Custom',
    'edit.token.scopes.add_custom.placeholder': 'e.g. custom:read',
    'edit.token.scopes.add_custom.button': 'Add',
    'edit.token.scopes.add_custom.error.empty': 'Scope name cannot be empty',
    'edit.token.scopes.add_custom.error.duplicate': 'This scope is already added',
    'edit.token.scopes.add_custom.error.invalid': 'Scope name must not contain spaces',
    'edit.token.scopes.openid_required': 'The openid scope is required and cannot be removed',
    'edit.advanced.audience.title': 'Default Audience',
    'edit.advanced.audience.description':
      "The default aud for access tokens that don't target a resource server (OIDC only or scopeless).",
    'edit.advanced.audience.label': 'Default audience (aud)',
    'edit.advanced.audience.placeholder': 'e.g. https://api.example.com',
    'edit.advanced.audience.hint': 'Leave empty to use the {{entity}} client ID.',
    'edit.token.scope_mapper.title': 'User Attribute Mapping',
    'edit.token.scope_mapper.hint':
      'Select a scope to configure which user attributes are exposed when it is requested.',
    'edit.token.scope_mapper.no_scopes': 'Add at least one scope above to start mapping attributes.',
    'edit.token.scope_mapper.mapped_label': 'Mapped Attributes',
    'edit.token.scope_mapper.available_label': 'Available Attributes',
    'edit.token.scope_mapper.no_mapped': 'No attributes mapped yet — click an attribute below to add it',
    'edit.token.scope_mapper.all_mapped': 'All available attributes are already mapped to this scope',
    'edit.token.scope_mapper.loading': 'Loading available attributes...',
    'edit.token.id_token.response_type_placeholder': 'Select response type',
    'edit.token.id_token.encryption_alg_placeholder': 'Select encryption algorithm',
    'edit.token.id_token.encryption_enc_placeholder': 'Select content encryption',
    'edit.token.user_info.response_type_placeholder': 'Select response type',
    'edit.token.user_info.signing_alg_placeholder': 'Select signing algorithm',
    'edit.token.user_info.encryption_alg_placeholder': 'Select encryption algorithm',
    'edit.token.user_info.encryption_enc_placeholder': 'Select content encryption',

    // Advanced section
    'edit.advanced.labels.oauth2Config': 'OAuth2 Configuration',
    'edit.advanced.labels.redirectUris': 'Redirect URIs',
    'edit.advanced.labels.grantTypes': 'Grant Types',
    'edit.advanced.labels.responseTypes': 'Response Types',
    'edit.advanced.labels.publicClient': 'Public Client',
    'edit.advanced.labels.pkceRequired': 'PKCE Required',
    'edit.advanced.labels.requirePAR': 'Require Pushed Authorization Requests',
    'edit.advanced.par.hint': 'Require the client to use the PAR endpoint before authorization.',
    'edit.advanced.labels.tokenEndpointAuthMethod': 'Token Endpoint Auth Method',
    'edit.advanced.labels.certificate': 'Certificate',
    'edit.advanced.labels.certificateType': 'Certificate Type',
    'edit.advanced.labels.metadata': 'Metadata',
    'edit.advanced.labels.createdAt': 'Created At',
    'edit.advanced.labels.updatedAt': 'Updated At',
    'edit.advanced.publicClient.yes': 'Yes',
    'edit.advanced.publicClient.no': 'No',
    'edit.advanced.pkce.yes': 'Yes',
    'edit.advanced.pkce.no': 'No',
    'edit.advanced.certificate.placeholder.jwksUri': 'https://example.com/.well-known/jwks',
    'edit.advanced.certificate.placeholder.jwks': 'Enter JWKS JSON',
    'edit.advanced.certificate.hint.jwksUri': 'URL to the JWKS endpoint',
    'edit.advanced.certificate.hint.jwks': 'JSON Web Key Set',
    'edit.advanced.labels.acrValues': 'ACR Values',
    'edit.advanced.acrValues.intro': 'Authentication context classes permitted for this application.',
    'edit.advanced.acrValues.placeholder': 'Select ACR values',
    'edit.advanced.acrValues.hint':
      'When acr_values is included in the authorization request, only values configured here are accepted.',
    'edit.advanced.grantTypes.labels.ciba': 'CIBA (Client-Initiated Backchannel Authentication)',
    'edit.advanced.grantTypes.labels.tokenExchange': 'Token Exchange',
    'edit.advanced.grantTypes.labels.jwtBearer': 'JWT Bearer',
    'edit.advanced.idJag.title': 'Identity Assertions (ID-JAG)',
    'edit.advanced.idJag.description':
      "Issue signed assertions of the signed-in user's identity that external services accept for token issuance.",
    'edit.advanced.idJag.publicClientGuard':
      'Identity assertions require a confidential client. Turn off Public Client to enable.',
    'edit.advanced.idJag.labels.allowedAudiences': 'Allowed audiences',
    'edit.advanced.idJag.allowedAudiences.placeholder': 'Type an audience and press Enter',
    'edit.advanced.idJag.allowedAudiences.error': 'Add at least one audience.',
    'edit.advanced.idJag.allowedAudiences.hint': 'Each assertion targets exactly one of these audiences.',
    'edit.advanced.idJag.labels.validityPeriod': 'Assertion validity',
    'edit.advanced.idJag.labels.seconds': 'seconds',
    'edit.advanced.idJag.validityPeriod.error': 'Enter a value of at least 1 second.',
    'edit.advanced.idJag.validityPeriod.hint': 'How long an issued assertion stays valid. Default 300.',
    'edit.advanced.idJag.grantTypeHint': 'The token exchange grant type is enabled together with this feature.',
    'create.success': 'Application created successfully.',
    'create.error': 'Failed to create application. Please try again.',
    'update.success': 'Application updated successfully.',
    'update.error': 'Failed to update application. Please try again.',
    'delete.success': 'Application deleted successfully.',
    'delete.error': 'Failed to delete application. Please try again.',
    'regenerateSecret.snackbar.success': 'Client secret regenerated successfully.',
    'errors.APP-1001': 'The requested application could not be found.',
    'errors.APP-1002': 'The provided application ID is invalid or empty.',
    'errors.APP-1003': 'The provided client ID is invalid or empty.',
    'errors.APP-1004': 'The provided application name is invalid or empty.',
    'errors.APP-1005': 'The provided application URL is not a valid URI.',
    'errors.APP-1006': 'The provided logo URL is not a valid URI.',
    'errors.APP-1007': 'The provided authentication flow ID is invalid.',
    'errors.APP-1008': 'The provided registration flow ID is invalid.',
    'errors.APP-1009': 'The provided inbound authentication configuration is invalid.',
    'errors.APP-1010': 'One or more provided grant types are invalid.',
    'errors.APP-1011': 'One or more provided response types are invalid.',
    'errors.APP-1012': 'One or more provided redirect URIs are not valid URIs.',
    'errors.APP-1013': 'The provided token endpoint authentication method is invalid.',
    'errors.APP-1014': 'The provided certificate type is not supported.',
    'errors.APP-1015': 'The provided certificate value is invalid.',
    'errors.APP-1016': 'The provided JWKS URI is not a valid URI.',
    'errors.APP-1017': 'The provided application object is nil.',
    'errors.APP-1018': 'The request body is malformed or contains invalid data.',
    'errors.APP-1019': 'An error occurred while processing the application certificate.',
    'errors.APP-1020': 'An application with the same name already exists.',
    'errors.APP-1021': 'An application with the same client ID already exists.',
    'errors.APP-1022': "'jwks_uri' must use HTTPS scheme.",
    'errors.APP-1023': 'The public client configuration is invalid.',
    'errors.APP-1024': 'The OAuth configuration is invalid.',
    'errors.APP-1025': 'One or more user types in allowed_user_types do not exist in the system.',
    'errors.APP-1026': 'The specified theme configuration does not exist.',
    'errors.APP-1027': 'The specified layout configuration does not exist.',
    'errors.APP-1028': 'An error occurred while retrieving the flow definition.',
    'errors.APP-1029': 'The result limit has been exceeded.',
    'errors.APP-1030': 'The application is declarative and cannot be modified or deleted.',
    'errors.APP-1031': 'Failed to synchronize consent configurations for the application.',
    'errors.APP-1032': 'Cannot enable consent for the application as the consent service is not enabled.',
    'errors.APP-1033': 'One or more ACR values in acr_values are not recognized by the system.',
    'errors.APP-1034': 'An application may have at most one inbound auth config per protocol.',
    'errors.APP-1035': 'One or more user attributes are not valid for the configured allowed user types.',
    'errors.APP-1036': 'The provided recovery flow ID is invalid.',
    'errors.APP-1037': 'Native flow execution is not allowed for single-page applications as it requires PKCE.',
    'errors.APP-1038': 'Attestation configuration may configure only one platform (android or apple) at a time.',
    'errors.APP-1039':
      'A referenced flow conflicts with the flow configured on the application. Both must point to the same flow.',
    'errors.APP-1040': 'The provided Terms of Service URI is not a valid URI.',
    'errors.APP-1041': 'The provided Privacy Policy URI is not a valid URI.',
    'errors.APP-5001': 'An unexpected error occurred while processing the request.',
    'errors.APP-5002': 'An error occurred while performing the certificate operation.',
  },

  // ============================================================================
  // Import / Export - Project import-export feature translations
  // ============================================================================
  importExport: {
    'landing.title': 'Import / Export',
    'landing.subtitle': 'Choose whether to import a configuration file or export your current one.',
    'landing.type.import.label': 'Import',
    'landing.type.import.description': 'Bring in an existing ThunderID configuration file.',
    'landing.type.export.label': 'Export',
    'landing.type.export.description': 'Download your current configuration as a file.',

    'export.page.title': 'Export Configuration',
    'export.page.loading': 'Loading export configuration...',
    'export.page.loadError': 'Failed to load export configuration: {{message}}',

    'upload.breadcrumb.openProject': 'Import Configuration',
    'upload.title': 'Import Configuration',
    'upload.subtitle': 'Upload your {{configFileName}} configuration file to import',
    'upload.tabs.uploadFile': 'Upload File',
    'upload.tabs.fromUrl': 'From URL',
    'upload.actions.changeFile': 'Change File',
    'upload.dropConfig': 'Drop your configuration file here',
    'upload.orClickBrowse': 'or click to browse',
    'upload.supportsYaml': 'Supports YAML files only ({{configFileName}})',
    'upload.url.label': 'Configuration URL',
    'upload.url.placeholder': 'https://example.com/{{configFileName}}',
    'upload.url.helperText': 'Enter a URL pointing to your {{configFileName}} configuration file',
    'upload.env.title': 'Environment Variables',
    'upload.env.subtitle': 'Upload your {{envFileName}} file to import environment-specific configuration',
    'upload.env.dropFile': 'Drop your {{envFileName}} file here',
    'upload.errors.uploadYaml': 'Please upload a YAML file ({{configFileName}})',
    'upload.errors.uploadEnv': 'Please upload an .env file',
    'upload.errors.selectFile': 'Please select a file to upload',
    'upload.errors.selectEnvFile': 'Please select an environment variables file to upload',
    'upload.errors.invalidUrl': 'Please enter a valid URL',
    'upload.errors.unknownResourceType':
      'Unknown resource type: "{{resourceType}}". Allowed types are: {{allowedTypes}}',
    'upload.errors.parseFailed': 'Failed to parse configuration file: {{message}}',

    'validate.steps.readingFile': 'Reading configuration file',
    'validate.steps.validatingYaml': 'Validating YAML syntax',
    'validate.steps.checkingCompatibility': 'Checking compatibility',
    'validate.steps.validatingResources': 'Validating resources',
    'validate.title': 'Validating Configuration',
    'validate.subtitle': 'Please wait while we validate your configuration',
    'validate.progress': 'Progress',
    'validate.parseErrors.invalidSections': 'Configuration file contains {{count}} invalid section(s)',
    'validate.parseErrors.summary': 'Successfully parsed {{successCount}} sections, {{failCount}} sections failed',
    'validate.parseErrors.title': 'Parse Errors:',
    'validate.parseErrors.unknownFile': 'Unknown File',
    'validate.actions.uploadDifferentFile': 'Upload Different File',

    'envViewer.title': 'Environment Variables',
    'envViewer.variableCount': '{{count}} variables detected',
    'envViewer.modified': '• Modified',
    'envViewer.download': 'Download .env',
    'envViewer.placeholderWarning':
      'Some environment variables contain placeholder values. Edit them below before importing.',

    'fileViewer.download': 'Download {{fileName}}',

    'table.item': 'Item',
    'table.resourceType': 'Resource Type',
    'table.status': 'Status',
    'table.dependencies': 'Dependencies',
    'table.count': 'Count',
    'table.noResources': 'No resources found',
    'table.noDetails': 'No details available',
    'export.table.applications': 'Applications',
    'export.table.flows': 'Flows',
    'export.app.clientId': 'Client ID',

    'templateVariable.valueMissing': 'value is missing',

    'configureExport.nextSteps.startWithConfig': '{{productName}} will start with your exported configuration',
    'configureExport.nextSteps.resourcesAvailable': 'All applications, flows, and connections will be available',
    'configureExport.nextSteps.testFlows': 'You can test your authentication flows immediately',
    'configureExport.actions.showLess': 'Show less',
    'configureExport.actions.more': '+ {{count}} more',
    'configureExport.labels.themes': 'Themes',
    'configureExport.labels.users': 'Users',
    'configureExport.labels.organizationUnits': 'Organization Units',
    'configureExport.labels.connections': 'Connections',
    'configureExport.labels.userTypes': 'User Types',
    'configureExport.labels.translations': 'Translations',
    'configureExport.labels.layouts': 'Layouts',
    'configureExport.labels.selfRegistration': 'Self Registration',
    'configureExport.labels.resourceServers': 'Resource Servers',
    'configureExport.labels.roles': 'Roles',
    'configureExport.labels.groups': 'Groups',
    'configureExport.fallback.unnamedApplication': 'Unnamed Application',
    'configureExport.fallback.unnamedProvider': 'Unnamed Provider',
    'configureExport.fallback.unnamedFlow': 'Unnamed Flow',
    'configureExport.fallback.unnamedTheme': 'Unnamed Theme',
    'configureExport.fallback.unnamedOrganization': 'Unnamed Organization',
    'configureExport.fallback.unnamedSchema': 'Unnamed Schema',
    'configureExport.fallback.unnamedTranslation': 'Unnamed Translation',
    'configureExport.fallback.unnamedLayout': 'Unnamed Layout',
    'configureExport.fallback.unnamedResourceServer': 'Unnamed Resource Server',
    'configureExport.fallback.unnamedRole': 'Unnamed Role',
    'configureExport.fallback.unnamedGroup': 'Unnamed Group',
    'configureExport.fallback.unnamedPresentationDefinition': 'Unnamed Presentation Definition',
    'configureExport.fallback.unnamedCredentialConfiguration': 'Unnamed Credential Configuration',
    'configureExport.labels.agents': 'Agents',
    'configureExport.fallback.unnamedAgent': 'Unnamed Agent',
    'configureExport.labels.serverConfigs': 'Server Configurations',
    'configureExport.fallback.unnamedServerConfig': 'Unnamed Server Configuration',
    'configureExport.fallback.unnamedUser': 'User {{index}}',
    'configureExport.labels.projectDetails': 'Project Details',
    'configureExport.labels.totalResources': 'Total Resources',
    'configureExport.labels.clientId': 'Client ID',
    'configureExport.labels.configurationResources': 'Configuration Resources',
    'configureExport.labels.downloadConfig': 'Download your {{fileName}} configuration file',
    'configureExport.actions.exportConfiguration': 'Export Configuration',
    'configureExport.runProduct.title': 'Run {{productName}}',
    'configureExport.runProduct.subtitle':
      'Use the following command to start {{productName}} with your configuration:',
    'configureExport.runProduct.nextStepsTitle': 'What happens next:',

    'summary.breadcrumb': 'Summary',
    'summary.title': 'Configuration Summary',
    'summary.valid': 'Valid',
    'summary.subtitle': 'Review the imported configuration before proceeding',
    'summary.projectDetails': 'Project Details',
    'summary.totalResources': 'Total Resources',
    'summary.preImportValidation': 'Pre-Import Validation',
    'summary.labels.presentationDefinitions': 'Presentation Definitions',
    'summary.labels.credentialConfigurations': 'Credential Configurations',
    'summary.actions.reuploadEnv': 'Re-upload .env file',
    'summary.env.editInfo':
      'You can edit the environment variables below to fix missing or placeholder values. The validation will update automatically.',
    'summary.fallback.flow': 'Flow {{index}}',
    'summary.fallback.theme': 'Theme {{index}}',
    'summary.fallback.user': 'User {{index}}',
    'summary.fallback.schema': 'Schema {{index}}',
    'summary.precheck.readyNoEnvRequired': 'Ready to proceed. No environment values are required for this import.',
    'summary.precheck.readyAllEnvAvailable':
      'Ready to proceed. All {{count}} referenced environment values are available.',
    'summary.precheck.missingEnvValues': '{{count}} environment value(s) are missing. Add them before importing.',
    'summary.precheck.availableEnvValues':
      '{{resolved}} of {{total}} referenced environment values are already available.',
    'summary.precheck.missingVariables': 'Missing variables',
    'summary.importTest.status': 'Import Test Status',
    'summary.importTest.runToValidate': 'Run import test to validate behavior.',
    'summary.importTest.configUnavailable': 'Configuration content is unavailable. Re-upload the configuration file.',
    'summary.importTest.fixMissingThenRun': 'Fix missing environment values, then run test.',
    'summary.importTest.running': 'Running pre-flight dry-run...',
    'summary.importTest.runningShort': 'Running...',
    'summary.importTest.test': 'Test',
    'summary.importTest.retry': 'Retry Import Test',
    'summary.importTest.passed': 'Import test passed. {{imported}} of {{totalDocuments}} resources validated.',
    'summary.importTest.failedCount': 'Import test failed for {{count}} resource',
    'summary.importTest.failedCount_plural': 'Import test failed for {{count}} resources',
    'summary.importTest.failedWithMessage': 'Import test failed: {{message}}',
    'summary.importTest.failures': 'Import Test failures',
    'summary.importTest.failed': 'failed',
    'summary.import.tooltip.missingVariables':
      'Cannot import: {{count}} environment variable is missing. Edit the environment variables above to fix.',
    'summary.import.tooltip.missingVariables_plural':
      'Cannot import: {{count}} environment variables are missing. Edit the environment variables above to fix.',
    'summary.import.tooltip.configUnavailable':
      'Cannot import: configuration content is unavailable. Re-upload the configuration file.',
    'summary.import.tooltip.runTestFirst': 'Cannot import: run pre-flight dry-run and ensure it passes.',
    'summary.import.action': 'Import Configuration',
    'summary.import.importing': 'Importing...',
    'summary.import.completedWithFailures': 'Import completed with {{count}} failed resource.',
    'summary.import.completedWithFailures_plural': 'Import completed with {{count}} failed resources.',
    'summary.import.completedSuccessfully': 'Import completed successfully. {{count}} resource imported.',
    'summary.import.completedSuccessfully_plural': 'Import completed successfully. {{count}} resources imported.',
    'summary.import.failedRetry': 'Import failed. Please try again.',
  },

  // ============================================================================
  // Sign In - Sign In page translations
  // ============================================================================
  signin: {
    'errors.signin.failed.message': 'Error',
    'errors.signin.failed.description': 'We are sorry, something has gone wrong here. Please try again.',
    'errors.signin.timeout': 'Time allowed to complete the step has expired.',
    'errors.signin.session.expired': 'Your session has expired. Please return to the application and sign in again.',
    'errors.passkey.failed': 'Passkey authentication failed. Please try again.',
    'redirect.to.signup': "Don't have an account? <1>Sign up</1>",
    heading: 'Sign In',
    // Passkey authentication
    'passkey.button.use': 'Sign in with Passkey',
    'passkey.signin.heading': 'Sign in with Passkey',
    'passkey.signin.description': 'Use your passkey to securely sign in to your account without a password.',
    'passkey.register.heading': 'Register Passkey',
    'passkey.register.description': 'Create a passkey to securely sign in to your account without a password.',
  },

  // ============================================================================
  // Sign Up - Sign Up page translations
  // ============================================================================
  signup: {
    'create_account.loading': 'Creating account...',
    'errors.signup.failed.message': 'Error',
    'errors.signup.failed.description': 'We are sorry, but we were unable to create your account. Please try again.',
    'redirect.to.signin': 'Already have an account? <1>Sign in</1>',
    heading: 'Sign Up',
    // Passkey registration translations
    'passkey.setup.heading': 'Set Up Passkey',
    'passkey.setup.description': 'Create a passkey to securely sign in to your account without a password.',
    'passkey.button.create': 'Create Passkey',
    'passkey.registering': 'Creating passkey...',
    'errors.passkey.failed': 'Failed to create passkey. Please try again.',
  },

  // ============================================================================
  // Components namespace - SDK component error translations
  // ============================================================================
  components: {
    'signIn.errors.generic': 'An error occurred during sign in. Please try again.',
    'signUp.errors.generic': 'An error occurred during sign up. Please try again.',
    'inviteUser.errors.generic': 'An error occurred while inviting the user. Please try again.',
    'acceptInvite.errors.generic': 'An error occurred while accepting the invite. Please try again.',
  },

  // ============================================================================
  // Elements - Low level reusable element translations
  // ============================================================================
  elements: {
    'buttons.github.text': 'Continue with GitHub',
    'buttons.google.text': 'Continue with Google',
    'buttons.submit.text': 'Continue',
    'display.divider.or_separator': 'OR',
    'fields.first_name.label': 'First Name',
    'fields.first_name.placeholder': 'Enter your first name',
    'fields.last_name.label': 'Last Name',
    'fields.last_name.placeholder': 'Enter your last name',
    'fields.email.label': 'Email',
    'fields.email.placeholder': 'Enter your email address',
    'fields.mobile.label': 'Mobile Number',
    'fields.mobile.placeholder': 'Enter your mobile number',
    'fields.password.label': 'Password',
    'fields.password.placeholder': 'Enter your password',
    'fields.username.label': 'Username',
    'fields.username.placeholder': 'Enter your username',
    'fields.usertype.label': 'User Type',
    'fields.usertype.placeholder': 'Select User Type',

    // Emoji picker
    'emoji_picker.search.placeholder': 'Search emojis...',
    'emoji_picker.search.label': 'Search emojis',
    'emoji_picker.empty_state.message': 'No emojis found for "{{search}}"',
    'emoji_picker.categories.smileys_emotion': 'Smileys & Emotion',
    'emoji_picker.categories.people_body': 'People & Body',
    'emoji_picker.categories.animals_nature': 'Animals & Nature',
    'emoji_picker.categories.food_drink': 'Food & Drink',
    'emoji_picker.categories.travel_places': 'Travel & Places',
    'emoji_picker.categories.activities': 'Activities',
    'emoji_picker.categories.objects': 'Objects',
    'emoji_picker.categories.symbols': 'Symbols',
    'emoji_picker.categories.flags': 'Flags',

    // Avatar picker
    'avatar_picker.variants.anonymous_animal': 'Anonymous animal',
    'avatar_picker.variants.anonymous_entity': 'Entity',
    'avatar_picker.background.label': 'Background color',
    'avatar_picker.background.reset': 'Auto',
    'avatar_picker.seed_text.label': 'Seed text',
    'avatar_picker.seed_text.placeholder': 'e.g. your app name',
    'avatar_picker.shuffle': 'Shuffle colors',
    'avatar_swatch_grid.shuffle': 'Shuffle',
    'avatar_swatch_grid.swatch': 'Background option',

    // Icon grid picker
    'icon_grid_picker.shuffle': 'Shuffle',

    // Logo picker
    'logo_picker.url.placeholder': 'Paste an image URL, e.g. https://example.com/logo.png',
    'logo_picker.url.helper_text':
      'Direct link to a PNG, SVG or JPG. For best results, use a square image less than 1MB in size.',
    'logo_picker.divider': 'OR PICK ONE',
    'logo_picker.groups.emoji': 'Emoji',
    'logo_picker.flyouts.emoji': 'Choose an emoji',
    'logo_picker.groups.more_emojis': 'More emojis',
    'logo_picker.content_type.avatar': 'Avatar',
    'logo_picker.content_type.text_avatar': 'Text Avatar',
    'logo_picker.shapes.rounded': 'Rounded',
    'logo_picker.shapes.circle': 'Circle',
    'logo_picker.flyouts.rounded_blank': 'Pick a background',
    'logo_picker.flyouts.rounded_text': 'Rounded avatar',
    'logo_picker.flyouts.circle_blank': 'Pick a background',
    'logo_picker.flyouts.circle_text': 'Circular avatar',
    'logo_picker.groups.animal': 'Animal',
    'logo_picker.flyouts.animal': 'Choose an animal',
    'logo_picker.groups.entity': 'Entity',
    'logo_picker.flyouts.entity': 'Choose an icon',
    'logo_picker.emoji_dialog.title': 'Choose an emoji',
    'logo_picker.shuffle': 'Shuffle',

    // Resource logo dialog
    'resource_logo_dialog.title': 'Choose a Logo',
    'resource_logo_dialog.divider.or': 'Or',
    'resource_logo_dialog.tabs.label': 'Logo source',
    'resource_logo_dialog.tabs.emoji': 'Emoji',
    'resource_logo_dialog.tabs.avatar': 'Avatar',
    'resource_logo_dialog.url_section.label': 'Use a custom image URL',
    'resource_logo_dialog.url_section.placeholder': 'https://example.com/logo.png',
    'resource_logo_dialog.url_section.helper_text': 'Enter a direct URL to a custom logo image',
    'resource_logo_dialog.actions.cancel': 'Cancel',
    'resource_logo_dialog.actions.close': 'Close',
    'resource_logo_dialog.actions.select': 'Select',
    'resource_logo_dialog.actions.save': 'Save',
  },

  // ============================================================================
  // Validations - Form & other validation messages translations
  // ============================================================================
  validations: {
    'form.field.required': '{{field}} is required.',
  },

  // ============================================================================
  // Flows - Flow builder feature translations
  // ============================================================================
  flows: {
    // Flow listing page
    'listing.title': 'Flows',
    'listing.subtitle': 'Create and manage authentication and registration flows for your applications',
    'listing.addFlow': 'Create New Flow',
    'listing.columns.name': 'Name',
    'listing.columns.flowType': 'Type',
    'listing.columns.updatedAt': 'Last Updated',
    'listing.columns.actions': 'Actions',
    'listing.error.title': 'Failed to load flows',
    'listing.error.unknown': 'An unknown error occurred',
    'delete.title': 'Delete Flow',
    'delete.message': 'Are you sure you want to delete this flow? This action cannot be undone.',
    'delete.disclaimer': 'Warning: All associated configurations will be permanently removed.',
    'delete.error': 'Failed to delete flow. Please try again.',
    'delete.usages.loading': 'Checking affected resources…',
    'delete.usages.none': 'No applications or agents are currently using this flow.',
    'delete.usages.title': 'The following resources will revert to the default flow:',
    'delete.usages.more': '+{{count}} more',

    // Create flow wizard
    'create.steps.type': 'Flow Type',
    'create.steps.template': 'Template',
    'create.steps.configure': 'Configure',
    'create.configure.title': 'Name your flow',
    'create.configure.name.label': 'Flow name',
    'create.configure.name.placeholder': 'e.g. Customer Sign-in',
    'create.configure.suggestions.label': 'Need inspiration? Try one of these:',
    'create.configure.handle.label': 'Handle',
    'create.configure.handle.placeholder': 'e.g. customer-sign-in',
    'create.configure.handle.hint': 'Lowercase letters, numbers, and hyphens only',
    'create.type.title': 'What kind of flow do you want to create?',
    'create.type.signin.label': 'Sign-in',
    'create.type.signin.description': 'Authenticate users with passwords, passkeys, or social providers',
    'create.type.signup.label': 'Self Sign-up',
    'create.type.signup.description': 'Let users register themselves with your application',
    'create.type.recovery.label': 'Password Recovery',
    'create.type.recovery.description': 'Let users recover their password or account',
    'create.type.onboarding.label': 'Onboarding',
    'create.type.onboarding.description': 'Onboard invited users to your organization',
    'create.template.title': 'Choose a starting template',
    'create.template.recommended': 'Recommended',
    'create.template.search': 'Search templates...',
    'create.template.noResults': 'No templates match your search.',
    'create.error.createFailed': 'Failed to create flow. Please try again.',

    // Flow labels and navigation
    label: 'Flows',
    'core.breadcrumb': '{{flowType}}',
    'core.autoSave.savingInProgress': 'Saving...',
    'core.labels.enableFlow': 'Enable Flow',
    'core.labels.disableFlow': 'Disable Flow',
    'core.tooltips.enableFlow': 'Enable this {{flowType}}',
    'core.tooltips.disableFlow': 'Disable this {{flowType}}',

    // Notification panel
    'core.notificationPanel.header': 'Notifications',
    'core.notificationPanel.trigger.label': 'View notifications',
    'core.notificationPanel.tabs.errors': 'Errors',
    'core.notificationPanel.tabs.warnings': 'Warnings',
    'core.notificationPanel.tabs.info': 'Info',
    'core.notificationPanel.emptyMessages.errors': 'No errors found',
    'core.notificationPanel.emptyMessages.warnings': 'No warnings found',
    'core.notificationPanel.emptyMessages.info': 'No information messages',

    // Execution steps - names
    'core.executions.names.google': 'Google',
    'core.executions.names.github': 'GitHub',
    'core.executions.names.oauth': 'OAuth',
    'core.executions.names.oidc': 'OIDC Auth',
    'core.executions.names.PasskeyAuthentication': 'Passkey Authentication',
    'core.executions.names.magicLink': 'Magic Link',
    'core.executions.names.sendSMS': 'Send SMS',
    'core.executions.names.verifySMSOTP': 'Verify SMS OTP',
    'core.executions.names.default': 'Execution',
    'core.executions.names.ouResolver': 'Resolve OU',
    'core.executions.names.invite': 'Invite',
    'core.executions.names.email': 'Send Email',
    'core.executions.names.sms': 'Send SMS',
    'core.executions.names.credentialSetter': 'Set Credentials',
    'core.executions.names.attributeUniqueness': 'Validate Attribute Uniqueness',
    'core.executions.names.permissionValidator': 'Validate Permission',
    'core.executions.names.provisioning': 'Provisioning',
    'core.executions.names.httpRequest': 'HTTP Request',
    'core.executions.names.ouCreation': 'OU Creation',
    'core.executions.names.userTypeResolver': 'User Type Resolver',

    // OTP executor
    'core.executions.otp.description': 'Configure the OTP executor settings.',
    'core.executions.otp.maxAttempts.label': 'Maximum Attempts',
    'core.executions.otp.maxAttempts.placeholder': 'e.g., 3',
    'core.executions.otp.maxAttempts.hint': 'The maximum number of OTP verification attempts before the flow fails.',

    // SMS OTP executor modes
    'core.executions.smsOtp.mode.send': 'Send OTP',
    'core.executions.smsOtp.mode.verify': 'Verify OTP',
    'core.executions.smsOtp.mode.label': 'Mode',
    'core.executions.smsOtp.mode.placeholder': 'Select a mode',
    'core.executions.smsOtp.description': 'Configure the SMS OTP executor settings.',

    // SMS OTP sender selection
    'core.executions.smsOtp.sender.label': 'Notification Sender',
    'core.executions.smsOtp.sender.placeholder': 'Select a notification sender',
    'core.executions.smsOtp.sender.required': 'Notification sender is required and must be selected.',
    'core.executions.smsOtp.sender.noSenders':
      'No notification senders available. Please create a notification sender first.',

    // Consent executor
    'core.executions.consent.description': 'Configure the consent executor settings.',
    'core.executions.consent.timeout.label': 'Consent Timeout (seconds)',
    'core.executions.consent.timeout.placeholder': '0',
    'core.executions.consent.timeout.hint': 'Time in seconds before the consent request expires. Use 0 for no timeout.',

    // Identifying executor modes
    'core.executions.identifying.mode.identify': 'Identify',
    'core.executions.identifying.mode.resolve': 'Resolve (Disambiguation)',
    'core.executions.identifying.mode.label': 'Mode',
    'core.executions.identifying.mode.placeholder': 'Select a mode',
    'core.executions.identifying.description':
      'Configure the identifying executor mode. Use "Resolve" to enable user disambiguation when multiple users match.',

    // Passkey executor modes
    'core.executions.passkey.mode.challenge': 'Challenge',
    'core.executions.passkey.mode.verify': 'Verify',
    'core.executions.passkey.mode.registerStart': 'Start Registration',
    'core.executions.passkey.mode.registerFinish': 'Finish Registration',
    'core.executions.passkey.mode.label': 'Mode',
    'core.executions.passkey.mode.placeholder': 'Select a mode',
    'core.executions.passkey.description': 'Configure the Passkey executor settings.',

    // Passkey relying party configuration
    'core.executions.passkey.relyingPartyId.label': 'Relying Party ID',
    'core.executions.passkey.relyingPartyId.placeholder': 'e.g., localhost or example.com',
    'core.executions.passkey.relyingPartyId.hint':
      'The domain identifier for passkey registration (typically your domain name)',
    'core.executions.passkey.relyingPartyName.label': 'Relying Party Name',
    'core.executions.passkey.relyingPartyName.placeholder': 'e.g., My Application',
    'core.executions.passkey.relyingPartyName.hint': 'A human-readable name shown to users during passkey registration',

    // OU Resolver executor
    'core.executions.ouResolver.description': 'Configure the OU resolution strategy.',
    'core.executions.ouResolver.resolveFrom.label': 'Resolve From',
    'core.executions.ouResolver.resolveFrom.placeholder': 'Select a resolution strategy',
    'core.executions.ouResolver.resolveFrom.caller': 'Caller',
    'core.executions.ouResolver.resolveFrom.prompt': 'Prompt',
    'core.executions.ouResolver.resolveFrom.promptAll': 'Prompt All',

    // Invite executor
    'core.executions.invite.description': 'Configure the invite executor mode.',
    'core.executions.invite.mode.label': 'Mode',
    'core.executions.invite.mode.placeholder': 'Select a mode',
    'core.executions.invite.mode.generate': 'Generate',
    'core.executions.invite.mode.verify': 'Verify',

    // Email executor
    'core.executions.email.description': 'Configure the email executor settings.',
    'core.executions.email.emailTemplate.label': 'Email Template',
    'core.executions.email.emailTemplate.placeholder': 'e.g., UserInvite',
    'core.executions.email.emailTemplate.hint': 'The email template scenario to use when sending the email.',

    // SMS executor
    'core.executions.sms.description': 'Configure the SMS executor settings.',
    'core.executions.sms.smsTemplate.label': 'SMS Template',
    'core.executions.sms.smsTemplate.placeholder': 'e.g., OTPVerification',
    'core.executions.sms.smsTemplate.hint': 'The SMS template scenario to use when sending the message.',

    // OpenID4VP verifier executor
    'core.executions.openid4vp.description':
      'Select the presentation definition this executor requests from the wallet.',
    'core.executions.openid4vp.allowAuthenticationWithoutLocalUser.label': 'Allow authentication without a local user',
    'core.executions.openid4vp.allowAuthenticationWithoutLocalUser.hint':
      'When enabled, a holder with no matching local user is provisioned just-in-time. When disabled, login requires an existing matching user.',

    // Permission validator executor
    'core.executions.permissionValidator.description': 'Configure required permission scopes.',
    'core.executions.permissionValidator.requiredScopes.label': 'Required Scopes',
    'core.executions.permissionValidator.requiredScopes.placeholder': 'e.g., system',
    'core.executions.permissionValidator.requiredScopes.hint':
      'Comma-separated list of scopes. The user must have at least one of these scopes.',

    // Federated auth connection
    'core.executions.federation.connection.description':
      'Select a connection from the following list to link it with the login flow.',
    'core.executions.federation.connection.label': 'Connection',
    'core.executions.federation.connection.placeholder': 'Select a connection',
    'core.executions.federation.connection.required': 'Connection is required and must be selected.',
    'core.executions.federation.connection.noConnections':
      'No connections available. Please create a connection to link with the login flow.',

    // Federated auth properties
    'core.executions.federation.allowAuthenticationWithoutLocalUser.label': 'Allow Authentication Without Local User',
    'core.executions.federation.allowAuthenticationWithoutLocalUser.hint':
      'Allow users to authenticate even when no matching local user exists.',
    'core.executions.federation.allowRegistrationWithExistingUser.label': 'Allow Registration With Existing User',
    'core.executions.federation.allowRegistrationWithExistingUser.hint':
      'Allow existing users to proceed through registration flows.',
    'core.executions.federation.allowCrossOUProvisioning.label': 'Allow Cross-OU Provisioning',
    'core.executions.federation.allowCrossOUProvisioning.hint':
      'Allow creating a user in a different organizational unit.',

    // Provisioning executor
    'core.executions.provisioning.description': 'Configure the provisioning executor settings.',
    'core.executions.provisioning.includeOptional.label': 'Allow Optional Non-Credential Attributes',
    'core.executions.provisioning.includeOptional.hint':
      'Prompt for optional non-credential attributes during dynamic input collection.',
    'core.executions.provisioning.includeOptionalCredentials.label': 'Include Optional Credentials',
    'core.executions.provisioning.includeOptionalCredentials.hint':
      'Prompt for optional credential attributes during dynamic input collection.',
    'core.executions.provisioning.maxPerPrompt.label': 'Max Per Prompt',
    'core.executions.provisioning.maxPerPrompt.placeholder': '0',
    'core.executions.provisioning.maxPerPrompt.hint':
      'Number of dynamic inputs to show per prompt when connected to this provisioning executor.',
    'core.executions.provisioning.assignGroup.label': 'Assign Group',
    'core.executions.provisioning.assignGroup.placeholder': 'Comma-separated group IDs to assign',
    'core.executions.provisioning.assignRole.label': 'Assign Role',
    'core.executions.provisioning.assignRole.placeholder': 'Comma-separated role IDs to assign',
    'core.placeholders.dynamicInputPlaceholder.title': 'Dynamic Input',
    'core.placeholders.dynamicInputPlaceholder.hint': 'Resolves input fields passed from runtime.',

    // OU executor
    'core.executions.ouExecutor.description': 'Configure the OU creation executor settings.',
    'core.executions.ouExecutor.parentOuId.label': 'Parent OU ID',
    'core.executions.ouExecutor.parentOuId.placeholder': 'Override the default parent OU',
    'core.executions.ouExecutor.parentOuId.hint': 'Overrides the default OU for new OU creation.',

    // User Type Resolver executor
    'core.executions.userTypeResolver.description': 'Configure the user type resolver settings.',
    'core.executions.userTypeResolver.allowedUserTypes.label': 'Allowed User Types',
    'core.executions.userTypeResolver.allowedUserTypes.placeholder': 'e.g., employee, customer',
    'core.executions.userTypeResolver.allowedUserTypes.hint':
      'Comma-separated list of allowed user type names to filter available types.',

    // HTTP Request executor
    'core.executions.httpRequest.description': 'Configure the HTTP request executor settings.',
    'core.executions.httpRequest.url.label': 'URL',
    'core.executions.httpRequest.url.placeholder': 'https://api.example.com/endpoint',
    'core.executions.httpRequest.method.label': 'Method',
    'core.executions.httpRequest.method.placeholder': 'Select HTTP method',
    'core.executions.httpRequest.headers.label': 'Headers',
    'core.executions.httpRequest.headers.keyPlaceholder': 'Header name',
    'core.executions.httpRequest.headers.valuePlaceholder': 'Header value',
    'core.executions.httpRequest.body.label': 'Request Body',
    'core.executions.httpRequest.body.placeholder': 'Enter JSON request body',
    'core.executions.httpRequest.timeout.label': 'Timeout (seconds)',
    'core.executions.httpRequest.timeout.placeholder': '10',
    'core.executions.httpRequest.timeout.hint': 'Request timeout in seconds (max 20).',
    'core.executions.httpRequest.responseMapping.label': 'Response Mapping',
    'core.executions.httpRequest.responseMapping.keyPlaceholder': 'Runtime data key',
    'core.executions.httpRequest.responseMapping.valuePlaceholder': 'Response path (e.g., data.userId)',
    'core.executions.httpRequest.errorHandling.label': 'Error Handling',
    'core.executions.httpRequest.errorHandling.failOnError.label': 'Fail on Error',
    'core.executions.httpRequest.errorHandling.retryCount.label': 'Retry Count',
    'core.executions.httpRequest.errorHandling.retryCount.placeholder': '0',
    'core.executions.httpRequest.errorHandling.retryCount.hint': 'Max retry attempts (max 5).',
    'core.executions.httpRequest.errorHandling.retryDelay.label': 'Retry Delay (ms)',
    'core.executions.httpRequest.errorHandling.retryDelay.placeholder': '0',
    'core.executions.httpRequest.errorHandling.retryDelay.hint': 'Delay between retries in milliseconds (max 5000).',

    // Executor input configuration
    'core.executions.inputs.title': 'Executor Inputs',
    'core.executions.inputs.typeLabel': 'Type',
    'core.executions.inputs.typePlaceholder': 'Select type',
    'core.executions.inputs.identifierLabel': 'Identifier',
    'core.executions.inputs.identifierPlaceholder': 'e.g., username, email',
    'core.executions.inputs.required': 'Required',
    'core.executions.inputs.add': 'Add Input',
    'core.executions.inputs.remove': 'Remove input',
    'core.executions.inputs.empty': 'No custom inputs configured. The executor will use its default inputs.',
    'core.executions.inputs.types.text': 'Text',
    'core.executions.inputs.types.email': 'Email',
    'core.executions.inputs.types.password': 'Password',
    'core.executions.inputs.types.otp': 'OTP',
    'core.executions.inputs.types.phone': 'Phone',
    'core.executions.inputs.types.consent': 'Consent',
    'core.executions.inputs.types.select': 'Select',

    // Execution steps - tooltips and messages
    // No-config executors
    'core.executions.noConfig.description': 'This executor has no configurable properties.',

    'core.executions.tooltip.configurationHint': 'Click to configure this step',
    'core.executions.landing.message': 'This {{executor}} step will redirect users to a landing page.',

    // Execution steps - branching handles
    'core.executions.handles.success': 'onSuccess',
    'core.executions.handles.failure': 'onFailure',
    'core.executions.handles.incomplete': 'onIncomplete',

    // Canvas hints and tips
    'core.canvas.hints.autoLayout': 'Tip: Use auto-layout to organize your flow',
    'core.canvas.buttons.autoLayout': 'Auto Layout',

    // Steps - end
    'core.steps.end.flowCompletionProperties': 'Flow Completion Properties',

    // Validation messages - input fields
    'core.validation.fields.input.general':
      'Required fields are not properly configured for the input field with ID <code>{{id}}</code>.',
    'core.validation.fields.input.idpName': 'Identity provider name is required',
    'core.validation.fields.input.idpId': 'Connection is required',
    'core.validation.fields.input.senderId': 'Notification sender is required',
    'core.validation.fields.input.label': 'Label is required',
    'core.validation.fields.input.ref': 'Attribute is required',

    // Validation messages - executor
    'core.validation.fields.executor.general': 'The executor <0>{{id}}</0> is not properly configured.',

    // Validation messages - call
    'core.validation.fields.call.general': 'The Call node <0>{{id}}</0> has no referenced flow.',
    'core.validation.fields.call.flowRef': 'Referenced flow is required',

    // Validation messages - button
    'core.validation.fields.button.general':
      'Required fields are not properly configured for the button with ID <code>{{id}}</code>.',
    'core.validation.fields.button.text': 'Button text is required',
    'core.validation.fields.button.label': 'Label is required',
    'core.validation.fields.button.action': 'Action is required',
    'core.validation.fields.button.variant': 'Variant is required',

    // Validation messages - checkbox
    'core.validation.fields.checkbox.general':
      'Required fields are not properly configured for the checkbox with ID <code>{{id}}</code>.',
    'core.validation.fields.checkbox.label': 'Label is required',
    'core.validation.fields.checkbox.ref': 'Attribute is required',

    // Validation messages - divider
    'core.validation.fields.divider.general':
      'Required fields are not properly configured for the divider with ID <code>{{id}}</code>.',
    'core.validation.fields.divider.variant': 'Variant is required',

    // Validation messages - typography
    'core.validation.fields.typography.general':
      'Required fields are not properly configured for the typography with ID <code>{{id}}</code>.',
    'core.validation.fields.typography.text': 'Text content is required',
    'core.validation.fields.typography.label': 'Label is required',
    'core.validation.fields.typography.variant': 'Variant is required',

    // Validation messages - image
    'core.validation.fields.image.general':
      'Required fields are not properly configured for the image with ID <code>{{id}}</code>.',
    'core.validation.fields.image.src': 'Image source is required',
    'core.validation.fields.image.variant': 'Variant is required',

    // Placeholders
    'core.placeholders.image': 'No image source',
    'core.placeholders.image.dynamicSrc': 'Resolved at runtime',
    'core.placeholders.customComponent': 'Custom',
    'core.placeholders.customComponent.identifier': 'Identifier: {{id}}',

    // Validation messages - rich text
    'core.validation.fields.richText.general':
      'Required fields are not properly configured for the rich text with ID <code>{{id}}</code>.',
    'core.validation.fields.richText.text': 'Rich text content is required',
    'core.validation.fields.richText.label': 'Label is required',

    // Validation messages - OTP input
    'core.validation.fields.otpInput.label': 'OTP input label is required',

    // Validation messages - phone number input
    'core.validation.fields.phoneNumberInput.label': 'Phone number label is required',
    'core.validation.fields.phoneNumberInput.ref': 'Phone number attribute is required',

    // Validation messages - form
    'core.validation.fields.form.noSubmitButton':
      'Form <code>{{id}}</code> has input fields but no submit button. Add a button with type "Submit" so that users can submit the form.',

    // Validation messages - SSO pairing
    'core.validation.sso.missingCheckpointRef':
      'SSO check <code>{{id}}</code> does not reference a session checkpoint. Select one in its properties, or remove the step.',
    'core.validation.sso.invalidCheckpointRef':
      'SSO check <code>{{id}}</code> references a session step that no longer exists. Select a valid session checkpoint in its properties.',
    'core.validation.sso.orphanSession':
      'Session step <code>{{id}}</code> is not referenced by any SSO check. Add an SSO check that uses it, or remove the step.',

    // Elements - rich text
    'core.elements.richText.placeholder': 'Enter text here...',
    'core.elements.richText.resolvedI18nValue': 'Resolved i18n value',
    'core.elements.richText.linkEditor.urlTypeLabel': 'URL Type',
    'core.elements.richText.linkEditor.placeholder': 'Type or paste a link',
    'core.elements.richText.linkEditor.textPlaceholder': 'Text',
    'core.elements.richText.linkEditor.apply': 'Apply',
    'core.elements.richText.linkEditor.editLink': 'Edit Link',
    'core.elements.richText.linkEditor.viewLink': 'Link',
    'core.elements.richText.action.description':
      'Turn this rich text into an interactive link. When on, the link inside triggers the connected step instead of navigating.',
    'core.elements.richText.action.enabled.label': 'Use as an interactive link',
    'core.elements.richText.action.ref.label': 'Connected step',

    // Call step
    'core.call.unconfiguredLabel': 'Flow',
    'core.call.selectFlow': 'Select a flow to invoke',
    'core.call.referencedFlow': 'Referenced flow',
    'core.call.tooltip.configure': 'Configure',
    'core.call.tooltip.delete': 'Delete',
    'core.call.tooltip.openFlow': 'Open referenced flow',
    'core.call.tooltip.openFlowDisabled': 'Configure a referenced flow to enable',
    'core.call.handles.success': 'On success',
    'core.call.handles.failure': 'On failure',
    'core.call.openFlow.dialog.title': 'Open referenced flow?',
    'core.call.openFlow.dialog.description': 'Any unsaved changes to the current flow will be lost.',
    'core.call.openFlow.dialog.cancel': 'Cancel',
    'core.call.openFlow.dialog.confirm': 'Continue',
    'core.call.properties.description': 'Pick the flow to invoke when this node executes.',
    'core.call.properties.loadError': 'Failed to load available flows',
    'core.call.properties.flow.label': 'Referenced flow',
    'core.call.properties.flow.placeholder': 'Select a flow',
    'core.call.properties.flow.loading': 'Loading flows…',
    'core.call.properties.flow.error.unknown': 'The referenced flow no longer exists. Pick a valid flow.',

    // Elements - text element
    'core.elements.text.align.label': 'Align',
    'core.elements.text.align.options.left': 'Left',
    'core.elements.text.align.options.center': 'Center',
    'core.elements.text.align.options.right': 'Right',
    'core.elements.text.align.options.justify': 'Justify',
    'core.elements.text.align.options.inherit': 'Inherit',

    // Elements - classes property field
    'core.elements.classesPropertyField.label': 'CSS Classes',
    'core.elements.classesPropertyField.placeholder': 'e.g. btn-primary',
    'core.elements.classesPropertyField.addClass': 'Add class',

    // Elements - text property field
    'core.elements.textPropertyField.placeholder': 'Enter {{propertyName}}',
    'core.elements.textPropertyField.tooltip.configureTranslation': 'Configure translation',
    'core.elements.textPropertyField.tooltip.configureDynamicValue': 'Insert dynamic value',
    'core.elements.textPropertyField.i18nKey': 'Translation Key',
    'core.elements.textPropertyField.resolvedValue': 'Resolved Value',

    // Elements - dynamic value popover
    'core.elements.textPropertyField.dynamicValuePopover.title': 'Dynamic Value for {{field}}',
    'core.elements.textPropertyField.dynamicValuePopover.tabs.translation': 'Translation',
    'core.elements.textPropertyField.dynamicValuePopover.tabs.variables': 'Variables',

    // Elements - meta card
    'core.elements.textPropertyField.metaCard.title': 'Variable for {{field}}',
    'core.elements.textPropertyField.metaCard.variablePath': 'Variable Path',
    'core.elements.textPropertyField.metaCard.variablePathPlaceholder': 'e.g. application.name',
    'core.elements.textPropertyField.metaCard.variablePathHint': 'Select a common variable or type a custom path',
    'core.elements.textPropertyField.metaCard.formattedValue': 'Formatted Value',

    // Elements - i18n card
    'core.elements.textPropertyField.i18nCard.title': 'Translation for {{field}}',
    'core.elements.textPropertyField.i18nCard.createTitle': 'Create Translation',
    'core.elements.textPropertyField.i18nCard.updateTitle': 'Update Translation',
    'core.elements.textPropertyField.i18nCard.i18nKey': 'Translation Key',
    'core.elements.textPropertyField.i18nCard.i18nKeyInputPlaceholder': 'Enter a unique translation key',
    'core.elements.textPropertyField.i18nCard.i18nKeyInputHint': 'Use format: screen.{{key}}',
    'core.elements.textPropertyField.i18nCard.selectI18nKey': 'Select an existing key',
    'core.elements.textPropertyField.i18nCard.language': 'Language',
    'core.elements.textPropertyField.i18nCard.languageText': 'Translation Text',
    'core.elements.textPropertyField.i18nCard.languageTextPlaceholder': 'Enter translation text',
    'core.elements.textPropertyField.i18nCard.commonKeyWarning':
      'This is a common key shared across screens. Changes will affect all usages.',
    'core.elements.textPropertyField.i18nCard.chip.commonScreen.label': 'Common',
    'core.elements.textPropertyField.i18nCard.tooltip.commonKeyTooltip': 'This key is shared across multiple screens',
    'core.elements.textPropertyField.i18nCard.tooltip.editExistingTranslation': 'Edit existing translation',
    'core.elements.textPropertyField.i18nCard.tooltip.addNewTranslation': 'Add new translation',
    'core.elements.textPropertyField.i18nCard.invalidKeyFormat':
      'Invalid key format. Use only letters, numbers, dots, underscores, and hyphens.',

    // Form requires view dialog
    'core.dialogs.formRequiresView.formOnCanvas.title': 'Form Requires a View',
    'core.dialogs.formRequiresView.formOnCanvas.description':
      'Form components cannot be placed directly on the canvas. They must be inside a View component.',
    'core.dialogs.formRequiresView.formOnCanvas.alertMessage':
      'Would you like to create a View and add the Form inside it?',
    'core.dialogs.formRequiresView.formOnCanvas.confirmButton': 'Add View with Form',
    'core.dialogs.formRequiresView.inputOnCanvas.title': 'Input Requires a Form and View',
    'core.dialogs.formRequiresView.inputOnCanvas.description':
      'Input components cannot be placed directly on the canvas. They must be inside a Form, which must be inside a View.',
    'core.dialogs.formRequiresView.inputOnCanvas.alertMessage':
      'Would you like to create a View with a Form and add the Input inside it?',
    'core.dialogs.formRequiresView.inputOnCanvas.confirmButton': 'Add View, Form and Input',
    'core.dialogs.formRequiresView.inputOnView.title': 'Input Requires a Form',
    'core.dialogs.formRequiresView.inputOnView.description':
      'Input components cannot be placed directly inside a View. They must be inside a Form component.',
    'core.dialogs.formRequiresView.inputOnView.alertMessage':
      'Would you like to create a Form and add the Input inside it?',
    'core.dialogs.formRequiresView.inputOnView.confirmButton': 'Add Form with Input',
    'core.dialogs.formRequiresView.widgetOnCanvas.title': 'Widget Requires a View',
    'core.dialogs.formRequiresView.widgetOnCanvas.description':
      'Widgets cannot be placed directly on the canvas. They must be inside a View component.',
    'core.dialogs.formRequiresView.widgetOnCanvas.alertMessage':
      'Would you like to create a View and add the Widget inside it?',
    'core.dialogs.formRequiresView.widgetOnCanvas.confirmButton': 'Add View with Widget',
    'core.dialogs.formRequiresView.cancelButton': 'Cancel',

    // SSO toggle (login flows)
    'sso.toggleLabel': 'Enable SSO',
    'sso.toggleDescription': 'Reuse an active session to skip sign-in',
    'sso.toggleTooltipOn': 'Single sign-on is active for this flow. Turn off to remove the SSO wiring.',
    'sso.disabledNoEntry': 'Connect the Start step to a view step to enable SSO.',
    'sso.disabledEntryNotPrompt':
      'To enable SSO, the flow must start with a view step. The SSO check needs a login screen to fall back to.',
    'sso.disabledNoAssert': 'Add an authentication completion step to the flow before enabling SSO.',
    'sso.disabledReadOnly': 'This flow is read-only and cannot be modified.',
    'sso.enabledSnackbar':
      'SSO enabled. A session check now runs after Start, and sessions are saved before the flow completes.',
    'sso.disabledSnackbar_one': 'SSO disabled. {{count}} checkpoint was removed and the flow reconnected.',
    'sso.disabledSnackbar_other': 'SSO disabled. {{count}} checkpoints were removed and the flow reconnected.',
    'sso.placementHint': 'Click a highlighted connection to choose where the session checkpoint joins the flow.',
    'sso.placementCancel': 'Cancel',
    'sso.confirmDialog.title': 'Remove single sign-on?',
    'sso.confirmDialog.description_one':
      'This removes {{count}} SSO checkpoint and its session step, and reconnects the flow. Users will authenticate with their credentials every time.',
    'sso.confirmDialog.description_other':
      'This removes {{count}} SSO checkpoints and their session steps, and reconnects the flow. Users will authenticate with their credentials every time.',
    'sso.confirmDialog.confirmButton': 'Remove SSO',
    'sso.confirmDialog.cancelButton': 'Cancel',
    'sso.properties.checkpointLabel': 'Session checkpoint',
    'sso.properties.checkpointDangling': 'The referenced session step no longer exists. Select a valid session step.',

    // Form adapter
    'core.adapters.form.badgeLabel': 'Form',
    'core.adapters.form.placeholder': 'DROP FORM COMPONENTS HERE',

    // Header panel
    'core.headerPanel.goBack': 'Go back to Flows',
    'core.headerPanel.autoLayout': 'Auto Layout',
    'core.headerPanel.save': 'Save',
    'core.headerPanel.editTitle': 'Edit flow name',
    'core.headerPanel.saveTitle': 'Save flow name',
    'core.headerPanel.cancelEdit': 'Cancel',
    'core.headerPanel.edgeStyleTooltip': 'Change edge style',
    'core.headerPanel.simulate': 'Preview',
    'core.headerPanel.stopSimulation': 'Stop preview',
    'core.headerPanel.saveDisabledDuringPreview': 'Stop the preview before saving',
    'core.headerPanel.edgeStyles.bezier': 'Bezier',
    'core.headerPanel.edgeStyles.smoothStep': 'Smooth Step',
    'core.headerPanel.edgeStyles.step': 'Step',

    // Flow simulation (preview mode)
    'core.simulation.stepCount_one': 'Step {{count}}',
    'core.simulation.stepCount_other': 'Step {{count}}',
    'core.simulation.chooseNext': 'Choose how the user proceeds from this step',
    'core.simulation.screenHint': 'Select an option on the preview screen to continue',
    'core.simulation.screenHintOr': 'or select an option on the preview screen',
    'core.simulation.complete': 'Flow complete — no outgoing transitions',
    'core.simulation.back': 'Go back one step',
    'core.simulation.staticView': 'Switch to a static canvas view',
    'core.simulation.followSteps': 'Follow steps on the canvas',
    'core.simulation.restart': 'Restart preview',
    'core.simulation.exit': 'Exit preview',
    'core.simulation.kinds.action': 'Continue',
    'core.simulation.kinds.success': 'On success',
    'core.simulation.kinds.incomplete': 'On incomplete',
    'core.simulation.kinds.failure': 'On failure',
    'core.simulation.preview.title': 'End-user preview',
    'core.simulation.preview.noScreen': 'No screen is shown for this step',
    'core.simulation.preview.noScreenHint': 'This step runs in the background before the flow continues',
    'core.simulation.preview.noScreenHintNamed': '{{id}} runs in the background before the flow continues',
    'core.simulation.preview.callStepLabel': 'Calls another flow',
    'core.simulation.preview.applicationLabel': 'Preview as application',
    'core.simulation.preview.devices.mobile': 'Mobile',
    'core.simulation.preview.devices.tablet': 'Tablet',
    'core.simulation.preview.devices.desktop': 'Desktop',
    'core.simulation.preview.consentEssentialPlaceholder': 'Attributes requested by the application',
    'core.simulation.preview.consentOptionalPlaceholder': 'Optional attributes the user can toggle',
    'core.simulation.preview.dynamicFieldsHint': 'Input fields resolved at runtime',
    'core.simulation.preview.darkMode': 'Switch to dark preview',
    'core.simulation.preview.lightMode': 'Switch to light preview',

    // Resource panel
    'core.resourcePanel.title': 'Resources',
    'core.resourcePanel.showResources': 'Show Resources',
    'core.resourcePanel.hideResources': 'Hide Resources',
    'core.resourcePanel.starterTemplates.title': 'Starter Templates',
    'core.resourcePanel.starterTemplates.description':
      'Choose one of these templates to start building registration experience',
    'core.resourcePanel.search.placeholder': 'Search (e.g. MFA, social, consent)',
    'core.resourcePanel.search.clear': 'Clear search',
    'core.resourcePanel.search.noResults': 'No matching resources',
    'core.resourcePanel.search.noResultsHint': 'Try a different keyword, such as "OTP", "Google", or "passkey"',
    'core.resourcePanel.widgets.title': 'Widgets',
    'core.resourcePanel.widgets.description': 'Ready-made blocks like social login, OTP, and passkey',
    'core.resourcePanel.steps.title': 'Steps',
    'core.resourcePanel.steps.description': 'Screens and logic that shape your flow',
    'core.resourcePanel.components.title': 'Components',
    'core.resourcePanel.components.description': 'Form fields, buttons, and display elements',
    'core.resourcePanel.executors.title': 'Executors',
    'core.resourcePanel.executors.description': 'Backend actions like verifying credentials or sending OTPs',

    // Steps (shared)
    'core.steps.renameTooltip': 'Double-click to edit the step ID',
    'core.steps.stepId': 'Step ID',

    // View step
    'core.steps.view.addComponent': 'Add Component',
    'core.steps.view.addField': 'Add Field',
    'core.steps.view.configure': 'Configure',
    'core.steps.view.preview': 'Preview this screen',
    'core.steps.view.remove': 'Remove',
    'core.steps.view.noComponentsAvailable': 'No components available',

    // Rule
    'core.rule.conditionalRule': 'Conditional Rule',
    'core.rule.remove': 'Remove',

    // Field extended properties
    'core.fieldExtendedProperties.attribute': 'Attribute',
    'core.fieldExtendedProperties.selectAttribute': 'Select an attribute',

    // Button extended properties
    'core.buttonExtendedProperties.type.label': 'Type',
    'core.buttonExtendedProperties.type.submit': 'Submit',
    'core.buttonExtendedProperties.type.trigger': 'Trigger',
    'core.buttonExtendedProperties.startIcon.label': 'Start Icon',
    'core.buttonExtendedProperties.startIcon.placeholder': 'Enter icon path (e.g., assets/images/icons/icon.svg)',
    'core.buttonExtendedProperties.startIcon.hint': 'Optional icon displayed before the button label',
    'core.buttonExtendedProperties.endIcon.label': 'End Icon',
    'core.buttonExtendedProperties.endIcon.placeholder': 'Enter icon path (e.g., assets/images/icons/icon.svg)',
    'core.buttonExtendedProperties.endIcon.hint': 'Optional icon displayed after the button label',

    // Rules properties
    'core.rulesProperties.description': 'Define a rule to how conditionally proceed to next steps in the flow',

    // Login flow builder
    'core.loginFlowBuilder.form': 'Form',
    'core.loginFlowBuilder.errors.validationRequired': 'Please fix all validation errors before saving.',
    'core.loginFlowBuilder.errors.structureValidationFailed': 'Flow structure validation failed: {{error}}',
    'core.loginFlowBuilder.errors.saveFailed': 'Failed to save flow. Please try again.',
    'core.loginFlowBuilder.success.flowCreated': 'Flow created successfully.',
    'core.loginFlowBuilder.success.flowUpdated': 'Flow updated successfully.',
  },

  /**
   * Appearance namespace - Theme and layout related translations
   */
  appearance: {
    'theme.defaultTheme': 'Default Theme',
    'theme.appTheme.displayName': '{{appName}} Theme',
  },

  // ============================================================================
  // Translations namespace - Text & Translations feature
  // ============================================================================
  translations: {
    'page.title': 'Translations',
    'page.subtitle': 'Manage and customize UI text and translations for your application.',

    'listing.addLanguage': 'Add Language',
    'listing.columns.language': 'Language',
    'listing.columns.actions': 'Actions',

    'language.selectPlaceholder': 'Select a language',
    'language.addOption': 'Add new language...',

    'language.create.steps.country': 'Country',
    'language.create.steps.language': 'Language',
    'language.create.steps.localeCode': 'Locale Code',
    'language.create.steps.initialize': 'Initialize',

    'language.create.country.title': 'Choose a Country',
    'language.create.country.subtitle': 'Select the country for the language you want to add.',
    'language.create.countryLabel': 'Country',
    'language.create.country.placeholder': 'Select a country',
    'language.create.country.helperText':
      'Country name will be used to derive a BCP 47 compliant locale code for the language.',

    'language.create.language.title': 'Choose a Language',
    'language.create.language.subtitle': 'Select the language variant spoken in {{country}}.',
    'language.create.language.label': 'Language',
    'language.create.language.placeholder': 'Select a language',
    'language.create.language.helperText':
      'Language picked here together with the country selection will determine the BCP 47 compliant locale code.',

    'language.create.localeCode.title': 'Review Locale Code',
    'language.create.localeCode.subtitle':
      'The locale code was derived from your selection. Override it here if you need a different tag.',

    'language.create.initialize.title': 'Initialize Translations',
    'language.create.initialize.subtitle': 'Choose how to populate the translation keys for this language.',
    'language.create.initialize.copyFromEnglish.label': 'Copy from English',
    'language.create.initialize.copyFromEnglish.description':
      'All keys will be pre-filled with English (en-US) text as a starting point. You can edit them afterwards.',
    'language.create.initialize.startEmpty.label': 'Start empty',
    'language.create.initialize.startEmpty.description':
      'All keys will be created with empty values. Useful when you have your own translations ready to paste in.',

    'language.create.createButton': 'Create Language',

    'language.add.dialogTitle': 'Add New Language',
    'language.add.code.label': 'Language Code',
    'language.add.codePlaceholder': 'e.g. fr-FR, de-DE, ja-JP',
    'language.add.code.helperText':
      'If you are manually modifying the generated code, use BCP 47 format (e.g. fr-FR for French, de-DE for German, etc.).',
    'language.add.populateLabel': 'Pre-populate from English (en-US)',
    'language.add.populateHelper': 'All keys will be pre-filled with English text. You can update them after.',
    'language.add.emptyHelper': 'All keys will be added with empty values. You can fill them in later.',
    'language.add.adding': 'Adding translations...',
    'language.add.success': '"{{code}}" added successfully.',
    'language.add.error': 'Failed to add language. Please try again.',

    'namespace.label': 'Namespaces',
    'namespace.noKeys': 'No translatable keys in this namespace.',

    'editor.panelHeader': 'Edit Translations',
    'editor.fieldsTab': 'Fields',
    'editor.jsonTab': 'JSON',
    'editor.searchPlaceholder': 'Search by key or value...',
    'editor.noResults': 'No matching translations.',
    'editor.noLanguageSelected': 'Select a language to start editing.',
    'editor.loading': 'Loading translations...',
    'editor.fieldSaveSuccess': 'Saved.',
    'editor.fieldSaveError': 'Failed to save.',
    'editor.jsonSaveAll': 'Save All',
    'editor.jsonSaveSuccess': 'All translations saved.',
    'editor.jsonSaveError': 'Failed to save some translations.',
    'editor.jsonInvalid': 'Invalid JSON — fix errors before saving.',
    'editor.resetField': 'Reset to saved value',
    'editor.noKeys': 'No translatable keys in this namespace.',
    'editor.unsavedCount': '{{count}} unsaved change',
    'editor.namespace': 'Namespace',
    'editor.namespace.helperText':
      'A namespace typically represents a page or a section within a page. It helps group and organize related translation keys for better structure and maintainability.',
    'editor.textFields': 'Fields',
    'editor.rawJson': 'Raw JSON',
    'editor.addKey': 'Add Key',
    'editor.addKey.keyLabel': 'Key',
    'editor.addKey.valueLabel': 'Value',
    'editor.addKey.keyPlaceholder': 'e.g. my.translation.key',
    'editor.addKey.valuePlaceholder': 'Translation value',
    'editor.addKey.submit': 'Add',
    'editor.addKey.cancel': 'Cancel',
    'editor.addKey.duplicateKey': 'This key already exists.',
    'editor.readOnlyKeys': 'Keys are fixed in this namespace. Only values can be edited.',

    'actions.saveChanges': 'Save Changes',
    'actions.discardChanges': 'Discard Changes',
    'actions.resetToDefault': 'Reset to Default',
    'preview.noTheme': 'No themes configured. Preview unavailable.',

    'delete.title': 'Delete Language',
    'delete.message':
      'Are you sure you want to delete all custom translations for "{{language}}"? This action cannot be undone.',
    'delete.disclaimer': 'All custom translations for this language will be permanently removed and reset to defaults.',
    'delete.error': 'Failed to delete translations. Please try again.',
  },

  design: {
    'page.title': 'Design',
    'page.subtitle': 'Create, customize, and manage visual themes & layouts for your applications.',
    'themes.section.title': 'Themes',
    'themes.actions.add.label': 'Add Theme',
    'themes.empty_state.message': 'No themes yet',
    'themes.show_more.label': 'Show {{count}} more',
    'themes.builder.actions.delete.label': 'Delete',
    'themes.builder.actions.save.label': 'Save',
    'themes.builder.actions.revert.label': 'Revert',
    'themes.builder.tooltips.show_sections': 'Show sections',
    'themes.builder.tooltips.hide_sections': 'Hide sections',
    'themes.builder.config.label': 'Config',
    'themes.builder.preview.label': 'Preview',
    'themes.builder.preview.iframe_title': 'Gate Preview',
    'themes.builder.actions.back_to_design.label': 'Back to Design',
    'themes.builder.sections.colors.label': 'Colors',
    'themes.builder.sections.colors.description': 'Light & dark color schemes',
    'themes.builder.sections.shape.label': 'Shape',
    'themes.builder.sections.shape.description': 'Border radius & corner styles',
    'themes.builder.sections.typography.label': 'Typography',
    'themes.builder.sections.typography.description': 'Font family & type scale',
    'themes.config.select_theme.message': 'Select a theme to view configuration',
    'themes.config.errors.load.message': 'Failed to load theme configuration.',
    'themes.forms.configure_name.title': 'Create a Theme',
    'themes.forms.configure_color.title': 'Primary Color',
    'themes.forms.configure_color.actions.back.label': 'Back',
    'themes.forms.configure_color.actions.continue.label': 'Continue',
    'themes.forms.configure_color.actions.create.label': 'Create Theme',
    'themes.forms.configure_color.errors.create_failed.message': 'Failed to create theme. Please try again.',
    'themes.forms.color_builder.primary.title': 'Primary',
    'themes.forms.color_builder.secondary.title': 'Secondary',
    'themes.forms.color_builder.error.title': 'Error',
    'themes.forms.color_builder.warning.title': 'Warning',
    'themes.forms.color_builder.info.title': 'Info',
    'themes.forms.color_builder.success.title': 'Success',
    'themes.forms.color_builder.backgrounds.title': 'Backgrounds',
    'themes.forms.color_builder.text.title': 'Text',
    'themes.forms.color_builder.common.title': 'Common',
    'themes.forms.color_builder.borders.title': 'Borders & Dividers',
    'themes.forms.color_builder.fields.main.label': 'Main',
    'themes.forms.color_builder.fields.light.label': 'Light',
    'themes.forms.color_builder.fields.dark.label': 'Dark',
    'themes.forms.color_builder.fields.contrast_text.label': 'Contrast Text',
    'themes.forms.color_builder.fields.default.label': 'Default',
    'themes.forms.color_builder.fields.surface.label': 'Surface',
    'themes.forms.color_builder.fields.acrylic.label': 'Acrylic',
    'themes.forms.color_builder.fields.primary.label': 'Primary',
    'themes.forms.color_builder.fields.secondary.label': 'Secondary',
    'themes.forms.color_builder.fields.disabled.label': 'Disabled',
    'themes.forms.color_builder.fields.black.label': 'Black',
    'themes.forms.color_builder.fields.white.label': 'White',
    'themes.forms.color_builder.fields.background.label': 'Background',
    'themes.forms.color_builder.fields.on_background.label': 'On Background',
    'themes.forms.color_builder.fields.divider.label': 'Divider',
    'themes.forms.shape_builder.border_radius.title': 'Border Radius',
    'themes.forms.shape_builder.fields.radius.label': 'Radius',
    'themes.forms.shape_builder.border_style.title': 'Border Style',
    'themes.forms.shape_builder.fields.width.label': 'Width',
    'themes.forms.shape_builder.fields.width.options.none.label': 'None (0px)',
    'themes.forms.shape_builder.fields.width.options.thin.label': 'Thin (1px)',
    'themes.forms.shape_builder.fields.width.options.medium.label': 'Medium (2px)',
    'themes.forms.shape_builder.fields.width.options.thick.label': 'Thick (3px)',
    'themes.forms.shape_builder.fields.style.label': 'Style',
    'themes.forms.shape_builder.fields.style.options.solid.label': 'Solid',
    'themes.forms.shape_builder.fields.style.options.dashed.label': 'Dashed',
    'themes.forms.shape_builder.fields.style.options.dotted.label': 'Dotted',
    'themes.forms.shape_builder.fields.style.options.none.label': 'None',
    'themes.forms.typography_builder.font_family.title': 'Font Family',
    'themes.forms.typography_builder.fields.font_family.placeholder': 'e.g. Inter, Arial, sans-serif',
    'themes.forms.typography_builder.fields.font_family.helper_text': 'Choose a preset or type any CSS font stack',
    'themes.forms.typography_builder.fields.preview.label': 'Preview',
    'themes.forms.typography_builder.font_weights.title': 'Font Weights',
    'themes.forms.typography_builder.fields.light.label': 'Light',
    'themes.forms.typography_builder.fields.regular.label': 'Regular',
    'themes.forms.typography_builder.fields.medium.label': 'Medium',
    'themes.forms.typography_builder.fields.bold.label': 'Bold',
    'themes.forms.typography_builder.base_sizes.title': 'Base Sizes',
    'themes.forms.typography_builder.fields.base_font_size.label': 'Base Font Size',
    'themes.forms.typography_builder.fields.html_font_size.label': 'HTML Font Size',
    'themes.forms.typography_builder.type_scale.title': 'Type Scale',
    'themes.forms.typography_builder.fields.type_scale.placeholder': 'e.g. 1.5rem',
    'themes.forms.typography_builder.actions.reset.label': 'Reset',
    'themes.forms.general_builder.internationalization.title': 'Internationalization',
    'themes.forms.general_builder.fields.text_direction.label': 'Text direction',
    'themes.forms.general_builder.fields.text_direction.options.ltr.label': 'LTR',
    'themes.forms.general_builder.fields.text_direction.options.rtl.label': 'RTL',
    'themes.forms.settings.heading': 'Settings',
    'themes.forms.settings.fields.default_color_scheme.label': 'Default Color Scheme',
    'themes.forms.settings.fields.default_color_scheme.helper_text':
      'Select whether you want a light, dark or system color scheme as the default.',
    'themes.forms.settings.fields.default_text_direction.label': 'Default Text Direction',
    'themes.forms.settings.fields.default_text_direction.helper_text':
      'Select the default text direction for your theme. This will affect the layout and alignment of components.',
    'themes.forms.settings.fields.default_text_direction.options.ltr.label': 'Left-to-Right (LTR)',
    'themes.forms.settings.fields.default_text_direction.options.rtl.label': 'Right-to-Left (RTL)',
    'themes.delete.title': 'Delete Theme',
    'themes.delete.message': 'Are you sure you want to delete "{{name}}"? This action cannot be undone.',
    'themes.delete.messageUnnamed': 'Are you sure you want to delete this theme? This action cannot be undone.',
    'themes.delete.disclaimer': 'Deleting this theme may affect applications using it.',
    'themes.delete.error': 'Failed to delete theme. Please try again.',
    'themes.delete.usages.loading': 'Checking affected resources…',
    'themes.delete.usages.none': 'No applications are currently using this theme.',
    'themes.delete.usages.title': 'The following applications will revert to the default theme:',
    'themes.delete.usages.more': '+{{count}} more',
    'layouts.section.title': 'Layouts',
    'layouts.presets.centered.label': 'Centered',
    'layouts.presets.split_screen.label': 'Split Screen',
    'layouts.presets.full_screen.label': 'Full Screen',
    'layouts.presets.popup.label': 'Popup',
    'layouts.badges.coming_soon.label': 'Coming Soon',
    'layouts.config.select_layout.message': 'Select a layout to view constraints',
    'layouts.config.errors.load.message': 'Failed to load layout configuration.',
    'layouts.config.no_screen_selected.message': 'No screen selected.',
    'layouts.preview.labels.base_layout': 'Base layout',
    'layouts.preview.labels.screen_variants': 'Screen variants',
    'layouts.preview.labels.slots': 'Slots:',
    'layouts.preview.slots.content.label': 'Content',
    'layouts.preview.slots.logo.label': 'Logo',
    'layouts.preview.slots.lang_selector.label': 'Lang selector',
    'layouts.preview.slots.back_button.label': 'Back button',
    'layouts.preview.slots.header.label': 'Header',
    'layouts.preview.slots.main.label': 'Main',
    'layouts.preview.slots.footer.label': 'Footer',
    'layouts.preview.slots.links.label': 'Links',
    'layouts.preview.errors.load.message': 'Failed to load layout',
    'layouts.preview.select_layout.message': 'Select a layout to preview',
    'layouts.builder.actions.back_to_design.tooltip': 'Back to Design',
    'layouts.builder.actions.save.label': 'Save',
    'layouts.config.custom_css.title': 'Custom CSS',
    'layouts.config.custom_css.fields.url.label': 'URL',
    'layouts.config.custom_css.fields.url.errors.invalid_url': 'URL must be a valid http:// or https:// address',
    'layouts.config.custom_css.fields.url.warnings.insecure_protocol':
      'Using HTTP is insecure. Consider using HTTPS instead.',
    'layouts.config.custom_css.actions.open_full_editor.tooltip': 'Open in full editor',
    'layouts.config.custom_css.actions.show_in_preview.tooltip': 'Show in preview',
    'layouts.config.custom_css.actions.hide_from_preview.tooltip': 'Hide from preview',
    'layouts.config.custom_css.actions.remove.tooltip': 'Remove stylesheet',
    'layouts.config.custom_css.empty_state.message': 'No custom stylesheets yet.',
    'layouts.config.custom_css.empty_state.description':
      'Add an inline stylesheet or link an external CSS file to customize the appearance.',
    'layouts.config.custom_css.actions.add_inline.label': 'Inline',
    'layouts.config.custom_css.actions.add_url.label': 'External URL',
    'layouts.builder.screens.label': 'Screens',
    'layouts.builder.constraints.label': 'Constraints',
    'layouts.builder.screen_list.base_screen.description': 'base screen',
    'layouts.forms.add_screen.actions.add.label': 'Add screen',
    'layouts.forms.add_screen.fields.name.placeholder': 'Screen name\u2026',
    'layouts.forms.add_screen.actions.add_confirm.label': 'Add',
    'layouts.forms.add_screen.actions.cancel.label': 'Cancel',
    'layouts.forms.slot_editor.position.title': 'Position',
    'layouts.forms.slot_editor.fields.anchor.label': 'Anchor',
    'layouts.forms.slot_editor.fields.anchor.options.center.label': 'Center',
    'layouts.forms.slot_editor.fields.anchor.options.left.label': 'Left',
    'layouts.forms.slot_editor.fields.anchor.options.right.label': 'Right',
    'layouts.forms.slot_editor.fields.v_align.label': 'V-align',
    'layouts.forms.slot_editor.fields.v_align.options.top.label': 'Top',
    'layouts.forms.slot_editor.fields.v_align.options.middle.label': 'Middle',
    'layouts.forms.slot_editor.fields.v_align.options.bottom.label': 'Bottom',
    'layouts.forms.slot_editor.container.title': 'Container',
    'layouts.forms.slot_editor.fields.max_width.label': 'Max width',
    'layouts.forms.slot_editor.fields.border_radius.label': 'Border radius',
    'layouts.forms.slot_editor.fields.elevation.label': 'Elevation',
    'layouts.forms.slot_editor.fields.background.label': 'Background',
    'layouts.forms.slot_editor.fields.background.options.paper.label': 'Paper',
    'layouts.forms.slot_editor.fields.background.options.default.label': 'Default',
    'layouts.forms.slot_editor.fields.background.options.transparent.label': 'Transparent',
    'layouts.forms.slot_editor.layout.title': 'Layout',
    'layouts.forms.slot_editor.fields.type.label': 'Type',
    'layouts.forms.slot_editor.fields.type.options.stack.label': 'Stack',
    'layouts.forms.slot_editor.fields.type.options.grid.label': 'Grid',
    'layouts.forms.slot_editor.fields.direction.label': 'Direction',
    'layouts.forms.slot_editor.fields.direction.options.column.label': 'Column',
    'layouts.forms.slot_editor.fields.direction.options.row.label': 'Row',
    'layouts.forms.slot_editor.fields.gap.label': 'Gap',
    'layouts.forms.slot_editor.fields.justify.label': 'Justify',
    'layouts.forms.slot_editor.fields.justify.options.start.label': 'Start',
    'layouts.forms.slot_editor.fields.justify.options.center.label': 'Center',
    'layouts.forms.slot_editor.fields.justify.options.end.label': 'End',
    'layouts.forms.slot_editor.fields.justify.options.between.label': 'Between',
    'layouts.forms.slot_editor.fields.align.label': 'Align',
    'layouts.forms.slot_editor.fields.align.options.start.label': 'Start',
    'layouts.forms.slot_editor.fields.align.options.center.label': 'Center',
    'layouts.forms.slot_editor.fields.align.options.end.label': 'End',
    'layouts.forms.slot_editor.fields.align.options.stretch.label': 'Stretch',
    'layouts.forms.slot_editor.fields.height.label': 'Height',
    'layouts.forms.slot_editor.fields.padding.label': 'Padding',
    'layouts.forms.slot_editor.fields.show_logo.label': 'Show logo',
    'layouts.forms.slot_editor.fields.back_button.label': 'Back button',
    'layouts.forms.slot_editor.fields.language_selector.label': 'Language selector',
    'layouts.forms.slot_editor.fields.links.label': 'Links',
    'layouts.forms.screen_editor.background.title': 'Background',
    'layouts.forms.screen_editor.fields.type.label': 'Type',
    'layouts.forms.screen_editor.fields.type.options.solid.label': 'Solid',
    'layouts.forms.screen_editor.fields.type.options.gradient.label': 'Gradient',
    'layouts.forms.screen_editor.fields.type.options.image.label': 'Image',
    'layouts.forms.screen_editor.fields.type.options.none.label': 'None',
    'layouts.forms.screen_editor.fields.value.label': 'Value',
    'layouts.forms.screen_editor.spacing.title': 'Spacing',
    'layouts.forms.screen_editor.fields.component_gap.label': 'Component gap',
    'layouts.forms.screen_editor.fields.section_gap.label': 'Section gap',
    'layouts.forms.screen_editor.slots.title': 'Slots',
    'layouts.forms.screen_editor.no_overrides.message': 'No overrides \u2014 inherits from base screen',
    'common.color_scheme.options.light.label': 'Light',
    'common.color_scheme.options.dark.label': 'Dark',
    'common.color_scheme.options.system.label': 'System',
    'common.preview.toolbar.fields.color_scheme.label': 'Color Scheme',
    'common.preview.toolbar.viewports.mobile.label': 'Mobile (390px)',
    'common.preview.toolbar.viewports.tablet.label': 'Tablet (768px)',
    'common.preview.toolbar.viewports.desktop.label': 'Desktop (1440px)',
    'common.preview.toolbar.actions.zoom_out.tooltip': 'Zoom out',
    'common.preview.toolbar.actions.zoom_in.tooltip': 'Zoom in',
    'common.item_card.actions.open_in_builder.label': 'Open in builder',
    'common.section_header.badges.coming_soon.label': 'COMING SOON',
  },

  // ============================================================================
  // Home namespace - Getting Started / home page translations
  // ============================================================================
  home: {
    // Greeting header
    'greeting.hello': 'Hello,',
    'greeting.fallback_name': 'there',
    'greeting.subtitle': 'What do you want to secure today?',

    // Start Building section
    'start_building.hero.title': 'Integrate {{product}} into your application',
    'start_building.hero.description':
      'Add secure sign-in, token management, and user sessions to your app in minutes.',
    'start_building.hero.actions.create.label': 'Create Application',
    'start_building.frameworks.label': 'Start with a framework',

    // Next Steps section
    'next_steps.section.title': 'Quick Links',

    // Invite Members card
    'next_steps.invite_members.title': 'Invite Members',
    'next_steps.invite_members.description': 'Add collaborators to help manage your organization and act as a backup.',
    'next_steps.invite_members.actions.primary.label': 'Add User',
    'next_steps.invite_members.actions.secondary.label': 'Invite User',
    'next_steps.invite_members.status.count': '{{count}} member',
    'next_steps.invite_members.status.count_other': '{{count}} members',
    'next_steps.invite_members.status.empty': 'No members yet — add collaborators',

    // Login Box card
    'next_steps.login_box.title': 'Sign-in Box',
    'next_steps.login_box.description':
      'Build themes and attach them to your applications to personalise the sign-in experience.',
    'next_steps.login_box.actions.primary.label': 'Open Design Studio',

    // Connections card
    'next_steps.connections.title': 'Connections',
    'next_steps.connections.description':
      'Manage the external services {{product}} connects to for social login, enterprise OIDC, SMS delivery, and more.',
    'next_steps.connections.actions.primary.label': 'Manage Connections',

    // Multi-factor Authentication card
    'next_steps.mfa.title': 'Multi-factor Authentication',
    'next_steps.mfa.description': 'Protect users by enabling an additional verification factor to the sign-in process.',
    'next_steps.mfa.actions.primary.label': 'Configure Flows',

    // Start Building dynamic
    'start_building.hero.status.app_count': '{{count}} application',
    'start_building.hero.status.app_count_other': '{{count}} applications',
    'start_building.hero.actions.view_apps.label': 'Create Applications',

    // Feature status labels
    'feature_status.new': 'New',
    'feature_status.coming_soon': 'Coming Soon',
  },

  // ============================================================================
  // How Solution Works Illustration - Shared illustration translations
  // ============================================================================
  consumerAppIllustration: {
    step1Title: 'Get the Wayfinder Sample',
    step1Sub: 'Download the sample distribution',
    step2Title: 'Register Application',
    step2Line1: 'Set redirect URIs',
    step2Line2: 'and client credentials',
    step2Line3: 'in the console',
    step2Sub: 'Creates the app in the console',
    step3Title: 'Run the Sample',
    step3Sub: 'Linux / macOS or Windows',
  },
  aiAgentsIllustration: {
    consumers: 'Consumers',
    johnDoe: 'John Doe',
    janeSmith: 'Jane Smith',
    use: 'Use',
    wayfinderWeb: 'Wayfinder Web',
    wayfinderWebSub: 'Browser SPA with chat widget',
    wayfinderWebDetail: 'Book travel, chat with the agent',
    identityAuthority: 'Identity Authority',
    managesIdentities: 'Manages identities',
    issuesTokens: 'and issues tokens',
    aiAgent: 'AI Agent',
    aiAgentSub: 'Wayfinder Concierge',
    drivesConversation: 'Drives the conversation',
    wayfinderServer: 'Wayfinder Server',
    wayfinderServerSub: 'Booking API + MCP tools',
    holdsData: 'Holds flights, hotels, bookings',
    signIn: 'Sign in',
    issueUserToken: 'Issue user token',
    chat: 'Chat',
    authenticatedCalls: 'Authenticated calls',
    callMcpTools: 'Call MCP tools',
    getAgentTokens: 'Get agent tokens',
    issueAgentTokens: 'Issue agent / on-behalf-of tokens',
    validateTokens: 'Validate tokens',
  },
  mcpIllustration: {
    user: 'User',
    johnDoe: 'John Doe',
    use: 'Use',
    externalMcpClient: 'External MCP Client',
    mcpInspector: 'MCP Inspector',
    discoversSignsIn: 'Discovers, signs in, calls MCP tools',
    identityAuthority: 'Identity Authority',
    managesIdentities: 'Manages identities',
    issuesTokens: 'and issues tokens',
    wayfinderServer: 'Wayfinder Server',
    wayfinderServerSub: 'Booking API + MCP tools',
    holdsData: 'Holds flights, hotels, bookings',
    signIn: 'Sign in',
    issueTokens: 'Issue tokens',
    callMcpTools: 'Call MCP tools',
    validateTokens: 'Validate tokens',
  },
  howSolutionWorksIllustration: {
    validateTest: 'Validate / Test',
    configureProject: 'Configure Project',
    run: 'Run',
    console: '{{productName}} Console',
    runtimeLocal: '{{productName}} Runtime Local',
    projectEnvConfigs: 'Project + ENV Configs',
    runtimeHosted: '{{productName}} Runtime Hosted',
    saveExport: 'Save & Export',
    import: 'Attach',
    runInProduction: 'Run {{productName}} in Production',
    runtimeComponentsOnly: '(with required runtime components only)',
    designConfigure: 'Configure {{productName}}',
    designComponents: '(with design components)',
    commandProduction: './start.sh project-foo.yml --env production.env',
    commandStart: './start.sh',
    adminApp: 'Admin App',
    loginApp: 'Login App',
  },
  resourceServers: {
    'listing.title': 'Resource Servers',
    'listing.subtitle': 'Define resource servers and their resources to manage access control.',
    'listing.addResourceServer': 'Add resource server',
    'listing.columns.name': 'Name',
    'listing.columns.type': 'Type',
    'listing.columns.identifier': 'Identifier',
    'listing.columns.actions': 'Actions',
    'listing.systemResourceServer': 'System resource server',
    'listing.default': 'Default',
    'listing.error': 'Failed to load resource servers.',
    'actions.setAsDefault': 'Set as default',
    'setDefault.title': 'Set default resource server',
    'setDefault.message':
      'will become the default resource server. Requests without a resource parameter will fall back to it.',
    'setDefault.confirm': 'Set as default',
    'setDefault.setting': 'Setting…',
    'setDefault.success': '{{name}} is now the default resource server.',
    'setDefault.error': 'Failed to set the default resource server.',
    'delete.title': 'Delete resource server',
    'delete.message': 'Are you sure you want to delete this resource server? This action cannot be undone.',
    'delete.disclaimer':
      'Warning: All associated resources, actions, and permission strings will be permanently removed.',
    'create.steps.type': 'Type',
    'create.steps.name': 'Name',
    'create.steps.separator': 'Permission Delimiter',
    'create.steps.organizationUnit': 'Organization',
    'create.type.title': 'What type of resource server are you adding?',
    'create.type.subtitle': 'Select the type that best describes this resource server.',
    'create.type.api.title': 'API',
    'create.type.api.description': 'REST or HTTP APIs secured as an OAuth2 audience.',
    'create.type.mcp.title': 'MCP',
    'create.type.mcp.description': 'Model Context Protocol servers.',
    'create.type.custom.title': 'Custom',
    'create.type.custom.description': 'Any other protected resource - database, file store, or service.',
    'create.form.name.label': 'Name',
    'create.form.name.placeholder': 'Enter resource server name',
    'create.form.name.required': 'Name is required.',
    'create.form.identifier.label': 'Identifier',
    'create.form.identifier.placeholder': 'https://api.example.com',
    'create.form.identifier.required': 'Identifier is required.',
    'create.form.identifier.hint': 'A unique URI that identifies this resource server.',
    'create.form.handle.label': 'Handle',
    'create.form.handle.placeholder': 'my-resource-server',
    'create.form.handle.hint': 'A short identifier for this resource server.',
    'edit.settings.general.title': 'General',
    'edit.settings.general.name.label': 'Name',
    'edit.settings.general.identifier.label': 'Identifier',
    'edit.settings.general.handle.label': 'Handle',
    'edit.settings.general.description.label': 'Description',
    'edit.settings.general.description.placeholder': 'Enter a description',
    'edit.settings.general.organizationUnit.label': 'Organization Unit',
    'edit.settings.save': 'Save Changes',
    'edit.settings.saved': 'Settings saved successfully.',
    'edit.settings.error': 'Failed to save settings.',
    'edit.resources.title': 'Resources & Actions',
    'edit.resources.subtitle': 'Define the resources and actions for this resource server.',
    'edit.resources.addResource': 'Add Resource',
    'edit.resources.addAction': 'Add Action',
    'edit.resources.noResources': 'No resources defined yet.',
    'edit.resources.deleteResource.title': 'Delete Resource',
    'edit.resources.deleteResource.message': 'Are you sure? All actions under this resource will also be removed.',
    'edit.resources.deleteAction.title': 'Delete Action',
    'edit.resources.deleteAction.message': 'Are you sure you want to delete this action?',
    'edit.resources.form.name.label': 'Name',
    'edit.resources.form.name.placeholder': 'Enter resource name',
    'edit.resources.form.name.required': 'Name is required.',
    'edit.resources.form.handle.label': 'Handle',
    'edit.resources.form.handle.placeholder': 'resource-handle',
    'edit.resources.form.description.label': 'Description',
    'edit.resources.form.description.placeholder': 'Enter description',
    'edit.resources.detail.title': 'Resource Details',
    'edit.resources.detail.name': 'Name',
    'edit.resources.detail.handle': 'Handle',
    'edit.resources.detail.description': 'Description',
    'edit.resources.detail.actions': 'Actions',
    'edit.resources.detail.noActions': 'No actions defined.',
    'common.save': 'Save',
    'common.cancel': 'Cancel',
    'common.delete': 'Delete',
    'common.create': 'Create',
    'common.edit': 'Edit',
    'common.loading': 'Loading...',
    'common.error': 'An error occurred.',
    'common.confirm': 'Confirm',
    'common.back': 'Back',
    'common.next': 'Next',
    'create.name.title': 'Name your resource server',
    'create.name.titleMcp': 'Name your MCP server',
    'create.name.nameLabel': 'Resource Server Name',
    'create.name.nameLabelMcp': 'MCP Server Name',
    'create.name.namePlaceholder': 'e.g. Payments API',
    'create.name.suggestions': 'Need inspiration? Pick one:',
    'create.name.identifierLabel': 'Identifier',
    'create.name.identifierPlaceholder': 'https://api.example.com',
    'create.name.identifierPlaceholderMcp': 'https://mcp.example.com',
    'create.name.identifierHint':
      'A unique identifier for this resource server. When set as an absolute URI, it becomes the token audience for RFC 8707 resource indicators.',
    'create.name.identifierHintMcp':
      'A unique identifier for this MCP server. When set as an absolute URI, it becomes the token audience for RFC 8707 resource indicators.',
    'create.separator.title': 'Choose your permission delimiter',
    'create.separator.subtitle':
      'The delimiter character joins parts of a permission string. This cannot be changed after creation.',
    'create.separator.label': 'Permission Delimiter',
    'create.separator.hint': 'Choose the character that separates parts of a permission string.',
    'create.separator.invalid': 'Select a valid delimiter: . _ : - /',
    'create.separator.previewLabel': 'Example permission',
    'create.separator.colon': 'Colon ( : )',
    'create.separator.dot': 'Dot ( . )',
    'create.separator.slash': 'Slash ( / )',
    'create.separator.hyphen': 'Hyphen ( - )',
    'create.separator.underscore': 'Underscore ( _ )',
    'create.success': 'Resource server created successfully.',
    'create.successMcp': 'MCP server created successfully.',
    'create.creating': 'Creating…',
    'create.submit': 'Create resource server',
    'create.submitMcp': 'Create MCP server',
    'edit.tab.resources': 'Resources',
    'edit.tab.advanced': 'Advanced Settings',
    'edit.defaultBadge': 'Default resource server',
    'edit.defaultBadgeManaged': 'Managed by server configuration.',
    'edit.back': 'Back to resource servers',
    'edit.identifierRequired': 'Identifier is required.',
    'edit.notFound': 'Resource server not found.',
    'edit.systemResourceServer': 'System',
    'edit.tabs': 'Resource server settings',
    'edit.noDescription': 'No description',
    'edit.descriptionPlaceholder': 'Add a description',
    'edit.saveError': 'Failed to save changes.',
    'edit.unsavedChanges': 'You have unsaved changes.',
    'edit.advanced.identifier.title': 'Configurations',
    'edit.advanced.identifier.description': 'Configuration settings for this resource server.',
    'edit.advanced.identifier.descriptionMcp': 'Configuration settings for this MCP server.',
    'edit.advanced.identifier.label': 'Identifier (Audience)',
    'edit.advanced.identifier.hint':
      'A unique value that identifies this resource server. When set as an URI, enables RFC 8707 resource indicator support in OAuth2 authorization requests.',
    'edit.advanced.identifier.hintMcp':
      'A unique value that identifies this MCP server. When set as an URI, enables RFC 8707 resource indicator support in OAuth2 authorization requests.',
    'edit.advanced.identifier.placeholder': 'https://api.example.com',
    'edit.advanced.identifier.placeholderMcp': 'https://mcp.example.com',
    'edit.advanced.identifier.saved': 'Identifier saved.',
    'edit.advanced.identifier.saveError': 'Failed to save identifier.',
    'edit.dangerZone.title': 'Danger Zone',
    'edit.dangerZone.description': 'Irreversible actions for this resource server.',
    'edit.dangerZone.descriptionMcp': 'Irreversible actions for this MCP server.',
    'edit.dangerZone.deleteServer': 'Delete resource server',
    'edit.dangerZone.deleteServerMcp': 'Delete MCP server',
    'tree.title': 'Resource Hierarchy',
    'detail.identifierRequired': 'Identifier is required.',
    'tree.add': 'Add',
    'tree.addResource.title': 'Add resource',
    'tree.addServerAction': 'Add server-level action',
    'tree.empty': 'No resources yet — add a resource or action to get started.',
    'tree.addSubResource': 'Add sub-resource',
    'tree.addAction': 'Add action',
    'tree.fields.handle': 'Handle',
    'tree.fields.handleDelimiterError': 'Handle cannot contain the delimiter character {{delimiter}}.',
    'tree.fields.handleHint':
      'Lowercase, alphanumeric, and {{allowedChars}} characters. Cannot be changed after creation.',
    'tree.fields.name': 'Name',
    'tree.deleteResource.success': 'Resource deleted.',
    'tree.deleteResource.error': 'Cannot delete — remove child resources and actions first.',
    'tree.deleteAction.success': 'Action deleted.',
    'tree.deleteAction.error': 'Failed to delete action.',
    'tree.copyPermission': 'Copy permission string',
    // Permission catalog
    'permissionCatalog.scopes.placeholder': 'No permissions selected',
    'permissionCatalog.scopes.copy': 'Copy scopes',
    'permissionCatalog.scopes.copied': 'Copied',
    'permissionCatalog.noResourceServers': 'No resource servers found. Create a resource server first.',
    'permissionCatalog.noPermissions': 'No permissions defined for this resource server.',
    'permissionCatalog.loadError': 'Failed to load permissions for this resource server.',
    'permissionCatalog.loadServersError': 'Failed to load resource servers.',
    'permissionCatalog.serverNotFound': 'Resource server not found',
  },

  // ============================================================================
  // Verifiable Presentations namespace - OpenID4VP presentation definitions
  // ============================================================================
  'verifiable-presentations': {
    // List page
    'listing.title': 'Presentation Definitions',
    'listing.subtitle': 'Define which verifiable credentials your verifier requests from wallets via OpenID4VP.',
    'listing.add': 'Add Definition',
    'listing.error': 'Failed to load presentation definitions',
    'listing.columns.name': 'Name',
    'listing.columns.organizationUnit': 'Organization Unit',
    'listing.columns.actions': 'Actions',
    'listing.verify': 'Verify',

    // Verification dialog
    'verify.title': 'Verify Presentation',
    'verify.scanHint':
      'Scan the QR code with your wallet or tap the button to open on this device, then approve the request to share your credential.',
    'verify.openInWallet': 'Open in wallet',
    'verify.copy': 'Copy request link',
    'verify.notConfigured':
      'Presentation verification is not enabled. Configure a verifier signing key to start verification requests.',
    'verify.waiting': 'Waiting for the wallet to respond…',
    'verify.completed': 'Verification complete',
    'verify.failed': 'Verification failed',
    'verify.expired': 'Verification request expired',
    'verify.claimsTitle': 'Verified claims',
    'verify.keyBindingVerified': 'Holder key binding verified',

    // Flow step selector
    'select.label': 'Presentation definition',
    'select.placeholder': 'Select a presentation definition',

    // Form - General tab
    'form.tabs.general': 'General',
    'form.tabs.protocolSettings': 'Settings',
    'form.tabs.claims': 'Claims',
    'form.tabs.issuerTrust': 'Issuer Trust',
    'form.quickCopy.title': 'Quick Copy',
    'form.quickCopy.description': "Copy the definition's identifiers for use in flows and API calls.",
    'form.quickCopy.idHint': 'Use this ID to reference the presentation definition in API calls.',
    'form.id.label': 'Definition ID',
    'form.copyId': 'Copy definition ID',
    'form.handle.label': 'Handle',
    'form.handle.hint': 'The handle identifies this presentation definition in flow steps and DCQL queries.',
    'form.name.label': 'Name',
    'form.name.placeholder': 'EUDI Wallet PID',
    'form.name.hint': 'A memorable label for this presentation definition, shown only in the console.',
    'form.organizationUnit.title': 'Organization Unit',
    'form.organizationUnit.description': 'The organization unit this presentation definition belongs to.',
    'form.organizationUnit.label': 'Organization Unit',
    'form.organizationUnit.pickerHint': 'Select the organization unit this presentation definition will belong to.',
    'form.organizationUnit.handleLabel': 'Handle',
    'form.organizationUnit.handleHint': 'Use this handle to reference the organization unit in configuration files.',
    'form.organizationUnit.idLabel': 'ID',
    'form.organizationUnit.idHint': 'Use this ID to reference the organization unit in API calls.',
    'form.protocol.title': 'Settings',
    'form.protocol.description': 'The credential type and format this presentation definition requests.',
    'form.vct.label': 'Credential Type (VCT)',
    'form.vct.hint': 'The credential type (vct) wallets must present to satisfy this request.',
    'form.format.label': 'Format',
    'form.format.sdJwt': 'SD-JWT VC (dc+sd-jwt)',
    'form.format.hint': 'Only SD-JWT VC is currently supported.',
    'form.dangerZone.title': 'Danger Zone',
    'form.dangerZone.description': 'Irreversible actions for this presentation definition.',
    'form.dangerZone.delete': 'Delete Presentation Definition',
    'form.dangerZone.deleteDescription':
      'Login flows referencing this presentation definition will stop working, and wallets will no longer be able to complete this verification request. Verifications already completed are not affected.',
    'form.unsavedChanges': 'You have unsaved changes',

    // Form - Issuer Trust tab
    'form.issuerTrust.title': 'Issuer Trust',
    'form.issuerTrust.description': 'Control which credential issuers are accepted for this presentation definition.',
    'form.issuerTrust.enforce.label': 'Enforce trusted issuer',
    'form.issuerTrust.enforce.hint': 'When enabled, only credentials issued by a trusted issuer are accepted.',
    'form.issuerTrust.enforce.noAnchorsHint':
      'No trust anchors are configured, so issuer trust cannot be enforced. Configure trust anchors first.',
    'form.issuerTrust.authorities.label': 'Trusted Issuers',
    'form.issuerTrust.authorities.hint': 'Leave empty to accept any registered trust anchor.',
    'form.issuerTrust.authorities.optionSecondary': '{{subject}} · expires {{notAfter}}',

    // Claims editor
    'claims.empty': 'No claims yet. Add the claims this definition should request from the wallet.',
    'claims.remove': 'Remove Claim',
    'claims.name': 'Claim',
    'claims.nameHint':
      'The claim path to request from the wallet, and whether it must be disclosed (Mandatory) or may be withheld (Optional).',
    'claims.requirement': 'Requirement',
    'claims.mandatory': 'Mandatory',
    'claims.optional': 'Optional',
    'claims.values': 'Allowed Values',
    'claims.valuesPlaceholder': 'Leave empty to allow any value',
    'claims.valuesHint':
      'If set, the disclosed value must match one of these (compared as text). Enforced at verification.',
    'claims.add': 'Add Claim',

    // Create wizard
    'createWizard.steps.name': 'Name',
    'createWizard.name.suggestions.label': 'In a hurry? Pick a random name:',
    'create.title': 'New Presentation Definition',
    'create.subtitle': 'Configure the credential type and the claims to request from the wallet.',
    'create.steps.details': 'Details',
    'create.steps.claims': 'Claims',
    'create.claims.help': 'Add each claim once and set whether it is mandatory and restrict its allowed values.',
    'create.success': 'Presentation definition created',
    'create.error': 'Failed to create presentation definition',

    // Edit page
    'edit.back': 'Back to Presentation Definitions',
    'edit.loadError': 'Failed to load presentation definition',
    'edit.notFound': 'Presentation definition not found',
    'edit.name.ariaLabel': 'Presentation definition name',
    'edit.name.editButton': 'Edit presentation definition name',
    'edit.description.ariaLabel': 'Presentation definition description',
    'edit.description.editButton': 'Edit presentation definition description',
    'edit.description.placeholder': 'Add a description...',
    'edit.description.empty': 'No description',
    'update.success': 'Presentation definition updated',
    'update.error': 'Failed to update presentation definition',

    // Delete
    'delete.title': 'Delete presentation definition',
    'delete.message': 'Are you sure you want to delete this presentation definition?',
    'delete.disclaimer': 'Login flows referencing this definition will stop working.',
    'delete.success': 'Presentation definition deleted',
    'delete.error': 'Failed to delete presentation definition',
  },
  'verifiable-credentials': {
    // List page
    'listing.title': 'Credential Templates',
    'listing.subtitle': 'Define the credentials your issuer can issue to wallets via OpenID4VCI.',
    'listing.add': 'Add Template',
    'listing.error': 'Failed to load credential templates',
    'listing.offer': 'Generate offer',
    'listing.columns.name': 'Name',
    'listing.columns.organizationUnit': 'Organization Unit',
    'listing.columns.actions': 'Actions',

    // Create page
    'createWizard.steps.name': 'Name',
    'createWizard.name.suggestions.label': 'In a hurry? Pick a random name:',
    'create.title': 'New Credential Template',
    'create.subtitle': 'Define the credential type, claims and display shown in wallets.',
    'create.steps.details': 'Details',
    'create.steps.claims': 'Claims',
    'create.claims.help':
      'Add each claim once and set the attribute name as it appears in the user profile and how it should be displayed in the wallet.',
    'create.success': 'Credential template created',
    'create.error': 'Failed to create credential template',

    // Form
    'form.tabs.general': 'General',
    'form.tabs.protocolSettings': 'Settings',
    'form.tabs.claims': 'Claims',
    'form.quickCopy.title': 'Quick Copy',
    'form.quickCopy.description': "Copy the credential template's identifiers for use in flows and API calls.",
    'form.quickCopy.idHint': 'Use this ID to reference the credential template in API calls.',
    'form.id.label': 'Template ID',
    'form.copyId': 'Copy template ID',
    'form.handle.label': 'Handle',
    'form.handle.hint': 'The handle is the credential identifier and OAuth scope wallets request.',
    'form.name.label': 'Name',
    'form.name.placeholder': 'EUDI Wallet PID',
    'form.name.hint': 'A memorable label for this credential template, shown only in the console.',
    'form.organizationUnit.title': 'Organization Unit',
    'form.organizationUnit.description': 'The organization unit this credential template belongs to.',
    'form.organizationUnit.label': 'Organization Unit',
    'form.organizationUnit.pickerHint': 'Select the organization unit this credential template will belong to.',
    'form.organizationUnit.handleLabel': 'Handle',
    'form.organizationUnit.handleHint': 'Use this handle to reference the organization unit in configuration files.',
    'form.organizationUnit.idLabel': 'ID',
    'form.organizationUnit.idHint': 'Use this ID to reference the organization unit in API calls.',
    'form.protocol.title': 'Settings',
    'form.protocol.description': 'The OpenID4VCI credential type, format, and wallet display for this credential.',
    'form.vct.label': 'Credential Type (VCT)',
    'form.vct.hint': 'The SD-JWT VC credential type wallets and verifiers use to identify this credential.',
    'form.format.label': 'Format',
    'form.format.sdJwt': 'SD-JWT VC (dc+sd-jwt)',
    'form.format.hint': 'Only SD-JWT VC is currently supported.',
    'form.display.locale': 'Locale',
    'form.display.localeHint': 'The BCP 47 language tag for the credential name shown in the wallet, e.g. en-US.',
    'form.display.logo': 'Logo URI',
    'form.display.logoHint': "A hosted image URL shown as the credential's logo in the wallet.",
    'form.dangerZone.title': 'Danger Zone',
    'form.dangerZone.description': 'Irreversible actions for this credential template.',
    'form.dangerZone.delete': 'Delete Credential Template',
    'form.dangerZone.deleteDescription':
      'Wallets will no longer be able to request this credential. Credentials already issued are not revoked and remain valid until they expire.',
    'form.unsavedChanges': 'You have unsaved changes',

    // Claims editor
    'claims.empty': 'No claims yet. Add the attributes this credential should disclose.',
    'claims.add': 'Add Claim',
    'claims.remove': 'Remove Claim',
    'claims.name': 'Attribute Name',
    'claims.displayName': 'Display Name',
    'claims.nameHint': 'Must match a user profile attribute name; the value is sourced from the user.',

    // Offer dialog
    'offer.title': 'Credential Offer',
    'offer.openInWallet': 'Open in wallet',
    'offer.copy': 'Copy offer link',
    'offer.notConfigured':
      'Credential issuance is not enabled. Configure an issuer signing key to generate credential offers.',

    // Edit page
    'edit.back': 'Back to Credential Templates',
    'edit.loadError': 'Failed to load credential template',
    'edit.notFound': 'Credential template not found',
    'edit.name.ariaLabel': 'Credential template name',
    'edit.name.editButton': 'Edit credential template name',
    'edit.description.ariaLabel': 'Credential template description',
    'edit.description.editButton': 'Edit credential template description',
    'edit.description.placeholder': 'Add a description...',
    'edit.description.empty': 'No description',
    'update.success': 'Credential template updated',
    'update.error': 'Failed to update credential template',

    // Delete
    'delete.title': 'Delete credential template',
    'delete.message': 'Are you sure you want to delete this credential template?',
    'delete.disclaimer': 'Wallets will no longer be able to request this credential.',
    'delete.success': 'Credential template deleted',
    'delete.error': 'Failed to delete credential template',
  },

  // ============================================================================
  // Settings namespace - Server-wide settings translations
  // ============================================================================
  settings: {
    'page.title': 'Settings',
    'page.subtitle': 'Settings that apply across your entire ThunderID deployment.',
    'tabs.ariaLabel': 'Settings sections',
    'tabs.cors': 'CORS',
    'cors.card.title': 'Allowed origins',
    'cors.card.description': 'Manage which origins are allowed to access your APIs.',
    'cors.readOnlyHint': "Some origins are read-only because they're managed declaratively.",
    'cors.addOrigin': 'Add origin',
    'cors.originPlaceholder': 'https://app.example.com',
    'cors.removeOrigin': 'Remove origin',
    'cors.validation.invalid': 'Enter a valid origin (e.g. https://app.example.com) or a valid regular expression.',
    'cors.validation.duplicate': 'This origin is already in the list.',
    'cors.unsavedChanges': 'You have unsaved changes',
    'cors.discard': 'Discard',
    'cors.save': 'Save changes',
    'cors.saving': 'Saving…',
    'cors.load.error': 'Failed to load allowed origins.',
    'cors.save.success': 'Allowed origins updated.',
    'cors.save.error': 'Failed to update allowed origins.',
  },
} as const;

export default translations;
