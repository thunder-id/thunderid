// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * English (US) translations for applications
 * All namespaces organized in a single file for better maintainability
 */
const translations = {
  // ============================================================================
  // Common namespace - Shared translations across all applications
  // ============================================================================
  common: {
    // Account status
    'accountStatus.active.label': 'Active',
    'accountStatus.active.hint.user': 'This account can sign in normally.',
    'accountStatus.active.hint.agent': 'This agent can authenticate normally.',
    'accountStatus.active.hint.application': 'This application can authenticate normally.',
    'accountStatus.locked.label': 'Locked',
    'accountStatus.locked.hint.user':
      'The account is locked and cannot sign in. The user can complete account recovery, or an administrator can unlock the account.',
    'accountStatus.locked.hint.agent': 'The agent is locked and cannot authenticate. An administrator can unlock it.',
    'accountStatus.locked.hint.failedAttempts.user':
      'The account was locked after too many failed sign-in attempts. It unlocks automatically at the time shown below. Account recovery or an administrator unlock also removes the lock.',
    'accountStatus.locked.hint.failedAttempts.agent':
      'The agent was locked after too many failed authentication attempts. It unlocks automatically at the time shown below. An administrator unlock also removes the lock.',
    'accountStatus.locked.hint.failedAttemptsPermanent.user':
      'The account was locked after too many failed sign-in attempts. It stays locked until the user completes account recovery or an administrator unlocks it.',
    'accountStatus.locked.hint.failedAttemptsPermanent.agent':
      'The agent was locked after too many failed authentication attempts. It stays locked until an administrator unlocks it.',
    'accountStatus.locked.hint.suspensionReleased.user':
      'Suspension removed. The account remains locked until the user completes account recovery or an administrator unlocks it.',
    'accountStatus.locked.title.someMethods.user': 'Sign-in locked for some methods',
    'accountStatus.locked.title.someMethods.agent': 'Authentication locked for some methods',
    'accountStatus.locked.hint.someMethods.user':
      'Some sign-in methods were locked after too many failed attempts. Other sign-in methods are not affected. Each lock below shows when it ends. Account recovery or an administrator unlock removes them.',
    'accountStatus.locked.hint.someMethods.agent':
      'Some authentication methods were locked after too many failed attempts. Other authentication methods are not affected. Each lock below shows when it ends. An administrator unlock removes them.',
    'accountStatus.locked.until': 'Locked until {{expiresAt}}',
    'accountStatus.locked.method.until': '{{method}}: locked until {{expiresAt}}',
    'accountStatus.locked.method.untilReleased': '{{method}}: locked until released',
    'accountStatus.method.credential': 'Password or other stored credential',
    'accountStatus.method.otp': 'One-time password',
    'accountStatus.suspended.label': 'Suspended',
    'accountStatus.suspended.hint.user':
      'The account is suspended and cannot sign in until an administrator unsuspends it.',
    'accountStatus.suspended.hint.agent':
      'The agent is suspended and cannot authenticate until an administrator unsuspends it.',
    'accountStatus.suspended.hint.application':
      'The application is suspended and cannot authenticate until an administrator unsuspends it.',
    'accountStatus.suspended.operatorNote': 'Operator note: {{operatorNote}}',

    // Account lifecycle actions
    'accountStatus.actions': 'Actions',
    'accountStatus.error.user': 'The user operation could not be completed.',
    'accountStatus.error.agent': 'The agent operation could not be completed.',
    'accountStatus.error.application': 'The application operation could not be completed.',
    'accountStatus.availabilityError': 'The account actions could not be loaded. Reload the page to try again.',
    'accountStatus.suspend.action': 'Suspend',
    'accountStatus.suspend.hint.user': 'Prevent sign-in and revoke existing access.',
    'accountStatus.suspend.hint.agent': 'Prevent authentication and revoke existing access.',
    'accountStatus.suspend.hint.application': 'Prevent authentication and revoke existing access.',
    'accountStatus.suspend.confirmTitle.user': 'Suspend user',
    'accountStatus.suspend.confirmTitle.agent': 'Suspend agent',
    'accountStatus.suspend.confirmTitle.application': 'Suspend application',
    'accountStatus.suspend.confirmBody.user':
      'The account will no longer be able to sign in. Existing sessions and tokens will be revoked.',
    'accountStatus.suspend.confirmBody.agent':
      'The agent will no longer be able to authenticate or obtain new tokens. Existing sessions and tokens will be revoked. You can still update the agent details and credentials while it is suspended.',
    'accountStatus.suspend.confirmBody.application':
      'The application will no longer be able to authenticate or obtain new tokens. Existing sessions and tokens will be revoked.',
    'accountStatus.suspend.operatorNoteLabel': 'Operator note (optional)',
    'accountStatus.suspend.operatorNoteHelp':
      'Shown to anyone who can read this record. It is never shown on a sign-in attempt.',
    'accountStatus.suspend.success.user': 'User suspended',
    'accountStatus.suspend.success.agent': 'Agent suspended',
    'accountStatus.suspend.success.application': 'Application suspended',
    'accountStatus.suspend.emailNotSent.user':
      'User suspended, but no notification email was sent. Check that the user has an email address and an email provider is configured.',
    'accountStatus.unlock.action': 'Unlock',
    'accountStatus.unlock.hint.user': 'Remove all sign-in locks.',
    'accountStatus.unlock.hint.agent': 'Remove all authentication locks.',
    'accountStatus.unlock.success.user': 'User unlocked',
    'accountStatus.unlock.success.agent': 'Agent unlocked',
    'accountStatus.unlock.emailNotSent.user':
      'User unlocked, but no notification email was sent. Check that the user has an email address and an email provider is configured.',
    'accountStatus.unsuspend.action': 'Unsuspend',
    'accountStatus.unsuspend.hint.user':
      'Remove the suspension. The account remains locked until the user completes account recovery or an administrator unlocks it.',
    'accountStatus.unsuspend.hint.agent': 'Remove the suspension.',
    'accountStatus.unsuspend.hint.application': 'Remove the suspension.',
    'accountStatus.unsuspend.confirmTitle.user': 'Unsuspend user',
    'accountStatus.unsuspend.confirmTitle.agent': 'Unsuspend agent',
    'accountStatus.unsuspend.confirmTitle.application': 'Unsuspend application',
    'accountStatus.unsuspend.confirmGuidance.user':
      "Before unsuspending, verify the user's contact information and reset any potentially compromised credentials.",
    'accountStatus.unsuspend.confirmGuidance.agent':
      'Before unsuspending, rotate any potentially compromised agent secrets to prevent their reuse.',
    'accountStatus.unsuspend.confirmGuidance.application':
      'Before unsuspending, rotate any potentially compromised application secrets to prevent their reuse.',
    'accountStatus.unsuspend.confirmBody.user':
      "The user's account will remain locked after unsuspension. To regain access, the user must complete account recovery or an administrator must unlock the account.",
    'accountStatus.unsuspend.confirmBody.agent':
      "This action will remove the suspension and restore the agent's access, unless the agent is also locked.",
    'accountStatus.unsuspend.confirmBody.application':
      "This action will remove the suspension and restore the application's access.",
    'accountStatus.unsuspend.success.user': 'User unsuspended',
    'accountStatus.unsuspend.success.agent': 'Agent unsuspended',
    'accountStatus.unsuspend.success.application': 'Application unsuspended',
    'accountStatus.unsuspend.emailNotSent.user':
      'User unsuspended, but no notification email was sent. Check that the user has an email address and an email provider is configured.',
    'accountStatus.unsuspendAndUnlock.action': 'Unsuspend and unlock',
    'accountStatus.unsuspendAndUnlock.hint.user':
      'Remove the suspension and unlock. The account can sign in right away.',
    'accountStatus.unsuspendAndUnlock.confirmTitle': 'Unsuspend and unlock user',
    'accountStatus.unsuspendAndUnlock.confirmGuidance':
      "Before restoring access, verify the user's contact information and reset any potentially compromised credentials.",
    'accountStatus.unsuspendAndUnlock.confirmBody':
      'This action will remove the suspension and unlock the account, allowing the user to sign in again.',
    'accountStatus.unsuspendAndUnlock.success': 'User unsuspended and unlocked',
    'accountStatus.unsuspendAndUnlock.emailNotSent':
      'User unsuspended and unlocked, but a notification email was not sent. Check that the user has an email address and an email provider is configured.',
    'accountStatus.unsuspendAndUnlock.incomplete':
      'The suspension was removed, but the unlock failed. Unlock it from the Actions menu.',

    // Actions
    'actions.add': 'Add',
    'actions.edit': 'Edit',
    'actions.change': 'Change',
    'actions.delete': 'Delete',
    'actions.cancel': 'Cancel',
    'actions.signOut': 'Sign out',
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
    'actions.stay': 'Stay',
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
    'actions.login': 'Login',
    'actions.collapse': 'Collapse',
    'actions.expand': 'Expand',

    // Dictionary
    'dictionary.unknown': 'Unknown',

    // Short action words (used as button labels, etc.)
    show: 'Show',
    publish: 'Publish',
    saveDraft: 'Save draft',
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
    'status.sending': 'Sending...',
    'status.creating': 'Creating...',
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
    'status.readOnly': 'Read only',
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

    // Backend error code translations shared across services (per the shared service error envelope).
    'errors.SSE-4030': 'You do not have permission to perform this action in this organization unit.',
    'errors.DCR-1002': 'This resource is managed declaratively and cannot be modified.',
    'errors.DCR-1003': 'This resource is managed declaratively and cannot be deleted.',

    // Backend error code translations for flow executor failures (per the flow execution error envelope).
    'errors.FET-1002': 'Could not check whether a matching {{entity}} already exists. Please try again.',
    'errors.FET-1003': 'The provided details match more than one existing {{entity}}.',
    'errors.FET-1006': 'The {{entity}} was created but could not be signed in.',
    'errors.FET-1007': 'Another {{entity}} with these details already exists.',
    'errors.FET-1020': 'Provide at least one {{entity}} attribute to continue.',
    'errors.FET-1021': 'Something went wrong while creating the {{entity}}. Please try again.',
    'errors.FET-1022': 'The {{entity}} was created but group or role assignment failed.',
    'errors.FET-1023': 'Select an organization unit before continuing.',
    'errors.FET-1024': 'Another {{entity}} with these details already exists in the selected organization unit.',
    'errors.FET-1061': 'Another {{entity}} already exists with the provided {{attribute}}.',
    'errors.FET-1080': 'Another {{entity}} with the same unique attribute value already exists.',
    'errors.FET-1084': 'This user cannot be deleted in their current state.',
    'errors.FET-1085': 'The user could not be deleted. Please try again.',
    'errors.FET-1086': 'This application cannot be deleted in its current state.',
    'errors.FET-1087': 'The application could not be deleted. Please try again.',
    'errors.FET-1088': 'This application authenticates without a client secret, so there is none to regenerate.',
    'errors.FET-1089': 'The client secret could not be regenerated. Please try again.',
    'errors.FET-1098': 'This role assignment cannot be removed in its current state.',
    'errors.FET-1099': 'The role assignment could not be removed. Please try again.',
    'errors.FET-1100': 'This role cannot be deleted in its current state.',
    'errors.FET-1101': 'The role could not be deleted. Please try again.',
    'errors.FET-1102': 'The permissions of this role cannot be changed in its current state.',
    'errors.FET-1103': 'The permissions of this role could not be changed. Please try again.',
    'errors.FET-1104': 'This group cannot be deleted in its current state.',
    'errors.FET-1105': 'The group could not be deleted. Please try again.',
    'errors.FET-1106': 'This member cannot be removed from the group in its current state.',
    'errors.FET-1107': 'The member could not be removed from the group. Please try again.',
    'errors.FET-1108': 'This action cannot be deleted in its current state.',
    'errors.FET-1109': 'The action could not be deleted. Please try again.',
    'errors.FET-1110': 'The permissions supplied for this role are not valid. Refresh the page and try again.',
    'errors.FET-1111': 'A required value for this change is missing. Refresh the page and try again.',
    'errors.FET-1112':
      'The account operation could not be completed. Check that the account still exists and that the operation applies to its current state.',
    'errors.RVS-1001':
      'This change would revoke access through {{entries}} entries, more than the {{max}} allowed for one change.',

    // Shared, since role changes expand a group's members and raise it too.
    'errors.GRP-1017':
      "This group's members are nested more than {{max}} levels deep, so the change cannot be applied.",

    // External links
    learnMore: 'Learn more',
    'externalLink.title': 'You are leaving {{productName}}',
    'externalLink.message': 'This link leads to an external site. Please verify the destination before proceeding:',

    // Content Security Policy hints shown next to fields that load external resources.
    'csp.hint.image':
      "This image loads from {{origin}}. Add {{origin}} to the img-src directive of your server's Content Security Policy so it is not blocked.",
    'csp.hint.stylesheet':
      "This stylesheet loads from {{origin}}. Add {{origin}} to the style-src-elem directive of your server's Content Security Policy so it is not blocked.",
    'csp.hint.font':
      "This font loads from {{origin}}. Add {{origin}} to the style-src-elem directive, and the origin that serves its font files to the font-src directive, of your server's Content Security Policy so it is not blocked.",
    'csp.hint.fontGoogle':
      "Google Fonts loads its stylesheet from fonts.googleapis.com and its font files from fonts.gstatic.com. Add fonts.googleapis.com to the style-src-elem directive and fonts.gstatic.com to the font-src directive of your server's Content Security Policy so this font is not blocked.",

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
    'welcome.sections.tryoutProduct': 'Try sample',
    'welcome.tryoutProduct.securingApplication': 'Secured web application',
    'welcome.tryoutProduct.securingApplicationDesc': 'Tryout user journeys of a secured web application',
    'welcome.tryoutProduct.aiAgents': 'Secured AI agent',
    'welcome.tryoutProduct.aiAgentsDesc': 'Tryout identity security patterns for AI agents and tools',
    'welcome.tryoutProduct.mcp': 'Secured MCP server',
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
    'welcome.walkthrough.learnFundamentals': 'Learn the fundamentals',
    'welcome.walkthrough.learnFundamentalsDesc': 'Understand core concepts and architecture',
    'welcome.createProject.breadcrumb': 'New',
    'welcome.createProject.title': 'How {{productName}} works!',
    'welcome.createProject.subtitle':
      'Onboard your digital resources, configure security, verify the setup, and run {{productName}} with your configuration in production deployment.',
    'welcome.createProject.cards.configure.title': 'Configure',
    'welcome.createProject.cards.configure.description':
      'Set up authentication flows, choose sign-in methods, and customize the user experience.',
    'welcome.createProject.cards.verify.title': 'Verify',
    'welcome.createProject.cards.verify.description': 'Test your configuration to ensure everything works as expected.',
    'welcome.createProject.cards.runServer.title': 'Run server',
    'welcome.createProject.cards.runServer.description':
      '{{productName}} will run in immutable mode with the attached configurations.',
    'welcome.createProject.actions.getStarted': 'Get started',
    'welcome.wayfinderSampleSetup.title': 'Setup Wayfinder sample',
    'welcome.wayfinderSampleSetup.oneTimeSetup': 'One-time setup',
    'welcome.wayfinderSampleSetup.setupComplete': 'Already set up — you can skip this section',
    'welcome.wayfinderSampleSetup.steps.getSample.title': 'Get the Wayfinder sample',
    'welcome.wayfinderSampleSetup.steps.getSample.description':
      'Download the latest Wayfinder sample distribution and extract the archive. It ships with the <strong>web frontend</strong>, <strong>AI agent service</strong>, <strong>MCP</strong>, <strong>SMTP mail server</strong> and <strong>Wayfinder server</strong>.',
    'welcome.wayfinderSampleSetup.steps.configure.title': 'Configure Wayfinder sample in {{productName}}',
    'welcome.wayfinderSampleSetup.steps.configure.description':
      'Apply the Wayfinder sample configurations to {{productName}}. Skip this step if already done.',
    'welcome.wayfinderSampleSetup.steps.run.title': 'Run the sample',
    'welcome.wayfinderSampleSetup.steps.run.description':
      'Start all Wayfinder services from the extracted sample directory.',
    'welcome.wayfinderSampleDownload.downloadButton': 'Download',

    'welcome.wayfinderFolderImport.actions.selectFolder': 'Select Wayfinder sample folder',
    'welcome.wayfinderFolderImport.actions.change': 'Change',
    'welcome.wayfinderFolderImport.actions.importConfig': 'Configure in {{productName}}',
    'welcome.wayfinderFolderImport.actions.reconfigure': 'Reconfigure',
    'welcome.wayfinderFolderImport.actions.reSelectFolder': 'Re-select folder',
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

    'welcome.tryout.breadcrumb': 'Try it out',
    'welcome.tryout.title': 'Try it out',
    'welcome.tryout.actions.readDocs': 'Read documentation for more guides and details',
    'welcome.tryout.importConfigs.title': 'Configure Wayfinder sample in {{productName}}',
    'welcome.tryout.importConfigs.description': 'Apply the Wayfinder sample configurations to {{productName}}.',

    'welcome.getStarted.breadcrumb': 'Get started',
    'welcome.getStarted.actions.skipToConsole': 'Skip to console',
    'welcome.getStarted.title': 'Get started',
    'welcome.getStarted.subtitle': 'Onboard your first resource and start securing it with {{productName}}.',
    'welcome.getStarted.options.onboardApp.title': 'Web application',
    'welcome.getStarted.options.onboardApp.description':
      'Register your application in {{productName}} and integrate authentication step by step.',
    'welcome.getStarted.options.onboardApp.action': 'Add application',
    'welcome.getStarted.options.secureAiAgent.title': 'Secure an AI agent',
    'welcome.getStarted.options.secureAiAgent.description':
      'Protect your AI agents with token-based access control and scope enforcement.',
    'welcome.getStarted.options.secureAiAgent.action': 'Add agent',
    'welcome.getStarted.options.secureMcp.title': 'Secure an MCP client',
    'welcome.getStarted.options.secureMcp.description':
      'Register your MCP client and authorize it with fine-grained, scoped permissions.',
    'welcome.getStarted.options.secureMcp.action': 'Add MCP client',
    'welcome.getStarted.options.comingSoon': 'Coming soon',
    'welcome.getStarted.options.skip.title': 'Skip for now',
    'welcome.getStarted.options.skip.description':
      'Head straight to the console and explore {{productName}} on your own.',
    'welcome.getStarted.options.skip.action': 'Go to Console',

    'welcome.applicationTryout.breadcrumb': 'Tryout secured web application',
    'welcome.applicationTryout.overline': 'Secured web application',
    'welcome.applicationTryout.title': 'Secure your application',
    'welcome.applicationTryout.subtitle':
      'Run web application use cases against the Wayfinder sample, a fictional travel-booking application.',
    'welcome.applicationTryout.steps.getSample.title': 'Get the Wayfinder sample',
    'welcome.applicationTryout.steps.getSample.description':
      'Download the latest Wayfinder sample distribution and extract the archive. It ships with the web frontend, AI agent service, MCP, SMTP mail server and Wayfinder server.',
    'welcome.applicationTryout.steps.getSample.action': 'Download sample',
    'welcome.applicationTryout.steps.importConfigs.title': 'Configure Wayfinder sample in {{productName}}',
    'welcome.applicationTryout.steps.importConfigs.description':
      'Apply the Wayfinder sample configurations to {{productName}}.',
    'welcome.applicationTryout.steps.runSample.title': 'Run the sample',
    'welcome.applicationTryout.steps.runSample.description':
      'Start all Wayfinder services from the extracted sample directory.',
    'welcome.applicationTryout.steps.runSample.action': 'See run instructions',
    'welcome.applicationTryout.steps.login.title': 'Sign-in to the app',
    'welcome.applicationTryout.steps.login.description':
      'Open the Wayfinder sample app and sign in with the demo credentials below.',

    'welcome.applicationTryout.scenarios.title': 'Try the following user journeys',
    'welcome.applicationTryout.scenarios.tabs.login': 'Sign-in',
    'welcome.applicationTryout.scenarios.tabs.signup': 'Self sign-up',
    'welcome.applicationTryout.scenarios.tabs.profile': 'View profile',
    'welcome.applicationTryout.scenarios.tabs.recovery': 'Account recovery',
    'welcome.applicationTryout.scenarios.tabs.onboard': 'Staff sign-up',

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
      'Fill in the registration form using these sample details and click Continue.',
    'welcome.applicationTryout.scenarios.signup.step5':
      '{{productName}} will create a Customer user and assign the Traveler role. The browser returns to the Wayfinder app signed in as the new user.',
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
    'welcome.applicationTryout.scenarios.onboard.step1': 'Sign in to the {{productName}} Console as your admin user.',
    'welcome.applicationTryout.scenarios.onboard.step2': 'Navigate to Users and select Add user.',
    'welcome.applicationTryout.scenarios.onboard.step3': 'Select Staff as the user type.',
    'welcome.applicationTryout.scenarios.onboard.step4':
      "Pick Support as the role, enter Sam Rivera's email (sam.rivera@example.com), and click Send invitation. An invite link is emailed to Sam.",
    'welcome.applicationTryout.scenarios.onboard.step5':
      "Open Sam's invitation email from the inbox at <mail>http://localhost:8788</mail> and open the link. The browser opens a Complete Your Profile page.",
    'welcome.applicationTryout.scenarios.onboard.step6':
      "Fill in the additional attributes and submit. Sam's account is now active with the Support role attached.",
    'welcome.applicationTryout.scenarios.onboard.step7':
      'Repeat the flow for Maya Patel (email maya.patel@example.com), picking DestinationsAdmin as the role.',

    'welcome.aiAgentsTryout.breadcrumb': 'Tryout secured AI agent',
    'welcome.aiAgentsTryout.overline': 'Secured AI agent',
    'welcome.aiAgentsTryout.subtitle':
      'Run AI agent use cases against the Wayfinder sample, a travel-booking app with a built-in AI chat assistant.',
    'welcome.aiAgentsTryout.steps.getSample.title': 'Get the Wayfinder sample',
    'welcome.aiAgentsTryout.steps.getSample.description':
      'Download the latest Wayfinder sample distribution and extract the archive. It ships with the web frontend, AI agent service, MCP, SMTP mail server and Wayfinder server.',
    'welcome.aiAgentsTryout.steps.importConfigs.title': 'Configure Wayfinder sample in {{productName}}',
    'welcome.aiAgentsTryout.steps.importConfigs.description':
      'Apply the Wayfinder sample configurations to {{productName}}.',
    'welcome.aiAgentsTryout.steps.configureSample.title': 'Configure the AI provider details in Wayfinder sample',
    'welcome.aiAgentsTryout.steps.configureSample.description':
      'Copy each .env.example to .env in backend/, ai-agent/, and frontend/. Fill in your LLM API key in ai-agent/.env — an Anthropic key from console.anthropic.com or a Gemini key from aistudio.google.com.',
    'welcome.aiAgentsTryout.steps.runSample.title': 'Run the sample',
    'welcome.aiAgentsTryout.steps.runSample.description':
      'Start all Wayfinder services from the extracted sample directory.',
    'welcome.aiAgentsTryout.steps.login.title': 'Sign-in to the app',
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
    'welcome.aiAgentsTryout.scenarios.tabs.protect': 'Protect the agent',
    'welcome.aiAgentsTryout.scenarios.tabs.browse': 'Browse with agent',
    'welcome.aiAgentsTryout.scenarios.tabs.book': 'Book on behalf',

    'welcome.aiAgentsTryout.scenarios.protect.description':
      'See scope-based access control in action - John can use the AI concierge, but Jane cannot.',
    'welcome.aiAgentsTryout.scenarios.protect.step1': 'Open <a>http://localhost:5173</a> and sign in as John Doe.',
    'welcome.aiAgentsTryout.scenarios.protect.step2':
      "Open the chat widget (bottom-right corner) and send any message. The concierge responds — John's token carries the <code>agent:access</code> scope.",
    'welcome.aiAgentsTryout.scenarios.protect.step3': 'Sign out and sign in as Jane Smith.',
    'welcome.aiAgentsTryout.scenarios.protect.step4':
      'Open the chat. Since Jane does not have the Wayfinder Chat User role. Chat agent will not be accessible and the widget will show an error message instead.',
    'welcome.aiAgentsTryout.scenarios.protect.johnLabel': 'John have access to chat with Wayfinder chat agent',
    'welcome.aiAgentsTryout.scenarios.protect.janeLabel': 'Jane does not have access to chat with Wayfinder chat agent',

    'welcome.aiAgentsTryout.scenarios.browse.description':
      'Watch the agent use its own machine-to-machine (M2M) token to call read-only tools - no user consent popup required.',
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

    'welcome.mcpTryout.breadcrumb': 'Tryout secured MCP server',
    'welcome.mcpTryout.overline': 'Secured MCP server',
    'welcome.mcpTryout.subtitle':
      'Connect an external MCP client to the Wayfinder MCP server, signed in through {{productName}}.',
    'welcome.mcpTryout.steps.prerequisite.title': 'Complete AI agents setup',
    'welcome.mcpTryout.steps.prerequisite.description':
      'This tryout extends the AI Agents environment. Complete the Securing AI Agents tryout setup first — the same bundle seeds the EXTERNAL-MCP-CLIENT application used here.',
    'welcome.mcpTryout.steps.importConfigs.title': 'Configure Wayfinder sample in {{productName}}',
    'welcome.mcpTryout.steps.importConfigs.description':
      'Apply the Wayfinder sample configurations to {{productName}}.',
    'welcome.mcpTryout.steps.verifyApp.title': 'Verify the application',
    'welcome.mcpTryout.steps.verifyApp.description':
      'In the {{productName}} Console, open Applications and confirm EXTERNAL-MCP-CLIENT is listed.',
    'welcome.mcpTryout.steps.verifyApp.action': 'Open applications',
    'welcome.mcpTryout.steps.installInspector.title': 'Launch MCP Inspector',
    'welcome.mcpTryout.steps.installInspector.description':
      'Launch MCP Inspector locally — a browser-based reference UI for MCP servers with built-in OAuth support. Open a new terminal in the sample app directory and run:',
    'welcome.mcpTryout.steps.allowCors.title': 'Allow Inspector in CORS',
    'welcome.mcpTryout.steps.allowCors.description':
      "Add Inspector's origin to {{productName}}'s CORS allow-list in repository/conf/deployment.yaml, then restart {{productName}}.",

    'welcome.mcpTryout.scenarios.title': 'Try out the following test scenarios',
    'welcome.mcpTryout.scenarios.tabs.connect': 'Connect & sign in',
    'welcome.mcpTryout.scenarios.tabs.permissions': 'Test permissions',

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
    'welcome.mcpTryout.scenarios.connect.fields.transport': 'Transport type',
    'welcome.mcpTryout.scenarios.connect.fields.serverUrl': 'URL',
    'welcome.mcpTryout.scenarios.connect.fields.connectionType': 'Connection type',
    'welcome.mcpTryout.scenarios.connect.fields.clientId': 'Client ID',
    'welcome.mcpTryout.scenarios.connect.fields.clientSecret': 'Client secret',
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
    'welcome.setupComplete.exploreDashboard': 'Start exploring',
    'welcome.setupComplete.nextSteps.onboardApp.title': 'Onboard an application',
    'welcome.setupComplete.nextSteps.onboardApp.description':
      'Register your first application to start integrating sign-in and identity flows.',
    'welcome.setupComplete.nextSteps.onboardApp.action': 'Add application',
    'welcome.setupComplete.nextSteps.exportConfig.title': 'Export configuration',
    'welcome.setupComplete.nextSteps.exportConfig.description':
      'Export your {{productName}} configuration to deploy and run {{productName}}.',
    'welcome.setupComplete.nextSteps.exportConfig.action': 'Export config',

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
    'dataTable.columnsManagementShowHideAllText': 'Show/hide all',
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
    'dataTable.footerTotalRows': 'Total rows:',

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
    'pages.importConfiguration': 'Import configuration',
    'pages.openProject': 'Import configuration',
    'pages.importExport': 'Import / Export',
    'pages.home': 'Home',
    'pages.users': 'Users',
    'pages.userTypes': 'User types',
    'pages.agentTypes': 'Agent types',
    'pages.organizationUnits': 'Organization units',
    'pages.groups': 'Groups',
    'pages.roles': 'Roles',
    'pages.connections': 'Connections',
    'pages.applications': 'Applications',
    'pages.apis': 'APIs',
    'pages.verifiablePresentations': 'Verifiable presentations',
    'pages.verifiableCredentials': 'Verifiable credentials',
    'pages.credentials': 'Templates',
    'pages.presentations': 'Presentations',
    'pages.dashboard': 'Dashboard',
    'pages.flows': 'Flows',
    'pages.design': 'Design',
    'pages.translations': 'Translations',
    'pages.notifications': 'Notifications',
    'pages.emailTemplates': 'Email templates',
    'pages.smsTemplates': 'SMS templates',
    'pages.settings': 'Settings',
    'pages.gateways': 'Gateway management',
    'breadcrumb.console': 'Console',
  },

  // ============================================================================
  // Manage namespace - Admin App pages for users, groups, roles and agents
  // ============================================================================
  manage: {
    title: 'Admin',
    'nav.label': 'Admin Console',
    'userMenu.signOut': 'Sign out',
    'organizationUnit.label': 'Organization unit',
    'organizationUnit.badge': 'OU',
    'nav.users': 'Users',
    'nav.groups': 'Groups',
    'nav.roles': 'Roles',
    'nav.agents': 'Agents',
    'actions.add': 'Add',
    'actions.add.users': 'Add people',
    'actions.invite.users': 'Invite people',
    'actions.add.groups': 'New group',
    'actions.add.roles': 'New role',
    'actions.add.agents': 'Register agent',
    'actions.back': 'Back',
    'actions.close': 'Close',
    'create.users': 'Add a user',
    'create.groups': 'Create a group',
    'create.roles': 'Create a role',
    'create.agents': 'Register an agent',
    'users.description': 'Everyone who can sign in through this organization unit.',
    'groups.description': 'Give roles to many people at once. Members inherit every role on the group.',
    'roles.description': 'Assign roles to people, groups and agents in this organization unit.',
    'agents.description': 'AI agents that act for this organization unit.',
    loading: 'Loading',
    'empty.title': 'Nothing to manage',
    'empty.description': 'You do not have permission to manage anything here.',
  },

  // ============================================================================
  // Users namespace - User management feature translations
  // ============================================================================
  users: {
    // Listing page
    title: 'Users',
    subtitle: 'Manage users, roles, and permissions across your organization',
    addUser: 'Add user',
    inviteUser: 'Invite user',
    inviteUserDescription: 'Send an invite link to a new user to complete their registration',
    inviteLinkGenerated: 'Invite link generated!',
    inviteLinkDescription: 'Share this link with the user to complete their registration.',
    inviteLink: 'Invite link',
    addAnother: 'Add another user',
    'invite.steps.complete': 'Complete',
    editUser: 'Edit user',
    deleteUser: 'Delete user',
    userDetails: 'User details',
    given_name: 'First name',
    family_name: 'Last name',
    email: 'Email address',
    username: 'Username',
    role: 'Role',
    status: 'Status',
    createdAt: 'Created at',
    lastLogin: 'Last login',
    'listing.columns.name': 'Name',
    'listing.columns.userId': 'User ID',
    'listing.columns.status': 'Status',
    'listing.columns.actions': 'Actions',
    noUsers: 'No users found',
    searchUsers: 'Search users...',
    confirmDeleteUser: 'Are you sure you want to delete this user?',
    userCreatedSuccess: 'User created successfully',
    userUpdatedSuccess: 'User updated successfully',
    userDeletedSuccess: 'User deleted successfully',
    'errors.failed.title': 'Error',
    'errors.failed.description': 'An error occurred. Please try again.',
    'errors.onboardingFlowUnavailable.title': 'User onboarding is unavailable',
    'errors.onboardingFlowUnavailable.description':
      'The user onboarding flow could not be resolved. Check that a user onboarding flow is configured for this deployment and that the flow it names exists.',

    // Edit page
    'manageUser.title': 'Manage user',
    'manageUser.subtitle': 'View and manage user information',
    'manageUser.back': 'Back to users',
    'manageUser.sections.quickCopy.title': 'Quick copy',
    'manageUser.sections.quickCopy.description': 'Copy user identifiers for use in your application.',
    'manageUser.sections.quickCopy.userId': 'User ID',
    'manageUser.sections.quickCopy.copyUserId': 'Copy user ID',

    // Create page
    'createUser.title': 'Create user',
    'createUser.subtitle': 'Add a new user to your organization',

    // Add form (native creation, no onboarding flow)
    'addForm.creating': 'Creating...',
    'addForm.noUserTypes': 'No user types are defined yet. Create one before adding a user.',
    'addForm.organizationUnit': 'Organization unit',
    'addForm.selectUserType': 'Select a user type',
    'addForm.submit': 'Create user',
    'addForm.subtitle': 'Choose a user type, then fill in what it requires.',
    'addForm.userType': 'User type',

    'create.success': 'User created successfully.',
    'create.error': 'Failed to create user. Please try again.',
    'update.success': 'User updated successfully.',
    'update.error': 'Failed to update user. Please try again.',
    'delete.title': 'Delete user',
    'delete.message': 'Are you sure you want to delete this user? This action cannot be undone.',
    'delete.disclaimer': 'All associated data will be permanently removed.',
    'delete.success': 'User deleted successfully.',
    'delete.error': 'Failed to delete user. Please try again.',
    'delete.usages.loading': 'Checking affected resources…',
    'delete.usages.title': 'The following agents list this user as their owner:',
    'delete.usages.more': '+{{count}} more',
    'delete.blocking.title': 'This user cannot be deleted until the following agents are reassigned or removed:',

    // Credentials section on edit page
    'manageUser.sections.credentials.title': 'Credentials',
    'manageUser.sections.credentials.description': 'Update credential values such as passwords for this user.',
    'manageUser.sections.credentials.info':
      'Credential values are write-only and cannot be viewed. You can set new values below.',

    // Update credentials
    'updateCredentials.button': 'Update credentials',
    'updateCredentials.hint': 'Fill in only the credentials you want to update. Empty fields will be skipped.',
    'updateCredentials.success': 'Credentials updated successfully.',
    'updateCredentials.error': 'Failed to update credentials. Please try again.',

    // Backend error code translations (per user service error envelope).
    'errors.USR-1003': 'This user no longer exists. It may have already been deleted.',
    'errors.USR-1005': 'The organization unit for this user no longer exists. Refresh and try again.',
    'errors.USR-1014': 'A user with the same unique attribute value already exists.',
    'errors.USR-1019':
      "Some attributes no longer match this user type's schema. Review the user type or the user's attributes.",
    'errors.USR-1021': 'The user type for this user no longer exists. Refresh and try again.',
    'errors.USR-1023': 'This organization unit is not allowed for the selected user type.',
    'errors.USR-1024': 'One or more credential fields are not valid for this user type.',
    'errors.USR-1025': 'This user is managed declaratively and cannot be edited or deleted.',
    'errors.USR-1027':
      'This user cannot be deleted because {{dependencies}} depend on it. Remove or reassign them first.',
    'errors.USR-1028': 'Credentials for this user cannot be updated here.',
    'errors.USR-1029':
      'This change is not allowed. A unit may have moved, or a unit that passed the user on would lose its own reach. Reopen the access and try again.',
    'errors.USR-1030': 'This share no longer exists. It may have been removed already.',
    'errors.USR-1031':
      'This unit already shares the user, from a change made elsewhere. Reset to load it, then redo your change.',
    'errors.USR-1032':
      "Someone else changed this user's sharing while you were editing. Reset to load the latest, then redo your change.",
    'errors.USR-1033': 'This share is declared in a resource file. Change it there.',
    'errors.USR-1034': 'The user is not shared into this unit, so the unit cannot share it on.',
    'errors.USR-1037':
      'Sharing with every unit or every other organization comes only from a resource file or an import.',

    // Read failures
    'listing.error': 'Failed to load users',
    'manageUser.loadError': 'Failed to load user information',
    'manageUser.notFound': 'User not found',

    // Affiliations
    'manageUser.tabs.affiliations': 'Affiliations',
    'organizationAccess.title': 'Where {{resource}} is affiliated',
    'organizationAccess.description':
      '{{resource}} can sign in and act within the units selected here. What they can do in each unit depends on the roles that unit grants.',
    'organizationAccess.ownerOnly': "{{resource}} isn't affiliated with any other unit.",
    'organizationAccess.stopSharingTitle': 'Keep {{resource}} in {{owner}} only?',
    'organizationAccess.stopSharingIntro': '{{resource}} will be affiliated with {{owner}} only. This clears:',
    'organizationAccess.stopSharingConfirm': 'Remove affiliations',
    'organizationAccess.saved': 'Affiliations updated.',
    'organizationAccess.loadFailedTitle': 'Failed to load affiliations',
    'organizationAccess.loadFailedMessage': 'Something went wrong while loading the affiliations.',
    'organizationAccess.save.error': 'Failed to update affiliations. Please try again.',
  },

  // ============================================================================
  // User Types namespace - User types feature translations
  // ============================================================================
  userTypes: {
    // Listing page
    title: 'User types',
    subtitle: 'Define and manage user types with custom schemas',
    addUserType: 'Add user type',
    createUserType: 'Create user type',
    editUserType: 'Edit user type',
    deleteUserType: 'Delete user type',
    userTypeDetails: 'User type details',
    typeName: 'Type name',
    typeNamePlaceholder: 'e.g., employee, customer, partner',
    organizationUnit: 'Organization unit',
    ouSelectPlaceholder: 'Select an organization unit',
    allowSelfRegistration: 'Allow self registration',
    description: 'Description',
    createDescription: 'Define a new user type schema for your organization',
    permissions: 'Permissions',
    schemaProperties: 'Schema properties',
    propertyName: 'Property name',
    propertyNamePlaceholder: 'e.g., email, age, address',
    propertyType: 'Type',
    addProperty: 'Add property',
    'attributes.libraryTitle': 'Available properties',
    'attributes.searchPlaceholder': 'Search properties',
    'attributes.allAdded': 'All available properties have been added.',
    'attributes.noResults': 'No properties match your search.',
    newAttribute: 'New property',
    credential: 'Credential',
    unique: 'Unique',
    removeProperty: 'Remove property',
    regexPattern: 'Regular expression pattern (optional)',
    contactRegexPattern: 'Regular expression pattern',
    'validationErrors.emailRegexRequired': 'Enter a regular expression for email attributes.',
    'validationErrors.mobileRegexRequired': 'Enter a regular expression for mobile attributes.',
    regexPlaceholder: 'e.g., ^[a-zA-Z0-9]+$',
    enumValues: 'Allowed values (enum) - optional',
    enumPlaceholder: 'Add value and press Enter',
    'tooltips.required': 'Users must provide a value for this field',
    'tooltips.unique': 'Each user must have a distinct value for this field',
    'tooltips.credential': 'Values will be hashed and not returned in API responses',
    'tooltips.primary': 'Used as the primary contact for users',
    'tooltips.primarySelected':
      'Used as the primary contact for users. To change it, select Primary on another attribute of this type',
    credentialHint: 'This field will be treated as a secret. Values will be hashed and cannot be retrieved.',
    'types.string': 'String',
    'types.email': 'Email',
    'types.mobile': 'Mobile',
    primary: 'Primary',
    'validationErrors.primaryEmail': 'Select one email attribute as the primary email.',
    'validationErrors.primaryMobile': 'Select one mobile attribute as the primary mobile.',
    'types.number': 'Number',
    'types.boolean': 'Boolean',
    'types.enum': 'Enum',
    'types.array': 'Array',
    'types.object': 'Object',
    'validationErrors.nameRequired': 'Please enter a user type name',
    'validationErrors.handleRequired': 'Please enter a user type handle',
    'validationErrors.handleInvalid':
      'Handle must start and end with a lowercase letter or number, and may contain only lowercase letters, numbers, hyphens, and underscores',
    'validationErrors.ouIdRequired': 'Please provide an organization unit ID',
    'validationErrors.propertiesRequired': 'Please add at least one property',
    'validationErrors.duplicateProperties': 'Duplicate property names found: {{duplicates}}',
    'errors.organizationUnitsFailedTitle': 'Failed to load organization units',
    noUserTypes: 'No user types found',
    'listing.error': 'Failed to load user types',
    'listing.columns.name': 'Name',
    'listing.columns.id': 'User type ID',
    'listing.columns.organizationUnit': 'Organization unit',
    'listing.columns.allowSelfRegistration': 'Self registration',
    'listing.columns.actions': 'Actions',
    noOrganizationUnits: 'No organization units available',
    confirmDeleteUserType: 'Are you sure you want to delete this user type?',
    'errors.USRS-1002': 'This user type no longer exists. It may have already been deleted.',
    'errors.USRS-1003': 'A user type with the same handle already exists.',
    'errors.USRS-1004': 'The user type request is missing required fields or contains invalid data.',
    'errors.USRS-1008': 'This user type is managed by the system and cannot be modified or deleted.',
    'errors.USRS-1011': 'Display attribute must reference an attribute defined in the schema.',
    'errors.USRS-1012': 'Display attribute must reference a string, email, mobile, or number type.',
    'errors.USRS-1013': 'Display attribute cannot reference a credential attribute.',
    'errors.USRS-1016':
      'The handle is invalid. It must start and end with a lowercase letter or number, and may contain only lowercase letters, numbers, hyphens, and underscores.',
    'errors.USRS-1017': 'The handle of a user type cannot be changed.',
    'errors.USRS-1018': 'Primary email and mobile must reference a top-level attribute defined in the schema.',
    'errors.USRS-1019':
      'Primary email must reference an email attribute and primary mobile must reference a mobile attribute.',

    // Edit page
    'manageUserType.title': 'Manage user type',
    'manageUserType.subtitle': 'View and manage user type information',

    // Edit page (new UI)
    'edit.back': 'Back to user types',
    'edit.editName': 'Edit user type name',
    'edit.copyId': 'Copy user type ID',
    'edit.copyHandle': 'Copy user type handle',
    'edit.tabs.general': 'General',
    'edit.tabs.schema': 'Schema',
    'edit.tabs.advanced': 'Advanced',
    'edit.unsavedChanges': 'You have unsaved changes',
    'edit.saveError': 'Failed to save user type',
    'edit.loadError': 'Failed to load user type information',
    'edit.loadErrorTitle': 'Failed to load user type',
    'edit.notFound': 'User type not found',
    'schemaChangeWarning.title': 'Confirm schema changes',
    'schemaChangeWarning.description':
      'Existing users may require updates if their attributes no longer match the revised schema. Applications that return removed attributes in tokens or userinfo must also be updated.',
    'schemaChangeWarning.affected': 'Affected attributes:',
    'schemaChangeWarning.areYouSure': 'Do you want to continue?',
    'schemaChangeWarning.confirm': 'Continue',
    'edit.general.sections.quickCopy.title': 'Quick copy',
    'edit.general.sections.quickCopy.description': 'Copy user type identifiers for use in your application.',
    'edit.general.labels.userTypeHandle': 'User type handle',
    'edit.general.labels.userTypeId': 'User type ID',
    'edit.general.organizationUnit.title': 'Organization unit',
    'edit.general.organizationUnit.description': 'The organization unit this user type belongs to.',
    'edit.general.selfRegistration.title': 'Self registration',
    'edit.general.selfRegistration.description': 'Allow users to self-register with this user type.',
    'edit.general.selfRegistration.enabledHint': 'Users can register themselves as this user type.',
    'edit.general.displayAttribute.title': 'Display attribute',
    'edit.general.displayAttribute.description': 'The attribute used to display user identity.',
    'edit.general.dangerZone.title': 'Danger zone',
    'edit.general.dangerZone.description': 'Irreversible actions for this user type.',
    'edit.general.dangerZone.deleteUserType': 'Delete user type',
    'edit.general.dangerZone.deleteUserTypeDescription':
      'Permanently delete this user type and all associated schema definitions. This action cannot be undone.',

    // Create page
    'createUserType.title': 'Create user type',
    'createUserType.subtitle': 'Add a new user type to your organization',

    // Create wizard steps
    'createWizard.steps.organizationUnit': 'Organization unit',
    'createWizard.steps.name': 'Details',
    'createWizard.steps.properties': 'Properties',
    'createWizard.organizationUnit.title': 'Where should this user type belong?',
    'createWizard.organizationUnit.subtitle':
      "Choose the organization unit that will own this user type. You can't change this once created.",
    'createWizard.organizationUnit.fieldLabel': 'Organization unit',
    'createWizard.name.title': "Let's collect some details about your user type",
    'createWizard.name.fieldLabel': 'User type name',
    'createWizard.name.placeholder': 'Enter your user type name',
    'createWizard.name.maxLength': 'User type name cannot exceed {{max}} characters',
    'createWizard.handle.fieldLabel': 'Handle',
    'createWizard.handle.placeholder': 'e.g., customer',
    'createWizard.handle.hint': "A unique identifier for this user type. You can't change it once created.",
    'createWizard.handle.invalid':
      'Handle must start and end with a lowercase letter or number, and may contain only lowercase letters, numbers, hyphens, and underscores',
    'createWizard.handle.maxLength': 'Handle cannot exceed {{max}} characters',
    'createWizard.general.subtitle': 'Set registration preferences for this user type.',
    'createWizard.properties.title': 'Define your schema properties',
    'createWizard.properties.subtitle': 'Add the fields that make up this user type',
    'create.success': 'User type created successfully.',
    'create.error': 'Failed to create user type. Please try again.',
    'update.success': 'User type updated successfully.',
    'update.error': 'Failed to update user type. Please try again.',
    'delete.title': 'Delete user type',
    'delete.message':
      'Are you sure you want to delete this user type? This action cannot be undone and may affect existing users of this type.',
    'delete.disclaimer': 'All associated schema definitions will be permanently removed.',
    'delete.success': 'User type deleted successfully.',
    'delete.error': 'Failed to delete user type. Please try again.',
    'removeCredentialDialog.title': 'Remove credential flag',
    'removeCredentialDialog.description':
      'Removing the credential flag will cause this field to no longer be hashed or protected. Existing hashed values may become inaccessible. Are you sure you want to proceed?',
    'removeCredentialDialog.descriptionNew':
      'Removing the credential flag will cause this field to no longer be hashed or protected. Are you sure you want to proceed?',
    'removeCredentialDialog.confirm': 'Remove credential',
  },

  // ============================================================================
  // Agent Types namespace - Agent types feature translations
  // ============================================================================
  agentTypes: {
    // Listing page
    title: 'Agent types',
    subtitle: 'Define and manage agent types with custom schemas',
    addAgentType: 'Add agent type',
    createAgentType: 'Create agent type',
    editAgentType: 'Edit agent type',
    deleteAgentType: 'Delete agent type',
    agentTypeDetails: 'Agent type details',
    typeName: 'Type name',
    typeNamePlaceholder: 'e.g., worker, assistant, tool',
    organizationUnit: 'Organization unit',
    ouSelectPlaceholder: 'Select an organization unit',
    description: 'Description',
    createDescription: 'Define a new agent type schema for your organization',
    permissions: 'Permissions',
    schemaProperties: 'Schema properties',
    propertyName: 'Property name',
    propertyNamePlaceholder: 'e.g., model, environment, team',
    propertyType: 'Type',
    addProperty: 'Add property',
    credential: 'Credential',
    unique: 'Unique',
    removeProperty: 'Remove property',
    regexPattern: 'Regular expression pattern (optional)',
    contactRegexPattern: 'Regular expression pattern',
    'validationErrors.emailRegexRequired': 'Enter a regular expression for email attributes.',
    'validationErrors.mobileRegexRequired': 'Enter a regular expression for mobile attributes.',
    regexPlaceholder: 'e.g., ^[a-zA-Z0-9]+$',
    enumValues: 'Allowed values (enum) - optional',
    enumPlaceholder: 'Add value and press Enter',
    'tooltips.required': 'Agents must provide a value for this field',
    'tooltips.unique': 'Each agent must have a distinct value for this field',
    'tooltips.credential': 'Values will be hashed and not returned in API responses',
    credentialHint: 'This field will be treated as a secret. Values will be hashed and cannot be retrieved.',
    'types.string': 'String',
    'types.email': 'Email',
    'types.mobile': 'Mobile',
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
    'listing.columns.id': 'Agent type ID',
    'listing.columns.organizationUnit': 'Organization unit',
    'listing.columns.actions': 'Actions',
    noOrganizationUnits: 'No organization units available',
    confirmDeleteAgentType: 'Are you sure you want to delete this agent type?',

    // Edit page
    'manageAgentType.title': 'Manage agent type',
    'manageAgentType.subtitle': 'View and manage agent type information',

    // Edit page (new UI)
    'edit.back': 'Back to agents',
    'edit.title': 'Agent schema',
    'edit.editName': 'Edit agent type name',
    'edit.copyId': 'Copy agent type ID',
    'edit.tabs.general': 'General',
    'edit.tabs.schema': 'Schema',
    'edit.unsavedChanges': 'You have unsaved changes',
    'edit.loadError': 'Failed to load agent type information',
    'edit.loadErrorTitle': 'Failed to load agent type',
    'edit.notFound': 'Agent type not found',
    'schemaChangeWarning.title': 'Confirm schema changes',
    'schemaChangeWarning.description':
      'Existing agents may require updates if their attributes no longer match the revised schema. Applications that return removed attributes in tokens or userinfo must also be updated.',
    'schemaChangeWarning.affected': 'Affected attributes:',
    'schemaChangeWarning.areYouSure': 'Do you want to continue?',
    'schemaChangeWarning.confirm': 'Continue',
    'edit.general.organizationUnit.title': 'Organization unit',
    'edit.general.organizationUnit.description': 'The organization unit this agent type belongs to.',
    'edit.general.displayAttribute.title': 'Display attribute',
    'edit.general.displayAttribute.description': 'The attribute used to display agent identity.',
    'edit.general.dangerZone.title': 'Danger zone',
    'edit.general.dangerZone.description': 'Irreversible actions for this agent type.',
    'edit.general.dangerZone.deleteAgentType': 'Delete agent type',
    'edit.general.dangerZone.deleteAgentTypeDescription':
      'Permanently delete this agent type and all associated schema definitions. This action cannot be undone.',

    // Create page
    'createAgentType.title': 'Create agent type',
    'createAgentType.subtitle': 'Add a new agent type to your organization',

    // Create wizard steps
    'createWizard.steps.name': 'Create an agent type',
    'createWizard.steps.general': 'General',
    'createWizard.steps.properties': 'Properties',
    'createWizard.name.title': "Let's name your agent type",
    'createWizard.name.fieldLabel': 'Agent type name',
    'createWizard.name.placeholder': 'Enter your agent type name',
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
    'removeCredentialDialog.title': 'Remove credential flag',
    'removeCredentialDialog.description':
      'Removing the credential flag will cause this field to no longer be hashed or protected. Existing hashed values may become inaccessible. Are you sure you want to proceed?',
    'removeCredentialDialog.confirm': 'Remove credential',

    // Backend error code translations (per entitytype service error envelope, agent-type wording).
    'errors.USRS-1002': 'This agent type no longer exists. It may have already been deleted.',
    'errors.USRS-1004': 'The agent type request is missing required fields or contains invalid data.',
    'errors.USRS-1008': 'This agent type is managed by the system and cannot be modified or deleted.',
    'errors.USRS-1011': 'Display attribute must reference an attribute defined in the schema.',
    'errors.USRS-1012': 'Display attribute must reference a string, email, mobile, or number type.',
    'errors.USRS-1013': 'Display attribute cannot reference a credential attribute.',
  },

  // ============================================================================
  // Agents namespace - Agent management feature translations
  // ============================================================================
  agents: {
    // Listing page
    addAgent: 'Add agent',
    'listing.title': 'Agents',
    'listing.subtitle': 'Manage service identities and machine clients',
    'listing.addAgent': 'Add agent',
    'listing.schema': 'Schema',
    'listing.search.placeholder': 'Search agents',
    'listing.loadError': 'Failed to load agents',
    'listing.columns.name': 'Name',
    'listing.columns.agentId': 'Agent ID',
    'listing.columns.organizationUnit': 'Organization unit',
    'listing.columns.status': 'Status',
    'listing.columns.actions': 'Actions',

    // Onboarding (flow driven)
    'onboarding.errors.notConfigured.title': 'Agent onboarding is not configured',
    'onboarding.errors.notConfigured.description':
      'No agent onboarding flow is configured for this deployment. Set flow.agentOnboardingFlow.defaultHandle in the server configuration to the handle of an administration flow that creates agents.',
    'onboarding.errors.flowMissing.title': 'Agent onboarding flow not found',
    'onboarding.errors.flowMissing.description':
      'The configured agent onboarding flow "{{handle}}" does not exist. Create an administration flow with that handle, or point flow.agentOnboardingFlow.defaultHandle at one that exists.',
    'onboarding.errors.unavailable.title': 'Could not start agent onboarding',
    'onboarding.errors.unavailable.description':
      'The agent onboarding flow could not be loaded. Check that the server is reachable and try again.',
    'onboarding.errors.stepFailed': 'This step could not be completed. Review the values and try again.',
    'onboarding.selectPlaceholder': 'Select an option',
    'onboarding.authFlow.loading': 'Loading login flows...',
    'onboarding.authFlow.error': 'Login flows could not be loaded.',
    'onboarding.authFlow.empty': 'No login flows are available.',
    'onboarding.userType.loading': 'Loading user types...',
    'onboarding.userType.error': 'User types could not be loaded.',
    'onboarding.userType.empty': 'No user types are available.',
    'onboarding.user.loading': 'Loading users...',
    'onboarding.user.error': 'Users could not be loaded.',
    'onboarding.user.empty': 'No users are available.',
    'onboarding.addAnother': 'Add another agent',

    // Add form (native creation, no onboarding flow)
    'addForm.agentType': 'Agent type',
    'addForm.creating': 'Creating...',
    'addForm.description': 'Description',
    'addForm.descriptionPlaceholder': 'What this agent does',
    'addForm.issueCredentials': 'Issue OAuth client credentials',
    'addForm.issueCredentialsHint':
      'A client ID and a client secret the agent authenticates with. The secret is shown once, when the agent is created.',
    'addForm.secretIssued.title': 'Agent created',
    'addForm.secretIssued.subtitle':
      "Copy the client secret and store it somewhere safe. It won't be shown again; the agent's page can only regenerate it.",
    'addForm.noAgentTypes': 'No agent types are defined yet. Create one before adding an agent.',
    'addForm.owner': 'Owner',
    'addForm.ownerDefault': 'Me (default)',
    'addForm.ownerTruncated': 'Showing the first {{shown}} of {{total}} users.',
    'addForm.ownersError': 'The users to choose an owner from could not be loaded.',
    'addForm.ownerHint':
      'Defaults to you. Choose a user created here if the agent is applied to a gateway, which needs an owner it can find.',
    'addForm.organizationUnit': 'Organization unit',
    'addForm.selectAgentType': 'Select an agent type',
    'addForm.submit': 'Create agent',
    'addForm.subtitle': 'Name the agent, choose its type, then fill in what the type requires.',
    'create.error': 'Failed to create agent. Please try again.',
    'create.success': 'Agent created successfully.',

    // Client secret (creation)
    'clientSecret.saveTitle': 'Save your client secret',
    'clientSecret.saveSubtitle': "This secret won't be shown again. Copy it and store it somewhere safe.",
    'clientSecret.agentNameLabel': 'Agent name',
    'clientSecret.clientIdLabel': 'Client ID',
    'clientSecret.clientSecretLabel': 'Client secret',
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
    'regenerateSecret.success.secretLabel': 'New client secret',
    'regenerateSecret.success.copySecret': 'Copy client secret',
    'regenerateSecret.success.copied': 'Copied',
    'regenerateSecret.success.securityReminder.title': "You won't be able to see this secret again",
    'regenerateSecret.success.securityReminder.description':
      "Store the new client secret somewhere safe. If you lose it, you'll need to regenerate it again.",

    // Edit page (header)
    'edit.page.errorTitle': 'Failed to load agent',
    'edit.page.notFound': 'Agent not found',
    'edit.page.back': 'Back to agents',
    'edit.page.logoUpdate.label': 'Update logo',
    'edit.page.description.empty': 'No description',
    'edit.page.description.placeholder': 'Add a description',
    'edit.page.tabs.overview': 'Overview',
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
    'edit.page.validation.invalidBackchannelLogoutUri': 'fix the back-channel logout URI',
    'edit.page.validation.invalidIdentityAssertions': 'fix the identity assertions (ID-JAG) settings',
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
    'edit.general.labels.agentId': 'Agent ID',
    'edit.general.labels.ownerId': 'Owner ID',
    'edit.general.sections.owner.title': 'Owner',
    'edit.general.sections.owner.description': 'The user accountable for this agent.',
    'edit.general.sections.owner.label': 'Owner',
    'edit.general.owner.empty': 'No owner assigned',
    'edit.general.sections.attributes.title': 'Attributes',
    'edit.general.sections.attributes.description':
      "A preview of this agent's attribute values. Manage them from the Attributes tab.",
    'edit.general.sections.dangerZone.title': 'Danger zone',
    'edit.general.sections.dangerZone.description': 'Actions here are permanent. Make sure before you proceed.',
    'edit.general.dangerZone.deleteAgent.title': 'Delete agent',
    'edit.general.dangerZone.deleteAgent.description':
      'Permanently deletes this agent and immediately invalidates any tokens it has issued. This action cannot be undone.',
    'edit.general.dangerZone.deleteAgent.button': 'Delete agent',

    // Edit page - Overview tab
    'edit.overview.agentDetails.title': 'Agent details',
    'edit.overview.agentDetails.description': 'Identifiers used in your integration code.',
    'edit.overview.agentDetails.organizationUnitId': 'Organization unit ID',
    'edit.overview.agentDetails.organizationUnitHandle': 'Organization unit handle',
    'edit.overview.endpoints.title': 'Useful endpoints',
    'edit.overview.endpoints.description': 'For authenticating this agent, on its own or on behalf of a user.',
    'edit.overview.endpoints.wellknown': 'OpenID configuration',
    'edit.overview.endpoints.authorization': 'Authorization endpoint',
    'edit.overview.endpoints.token': 'Token endpoint',
    'edit.overview.endpoints.userinfo': 'Userinfo endpoint',
    'edit.overview.endpoints.jwks': 'JWKS URI',
    'edit.overview.attributes.title': 'Attributes',
    'edit.overview.attributes.description':
      "A preview of this agent's attribute values. Manage them from the Attributes tab.",
    'edit.overview.accessMode.title': 'Access mode',
    'edit.overview.accessMode.description': 'How this agent is allowed to authenticate.',
    'edit.overview.accessMode.editAdvanced': 'Edit in advanced',
    'edit.overview.accessMode.own': 'On its own behalf',
    'edit.overview.accessMode.delegated': 'On behalf of a user',
    'edit.overview.accessMode.allowedUserTypes': 'Allowed user types',
    'edit.overview.signInPreview.notConfigured': 'Not configured',
    'edit.overview.quickstart.title': 'Connect with LangChain',
    'edit.overview.quickstart.description':
      "Register this agent and call it from a LangChain app, running under its own identity or a user's delegated authority.",
    'edit.overview.quickstart.action': 'Open quickstart',

    // Edit page - Credentials tab
    'edit.credentials.clientId.title': 'Client ID',
    'edit.credentials.sections.identifier.title': 'Identifier',
    'edit.credentials.sections.identifier.description': 'Unique identifier used to reference this agent.',
    'edit.credentials.sections.identifier.clientIdLabel': 'Client ID',
    'edit.credentials.sections.identifier.clientIdHint':
      'The public OAuth2 client identifier this agent uses to authenticate as a client.',
    'edit.credentials.sections.secret.title': 'Secret',
    'edit.credentials.sections.secret.description':
      'Regenerating the secret immediately invalidates the current one and cannot be undone.',
    'edit.credentials.sections.secret.clientSecretLabel': 'Client secret',
    'edit.credentials.sections.secret.hint':
      'A confidential credential used with the Client ID to authenticate this agent. Keep it secret.',
    'edit.credentials.sections.secret.regenerateButton': 'Regenerate client secret',
    'edit.credentials.tokenEndpointAuthMethod.title': 'Client authentication method',
    'edit.credentials.tokenEndpointAuthMethod.description':
      'Defines how this agent authenticates at protected endpoints.',
    'edit.credentials.tokenEndpointAuthMethod.placeholder': 'Select an auth method',
    'edit.credentials.tokenEndpointAuthMethod.hint': 'How this agent proves its identity at protected endpoints.',
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
    'edit.credentials.certificate.error.encryptionDependsOnCert':
      'This certificate is used to encrypt the ID token. Change the ID token format to a non-encrypted type before removing the certificate.',

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
    'edit.flows.allowedUserTypes.title': 'Allowed user types',
    'edit.flows.allowedUserTypes.description': 'Restrict which user types can sign up through this agent.',
    'edit.flows.allowedUserTypes.label': 'User types',
    'edit.flows.allowedUserTypes.placeholder': 'Select user types',
    'edit.flows.allowedUserTypes.hint': 'Users of these types can sign up through this agent.',
    'edit.flows.allowedUserTypes.required': 'Select at least one user type that can sign up through this agent.',
    'edit.flows.delegationLock.message':
      'These settings are frozen for this agent. Turn on Delegated mode in the Advanced tab to unlock and start using them.',
    'edit.flows.signInGrantLock.message':
      'Sign-in flows apply only when this agent signs users in with the authorization code or CIBA grant type.',

    // Edit page - Advanced tab
    'edit.advanced.mode.title': 'Operating mode',
    'edit.advanced.delegationToggle.label': 'Delegated mode',
    'edit.advanced.mode.onOwnBehalf.description':
      'This agent authenticates with its own credentials without user interaction, using client credentials.',
    'edit.advanced.mode.onBehalfOfUser.description': 'This agent can act on behalf of a user.',
    'edit.advanced.agentSignIn.title': 'Agent sign-in',
    'edit.advanced.agentSignIn.description': 'Allow agents to sign in through this agent using the sign-in flow.',
    'edit.advanced.agentSignIn.toggle.label': 'Enable agent sign-in',
    'edit.advanced.redirectUris.title': 'Authorized redirect URIs',
    'edit.advanced.redirectUris.description': 'For use with requests from a web server',
    'edit.advanced.redirectUris.empty': 'No redirect URIs configured.',
    'edit.advanced.redirectUris.addUri': 'Add URI',
    'edit.advanced.redirectUris.error.empty': 'URI cannot be empty',
    'edit.advanced.redirectUris.error.invalid': 'Enter a valid URL',
    'edit.advanced.redirectUris.required': 'The authorization code grant requires at least one valid redirect URI.',
    'edit.advanced.oauthAccess.title': 'OAuth 2 configuration',
    'edit.advanced.oauthAccess.description': 'Manage OAuth 2 settings for this agent.',
    'edit.advanced.oauthAccess.grantTypes.label': 'Grant types',
    'edit.advanced.oauthAccess.grantTypes.hint':
      'The greyed-out grants unlock once you turn on Delegated mode at the top of this tab.',
    'edit.advanced.security.title': 'Security',
    'edit.advanced.security.description':
      'Controls how this agent protects the authorization code exchange when a user signs in.',
    'edit.advanced.security.pkce.label': 'Require PKCE',
    'edit.advanced.security.pkce.forced':
      'This agent is set up as a public client, so PKCE is required and cannot be turned off.',
    'edit.advanced.security.pkce.on': 'authorization_code is on for this agent, so PKCE is required automatically.',
    'edit.advanced.security.pkce.notApplicable':
      'PKCE only applies to the <code>authorization_code</code> grant. Turn that on to enable this setting.',
    'edit.advanced.security.par.label': 'Require pushed authorization requests',
    'edit.advanced.security.par.hint':
      'Require this agent to push its authorization request to the PAR endpoint before redirecting a user to sign in.',
    'edit.advanced.security.par.notApplicable':
      'Pushed authorization requests only apply to the <code>authorization_code</code> grant. Turn that on to enable this setting.',

    // Edit page - Tokens tab
    'edit.tokens.tabs.user': 'User',
    'edit.tokens.tabs.agent': 'Agent',
    'edit.tokens.audience.title': 'Issued to',
    'edit.tokens.audience.agent.description': 'Tokens for the agent acting on its own',
    'edit.tokens.audience.user.description': 'Tokens for the agent acting on behalf of a user',
    'edit.tokens.audience.footnote': 'Attribute sets are configured independently for each audience.',
    'edit.tokens.delegationLock.message':
      'This agent does not receive tokens on behalf of a user. Turn on Delegated mode in the <advancedLink>Advanced tab</advancedLink> to configure them.',
    'edit.tokens.user.tokenExchangeAttributes.note':
      'Token exchange keeps only the selected attributes that the incoming token already carries.',
    'edit.tokens.user.jwtBearerAttributes.note': 'Tokens from a redeemed ID-JAG carry no user attributes.',
    'edit.tokens.agent.attributes.title': 'Access token attributes',
    'edit.tokens.agent.attributes.description':
      'Extra attributes to add to the access token this agent receives for itself.',
    'edit.tokens.agent.attributes.label': 'Add or remove attributes',
    'edit.tokens.agent.attributes.hint': 'Click on agent attributes to add them to its access token.',
    'edit.tokens.agent.attributes.empty':
      'No attributes available. Configure attributes for this agent in the Attributes tab.',
    'edit.tokens.agent.validity.title': 'Token validity',
    'edit.tokens.agent.validity.label': 'Token validity',
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
    'errors.AGT-1010': 'The agent type must be provided.',
    'errors.AGT-1011': 'The limit parameter must be between 1 and 100.',
    'errors.AGT-1012': 'The offset parameter must be a non-negative integer.',
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
    'errors.AGT-1036': 'A certificate operation failed due to invalid input.',
    'errors.AGT-1037': 'An entity with the same client ID already exists.',
    'errors.AGT-1038': 'An entity may have at most one inbound auth config per protocol.',
    'errors.AGT-1039': 'The specified owner does not match any known user, application, or agent.',
    'errors.AGT-1040': 'One or more user attributes are not valid for the configured allowed user types.',
    'errors.AGT-1041': 'The provided logo URL is not valid.',
    'errors.AGT-1042':
      'The subject attribute mapping must reference an attribute that is unique, required, and string typed in an allowed agent type.',
    'errors.AGT-1044':
      'A delegated agent needs at least one grant type that lets it act for a user, such as authorization code.',
  },

  // ============================================================================
  // Organization Units namespace - Organization unit management feature translations
  // ============================================================================
  organizationUnits: {
    // Tree picker (shared component)
    'treePicker.empty': 'No organization units available',
    'treePicker.error': 'Failed to load organization unit data',

    // Listing page
    'listing.title': 'Organization units (OU)',
    'listing.subtitle': 'Manage organization units and hierarchies',
    'listing.addRootOrganizationUnit': 'Add root organization unit',
    'listing.error.title': 'Failed to load organization units',
    'listing.error.unknown': 'An unknown error occurred',
    'listing.treeView.empty': 'No organization units found',
    'listing.treeView.noChildren': 'No child organization units',
    'listing.treeView.loadError': 'Failed to load child organization units',
    'listing.treeView.addChild': 'Add child OU',
    'listing.treeView.addChildOrganizationUnit': 'Add child organization unit',
    'listing.treeView.loadMore': 'Load more',
    'listing.columns.name': 'Name',
    'listing.columns.handle': 'Handle',
    'listing.columns.description': 'Description',

    // Create page
    'create.title': 'Create organization unit',
    'create.heading': "Let's set up your organization unit",
    'create.subtitle': 'Define a new organization unit',
    'create.error': 'Failed to create organization unit. Please try again.',

    'delete.dialog.title': 'Delete organization unit',
    'delete.dialog.message': 'Are you sure you want to delete this organization unit? This action cannot be undone.',
    'delete.dialog.disclaimer':
      'Warning: All associated data, configurations, and user assignments will be permanently removed.',
    'delete.dialog.error': 'Failed to delete organization unit. Please try again.',

    /* -------------------- Edit page -------------------- */
    // Common
    'edit.page.error': 'Failed to load organization unit information',
    'edit.page.notFound': 'Organization unit not found',
    'edit.page.logoUpdate.label': 'Update logo',
    'edit.page.copyOuId': 'Copy organization unit ID',
    'edit.page.back': 'Back to organization units',
    'edit.page.backToOU': 'Back to {{name}}',
    'edit.page.description.empty': 'No description',
    'edit.page.description.placeholder': 'Enter a description...',
    'edit.page.tabs.general': 'General',
    'edit.page.tabs.childOUs': 'Child OUs',
    'edit.page.tabs.users': 'Users',
    'edit.page.tabs.groups': 'Groups',
    'edit.page.tabs.customization': 'Customization',
    'edit.page.tabs.defaultFlows': 'Flows',
    'edit.page.tabs.advanced': 'Advanced',
    'edit.actions.unsavedChanges.label': 'You have unsaved changes',
    'edit.actions.reset.label': 'Reset',
    'edit.actions.save.label': 'Save changes',
    'edit.actions.saving.label': 'Saving...',

    // General section
    'edit.general.sections.quickCopy.title': 'Quick copy',
    'edit.general.sections.quickCopy.description': 'Copy organization unit identifiers for quick reference.',
    'edit.general.sections.parentOUSettings.title': 'Parent organization unit',
    'edit.general.sections.parentOUSettings.description': 'The parent organization unit in the hierarchy.',
    'edit.general.sections.dangerZone.title': 'Danger zone',
    'edit.general.sections.dangerZone.description': 'Actions in this section are irreversible. Proceed with caution.',
    'edit.general.sections.dangerZone.deleteOU.title': 'Delete organization unit',
    'edit.general.sections.dangerZone.deleteOU.description':
      'Deleting this organization unit is permanent and cannot be undone.',
    'edit.general.ou.id.label': 'Organization unit ID',
    'edit.general.ou.parent.label': 'Parent organization unit',
    'edit.general.ou.noParent.label': 'This is a root organization unit.',
    'edit.general.dangerZone.delete.button.label': 'Delete organization unit',
    // Form fields
    'edit.general.handle.label': 'Handle',
    'edit.general.handle.placeholder': 'e.g., engineering, sales, hr',
    'edit.general.handle.hint': 'A unique identifier for this organization unit',
    'edit.general.handle.validations.required': 'Handle is required',
    'edit.general.handle.validations.format': 'Handle must be lowercase alphanumeric with hyphens only',
    'edit.general.handle.validations.maxLength': 'Handle cannot exceed {{max}} characters',
    'edit.general.name.label': 'Name',
    'edit.general.name.placeholder': 'e.g., engineering department',
    'edit.general.name.validations.required': 'Name is required',
    'edit.general.name.validations.maxLength': 'Name cannot exceed {{max}} characters',
    'edit.general.description.label': 'Description',
    'edit.general.description.placeholder': 'Enter a description for this organization unit',
    'edit.general.parent.label': 'Parent organization unit',
    'edit.general.parent.hint': 'The parent organization unit for this new unit',
    'edit.general.dangerZone.delete.title': 'Delete organization unit',
    'edit.general.dangerZone.delete.message':
      'Are you sure you want to delete this organization unit? This action cannot be undone.',
    'edit.general.dangerZone.delete.error': 'Failed to delete organization unit. Please try again.',
    'edit.general.dangerZone.delete.success': 'Organization unit deleted successfully.',

    // Child OUs Section
    'edit.childOUs.sections.manage.title': 'Child organization units',
    'edit.childOUs.sections.manage.description': 'View and manage child organization units under this OU',

    // Users Section
    'edit.users.sections.manage.title': 'Users',
    'edit.users.sections.manage.description': 'View users belonging to this organization unit',
    'edit.users.sections.manage.listing.columns.id': 'User ID',
    'edit.users.sections.manage.listing.columns.type': 'User type',
    'edit.users.sections.manage.listing.columns.name': 'Display name',

    // Groups Section
    'edit.groups.sections.manage.title': 'Groups',
    'edit.groups.sections.manage.description': 'View groups belonging to this organization unit',
    'edit.groups.sections.manage.listing.columns.name': 'Group name',
    'edit.groups.sections.manage.listing.columns.id': 'Group ID',

    // Customization tab
    'edit.customization.sections.appearance': 'Appearance',
    'edit.customization.sections.appearance.description': 'Customize the look and feel of this organization unit.',
    'edit.customization.labels.theme': 'Theme',
    'edit.customization.theme.placeholder': 'Select a theme',
    'edit.customization.theme.hint': 'The theme applied to this organization unit.',
    'edit.customization.labels.layout': 'Layout',
    'edit.customization.layout.placeholder': 'Select a layout',
    'edit.customization.layout.hint': 'The layout applied to this organization unit.',

    // Default Flows tab
    'edit.flows.labels.authenticationFlow': 'Sign-in flow',
    'edit.flows.authenticationFlow.description':
      'Choose the default flow that handles user login and authentication for applications under this organization unit.',
    'edit.flows.authenticationFlow.placeholder': 'Select an authentication flow',
    'edit.flows.authenticationFlow.hint':
      'Select the flow that handles user sign-in for applications under this organization unit.',
    'edit.flows.authenticationFlow.alert':
      'To modify the selected flow, <0>open the flow builder</0>. To create a new flow, visit the <1>Flows page</1>.',
    'edit.flows.labels.registrationFlow': 'Sign-up flow',
    'edit.flows.registrationFlow.description':
      'Choose the default flow that handles user sign-up and account creation for applications under this organization unit.',
    'edit.flows.registrationFlow.placeholder': 'Select a registration flow',
    'edit.flows.registrationFlow.hint':
      'Select the flow that handles user registration for applications under this organization unit.',
    'edit.flows.registrationFlow.alert':
      'To modify the selected flow, <0>open the flow builder</0>. To create a new flow, visit the <1>Flows page</1>.',
    'edit.flows.labels.recoveryFlow': 'Recovery flow',
    'edit.flows.recoveryFlow.description':
      'Choose the default flow that handles password and account recovery for applications under this organization unit.',
    'edit.flows.recoveryFlow.placeholder': 'Select a recovery flow',
    'edit.flows.recoveryFlow.hint':
      'Select the flow that handles account recovery for applications under this organization unit.',
    'edit.flows.recoveryFlow.alert':
      'To modify the selected flow, <0>open the flow builder</0>. To create a new flow, visit the <1>Flows page</1>.',
    'edit.flows.labels.signOutFlow': 'Sign out flow',
    'edit.flows.signOutFlow.description':
      'Choose the default flow that handles user sign-out and session termination for applications under this organization unit.',
    'edit.flows.signOutFlow.placeholder': 'Select a sign-out flow',
    'edit.flows.signOutFlow.hint':
      'Select the flow that runs when a user signs out of applications under this organization unit.',
    'edit.flows.signOutFlow.alert':
      'To modify the selected flow, <0>open the flow builder</0>. To create a new flow, visit the <1>Flows page</1>.',
    'create.success': 'Organization unit created successfully.',
    'update.success': 'Organization unit updated successfully.',
    'update.error': 'Failed to update organization unit. Please try again.',
    'delete.success': 'Organization unit deleted successfully.',
    'delete.error': 'Failed to delete organization unit. Please try again.',

    // Edit page - read error
    'edit.page.errorTitle': 'Failed to load organization unit',

    // Tab sections - read errors
    'edit.users.sections.manage.error': 'Failed to load users',
    'edit.groups.sections.manage.error': 'Failed to load groups',
    'edit.childOUs.sections.manage.error': 'Failed to load child organization units',

    // Backend error codes (organization unit service)
    'errors.OU-1003': 'This organization unit no longer exists. It may have already been deleted.',
    'errors.OU-1004': 'An organization unit with the same name already exists under the same parent.',
    'errors.OU-1005': 'The selected parent organization unit could not be found.',
    'errors.OU-1006':
      'This organization unit has child units, users, groups, connections, or flows and cannot be deleted while they exist.',
    'errors.OU-1007': 'This parent selection would create a circular dependency in the organization hierarchy.',
    'errors.OU-1008': 'An organization unit with the same handle already exists under the same parent.',
    'errors.OU-1012': 'This organization unit is managed by configuration and cannot be modified or deleted.',
    'errors.OU-1013': 'Too many results were returned for this request. Please narrow your search and try again.',
    'errors.OU-1015': 'The selected sign-in flow could not be found. Refresh and try again.',
    'errors.OU-1016': 'The selected sign-up flow could not be found. Refresh and try again.',
    'errors.OU-1017': 'The selected recovery flow could not be found. Refresh and try again.',
    'errors.OU-1018': 'The selected sign-out flow could not be found. Refresh and try again.',
  },

  // ============================================================================
  // Resource Sharing namespace - Organization Access of every shareable resource
  // ============================================================================
  resourceSharing: {
    'changeScope.action': 'Change scope',
    'changeScope.description':
      'Changing the scope deletes the "Every organization unit" policy and its exceptions, then lets you choose child units and other trees. Once you save, it can\'t be set again here.',
    'changeScope.question': 'Need to grant only some units?',
    'childUnits.choices.all.description': 'Every unit under {{owner}}, now and later.',
    'childUnits.choices.all.title': 'All child units',
    'childUnits.choices.none.description': 'No child unit has access.',
    'childUnits.choices.none.title': 'None',
    'childUnits.choices.selected.description': 'Pick units and how far below each one access reaches.',
    'childUnits.choices.selected.title': 'Selected child units',
    'childUnits.empty': 'No child units selected yet. Use Grant access to pick them.',
    'childUnits.grant': 'Grant access',
    'childUnits.sectionDescription': "Units in {{owner}}'s own hierarchy.",
    'childUnits.sectionTitle': 'Child organization units of {{owner}}',
    'dialogs.childUnits.description':
      'Only units directly under {{owner}} can be picked. Units deeper down are reached through their parent.',
    'dialogs.childUnits.title': 'Grant access to child units',
    'dialogs.exceptions.anyDescription':
      "Choose any unit, at any depth, that shouldn't get access. Everything below it is excluded too.",
    'dialogs.exceptions.childDescription':
      "Choose child units of {{owner}} that shouldn't get access. Everything below an excluded unit is excluded too.",
    'dialogs.exceptions.confirm': 'Apply exceptions',
    'dialogs.exceptions.rootDescription': "Choose top units that shouldn't get access.",
    'dialogs.exceptions.title': 'Add exceptions',
    'dialogs.trees.description':
      'Pick the top unit of each tree. Units below it get access only if it passes access on.',
    'dialogs.trees.title': 'Grant access to other trees',
    'everyUnit.summary':
      'Every organization unit in every tree, at any depth. Units and trees created later get access automatically.',
    'exceptions.add': 'Add exception',
    'exceptions.anyHint': 'Any unit in any tree. Excluding a unit also excludes everything below it.',
    'exceptions.childHint': 'Direct child units only. Excluding a unit also excludes everything below it.',
    'exceptions.empty': 'No exceptions.',
    'exceptions.remove': 'Remove {{name}} from exceptions',
    'exceptions.rootHint': 'Top units only.',
    'exceptions.title': 'Except',
    'loadError.message': 'Something went wrong while loading who can use this resource.',
    'loadError.title': 'Failed to load organization access',
    'lost.everyTree':
      'Saving removes "Every tree". It can only be set in a resource file or by importing, so it can\'t be chosen again here.',
    'lost.everyUnit':
      'Saving deletes the "Every organization unit" policy. It can only be set in a resource file or by importing, so it can\'t be chosen again here.',
    'noneBlocked.childUnits': 'child units',
    'noneBlocked.dropsEveryTree': 'Not available. It also removes Every tree, which leaves nothing granted.',
    'noneBlocked.otherIsNone': 'Not available while {{section}} is set to None.',
    'noneBlocked.otherTrees': 'other trees',
    nothingChosen: 'Nothing is granted below yet. Saving like this is the same as "{{owner}} only".',
    'picker.alreadyExcluded': 'Already excluded',
    'picker.alreadyGranted': 'Already granted',
    'picker.collapse': 'Hide units under {{name}}',
    'picker.empty': 'No organization units to show.',
    'picker.expand': 'Show units under {{name}}',
    'picker.includedWith': 'Included with {{name}}',
    'picker.loadError': 'Failed to load organization units.',
    'picker.loadMore': 'Load more',
    'picker.noneSelected': 'Nothing selected yet.',
    'picker.search': 'Search organization units',
    'picker.toExclude': 'To exclude',
    'picker.toGrant': 'To grant',
    'picker.unselect': 'Remove {{name}}',
    'reach.label': 'Reach for {{name}}',
    'reach.subtree': 'And below',
    'reach.subtreeHint': 'Includes every unit below it, now and later.',
    'reach.unit': 'This unit only',
    'reach.unitHint': "Units below it don't get access.",
    revoke: 'Revoke access for {{name}}',
    'save.success': 'Organization access updated.',
    'scope.chosen.description': 'Choose child units of {{owner}} and other trees below.',
    'scope.chosen.title': '{{owner}} and chosen units',
    'scope.everyUnit.description': 'Every unit in every tree, now and later, with optional exceptions.',
    'scope.everyUnit.title': 'Every organization unit',
    'scope.owner.description': 'No child unit and no other tree can use it.',
    'scope.owner.title': '{{owner}} only',
    'switch.cantRestore.everyTree':
      '"Every tree" can only be set in a resource file or by importing. Once you save, it can\'t be chosen again here.',
    'switch.cantRestore.everyUnit':
      '"Every organization unit" can only be set in a resource file or by importing. Once you save, it can\'t be chosen again here.',
    'switch.cleared.childUnitExclusions': 'Child unit exceptions: {{names}}',
    'switch.cleared.childUnits': 'Granted child units: {{names}}',
    'switch.cleared.everyTree': 'Every tree',
    'switch.cleared.everyTreeWithExceptions': 'Every tree, with its exceptions: {{names}}',
    'switch.cleared.everyUnit': 'The "Every organization unit" policy',
    'switch.cleared.everyUnitWithExceptions': 'The "Every organization unit" policy, with its exceptions: {{names}}',
    'switch.cleared.trees': 'Granted trees: {{names}}',
    'switch.confirm': 'Switch and clear',
    'switch.confirmChangeScope': 'Delete policy and change scope',
    'switch.confirmOwner': 'Stop sharing',
    'switch.intro': "What you set up for {{choice}} doesn't carry over. Switching clears:",
    'switch.introChangeScope':
      'This deletes the "Every organization unit" policy, so {{resource}} stops reaching every organization unit. Access then starts as All child units of {{owner}}, which you can change below. Deleting the policy clears:',
    'switch.introOwner': 'Only {{owner}} will be able to use {{resource}}. This clears:',
    'switch.keep': 'Keep current choice',
    'switch.notYet': 'Nothing is removed until you save. Until then, Reset brings everything back.',
    'switch.title': 'Switch to {{choice}}?',
    'switch.titleChangeScope': 'Change the scope?',
    'switch.titleOwner': 'Stop sharing {{resource}}?',
    'switch.unitSettings':
      'Unit settings are cleared too. Units lose the settings they chose for themselves within their terms.',
    'trees.add': 'Add a tree',
    'trees.choices.every.description': 'The top unit of every tree, now and later, with optional exceptions.',
    'trees.choices.every.title': 'Every tree',
    'trees.choices.every.unavailable': "Every tree's top unit. Only set through a resource file or an import.",
    'trees.choices.none.description': 'No other tree has access.',
    'trees.choices.none.title': 'None',
    'trees.choices.selected.description': 'Grant top units one by one.',
    'trees.choices.selected.title': 'Selected trees',
    'trees.description':
      "Top units of trees outside {{owner}}'s. Units below a top unit don't get access unless it passes access on to them.",
    'trees.empty': 'No trees picked yet. Use Add a tree to pick them.',
    'trees.title': 'Other organization trees',
    'trees.topUnitOnly': 'Top unit only',
    'who.description': 'Users and agents of a granted organization unit can sign in to {{resource}}.',
    'who.title': 'Who can use {{resource}}',
  },

  // ============================================================================
  // Groups namespace - Group management feature translations
  // ============================================================================
  groups: {
    // List page
    'listing.title': 'Groups',
    'listing.subtitle': 'Manage groups and their members across organization units',
    'listing.addGroup': 'Add group',
    'listing.error': 'Failed to load groups',
    'listing.search.placeholder': 'Search groups...',
    'listing.columns.name': 'Name',
    'listing.columns.description': 'Description',
    'listing.columns.organizationUnit': 'Organization unit',
    'listing.columns.actions': 'Actions',

    // Create page
    'create.title': 'Create group',
    'create.heading': 'Create a new group',
    'create.error': 'Failed to create group. Please try again.',
    'create.form.name.label': 'Group name',
    'create.form.name.placeholder': 'Enter group name',
    'create.form.name.required': 'Group name is required',
    'create.form.name.maxLength': 'Group name cannot exceed {{max}} characters',
    'create.form.description.label': 'Description',
    'create.form.description.placeholder': 'Enter group description',
    'create.form.organizationUnit.label': 'Organization unit',
    'create.form.organizationUnit.placeholder': 'Select an organization unit',
    'create.form.organizationUnit.required': 'Organization unit is required',

    // Create wizard
    'createWizard.steps.name': 'Details',
    'createWizard.steps.organizationUnit': 'Organization unit',
    'createWizard.name.title': "Let's collect some details about your group",
    'createWizard.organizationUnit.title': 'Where should this group belong?',
    'createWizard.organizationUnit.subtitle':
      "Choose the organization unit that will own this group. You can't change this once created.",
    'createWizard.organizationUnit.fieldLabel': 'Organization unit',
    'createWizard.createGroup': 'Create group',

    // Edit page
    'edit.page.back': 'Back to groups',
    'edit.page.error': 'Failed to load group',
    'edit.page.notFound': 'Group not found',
    'edit.page.description.placeholder': 'Add a description...',
    'edit.page.description.empty': 'No description',
    'edit.page.header.groupId': 'ID',
    'edit.page.header.ouId': 'Organization unit',
    'edit.page.header.editName': 'Edit group name',
    'edit.page.header.editDescription': 'Edit description',
    'edit.page.tabs.general': 'General',
    'edit.page.tabs.members': 'Members',
    'edit.page.tabs.advanced': 'Advanced',
    'edit.page.unsavedChanges': 'You have unsaved changes',
    'edit.page.reset': 'Reset',
    'edit.page.save': 'Save changes',
    'edit.page.saving': 'Saving...',

    // General settings
    'edit.general.sections.quickCopy.title': 'Quick copy',
    'edit.general.sections.quickCopy.description': 'Copy group identifiers for quick reference.',
    'edit.general.sections.quickCopy.groupId': 'Group ID',
    'edit.general.sections.quickCopy.copyGroupId': 'Copy group ID',
    'edit.general.sections.quickCopy.copyOrganizationUnitId': 'Copy organization unit ID',
    'edit.general.sections.organizationUnit.title': 'Organization unit',
    'edit.general.sections.organizationUnit.description': 'The organization unit this group belongs to.',
    'edit.general.sections.organizationUnit.handleLabel': 'Handle',
    'edit.general.sections.organizationUnit.idLabel': 'ID',
    'edit.general.sections.quickCopy.copyOrganizationUnitHandle': 'Copy organization unit handle',
    'edit.general.sections.dangerZone.title': 'Danger zone',
    'edit.general.sections.dangerZone.description': 'Actions in this section are irreversible. Proceed with caution.',
    'edit.general.sections.dangerZone.deleteGroup': 'Delete this group',
    'edit.general.sections.dangerZone.deleteGroupDescription': 'Deleting this group is permanent and cannot be undone.',

    // Members settings
    'edit.members.sections.manage.title': 'Members',
    'edit.members.sections.manage.description': 'Manage the members of this group',
    'edit.members.sections.manage.listing.columns.name': 'Name',
    'edit.members.sections.manage.listing.columns.id': 'Member ID',
    'edit.members.sections.manage.listing.columns.type': 'Type',
    'edit.members.sections.manage.addMember': 'Add member',
    'edit.members.sections.manage.noMembers': 'No members in this group',

    // Add member dialog
    'addMember.title': 'Add member',
    'addMember.tabs.users': 'Users',
    'addMember.tabs.apps': 'Apps',
    'addMember.tabs.agents': 'Agents',
    'addMember.tabs.groups': 'Groups',
    'addMember.search.placeholder': 'Search users...',
    'addMember.noResults': 'No users found',
    'addMember.noResultsApps': 'No apps found',
    'addMember.noResultsAgents': 'No agents found',
    'addMember.noResultsGroups': 'No groups found',
    'addMember.add': 'Add selected',
    'addMember.columns.displayName': 'Display name',
    'addMember.columns.userType': 'User type',
    'addMember.columns.userId': 'User ID',
    'addMember.error': 'Failed to add member. Please try again.',
    'addMember.fetchError': 'Failed to load users. Please try again.',
    'addMember.fetchAppsError': 'Failed to load apps. Please try again.',
    'addMember.fetchAgentsError': 'Failed to load agents. Please try again.',
    'addMember.fetchGroupsError': 'Failed to load groups. Please try again.',
    'removeMember.error': 'Failed to remove member. Please try again.',

    // Delete dialog
    'delete.title': 'Delete group',
    'delete.message': 'Are you sure you want to delete this group?',
    'delete.disclaimer': 'This action cannot be undone. All group associations will be permanently removed.',
    'delete.error': 'Failed to delete group. Please try again.',
    'create.success': 'Group created successfully.',
    'update.success': 'Group updated successfully.',
    'update.error': 'Failed to update group. Please try again.',
    'delete.success': 'Group deleted successfully.',
    'addMember.success': 'Member added successfully.',
    'removeMember.success': 'Member removed successfully.',
    'errors.GRP-1003': 'This group no longer exists. It may have already been deleted.',
    'errors.GRP-1004': 'A group with this name already exists in this organization unit. Choose a different name.',
    'errors.GRP-1007': 'One or more selected members no longer exist. Refresh and try again.',
    'errors.GRP-1015': 'This group is managed declaratively and cannot be edited or deleted.',
    'errors.GRP-1016': 'New groups cannot be created while the server is running in declarative-only mode.',
  },

  // ============================================================================
  // Roles namespace - Role management feature translations
  // ============================================================================
  roles: {
    // List page
    'listing.title': 'Roles',
    'listing.subtitle': 'Manage roles and their permissions across organization units',
    'listing.addRole': 'Add role',
    'listing.error': 'Failed to load roles',
    'listing.search.placeholder': 'Search roles...',
    'listing.columns.name': 'Name',
    'listing.columns.description': 'Description',
    'listing.columns.organizationUnit': 'Organization unit',
    'listing.columns.actions': 'Actions',

    // Create page
    'create.title': 'Create role',
    'create.error': 'Failed to create role. Please try again.',
    'create.form.name.label': 'Role name',
    'create.form.name.placeholder': 'Enter role name',
    'create.form.name.required': 'Role name is required',
    'create.form.name.maxLength': 'Role name cannot exceed {{max}} characters',
    'create.form.description.label': 'Description',
    'create.form.description.placeholder': 'Enter role description',
    'create.form.organizationUnit.label': 'Organization unit',
    'create.form.organizationUnit.required': 'Organization unit is required',

    // Create wizard
    'createWizard.steps.basicInfo': 'Details',
    'createWizard.steps.organizationUnit': 'Organization unit',
    'createWizard.steps.permissions': 'Permissions',
    'createWizard.basicInfo.title': "Let's collect some details about your role",
    'createWizard.organizationUnit.title': 'Where should this role belong?',
    'createWizard.organizationUnit.subtitle':
      "Choose the organization unit that will own this role. You can't change this once created.",
    'createWizard.organizationUnit.fieldLabel': 'Organization unit',
    'createWizard.permissions.title': 'Assign permissions (optional)',
    'createWizard.permissions.subtitle':
      'Choose what this role grants. You can skip this step and add permissions later.',
    'createWizard.permissions.scopes.label': 'Selected scopes',

    // Edit page
    'edit.page.back': 'Back to roles',
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
    'edit.page.tabs.advanced': 'Advanced',
    'edit.page.unsavedChanges': 'You have unsaved changes',
    'edit.page.reset': 'Reset',
    'edit.page.save': 'Save changes',
    'edit.page.saving': 'Saving...',

    // General settings
    'edit.general.sections.quickCopy.copyRoleId': 'Copy role ID',
    'edit.general.sections.organizationUnit.title': 'Organization unit',
    'edit.general.sections.organizationUnit.description': 'The organization unit this role belongs to.',
    'edit.general.sections.organizationUnit.copyId': 'Copy organization unit ID',
    'edit.general.sections.dangerZone.title': 'Danger zone',
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
    'edit.assignments.sections.manage.title': 'Assigned users, groups, apps & agents',
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
    'assignments.dialog.title': 'Add assignment',
    'assignments.dialog.tabs.users': 'Users',
    'assignments.dialog.tabs.groups': 'Groups',
    'assignments.dialog.tabs.apps': 'Apps',
    'assignments.dialog.tabs.agents': 'Agents',
    'assignments.dialog.columns.displayName': 'Display name',
    'assignments.dialog.columns.name': 'Name',
    'assignments.dialog.columns.description': 'Description',
    'assignments.dialog.columns.userType': 'User type',
    'assignments.dialog.add': 'Add selected',
    'assignments.dialog.fetchError': 'Failed to load data. Please try again.',
    'assignments.add.error': 'Failed to add assignment. Please try again.',
    'assignments.remove.error': 'Failed to remove assignment. Please try again.',

    // Delete dialog
    'delete.title': 'Delete role',
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
    'errors.ROL-1003': 'This role no longer exists. It may have already been deleted.',
    'errors.ROL-1004': 'A role with this name already exists in this organization unit. Choose a different name.',
    'errors.ROL-1007': 'One or more selected assignees no longer exist. Refresh and try again.',
    'errors.ROL-1012': 'One or more selected permissions are no longer valid. Refresh the page and try again.',
    'errors.ROL-1013': 'This role is managed declaratively and cannot be edited or deleted.',
    'errors.ROL-1015': 'New roles cannot be created while the server is running in declarative-only mode.',
    'errors.ROL-1019': 'Select the assignee to remove from this role.',
    'errors.ROL-1020': 'This assignee no longer holds the role. Refresh and try again.',
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
    'listing.loadError': 'Failed to load connections',

    // Filters / categories
    'categories.all': 'All',
    'categories.social-login': 'Social login',
    'categories.enterprise': 'Enterprise',
    'categories.sms': 'SMS',
    'categories.email': 'Email',
    'categories.identity-verification': 'Identity verification',
    'categories.crm': 'CRM',
    'categories.data-store': 'Data store',
    'categories.authorization': 'Authorization',
    'categories.trusted-idp': 'Trusted token issuer',
    'categories.custom': 'Custom',

    // Card
    'card.configured': 'Configured',
    'card.notConfigured': 'Not configured',
    'card.comingSoon': 'Coming soon',
    'card.default': 'Default',
    'card.addCustom.title': 'Add custom connection',
    'card.addCustom.description': "Set up a connection that isn't in the catalog.",

    // Vendor descriptions
    'vendor.google.description': 'Let users sign in with their Google account.',
    'vendor.github.description': 'Let users sign in with their GitHub account.',
    'vendor.oidc.description': 'Connect any OpenID Connect identity provider.',
    'vendor.oauth.description': 'Connect any OAuth 2 identity provider.',
    'vendor.twilio.description': 'Send SMS one-time passcodes via Twilio.',
    'vendor.vonage.description': 'Deliver SMS and email passcodes through Vonage.',
    'vendor.sms-gateway.description': 'Route SMS through your own HTTP gateway.',
    'vendor.authzen-pdp.description': 'Delegate authorization decisions to an AuthZEN-compatible PDP.',
    'vendor.smtp.description': 'Deliver email through your own SMTP server.',
    'vendor.http.description': 'Call an HTTP service from a flow, such as a risk engine or a custom check.',
    'vendor.trustedIdp.description': 'Trusted token issuer for token exchange and ID-JAG.',

    // Add custom connection wizard
    'wizard.title': 'Add custom connection',
    'wizard.steps.type': 'Connection type',
    'wizard.steps.name': 'Details',
    'wizard.steps.configure': 'Configure',
    'wizard.organizationUnit.title': 'Where should this connection belong?',
    'wizard.organizationUnit.subtitle':
      "Choose the organization unit that will own this connection. You can't change this once created.",
    'wizard.organizationUnit.required': 'Organization unit is required',
    'wizard.type.heading': 'What kind of connection do you want to add?',
    'wizard.type.subheading':
      "Custom connections aren't in the vendor catalog. Pick the type of integration you want to wire up.",
    'wizard.type.oidc.label': 'OpenID Connect provider',
    'wizard.type.oidc.description': 'Connect any OpenID Connect identity provider.',
    'wizard.type.oidc.tag': 'Login provider · Enterprise',
    'wizard.type.oauth.label': 'OAuth 2 provider',
    'wizard.type.oauth.description': 'Connect any OAuth 2 identity provider.',
    'wizard.type.oauth.tag': 'Login provider · Enterprise',
    'wizard.type.sms.label': 'SMS gateway',
    'wizard.type.sms.description': 'Route SMS through your own HTTP gateway.',
    'wizard.type.sms.tag': 'Message sender · SMS',
    'wizard.type.policyDecisionPoint.label': 'Policy decision point (PDP)',
    'wizard.type.policyDecisionPoint.description':
      'Connect a policy decision point using a supported authorization protocol.',
    'wizard.type.policyDecisionPoint.tag': 'Authorization · PDP',
    'wizard.protocol.heading': 'Choose a policy decision point protocol',
    'wizard.protocol.subheading': 'Select the protocol supported by your policy decision point.',
    'wizard.protocol.authzen.label': 'AuthZEN',
    'wizard.protocol.authzen.description': 'Use the AuthZEN protocol to send authorization evaluation requests.',
    'wizard.type.smtp.label': 'Email provider (SMTP)',
    'wizard.type.smtp.description': 'Deliver email through your own SMTP server.',
    'wizard.type.smtp.tag': 'Message sender · Email',
    'wizard.type.http.label': 'Service endpoint',
    'wizard.type.http.description': 'Call an HTTP service from a flow, such as a risk engine or a custom check.',
    'wizard.type.http.tag': 'Extension · HTTP',
    'wizard.type.trustedIdp.label': 'Trusted token issuer',
    'wizard.type.trustedIdp.description':
      "Trust an external IdP's identity assertions and exchange them for access tokens.",
    'wizard.type.trustedIdp.tag': 'Token exchange · ID-JAG',
    'wizard.name.title': "Let's collect some details about your connection",
    'wizard.name.fieldLabel': 'Connection name',
    'wizard.name.placeholder': 'Enter your connection name',
    'wizard.configure.heading': 'Configure your connection',
    'wizard.configure.subheading': 'Configure the connection settings below.',
    'wizard.configure.pdpSubheading': 'Configure the endpoints and runtime settings for this connection.',
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
    'detail.backToConnections': 'Back to connections',
    'detail.defaultBadgeManaged': 'Managed by server configuration.',
    'detail.loadError.title': 'Failed to load connection',
    'detail.notFound.title': 'Connection not found',
    'detail.notFound.description': 'This connection may have been deleted or the link is incorrect.',
    'detail.tabs.general': 'General',
    'detail.tabs.authentication': 'Authentication',
    'detail.tabs.attributeMapping': 'Attribute configuration',
    'detail.tabs.subjectMapping': 'Attribute configuration',
    'detail.tabs.advanced': 'Advanced',
    'detail.quickCopy.title': 'Quick copy',
    'detail.quickCopy.description': 'Copy connection identifiers for use in your integration.',
    'detail.connectionId': 'Connection ID',
    'detail.connectionId.hint': 'Unique identifier for this connection.',
    'detail.organizationUnit.title': 'Organization unit',
    'detail.organizationUnit.description': 'The organization unit this connection belongs to.',
    'detail.organizationUnit.handleLabel': 'Handle',
    'detail.organizationUnit.idLabel': 'ID',
    'detail.organizationUnit.copyHandle': 'Copy handle',
    'detail.organizationUnit.copyId': 'Copy organization unit ID',
    'detail.credentials.title': 'Credentials',
    'detail.credentials.description': 'Credentials and endpoints for this connection. Secrets are stored write-only.',
    'detail.configuration.title': 'Connection configuration',
    'detail.configuration.description': 'Provide the details ThunderID needs to connect to this service.',
    'detail.authentication.title': 'Authentication',
    'detail.authentication.endpointChanged': 'The PDP endpoint changed. Enter fresh credentials before saving.',
    'detail.authentication.gatewayUrlChanged': 'The SMS gateway URL changed. Enter fresh credentials before saving.',
    'detail.authentication.description':
      'Configure how ThunderID authenticates requests sent to this policy decision point.',
    'detail.authentication.gatewayDescription':
      'Configure how ThunderID authenticates requests sent to this SMS gateway.',
    'detail.authentication.originChanged':
      'The URL origin changed. Enter the credentials again under Authentication before saving.',
    'detail.authentication.method.label': 'Authentication method',
    'detail.authentication.method.none': 'None',
    'detail.outboundAuthentication.description':
      'Credentials ThunderID uses to sign in to this provider. Secrets are stored write-only.',
    'detail.usages.title': 'Usages',
    'detail.usages.description':
      'Resources that use this connection. A connection cannot be deleted while a resource uses it.',
    'detail.usages.empty': 'No resources use this connection.',
    'detail.usages.unavailable': 'The resources that use this connection could not be determined.',
    'detail.usages.loadError': 'Failed to load the resources that use this connection.',
    'detail.usages.resourceType.flow': 'Flow',
    'detail.usages.resourceType.other': 'Resource',
    'detail.dangerZone.title': 'Danger zone',
    'detail.dangerZone.description': 'Actions in this section are irreversible. Proceed with caution.',
    'detail.dangerZone.delete.title': 'Delete connection',
    'detail.dangerZone.delete.description':
      'Permanently delete this connection and all associated data. Any application relying on it will stop accepting logins through this provider. This action cannot be undone.',
    'detail.saveBar.unsaved': 'You have unsaved changes',
    'detail.saveBar.save': 'Save changes',
    'detail.saveBar.saving': 'Saving...',
    'detail.saveBar.reset': 'Reset',

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
    'form.fields.authScheme.label': 'Authentication method',
    'form.fields.authScheme.none': 'None',
    'form.fields.authScheme.bearer': 'Bearer token',
    'form.fields.authScheme.basic': 'Basic authentication',
    'form.fields.authScheme.apiKey': 'API key',
    'form.fields.bearerToken.label': 'Bearer token',
    'form.fields.bearerToken.hint': 'Token sent in the Authorization header for each PDP request.',
    'form.fields.basicUsername.label': 'Username',
    'form.fields.basicUsername.hint': 'Username sent with each PDP request.',
    'form.fields.basicPassword.label': 'Password',
    'form.fields.basicPassword.hint': 'Password sent with each PDP request.',
    'form.fields.apiKeyHeader.hint': 'Header name used to send the API key to the PDP.',
    'form.fields.apiKeyValue.hint': 'API key value sent in the configured header.',
    'form.fields.redirectUri.label': 'Redirect URI',
    'form.fields.redirectUri.help': 'Add this exact URI to your {{vendor}} OAuth client.',
    'form.fields.redirectUri.hint':
      'Register this exact URL as an authorized redirect URI in your provider. It must match your login gate’s callback URL.',
    'form.fields.scopes.label': 'Scopes',
    'form.fields.scopes.hint':
      'Space-separated scopes to request during sign-in. Defaults to <code>openid email profile</code> if not set.',
    'form.fields.scopes.githubHint':
      'Space-separated scopes to request during sign-in. Defaults to <code>user:email</code> if not set.',
    'form.fields.scopes.placeholder': 'openid email profile',
    'form.fields.authorizationEndpoint.label': 'Authorization endpoint',
    'form.fields.authorizationEndpoint.hint': 'Authorization endpoint used to start the OAuth2 sign-in flow.',
    'form.fields.tokenEndpoint.label': 'Token endpoint',
    'form.fields.tokenEndpoint.hint': 'Token endpoint used to exchange the authorization code for tokens.',
    'form.fields.userInfoEndpoint.label': 'UserInfo endpoint',
    'form.fields.userInfoEndpoint.hint': 'Endpoint used to fetch additional profile claims for the signed-in user.',
    'form.fields.userProfileEndpoint.label': 'User profile endpoint',
    'form.fields.userProfileEndpoint.hint':
      "The provider's own profile API, called with the access token, for example https://api.github.com/user. Leave it empty only if the provider issues a JWT access token carrying a sub claim, which then supplies the subject and no other attributes. For an OpenID Connect provider, create an OIDC connection instead.",
    'form.fields.jwksEndpoint.label': 'JWKS endpoint',
    'form.fields.jwksEndpoint.hint': 'Endpoint that exposes signing keys for verifying identity tokens.',
    'form.fields.prompt.label': 'Prompt',
    'form.fields.prompt.hint':
      "Optional prompt value forwarded to the provider's authorization request, e.g. select_account or consent.",
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
    'form.fields.smsGatewayUrl.label': 'Gateway URL',
    'form.fields.smsGatewayUrl.hint': 'HTTP endpoint ThunderID calls to send an SMS.',
    'form.fields.httpMethod.label': 'HTTP method',
    'form.fields.httpMethod.hint': 'Method used to call the gateway URL.',
    'form.fields.contentType.label': 'Content type',
    'form.fields.contentType.hint': 'Format of the request body sent to the gateway.',
    'form.fields.httpHeaders.label': 'Additional HTTP headers',
    'form.fields.httpHeaders.hint':
      'Optional headers sent with every request. Commas are not supported in a name or value.',
    'form.fields.httpHeaders.replaceHint':
      'Additional headers are already configured. Add one or more pairs to replace them.',
    'form.fields.authzenHeaders.hint':
      'Add the API key header and any other headers required by the PDP. Commas are not supported in a name or value.',
    'form.fields.authzenHeaders.add': 'Add header',
    'form.fields.httpHeaders.add': 'Add header',
    'form.fields.authzenEndpoint.label': 'AuthZEN evaluation endpoint',
    'form.fields.authzenBatchEndpoint.label': 'AuthZEN batch evaluation endpoint',
    'form.fields.timeoutMs.label': 'Timeout in milliseconds',
    'form.fields.timeoutMs.hint': 'Maximum time ThunderID waits for one PDP request.',
    'form.fields.httpEndpointDescription.label': 'Description',
    'form.fields.httpEndpointDescription.hint': 'Optional note about what this endpoint does, up to 500 characters.',
    'form.fields.httpEndpointUrl.label': 'Endpoint URL',
    'form.fields.httpEndpointUrl.hint':
      'The http or https URL {{productName}} sends requests to. Setting an authentication method requires https.',
    'form.fields.httpEndpointTimeoutMs.label': 'Timeout in milliseconds',
    'form.fields.httpEndpointTimeoutMs.hint':
      'Maximum time {{productName}} waits for a response, from 1 to 20000 milliseconds.',
    'form.fields.httpEndpointHeaders.label': 'HTTP headers',
    'form.fields.httpEndpointHeaders.hint':
      'Headers sent with every request. Set credentials under Authentication instead.',
    'form.fields.httpEndpointHeaders.add': 'Add header',
    'form.fields.retryCount.label': 'Retry count',
    'form.fields.retryCount.hint': 'Number of retries for transient PDP or network failures.',
    'subjectMapping.attributes.title': 'Subject attribute mapping',
    'subjectMapping.attributes.description':
      'Choose user or agent attributes this PDP needs and optionally rename them for the AuthZEN request.',
    'subjectMapping.attributes.category.user': 'User',
    'subjectMapping.attributes.category.agent': 'Agent',
    'subjectMapping.attributes.userType.label': 'User type',
    'subjectMapping.attributes.userType.placeholder': 'Select a user type',
    'subjectMapping.attributes.agentType.label': 'Agent type',
    'subjectMapping.attributes.agentType.placeholder': 'Select an agent type',
    'subjectMapping.attributes.label': 'Allowed subject attributes',
    'subjectMapping.attributes.placeholder': 'email department riskScore',
    'subjectMapping.attributes.hint':
      'Select known subject attributes or type custom runtime attributes. Leave empty to send only the default subject fields.',
    'subjectMapping.mappings.title': 'Attribute mappings',
    'subjectMapping.mappings.description':
      'Map ThunderID subject attribute names to the attribute names expected by the PDP.',
    'subjectMapping.mappings.hint':
      'Select extra user or agent attributes to include in the PDP request. The PDP attribute is optional and is only needed when the PDP expects a different name.',
    'subjectMapping.mappings.thunderIdAttribute': 'ThunderID attribute',
    'subjectMapping.mappings.pdpAttribute': 'PDP attribute',
    'subjectMapping.mappings.pdpAttributeOptional': 'PDP attribute (optional)',
    'subjectMapping.mappings.mapPdpAttribute': 'Map to a different PDP attribute',
    'subjectMapping.mappings.thunderIdPlaceholder': 'e.g. email',
    'subjectMapping.mappings.add': 'Add mapping',
    'subjectMapping.mappings.user': 'User mappings',
    'subjectMapping.mappings.agent': 'Agent mappings',
    'subjectMapping.mappings.addUser': 'Add user mapping',
    'subjectMapping.mappings.addAgent': 'Add agent mapping',
    'subjectMapping.mappings.loadingTypes': 'Loading user and agent types...',
    'subjectMapping.mappings.loadTypesError': 'Failed to load user and agent types.',
    'subjectMapping.mappings.remove': 'Remove',
    'subjectMapping.mappings.duplicateAttribute':
      'Each ThunderID attribute can be mapped only once for an entity type.',
    'subjectMapping.mappings.unresolved.warning':
      'These mappings could not be matched to exactly one user or agent type. Remove them before saving this connection.',
    'subjectMapping.mappings.unresolved.entityType': 'Entity type: {{entityType}}',
    'subjectMapping.mappings.unresolved.attributes': 'Attributes: {{attributes}}',
    'form.fields.apiKeyHeaders.label': 'API key headers',
    'form.fields.apiKeyHeaders.hint':
      'Use headers that authenticate your SMS gateway, for example X-API-Key. Stored values are masked.',
    'form.fields.apiKeyHeaders.add': 'Add API key header',
    'form.fields.smsHttpHeaders.label': 'Other HTTP headers',
    'form.fields.smsHttpHeaders.hint':
      'Add other headers your SMS gateway needs, such as X-Campaign-ID. Values remain visible after saving. Commas are not supported.',
    'form.fields.smsHttpHeaders.add': 'Add HTTP header',
    'form.fields.smtpHost.label': 'Host',
    'form.fields.smtpHost.hint': 'Hostname of the SMTP server email is sent through.',
    'form.fields.smtpPort.label': 'Port',
    'form.fields.smtpPort.hint': 'Port of the SMTP server. Typically 587 for STARTTLS or 465 for implicit TLS.',
    'form.fields.smtpFromAddress.label': 'From address',
    'form.fields.smtpFromAddress.hint': 'Address recipients see the email come from.',
    'form.fields.smtpFromName.label': 'Sender name',
    'form.fields.smtpFromName.hint':
      'Optional name shown beside the address, for example Acme Support. Leave blank to show the address alone.',
    'form.fields.smtpTls.label': 'Transport security',
    'form.fields.smtpTls.hint':
      'How the connection is secured. STARTTLS upgrades a plain connection, implicit TLS connects over TLS directly. None sends email unencrypted.',
    'form.fields.authenticationType.label': 'Authentication method',
    'form.fields.authenticationType.hint': 'How {{productName}} signs in to this server when sending.',
    'form.keyValue.name': 'Name',
    'form.keyValue.value': 'Value',
    'form.keyValue.showValue': 'Show value',
    'form.keyValue.hideValue': 'Hide value',
    'form.keyValue.add': 'Add',
    'form.keyValue.update': 'Update',
    'form.keyValue.cancel': 'Cancel',
    'form.keyValue.remove': 'Remove',
    'form.sections.authentication': 'Authentication',
    'form.authentication.loadError': 'Failed to load authentication methods',
    'form.sections.federation': 'Federation',
    'form.secret.update': 'Update',
    'form.secret.keepHelp': 'Leave unchanged to keep the stored secret.',
    'form.headers.configuredName': 'Configured header name',
    'form.headers.configuredValue': 'Configured header value',
    'form.headers.keepHelp': 'Leave unchanged to keep the configured headers.',
    'form.copy': 'Copy',
    'form.copied': 'Copied to clipboard',
    'form.actions.create': 'Create connection',
    'form.actions.save': 'Save changes',
    'form.actions.delete': 'Delete connection',

    // Attribute mapping (authentication providers)
    'attributeMapping.userType.label': 'Default user type',
    'attributeMapping.userType.placeholder': 'Select a user type',
    'attributeMapping.userTypeRequired': 'Select a default user type.',
    'attributeMapping.add': 'Add mapping',
    'attributeMapping.externalAttribute.label': 'External attribute',
    'attributeMapping.externalAttribute.placeholder': 'e.g. given_name',
    'attributeMapping.localAttribute.label': 'Local attribute',
    'attributeMapping.localAttribute.placeholder': 'e.g. firstName',
    // Section 1 — user type resolution
    'attributeMapping.resolution.title': 'User type resolution',
    'attributeMapping.resolution.description':
      'Select which local user type an external identity resolves to, choosing the attribute-mapping profile applied to it.',
    'attributeMapping.resolution.dynamic.label': 'Resolve user type from an attribute',
    'attributeMapping.resolution.externalAttribute.placeholder': 'e.g. user_type',
    'attributeMapping.resolution.externalAttribute.helper': 'The attribute whose value decides the user type.',
    'attributeMapping.resolution.valueMapping.title': 'Value mapping',
    'attributeMapping.resolution.valueMapping.enable': 'Enable value mapping',
    'attributeMapping.resolution.valueMapping.hint': 'Map each attribute value to a user type.',
    'attributeMapping.resolution.valueMapping.externalValue': 'External value',
    'attributeMapping.resolution.valueMapping.localUserType': 'Local user type',
    'attributeMapping.resolution.valueMapping.valuePlaceholder': 'e.g. employee',
    'attributeMapping.resolution.addValue': 'Add value',
    'attributeMapping.resolution.default.helperFallback':
      "Used when the attribute is missing or its value isn't mapped.",
    // Section 2 — attribute mappings by user type
    'attributeMapping.mappings.title': 'Attribute mappings',
    'attributeMapping.mappings.description':
      'Map the attributes this provider returns onto your local user schema. Define a separate mapping set for each user type.',
    'attributeMapping.mappings.userType': 'User type',
    'attributeMapping.mappings.userTypeRequired': 'Select a user type for this mapping set.',
    'attributeMapping.mappings.addUserType': 'Add user type',
    'attributeMapping.mappings.remove': 'Remove',
    // Section 3 — account linking
    'attributeMapping.linking.title': 'Account linking',
    'attributeMapping.linking.description':
      'The attributes used to find the associated local user. Each one matches on every local attribute it is mapped to, and on a local attribute of the same name.',
    'attributeMapping.linking.label': 'External attribute',
    'attributeMapping.linking.labelCombo': 'External attributes',
    'attributeMapping.linking.placeholder': 'e.g. email',
    'attributeMapping.linking.addAttribute': 'Add attribute',
    'attributeMapping.linking.and': 'AND',

    // Authorization mapping (federated role, group, and permission assignment)
    'authorizationMapping.title': 'Authorization mapping',
    'authorizationMapping.description':
      'Map values this provider asserts, such as group membership, to local roles, groups, or permissions.',
    'authorizationMapping.advanced.toggle.label': 'Advanced rules',
    'authorizationMapping.advanced.toggle.helper':
      'By default, each value is matched by name against an existing role, group, or permission. Turn ' +
      'this on to compare values against explicit rules and grant different targets per value.',

    'authorizationRules.claim.label': 'Key',
    'authorizationRules.claim.placeholder': 'e.g. groups',
    'authorizationRules.valueType.label': 'Value type',
    'authorizationRules.valueType.string': 'String',
    'authorizationRules.valueType.number': 'Number',
    'authorizationRules.valueType.boolean': 'Boolean',
    'authorizationRules.valueType.array': 'Array',
    'authorizationRules.delimiter.label': 'Delimiter',
    'authorizationRules.delimiter.preset.none': 'No delimiter (single value)',
    'authorizationRules.delimiter.preset.space': 'Space',
    'authorizationRules.delimiter.preset.comma': 'Comma ( , )',
    'authorizationRules.delimiter.preset.semicolon': 'Semicolon ( ; )',
    'authorizationRules.delimiter.preset.pipe': 'Pipe ( | )',
    'authorizationRules.delimiter.preset.custom': 'Custom…',
    'authorizationRules.delimiter.custom.label': 'Custom delimiter',
    'authorizationRules.delimiter.custom.placeholder': 'Enter a delimiter',
    'authorizationRules.mapping.add': 'Add mapping',
    'authorizationRules.mapping.remove': 'Remove mapping',
    'authorizationRules.values.title': 'Values',
    'authorizationRules.values.helper': "If the key's value satisfies a rule below, grant everything listed under it.",
    'authorizationRules.value.label': 'External value',
    'authorizationRules.value.placeholder': 'e.g. engineering',
    'authorizationRules.value.add': 'Add value',
    'authorizationRules.value.remove': 'Remove value',
    'authorizationRules.value.boolean.true': 'True',
    'authorizationRules.value.boolean.false': 'False',
    'authorizationRules.operator.label': 'Operator',
    'authorizationRules.operator.equals': 'Equals',
    'authorizationRules.operator.not_equals': 'Not equals',
    'authorizationRules.operator.greater_than': 'Greater than',
    'authorizationRules.operator.less_than': 'Less than',
    'authorizationRules.operator.greater_than_or_equal': 'Greater than or equal',
    'authorizationRules.operator.less_than_or_equal': 'Less than or equal',
    'authorizationRules.operator.includes': 'Includes',
    'authorizationRules.operator.not_includes': 'Not includes',
    'authorizationRules.target.sectionLabel': 'Targets',
    'authorizationRules.target.type.role': 'Role',
    'authorizationRules.target.type.group': 'Group',
    'authorizationRules.target.type.permission': 'Permission',
    'authorizationRules.target.role.label': 'Role',
    'authorizationRules.target.role.placeholder': 'Select a role',
    'authorizationRules.target.group.label': 'Group',
    'authorizationRules.target.group.placeholder': 'Select a group',
    'authorizationRules.target.resourceServer.label': 'Resource server',
    'authorizationRules.target.resourceServer.placeholder': 'Select a resource server',
    'authorizationRules.targetType.label': 'Target type',

    // Delete dialog
    'delete.title': 'Delete connection',
    'delete.message': 'Are you sure you want to delete “{{name}}”? This action cannot be undone.',
    'delete.usages.loading': 'Checking affected resources…',
    'delete.usages.more': '+{{count}} more',
    'delete.blocking.title': 'This connection cannot be deleted until the following resources are updated or removed:',
    'delete.defaultEmailSender':
      'This is the default email sender. After you delete it, system-generated emails, such as account lock notifications, are not sent until another sender is set as the default.',

    // Toasts
    'create.success': 'Connection created successfully.',
    'create.error': 'Failed to create connection.',
    'create.setDefaultError': 'Connection created, but it could not be made the default email sender.',
    'update.success': 'Connection updated successfully.',
    'update.error': 'Failed to update connection.',
    'delete.success': 'Connection deleted successfully.',
    'delete.error': 'Failed to delete connection.',

    // Default email sender
    'setDefault.action': 'Set as default',
    'setDefault.title': 'Set default email sender',
    'setDefault.message':
      '<bold>{{name}}</bold> will be used as the default sender for system-generated emails, such as account lock notifications.',
    'setDefault.replaces': 'This will replace the current default email sender.',
    'setDefault.confirm': 'Set as default',
    'setDefault.setting': 'Setting…',
    'setDefault.success': '{{name}} is now the default email sender.',
    'setDefault.error': 'Failed to set the default email sender.',
    'setDefault.lockedReason': 'The default email sender is fixed by the deployment configuration.',
    'defaultSender.loadError': 'Failed to load the default email sender.',

    // Errors / validation
    'error.duplicateName': 'A connection with this name already exists.',
    'error.loadFailed': 'Failed to load connection.',
    'validation.required': 'This field is required.',
    'validation.url': 'Enter a valid URL.',
    'validation.authenticatedEndpointHttps':
      'Use HTTPS for authenticated PDP endpoints. HTTP is allowed only for localhost and loopback IP addresses.',
    'validation.positiveInteger': 'Enter a positive integer.',
    'validation.nonNegativeInteger': 'Enter zero or a positive integer.',
    'validation.keyValuePair': 'Enter both a name and value for every header.',
    'validation.httpEndpointTimeoutRange': 'Enter a whole number from 1 to 20000.',
    'validation.httpEndpointNameLength': 'Enter a name of at most 255 characters.',
    'validation.httpEndpointDescriptionLength': 'Enter a description of at most 500 characters.',
    'validation.httpEndpointUrlUserInfo':
      'Remove the user name and password from the URL. Set credentials under Authentication.',
    'validation.httpEndpointHeaderReserved':
      'Remove reserved headers such as Authorization, Cookie, X-Api-Key, Host or Content-Type. Set credentials under Authentication instead.',
    'validation.httpEndpointHeaderDuplicate': 'Use each header name only once. Names are not case-sensitive.',
    'validation.httpsRequiredWithAuthentication':
      'Use an https URL when an authentication method is set, or set the method to None.',
    'validation.accountSid': 'Enter a valid Account SID: “AC” followed by 32 hexadecimal characters.',
    'validation.emailAddress': 'Enter a valid email address.',
    'validation.tlsRequiredForAuthentication':
      'Credentials cannot be sent over an unencrypted connection. Choose STARTTLS or implicit TLS, or set the authentication method to None.',

    // Error codes (backend)
    'errors.CON-1001': 'The requested connection category is not supported.',
    'errors.CON-1002': 'The limit parameter must be a positive integer.',
    'errors.CON-1003': 'The offset parameter must be a non-negative integer.',
    'errors.CON-1005': 'The selected authentication method is not supported by this deployment.',
    'errors.CON-1012': 'The selected authentication method is not supported.',
    'errors.CON-1014': 'The PDP endpoint changed. Enter fresh credentials before saving.',
    'errors.CON-1015': 'The service endpoint connection was not found. It may have been deleted.',
    'errors.CON-1016': 'A service endpoint connection with this name already exists.',
    'errors.CON-1017': 'Enter a connection name of 1 to 255 characters.',
    'errors.CON-1018':
      'The URL must be an absolute http or https URL without a user name or password, and must use https when authentication is set.',
    'errors.CON-1019': 'The timeout must be a whole number from 1 to 20000 milliseconds.',
    'errors.CON-1020':
      'The headers are not valid. Use each name once, do not use reserved names such as Authorization, Cookie, X-Api-Key, Host or Content-Type, and keep values on one line.',
    'errors.CON-1021':
      'The authentication settings are not valid. Check the method and enter every required credential. After changing the URL origin, enter the password again.',
    'errors.CON-1022': 'Enter a description of at most 500 characters.',
    'errors.CON-1023': 'The selected organization unit no longer exists. Choose another one and try again.',
    'errors.CON-1024': "A connection's organization unit can't be changed once it is created.",
    'errors.CON-1025': 'Choose an organization unit for this connection.',
    'errors.CON-1026':
      'This connection is used by one or more resources. Remove it from those resources before deleting it.',
    'errors.CON-1027': 'The number of service endpoint connections exceeds the maximum allowed in composite mode.',
    'errors.CON-1028': 'The selected organization unit no longer exists. Choose another one and try again.',
    'errors.CON-1029': "A connection's organization unit can't be changed once it is created.",
    'errors.CON-1030': 'Choose an organization unit for this connection.',
    'errors.IDP-1001': 'This identity provider no longer exists. It may have already been deleted.',
    'errors.IDP-1002': 'The identity provider ID is invalid or missing.',
    'errors.IDP-1003': 'The identity provider name is invalid or missing.',
    'errors.IDP-1004': 'The identity provider type is invalid or missing.',
    'errors.IDP-1005': 'An identity provider with the same name already exists.',
    'errors.IDP-1006': 'One or more identity provider properties are invalid or missing.',
    'errors.IDP-1007': 'One or more identity provider properties are not supported.',
    'errors.IDP-1008': 'The identity provider data is missing.',
    'errors.IDP-1009': 'The request body is malformed or contains invalid data.',
    'errors.IDP-1010': 'This identity provider is managed declaratively and cannot be modified or deleted.',
    'errors.IDP-1011': 'The number of records exceeds the maximum allowed in composite mode.',
    'errors.IDP-1012': "This identity provider's attribute configuration is invalid.",
    'errors.IDP-1013':
      'This identity provider cannot be deleted because other resources depend on it. Remove or reassign them first.',
    'errors.IDP-1014': 'The selected organization unit no longer exists. Choose another one and try again.',
    'errors.IDP-1015': "An identity provider's organization unit can't be changed once it is created.",
    'errors.IDP-1016': 'Choose an organization unit for this identity provider.',
    'errors.MNS-1001': 'This connection no longer exists. It may have already been deleted.',
    'errors.MNS-1002': 'The connection ID is invalid or missing.',
    'errors.MNS-1004': 'The selected SMS provider is invalid or unsupported.',
    'errors.MNS-1005': 'A connection with this name already exists.',
    'errors.MNS-1016':
      'This connection cannot be deleted because other resources depend on it. Remove or reassign them first.',
    'errors.MNS-1018': 'The selected organization unit no longer exists. Choose another one and try again.',
    'errors.MNS-1019': "A connection's organization unit can't be changed once it is created.",
    'errors.MNS-1020': 'Choose an organization unit for this connection.',
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

    // Shared by the create form and the detail page
    'idJag.acceptanceUnavailable': 'This server does not accept identity assertions from any issuer.',

    // Detail page
    'detail.back': 'Back to connections',
    'detail.loadError': 'Failed to load trusted issuer.',
    'detail.duplicateName': 'A trusted issuer with this name already exists.',
    'detail.general.title': 'General',
    'detail.general.description': 'Core identity of this trusted issuer.',
    'detail.tokenExchange.title': 'Token exchange',
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
    'detail.saveBar.reset': 'Reset',
    'detail.saveBar.save': 'Save changes',

    // Toasts
    'create.success': 'Trusted issuer created successfully.',
    'create.error': 'Failed to create trusted issuer. Please try again.',
    'update.success': 'Trusted issuer updated successfully.',
    'update.error': 'Failed to update trusted issuer. Please try again.',
    'delete.error': 'Failed to delete trusted issuer. Please try again.',

    // Not found
    'detail.notFound.title': 'Trusted issuer not found',
    'detail.notFound.description': 'This trusted issuer may have been deleted or the link is incorrect.',

    // Error codes (backend)
    'errors.IDP-1001': 'This trusted issuer no longer exists. It may have already been deleted.',
    'errors.IDP-1002': 'The trusted issuer ID is invalid or missing.',
    'errors.IDP-1003': 'The trusted issuer name is invalid or missing.',
    'errors.IDP-1004': 'The trusted issuer type is invalid or missing.',
    'errors.IDP-1005': 'A trusted issuer with the same name already exists.',
    'errors.IDP-1006': 'One or more trusted issuer properties are invalid or missing.',
    'errors.IDP-1007': 'One or more trusted issuer properties are not supported.',
    'errors.IDP-1008': 'The trusted issuer data is missing.',
    'errors.IDP-1009': 'The request body is malformed or contains invalid data.',
    'errors.IDP-1010': 'This trusted issuer is managed declaratively and cannot be modified or deleted.',
    'errors.IDP-1011': 'The number of records exceeds the maximum allowed in composite mode.',
    'errors.IDP-1012': "This trusted issuer's attribute configuration is invalid.",
    'errors.IDP-1013':
      'This trusted issuer cannot be deleted because other resources depend on it. Remove or reassign them first.',
    'errors.IDP-1014': 'The selected organization unit no longer exists. Choose another one and try again.',
    'errors.IDP-1015': "A trusted issuer's organization unit can't be changed once it is created.",
    'errors.IDP-1016': 'Choose an organization unit for this trusted issuer.',
  },

  // ============================================================================
  // Authentication namespace - Authentication feature translations
  // ============================================================================
  auth: {
    signIn: 'Sign in',
    signUp: 'Sign up',
    forgotPassword: 'Forgot password?',
    resetPassword: 'Reset password',
    acceptInvite: 'Accept invite',
    changePassword: 'Change password',
    rememberMe: 'Remember me',
    welcomeBack: 'Welcome back',
    createAccount: 'Create account',
    alreadyHaveAccount: 'Already have an account?',
    dontHaveAccount: "Don't have an account?",
    enterEmail: 'Enter your email',
    enterPassword: 'Enter your password',
    confirmPassword: 'Confirm password',
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
    title: 'Multi-factor authentication',
    setupMfa: 'Set up MFA',
    enableMfa: 'Enable MFA',
    disableMfa: 'Disable MFA',
    verificationCode: 'Verification code',
    enterCode: 'Enter verification code',
    sendCode: 'Send code',
    resendCode: 'Resend code',
    invalidCode: 'Invalid verification code',
    codeExpired: 'Verification code expired',
    scanQrCode: 'Scan this QR code with your authenticator app',
    backupCodes: 'Backup codes',
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
    title: 'Consent required',
    message: 'The application is requesting access to your information',
    requestedPermissions: 'Requested permissions',
    allow: 'Allow',
    deny: 'Deny',
    learnMore: 'Learn more',
    'authorizationDetails.approve': 'Approve',
    'authorizationDetails.showMore': 'Show more',
    'authorizationDetails.showLess': 'Show less',
    'authorizationDetails.moreItems': '+{{count}} more',
    'authorizationDetails.invalidNumber': 'Enter a number.',
    'authorizationDetails.numberMax': 'Enter {{value}} or less.',
    'authorizationDetails.numberMin': 'Enter {{value}} or more.',
    'authorizationDetails.invalidDate': 'Enter a date.',
    'authorizationDetails.dateMax': 'Choose {{value}} or earlier.',
    'authorizationDetails.dateMin': 'Choose {{value}} or later.',
    'authorizationDetails.removeItem': '{{item}}, remove',
    'authorizationDetails.addItem': '{{item}}, add',
    'authorizationDetails.addBackItem': '{{item}}, add back',
    'authorizationDetails.newItem': '{{label}}, add',
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
    'listing.addApplication': 'Add application',
    'listing.columns.name': 'Name',
    'listing.columns.clientId': 'Client ID',
    'listing.columns.status': 'Status',
    'listing.columns.actions': 'Actions',
    'listing.columns.template': 'Type',
    'listing.search.placeholder': 'Search ..',
    'listing.error': 'Failed to load applications',
    'delete.title': 'Delete application',
    'delete.message': 'Are you sure you want to delete this application? This action cannot be undone.',
    'delete.disclaimer': 'Warning: All associated data, configurations, and access tokens will be permanently removed.',
    'regenerateSecret.dialog.title': 'Regenerate client secret',
    'regenerateSecret.dialog.message':
      'Are you sure you want to regenerate the client secret for this application? This will immediately invalidate the current client secret and generate a new one.',
    'regenerateSecret.dialog.disclaimer':
      'Warning: Regenerating the client secret will invalidate the current secret and the application may stop working until the new client secret is updated in its configuration.',
    'regenerateSecret.dialog.confirmButton': 'Regenerate',
    'regenerateSecret.dialog.regenerating': 'Regenerating...',
    'regenerateSecret.dialog.error': 'Failed to regenerate client secret. Please try again.',
    'regenerateSecret.success.title': 'Save your new client secret',
    'regenerateSecret.success.subtitle': "This is the only time you'll see this secret. Store it somewhere safe.",
    'regenerateSecret.success.secretLabel': 'New client secret',
    'regenerateSecret.success.copyButton': 'Copy to clipboard',
    'regenerateSecret.success.toggleVisibility': 'Toggle secret visibility',
    'regenerateSecret.success.copySecret': 'Copy secret',
    'regenerateSecret.success.copied': 'Copied to clipboard',
    'regenerateSecret.success.securityReminder.title': 'Security reminder',
    'regenerateSecret.success.securityReminder.description':
      'Never share your client secret publicly or store it in version control. If you believe your secret has been compromised, regenerate it immediately.',
    'regenerateFlowSecret.dialog.title': 'Regenerate flow secret',
    'regenerateFlowSecret.dialog.message':
      'Are you sure you want to regenerate the flow secret for this application? This will immediately invalidate the current flow secret and generate a new one.',
    'regenerateFlowSecret.dialog.disclaimer':
      'Warning: Regenerating the flow secret invalidates the current secret. Server-side flow initiation will fail until the new flow secret is deployed.',
    'regenerateFlowSecret.dialog.confirmButton': 'Regenerate',
    'regenerateFlowSecret.dialog.regenerating': 'Regenerating...',
    'regenerateFlowSecret.dialog.error': 'Failed to regenerate flow secret. Please try again.',
    'regenerateFlowSecret.success.title': 'Save your new flow secret',
    'regenerateFlowSecret.success.subtitle':
      "This is the only time you'll see this flow secret. Store it somewhere safe.",
    'regenerateFlowSecret.success.secretLabel': 'New flow secret',
    'regenerateFlowSecret.success.copySecret': 'Copy flow secret',
    'regenerateFlowSecret.success.copied': 'Copied to clipboard',
    'regenerateFlowSecret.success.securityReminder.title': 'Security reminder',
    'regenerateFlowSecret.success.securityReminder.description':
      'Never share your flow secret publicly or store it in version control. If you believe your flow secret has been compromised, regenerate it immediately.',
    'regenerateFlowSecret.snackbar.success': 'Flow secret regenerated successfully.',
    'onboarding.preview.title': 'Preview',
    'onboarding.preview.signin': 'Sign in',
    'onboarding.preview.username': 'Username',
    'onboarding.preview.usernamePlaceholder': 'Enter your username',
    'onboarding.preview.password': 'Password',
    'onboarding.preview.passwordPlaceholder': 'Enter your password',
    'onboarding.preview.signInButton': 'Sign in',
    'onboarding.preview.passkeySignIn': 'Sign in with passkey',
    'onboarding.preview.mobileNumber': 'Mobile number',
    'onboarding.preview.mobileNumberPlaceholder': 'Enter your mobile number',
    'onboarding.preview.sendOtpButton': 'Send OTP',
    'onboarding.preview.dividerText': 'or',
    'onboarding.preview.continueWith': 'Continue with {{providerName}}',
    'onboarding.preview.stepOf': 'Step {{n}} of {{total}}',
    'onboarding.steps.organizationUnit': 'Organization unit',
    'onboarding.steps.details': 'Details',
    'onboarding.steps.design': 'Experience',
    'onboarding.steps.security': 'Security',
    'onboarding.steps.configure': 'Configuration',
    'onboarding.steps.quickTest': 'Quick test',
    'onboarding.steps.export': 'Integration setup',
    'onboarding.steps.complete': 'Setup complete',
    'onboarding.steps.summary': 'Summary',
    'onboarding.organizationUnit.title': 'Where should this application belong?',
    'onboarding.organizationUnit.subtitle':
      "Choose the organization unit that will own this application. You can't change this once created.",
    'onboarding.organizationUnit.fieldLabel': 'Organization unit',
    'onboarding.configure.name.title': "Let's give a name to your application",
    'onboarding.configure.name.fieldLabel': 'Name & logo',
    'onboarding.configure.name.placeholder': 'Enter your application name',
    'onboarding.configure.name.logoAriaLabel': 'Change application logo',
    'onboarding.configure.name.maxLength': 'Application name cannot exceed {{max}} characters',
    'onboarding.configure.applicationDetails.title': "Let's collect some details about your application",
    'onboarding.configure.applicationDetails.ouDefaults.title': 'Use organization defaults',
    'onboarding.configure.applicationDetails.ouDefaults.subtitle':
      "Inherit {{ouName}}'s flows, theme & layout instead of configuring from scratch.",
    'onboarding.configure.applicationDetails.ouDefaults.flows.title': 'Sign-in, sign-up & recovery flows',
    'onboarding.configure.applicationDetails.ouDefaults.flows.description':
      'Follow the flows configured for {{ouName}}',
    'onboarding.configure.applicationDetails.ouDefaults.design.title': 'Design',
    'onboarding.configure.applicationDetails.ouDefaults.design.description':
      'Use the same theme & layout as {{ouName}}',
    'onboarding.configure.applicationDetails.userAccess.title': 'Allow all user types to sign up for this application',
    'onboarding.configure.applicationDetails.userAccess.subtitle': 'Users can sign up as any user type',
    'onboarding.configure.applicationDetails.userAccess.placeholder': 'Select user types',
    'onboarding.mcp.clientType.title': 'Client type',
    'onboarding.mcp.clientType.subtitle': 'How will this client obtain tokens?',
    'onboarding.mcp.clientType.userDelegated.title': 'On behalf of a user',
    'onboarding.mcp.clientType.userDelegated.description':
      'A client in a host app (IDE, desktop app, or chat client) that acts on behalf of a signed-in user. Uses authorization code with PKCE.',
    'onboarding.mcp.clientType.m2m.title': 'On its own behalf',
    'onboarding.mcp.clientType.m2m.description':
      'A client that authenticates with its own credentials without user interaction. Uses client credentials.',
    'onboarding.mcp.clientType.preview.label': 'What you get',
    'onboarding.mcp.clientType.confidentialClient.label': 'Confidential client',
    'onboarding.mcp.clientType.confidentialClient.description':
      'Turn on for clients with a backend that can keep a secret. Required to use identity assertions (ID-JAG).',
    'onboarding.mcp.connection.title': 'Add a redirect URI',
    'onboarding.mcp.connection.subtitle': 'Where should users be sent after they authorize this client?',
    'onboarding.mcp.connection.redirectUris.label': 'Redirect URIs',
    'onboarding.mcp.connection.redirectUris.hint':
      'Each URI must be a loopback address (http://localhost or http://127.0.0.1) or use HTTPS. At least one is required.',
    'onboarding.mcp.connection.redirectUris.addUri': 'Add redirect URI',
    'onboarding.mcp.connection.redirectUris.remove': 'Remove redirect URI',
    'onboarding.mcp.connection.redirectUris.error.empty': 'Enter a redirect URI.',
    'onboarding.mcp.connection.redirectUris.error.invalid': 'Enter a valid loopback (http://127.0.0.1) or HTTPS URI.',
    'onboarding.mcp.connection.inspectorHint.prefix': 'Testing with MCP Inspector? Use',
    'onboarding.mcp.oauthProfile.label': 'OAuth profile',
    'onboarding.mcp.oauthProfile.authCodePkce': 'Authorization code + PKCE (required)',
    'onboarding.mcp.oauthProfile.publicClient': 'Public client',
    'onboarding.mcp.oauthProfile.refreshTokens': 'Refresh tokens',
    'onboarding.mcp.oauthProfile.clientCredentials': 'Client credentials',
    'onboarding.mcp.oauthProfile.confidentialClient': 'Confidential client',
    'onboarding.mcp.oauthProfile.clientSecretIssued': 'Client secret issued',
    'onboarding.configure.design.title': 'Customize the appearance of your application',
    'onboarding.configure.design.theme.title': 'Theme',
    'onboarding.configure.design.theme.emptyState': 'No themes configured',
    'onboarding.configure.design.theme.noDescription': 'No description',
    'onboarding.configure.design.theme.emptyStateHint': 'You can configure themes later from the Design settings.',
    'onboarding.configure.design.layout.title': 'Layout',
    'onboarding.configure.security.title': 'Choose how people sign in to your application',
    'onboarding.configure.SignInOptions.title': 'Sign in options',
    'onboarding.configure.SignInOptions.subtitle': 'Choose how users will sign-in to your application',
    'onboarding.configure.SignInOptions.usernamePassword': 'Username & password',
    'onboarding.configure.SignInOptions.google': 'Google',
    'onboarding.configure.SignInOptions.github': 'GitHub',
    'onboarding.configure.SignInOptions.passkey': 'Passkey',
    'onboarding.configure.SignInOptions.magicLink': 'Magic link',
    'onboarding.configure.SignInOptions.magicLink.hint': "Send a one-time sign-in link to the user's email",
    'onboarding.configure.SignInOptions.notConfigured': 'Not configured',
    'onboarding.configure.SignInOptions.noFlowFound':
      'No flow found for the selected sign-in options. Please try a different combination.',
    'onboarding.configure.SignInOptions.noSelectionWarning':
      'At least one sign-in option is required. Please select at least one authentication method.',
    'onboarding.configure.SignInOptions.noIntegrations':
      'No social sign-in integrations available. Please configure an integration first.',
    'onboarding.configure.SignInOptions.hint':
      'You can always change these settings later in the application settings.',
    'onboarding.configure.SignInOptions.preConfiguredFlows.searchFlows': 'Search flows...',
    'onboarding.configure.SignInOptions.preConfiguredFlows.toggleLabel': 'Use a pre-configured flow instead',
    'onboarding.configure.SignInOptions.smsOtp': 'SMS OTP',
    'onboarding.configure.SignInOptions.loading': 'Loading...',
    'onboarding.configure.SignInOptions.error': 'Failed to load authentication methods',
    'onboarding.configure.security.promptForCredentials.title': 'Prompt for credentials',
    'onboarding.configure.security.passwordlessLogin.title': 'Passwordless login',
    'onboarding.configure.security.socialLogin.title': 'Social login',
    'onboarding.configure.security.mfa.title': 'Multi-factor login',
    'onboarding.configure.security.mfa.disabledForPreConfiguredFlow':
      "MFA isn't available for the selected pre-configured flow. Clear it, or choose individual sign-in methods above instead.",
    'onboarding.configure.security.mfa.emailOtp': 'Email OTP',
    'onboarding.configure.security.mfa.emailOtp.hint': "Send a one-time code to the user's email",
    'onboarding.configure.security.mfa.smsOtp.hint': 'Send a one-time code via SMS',
    'onboarding.configure.security.mfa.smsOtp.noSenders':
      'Connect an SMS provider under Connections to enable SMS OTP.',
    'onboarding.configure.approach.title': 'Sign-in approach',
    'onboarding.configure.approach.subtitle': 'Select how users will access this application',
    'onboarding.configure.approach.inbuilt.title': 'Redirect to {{product}} Gate',
    'onboarding.configure.approach.inbuilt.description':
      "Users are redirected to Gate, {{product}}'s hosted sign-in and sign-up experience. Brand it in minutes with the Flow Designer, then wire it up with an SDK.",
    'onboarding.configure.approach.inbuilt.preview.title': 'Sign in to {{appName}}',
    'onboarding.configure.approach.native.title': 'Bring your own UI',
    'onboarding.configure.approach.native.description':
      "Build your own sign-in and sign-up screens and call {{product}}'s APIs directly, or move faster with our SDK suite: prebuilt components, hooks, and helpers for web and mobile.",
    'onboarding.configure.approach.native.attestationNotice':
      'Mobile apps authenticate to the Flow Execution API using platform attestation instead of a flow secret. After creating this application, configure attestation under Advanced settings to access the flow APIs.',
    'onboarding.configure.stack.title': 'Choose a type',
    'onboarding.configure.stack.subtitle': 'Select the type that best matches your application.',
    'onboarding.templateSelect.subtitle':
      'Pick the technology that best matches your application, selecting one starts the setup.',
    'onboarding.templateSelect.searchPlaceholder': 'Search types by name',
    'onboarding.templateSelect.count_one': 'Showing {{count}} type',
    'onboarding.templateSelect.count_other': 'Showing {{count}} types',
    'onboarding.configure.stack.categoriesLabel': 'Categories',
    'onboarding.configure.stack.comingSoon': 'Coming soon',
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
    'onboarding.configure.stack.technology.flutter.title': 'Flutter',
    'onboarding.configure.stack.technology.flutter.description': 'Cross-platform mobile application built with Flutter',
    'onboarding.configure.stack.technology.springboot.title': 'Spring Boot',
    'onboarding.configure.stack.technology.springboot.description': 'Java backend application with Spring Boot',
    'onboarding.configure.stack.technology.nodejs.title': 'Node.js',
    'onboarding.configure.stack.technology.nodejs.description': 'Backend service built with Node.js',
    'onboarding.configure.stack.technology.nuxt.title': 'Nuxt',
    'onboarding.configure.stack.technology.nuxt.description': 'Full-stack Vue framework with server-side rendering',
    'onboarding.configure.stack.technology.vanillaJs.title': 'JavaScript',
    'onboarding.configure.stack.technology.vanillaJs.description': 'Browser application built with vanilla JavaScript',
    'onboarding.configure.stack.technology.mcpClient.title': 'MCP client',
    'onboarding.configure.stack.technology.mcpClient.description':
      'Register an MCP client to connect to MCP servers with OAuth 2.1 authorization.',
    'onboarding.configure.stack.platform.title': 'Application type',
    'onboarding.configure.stack.platform.subtitle': 'This helps us configure the right settings for your app',
    'onboarding.configure.stack.dividerLabel': 'OR',
    'onboarding.configure.stack.platform.browser.title': 'Browser app',
    'onboarding.configure.stack.platform.browser.description': 'Single-page apps running in browsers',
    'onboarding.configure.stack.platform.full_stack.title': 'Full-stack app',
    'onboarding.configure.stack.platform.full_stack.description': 'Apps with both server and client code',
    'onboarding.configure.stack.platform.mobile.title': 'Mobile app',
    'onboarding.configure.stack.platform.mobile.description': 'Native or hybrid mobile applications',
    'onboarding.configure.stack.platform.backend.title': 'Backend service',
    'onboarding.configure.stack.platform.backend.description': 'Server-to-server APIs and services',
    'onboarding.configure.stack.platform.wallet.title': 'Digital wallet',
    'onboarding.configure.stack.platform.wallet.description': 'OpenID4VCI wallet that requests verifiable credentials',
    'onboarding.configure.stack.platform.cimd.title': 'CIMD',
    'onboarding.configure.stack.platform.cimd.description':
      'Register a client from the metadata document its vendor publishes.',
    'onboarding.configure.stack.platform.custom.title': 'Custom',
    'onboarding.configure.stack.platform.custom.description':
      'Fully customizable application with all configuration options available',
    'onboarding.configure.details.wallet.title': 'Wallet',
    'onboarding.configure.details.wallet.vendor.custom': 'Custom',
    'onboarding.configure.details.wallet.clientId.label': 'Client ID',
    'onboarding.configure.details.wallet.clientId.placeholder': 'The wallet’s client ID',
    'onboarding.configure.details.wallet.clientId.helperText':
      'Leave empty to auto-generate. Known wallets pre-fill their fixed client ID.',
    'onboarding.configure.details.wallet.prefilled.helperText': 'Pre-filled for the selected wallet and not editable.',
    'onboarding.configure.details.wallet.duplicate.known':
      'A {{vendor}} wallet application already exists. Each wallet can be connected only once.',
    'onboarding.configure.details.wallet.duplicate.custom':
      'An application with this client ID already exists. Enter a different client ID.',
    'onboarding.configure.details.redirectUris.placeholder': 'https://example.com/callback',
    'onboarding.configure.details.postLogoutRedirectUris.placeholder': 'https://example.com/logged-out',
    'onboarding.configure.details.urls.title': 'URLs',
    'onboarding.configure.details.hostingUrl.label': 'Where is your application hosted?',
    'onboarding.configure.details.hostingUrl.placeholder': 'https://myapp.example.com',
    'onboarding.configure.details.hostingUrl.helperText': 'The URL where users will access your application',
    'onboarding.configure.details.hostingUrl.error.required': 'Application hosting URL is required',
    'onboarding.configure.details.hostingUrl.error.invalid':
      'Please enter a valid URL (must start with http:// or https://)',
    'onboarding.configure.details.callbackUrl.label': 'After sign-in URL',
    'onboarding.configure.details.callbackUrl.placeholder': 'https://myapp.example.com/callback',
    'onboarding.configure.details.callbackUrl.helperText': 'The URL where users will be redirected after signing in',
    'onboarding.configure.details.callbackUrl.error.required': 'After sign-in URL is required',
    'onboarding.configure.details.callbackUrl.error.invalid':
      'Please enter a valid URL (must start with http:// or https://)',
    'onboarding.configure.details.callbackUrl.info':
      'The URL is where users will be redirected after sign-in. For most applications, using the same URL as your application access URL is recommended.',
    'onboarding.configure.details.sameAsHosting': 'Same as the application URL',
    'onboarding.configure.details.callbackMode.same': 'Same as application access URL',
    'onboarding.configure.details.callbackMode.custom': 'Custom URL',
    'onboarding.configure.details.mobile.title': 'Mobile application configuration',
    'onboarding.configure.details.mobile.info':
      'Deep links (e.g., myapp://callback) or universal links (e.g., https://example.com/callback) are used to redirect users back to your mobile app after authentication.',
    'onboarding.configure.details.deeplink.label': 'Deep link / Universal link',
    'onboarding.configure.details.deeplink.placeholder': 'myapp://callback or https://example.com/callback',
    'onboarding.configure.details.deeplink.helperText': 'The custom URL scheme or universal link for your mobile app',
    'onboarding.configure.details.passkey.title': 'Passkeys',

    'onboarding.configure.passkey.title': 'Passkey configuration',
    'onboarding.configure.passkey.description': 'Configure the Relying Party details for Passkey authentication.',
    'onboarding.configure.details.relyingPartyId.label': 'Relying party ID',
    'onboarding.configure.details.relyingPartyId.placeholder': 'e.g., example.com',
    'onboarding.configure.details.relyingPartyId.helperText': 'The domain where the WebAuthn credential is valid',
    'onboarding.configure.details.relyingPartyName.label': 'Relying party name',
    'onboarding.configure.details.relyingPartyName.placeholder': 'e.g., my application',
    'onboarding.configure.details.relyingPartyName.helperText': 'A user-friendly name for the relying party',
    'onboarding.configure.details.userTypes.error': 'Please select at least one user type',
    'onboarding.configure.setup.title': 'Application setup',
    'onboarding.configure.setup.subtitle': 'Select the technology stack for your application',
    'onboarding.configure.setup.platform.label': 'What technology are you using?',
    'onboarding.configure.setup.platform.browser.title': 'Browser',
    'onboarding.configure.setup.platform.browser.description': 'Single page apps (React, Vue, Angular)',
    'onboarding.configure.setup.platform.full_stack.title': 'Server + browser',
    'onboarding.configure.setup.platform.full_stack.description': 'Full-stack apps (Next.js, Remix)',
    'onboarding.configure.setup.platform.mobile.title': 'Mobile device',
    'onboarding.configure.setup.platform.mobile.description': 'iOS, Android, React Native',
    'onboarding.configure.setup.platform.desktop.title': 'Desktop',
    'onboarding.configure.setup.platform.desktop.description': 'Electron, Tauri apps',
    'onboarding.configure.setup.platform.backend.title': 'Backend service',
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
    'onboarding.configure.oauth.publicClient.label': 'Public client',
    'onboarding.configure.oauth.pkce.label': 'Enable PKCE',
    'onboarding.configure.oauth.redirectURIs.fieldLabel': 'Redirect URIs',
    'onboarding.configure.oauth.redirectURIs.placeholder': 'https://localhost:3000/callback',
    'onboarding.configure.oauth.redirectURIs.addButton': 'Add',
    'onboarding.configure.oauth.redirectURIs.errors.empty': 'Please enter a redirect URI',
    'onboarding.configure.oauth.redirectURIs.errors.invalid':
      'Please enter a valid URL (must start with http:// or https://)',
    'onboarding.configure.oauth.redirectURIs.errors.duplicate': 'This redirect URI has already been added',
    'onboarding.configure.oauth.grantTypes.label': 'Grant types',
    'onboarding.configure.oauth.grantTypes.authorizationCode': 'Authorization code',
    'onboarding.configure.oauth.grantTypes.refreshToken': 'Refresh token',
    'onboarding.configure.oauth.grantTypes.clientCredentials': 'Client credentials',
    'onboarding.configure.oauth.tokenEndpointAuthMethod.label': 'Client authentication method',
    'onboarding.configure.oauth.tokenEndpointAuthMethod.clientSecretBasic': 'Client secret basic',
    'onboarding.configure.oauth.tokenEndpointAuthMethod.clientSecretPost': 'Client secret post',
    'onboarding.configure.oauth.tokenEndpointAuthMethod.none': 'None',
    'onboarding.configure.oauth.errors.publicClientRequiresPKCE':
      'Public clients must have PKCE enabled. PKCE is automatically enabled for public clients.',
    'onboarding.configure.oauth.errors.publicClientRequiresNone':
      'Public clients must use "None" as the token endpoint authentication method.',
    'onboarding.configure.oauth.errors.publicClientNoClientCredentials':
      'Public clients cannot use the client_credentials grant type.',
    'onboarding.configure.oauth.errors.authorizationCodeRequiresRedirectURIs':
      'Authorization code grant type requires at least one redirect URI.',
    'onboarding.configure.oauth.errors.clientCredentialsRequiresAuth':
      'Client credentials grant type cannot use "None" authentication method.',
    'onboarding.configure.oauth.errors.atLeastOneGrantTypeRequired': 'At least one grant type must be selected.',
    'onboarding.configure.oauth.errors.refreshTokenRequiresAuthorizationCode':
      'Refresh token grant type requires authorization code grant type to be selected.',
    'onboarding.complete.title': 'Application created successfully',
    'onboarding.complete.description':
      'Your application has been created. Please save the client secret below as it will only be shown once.',
    'onboarding.complete.warning.title': 'Important: Save your client secret',
    'onboarding.complete.warning.message':
      'This is the only time you will see this client secret. Please copy it now and store it securely. You will not be able to retrieve it later.',
    'onboarding.complete.clientSecret.label': 'Client secret',
    'onboarding.creating': 'Creating...',
    'onboarding.skipAndCreate': 'Skip & create',
    'onboarding.createApplication': 'Create application',
    'quickTest.title': 'Test your application',
    'quickTest.subtitle': 'Verify that your sign-in configuration works with a test user before going live.',
    'quickTest.summary.text': '{{appName}} is configured with {{methods}} sign-in.',
    'quickTest.summary.method.usernamePassword': 'username & password',
    'quickTest.summary.method.passkey': 'passkeys',
    'quickTest.summary.method.social': '{{count}} social provider',
    'quickTest.summary.method.social_plural': '{{count}} social providers',
    'quickTest.summary.and': 'and',
    'quickTest.createUser.title': 'Test user',
    'quickTest.createUser.subtitle': 'Create a test user with generated credentials to test your sign-in flow.',
    'quickTest.createUser.usernameLabel': 'Username',
    'quickTest.createUser.passwordLabel': 'Password',
    'quickTest.createUser.creating': 'Creating test user...',
    'quickTest.createUser.success': 'Test user created successfully.',
    'quickTest.createUser.error': 'Failed to create test user. You can still proceed.',
    'quickTest.createUser.regenerate': 'Regenerate',
    'quickTest.createUser.button': 'Create test user',
    'quickTest.createUser.details.title': 'Created user',
    'quickTest.createUser.details.userId': 'User ID',
    'quickTest.createUser.details.type': 'Type',
    'quickTest.testLogin.title': 'Test sign-in',
    'quickTest.testLogin.subtitle': 'Sign in with the test user credentials to verify your authentication setup.',
    'quickTest.testLogin.button': 'Test sign in',
    'quickTest.testLogin.testing': 'Testing...',
    'quickTest.testLogin.error': 'Sign-in test failed.',
    'quickTest.testLogin.successTitle': 'Sign-in successful!',
    'quickTest.testLogin.tokenType': 'Type',
    'quickTest.testLogin.expiresIn': 'Expires in',
    'quickTest.testLogin.scope': 'Scope',
    'quickTest.testLogin.idTokenReceived': 'ID token received.',
    'quickTest.openSampleApp': 'Open sample app',
    'quickTest.copied': 'Copied!',
    'export.title': 'Organization export summary',
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
    'export.preflight.title': 'Pre-flight check',
    'export.preflight.samlWarning': '"Admin Console" SAML certificate is missing',
    'export.preflight.secretNote': '2 client secrets will be encrypted using the target environment key',
    'export.actions.saveAndExit': 'Save and exit',
    'export.actions.exportConfig': 'Export config',
    'setupComplete.title': 'All done!',
    'setupComplete.subtitle': '{{appName}} is ready. Start integrating sign-in into your application.',
    'setupComplete.appNameLabel': 'Application name',
    'setupComplete.clientIdLabel': 'Client ID',
    'setupComplete.copied': 'Copied!',
    'setupComplete.securityNote':
      'Your client secret was shown on the previous screen. Store it securely — it cannot be retrieved again.',
    'setupComplete.goToDashboard': 'Go to dashboard',
    'setupComplete.viewApplication': 'View application',
    'onboarding.summary.title': 'Application created successfully!',
    'onboarding.summary.subtitle': 'Your application is ready to use',
    'onboarding.summary.viewAppAriaLabel': 'Click to view application details',
    'onboarding.summary.appDetails': 'Application successfully created',
    'onboarding.summary.guides.subtitle': 'Choose how you want to integrate sign-in to your application',
    'onboarding.summary.guides.divider': 'or',
    'clientSecret.saveTitle': 'Save your client secret',
    'clientSecret.saveSubtitle': "This is the only time you'll see this secret. Store it somewhere safe.",
    'clientSecret.warning':
      "Make sure to copy your client secret now. You won't be able to see it again for security reasons.",
    'clientSecret.clientIdLabel': 'Client ID',
    'clientSecret.clientSecretLabel': 'Client secret',
    'clientSecret.purpose': 'Used to authenticate your application at the OAuth 2 token endpoint.',
    'clientSecret.copied': 'Copied to clipboard',
    'clientSecret.copySecret': 'Copy client secret',
    'clientSecret.securityReminder.title': 'Security reminder',
    'clientSecret.securityReminder.description':
      'Your client secret is a confidential key used to authenticate your application. It should be treated with the same level of security as a password. Never expose it in browser console, version control, or logs.',
    'flowSecret.label': 'Flow secret',
    'flowSecret.purpose':
      'Used to authenticate your server when it starts a sign-in flow directly via the Flow Execution API.',
    'flowSecret.saveTitle': 'Save your flow secret',
    'flowSecret.saveSubtitle': "This is the only time you'll see this secret. Store it somewhere safe.",
    'flowSecret.copySecret': 'Copy flow secret',
    'flowSecret.securityReminder.description':
      'Your flow secret is a confidential key used to authenticate your application when it starts a sign-in flow. It should be treated with the same level of security as a password. Never expose it in browser console, version control, or logs.',
    'secrets.saveTitle': 'Save your secrets',
    'secrets.saveSubtitle': "This is the only time you'll see these secrets. Store them somewhere safe.",
    'secrets.securityReminder.description':
      'These secrets are confidential keys used to authenticate your application. They should be treated with the same level of security as passwords. Never expose them in browser console, version control, or logs.',
    'view.title': 'Application details',
    'view.subtitle': 'View application details and configuration',
    'view.sections.basicInformation': 'Basic information',
    'view.sections.flowConfiguration': 'Flow configuration',
    'view.sections.userAttributes': 'User attributes',
    'view.sections.oauth2Configuration': 'OAuth 2 configuration',
    'view.sections.timestamps': 'Timestamps',
    'view.fields.applicationId': 'Application ID',
    'view.fields.description': 'Description',
    'view.fields.url': 'URL',
    'view.fields.tosUri': 'Terms of service URI',
    'view.fields.policyUri': 'Privacy policy URI',
    'view.fields.contacts': 'Contacts',
    'view.fields.authFlowId': 'Authentication flow ID',
    'view.fields.registrationFlowId': 'Registration flow ID',
    'view.fields.registrationFlowEnabled': 'Registration flow enabled',
    'view.fields.clientId': 'Client ID',
    'view.fields.redirectUris': 'Redirect URIs',
    'view.fields.grantTypes': 'Grant types',
    'view.fields.responseTypes': 'Response types',
    'view.fields.scopes': 'Scopes',
    'view.fields.publicClient': 'Public client',
    'view.fields.pkceRequired': 'PKCE required',
    'view.fields.createdAt': 'Created at',
    'view.fields.updatedAt': 'Updated at',
    'view.values.yes': 'Yes',
    'view.values.no': 'No',
    'edit.customization.tosUri.hint':
      "URL to your application's Terms of Service. May be displayed to users during consent or user sign-in, sign-up or recovery flows.",
    'edit.customization.policyUri.hint':
      "URL to your application's Privacy Policy. May be displayed to users during consent or user sign-in, sign-up or recovery flows.",
    'edit.advanced.oauth2Config.intro': 'Manage OAuth 2 settings for this {{entity}}.',
    'edit.advanced.redirectUris.hint':
      'Allowed callback URLs where users will be redirected after authentication. Must be exact matches for security.',
    'edit.advanced.grantTypes.placeholder': 'Select grant types',
    'edit.advanced.grantTypes.hint':
      'OAuth 2 flows this {{entity}} can use (e.g., authorization_code, client_credentials, refresh_token).',
    'edit.advanced.responseTypes.placeholder': 'Select response types',
    'edit.advanced.publicClient.public':
      'This is a public client (SPA, mobile app) that cannot securely store credentials.',
    'edit.advanced.publicClient.confidential':
      'This is a confidential client (server-side app) that can securely store credentials.',
    'edit.advanced.pkce.enabled': 'PKCE is required for authorization code flow, providing additional security.',
    'edit.advanced.pkce.disabled': 'PKCE is not required. Consider enabling for public clients (SPAs, mobile apps).',
    'edit.advanced.pkce.requiresAuthorizationCode': 'PKCE applies only to the authorization code flow.',
    'edit.advanced.pkce.requiredForPublicClient': 'Always required for public clients.',
    'edit.advanced.pkce.requiredForMetadataDocument':
      'Always required for clients registered from a metadata document.',
    'edit.advanced.publicClient.setByMetadataDocument': "Set by the client's metadata document.",
    'edit.advanced.publicClient.requiresAuthorizationCode':
      'Available only for clients using the authorization code flow.',
    'edit.advanced.publicClient.incompatibleWithClientCredentials': 'Not available for machine-to-machine clients.',
    'edit.advanced.publicClient.incompatibleWithJwtBearer':
      'Not available with the JWT bearer grant, which requires a confidential client.',
    'edit.advanced.publicClient.incompatibleWithIdJag':
      'Not available while identity assertions (ID-JAG) are on, since they require a confidential client.',
    'edit.advanced.responseTypes.codeRequiredHint': 'Required for the authorization code flow.',
    'edit.advanced.responseTypes.notApplicable': 'Response types apply only to the authorization code flow.',
    'edit.advanced.tokenEndpointAuthMethod.placeholder': 'Select authentication method',
    'edit.advanced.tokenEndpointAuthMethod.hint':
      'Defines how the client authenticates at protected endpoints. Use a confidential method (for example, client_secret_basic, client_secret_post, or private_key_jwt) for confidential clients, and none for public clients.',
    'edit.advanced.tokenEndpointAuthMethod.lockedHint': 'Locked to "none" because the client is public.',
    'edit.advanced.tokenEndpointAuthMethod.metadataDocumentLockedHint': "Set by the client's metadata document.",
    'edit.advanced.lockedByTemplate': 'Set by template',
    'edit.advanced.lockedByMetadataDocument': "Set by the client's metadata document",
    'edit.advanced.certificate.intro': 'Configure certificates for client authentication and token encryption.',
    'edit.advanced.certificate.type.none': 'None',
    'edit.advanced.certificate.type.jwks': 'JWKS (inline JSON Web Key Set)',
    'edit.advanced.certificate.type.jwksUri': 'JWKS URI (URL to JWKS endpoint)',
    'edit.advanced.labels.attestation': 'Platform attestation',
    'edit.advanced.attestation.intro':
      'Verify the binary identity of a mobile client when it initiates a flow directly. Choose the platform the application is built for.',
    'edit.advanced.attestation.labels.platform': 'Platform',
    'edit.advanced.attestation.platform.none': 'None',
    'edit.advanced.attestation.platform.android': 'Android (Play Integrity)',
    'edit.advanced.attestation.platform.apple': 'iOS (App Attest)',
    'edit.advanced.attestation.labels.packageName': 'Package name',
    'edit.advanced.attestation.labels.certificateSha256Digests': 'Signing certificate SHA-256 digests',
    'edit.advanced.attestation.labels.serviceAccountCredentials': 'Service account credentials',
    'edit.advanced.attestation.placeholder.packageName': 'com.example.myapp',
    'edit.advanced.attestation.placeholder.certificateSha256Digest': 'URL-safe base64 SHA-256 digest',
    'edit.advanced.attestation.placeholder.serviceAccountCredentials': 'Paste the Google Cloud service account JSON',
    'edit.advanced.attestation.hint.packageName':
      'The Android application package name that must match the attested app.',
    'edit.advanced.attestation.hint.certificateSha256Digests':
      'Allowed signing certificate digests, in the URL-safe base64 form reported by Play Integrity. The attested app must match one of these.',
    'edit.advanced.attestation.hint.serviceAccountCredentials':
      'Write-only. Used to call the Play Integrity API. Leave blank to keep the existing credentials.',
    'edit.advanced.attestation.addDigest': 'Add digest',
    'edit.advanced.attestation.labels.teamId': 'Team ID',
    'edit.advanced.attestation.labels.bundleId': 'Bundle ID',
    'edit.advanced.attestation.placeholder.teamId': 'ABCDE12345',
    'edit.advanced.attestation.placeholder.bundleId': 'com.example.myapp',
    'edit.advanced.attestation.hint.teamId': 'The Apple Developer Team ID.',
    'edit.advanced.attestation.hint.bundleId': 'The iOS bundle identifier that must match the attested app.',
    'edit.advanced.attestation.error.appleIncomplete': 'Both team ID and bundle ID are required together.',
    'edit.advanced.attestation.labels.devMode': 'Dev mode',
    'edit.advanced.attestation.hint.devMode':
      'Skips attestation verification for this application. Enable only for testing or trying out ' +
      'sample/development mobile clients; leave disabled otherwise.',
    'edit.advanced.attestation.warning.devMode':
      'Dev mode is enabled. Attestation verification is skipped for this application. Use this only for ' +
      'testing, do not enable it in production.',
    'edit.advanced.attestation.devModeConfirmDialog.title': 'Enable dev mode?',
    'edit.advanced.attestation.devModeConfirmDialog.description':
      'This skips attestation verification for this application, so it can initiate a sign-in flow ' +
      'without presenting an attestation token. Use it only for testing, or to try out a sample or ' +
      'development client. Do not enable it in production.',
    'edit.advanced.attestation.devModeConfirmDialog.cancelButton': 'Cancel',
    'edit.advanced.attestation.devModeConfirmDialog.confirmButton': 'Enable dev mode',

    /* Passkeys section */
    'edit.advanced.labels.passkeys': 'Passkeys',
    'edit.advanced.passkeys.intro': 'Passkey settings for this application.',
    'edit.advanced.passkeys.serverFallbackHint':
      'Origins listed here are automatically included in the server CORS allowed origins.',
    'edit.advanced.passkeys.allowedOrigins.label': 'Allowed origins',
    'edit.advanced.passkeys.allowedOrigins.hint':
      'Allowed origins for passkey operations initiated through this application.',
    'edit.advanced.passkeys.allowedOrigins.placeholder': 'https://app.example.com',
    'edit.advanced.passkeys.allowedOrigins.addOrigin': 'Add origin',
    'edit.advanced.passkeys.allowedOrigins.error.empty': 'Origin cannot be empty',
    'edit.advanced.passkeys.allowedOrigins.error.invalid': 'Enter a valid URL',

    /* -------------------- Edit page -------------------- */
    // Common
    'edit.page.error': 'Failed to load application information',
    'edit.page.notFound': 'Application not found',
    'edit.page.back': 'Back to applications',
    'edit.page.logoUpdate.label': 'Update logo',
    'edit.page.description.empty': 'No description',
    'edit.page.description.placeholder': 'Add a description',
    'edit.page.tabs.overview': 'Overview',
    'edit.page.tabs.general': 'General',
    'edit.page.tabs.access': 'Access',
    'edit.page.tabs.credentials': 'Credentials',
    'edit.page.tabs.flows': 'Flows',
    'edit.page.tabs.customization': 'Customization',
    'edit.page.tabs.token': 'Token',
    'edit.page.tabs.organizationAccess': 'Organization Access',
    'edit.organizationAccess.save.error': 'Failed to update organization access. Please try again.',
    'organizationAccess.title': 'Where {{resource}} is available',
    'organizationAccess.description': '{{resource}} can request tokens to act within the units selected here.',
    'organizationAccess.ownerOnly': "{{resource}} isn't available in any other unit.",
    'edit.page.tabs.advanced': 'Advanced',
    'edit.page.unsavedChanges': 'Unsaved changes',
    'edit.page.validation.missingRedirectUri': 'A redirect URI is required.',
    'edit.page.validation.missingCertificate': 'A certificate is required.',
    'edit.page.validation.invalidBackchannelLogoutUri': 'The back-channel logout URI is not valid.',
    'edit.page.reset': 'Reset',
    'edit.page.save': 'Save',
    'edit.page.saving': 'Saving...',
    'edit.userAccessLock.message':
      'These settings apply only to user-facing flows. Enable a user-facing grant (e.g. authorization code) in the Advanced tab to configure them.',

    'edit.mcp.connect.sections.identity': 'Connection',
    'edit.mcp.connect.sections.identity.description': 'Client identity for connecting to MCP servers.',
    'edit.mcp.connect.profileBadge.userDelegated': 'On behalf of a user (authorization code + PKCE)',
    'edit.mcp.connect.profileBadge.m2m': 'On its own behalf (client credentials)',
    'edit.mcp.connect.profileBadge.jwtBearer': 'On behalf of a user (JWT bearer with ID-JAG)',
    'edit.mcp.connect.credentialsPointer':
      'Manage the client secret and keys on the <credentialsLink>Credentials tab</credentialsLink>.',
    'edit.mcp.connect.redirectUris.optionalHint':
      'Each URI must be a loopback address (http://localhost or http://127.0.0.1) or use HTTPS. Needed only for the authorization code grant.',
    'edit.mcp.connect.sections.endpoints': 'Endpoints',
    'edit.mcp.connect.sections.endpoints.description': 'ThunderID OAuth 2.1 endpoints for this client.',
    'edit.mcp.connect.clientUri.label': 'Client URI',
    'edit.mcp.connect.clientUri.hint': 'Public homepage of this client (optional).',
    'edit.mcp.connect.clientUri.error.invalid': 'Please enter a valid URL',

    // Overview section
    'edit.overview.stackblitz.heading': 'Try the live quickstart',
    'edit.overview.stackblitz.subheading': 'Run {{name}} in StackBlitz',
    'edit.overview.stackblitz.cta': 'Open on StackBlitz',
    'edit.overview.readGuide.title': 'Read the quickstart guide',
    'edit.overview.readGuide.description': 'Step-by-step integration guide on the docs site.',
    'edit.overview.readGuide.titleFor': '{{label}} quickstart guide',
    'edit.overview.readGuide.descriptionFor': 'Step-by-step {{label}} integration guide on the docs site.',
    'edit.overview.readGuide.action': 'Open quickstart',
    'edit.overview.agentPrompt.title': 'Integrate with a coding agent',
    'edit.overview.agentPrompt.badge': 'AI',
    'edit.overview.agentPrompt.description': 'Copy a ready-made prompt for Claude, Cursor, or any agent.',
    'edit.overview.agentPrompt.action': 'Copy prompt',
    'edit.overview.signInPreview.title': 'Preview',
    'edit.overview.signInPreview.description': 'What users see when they sign in to {{name}}.',
    'edit.overview.signInPreview.themeLabel': 'Theme used',
    'edit.overview.signInPreview.defaultTheme': 'Default',
    'edit.overview.signInPreview.flowsLabel': 'Flows',
    'edit.overview.signInPreview.flows.signIn': 'Sign-in',
    'edit.overview.signInPreview.flows.signUp': 'Sign-up',
    'edit.overview.signInPreview.flows.recovery': 'Password recovery',
    'edit.overview.signInPreview.flows.signOut': 'Sign-out',
    'edit.overview.signInPreview.notConfigured': 'Not configured',
    'edit.overview.signInPreview.editCustomization': 'Edit in customization',
    'edit.overview.signInPreview.editFlows': 'Edit in flows',
    'edit.overview.appDetails.title': 'Application details',
    'edit.overview.appDetails.description': 'Identifiers used in your integration code.',
    'edit.overview.endpoints.appNativeTitle': 'Useful endpoints',
    'edit.overview.endpoints.appNativeDescription': 'For driving sign-in and registration with a fully custom UI.',
    'edit.overview.endpoints.wellknown': 'OpenID configuration',
    'edit.overview.endpoints.authorization': 'Authorization endpoint',
    'edit.overview.endpoints.token': 'Token endpoint',
    'edit.overview.endpoints.userinfo': 'Userinfo endpoint',
    'edit.overview.endpoints.jwks': 'JWKS URI',
    'edit.overview.endpoints.flowExecute': 'Flow execution endpoint',
    'edit.overview.endpoints.flowMeta': 'Flow metadata endpoint',
    'edit.overview.endpoints.passkeyRegisterStart': 'Passkey registration (start)',
    'edit.overview.endpoints.passkeyRegisterFinish': 'Passkey registration (finish)',

    // Access tab
    'edit.access.sections.userTypes.title': 'Allowed user types',
    'edit.access.sections.userTypes.description': 'Choose which user types can sign up through this application.',
    'edit.access.sections.agentSignIn.title': 'Agent sign-in',
    'edit.access.sections.agentSignIn.description':
      'Allow agents to sign in to this application using the sign-in flow.',
    'edit.access.agentSignIn.toggle.label': 'Enable agent sign-in',
    'edit.access.agentSignIn.toggle.hint': 'When enabled, agents can sign in to this client using the sign-in flow.',
    'edit.access.sections.applicationAccess.title': 'Application access',
    'edit.access.sections.applicationAccess.description': 'Configure where this application is accessed from.',

    // Credentials tab
    'edit.credentials.sections.identifier.title': 'Identifier',
    'edit.credentials.sections.identifier.description': 'Unique identifier used to reference this application.',
    'edit.credentials.sections.secret.title': 'Secret',
    'edit.credentials.sections.secret.description':
      'Regenerating the secret immediately invalidates the current one and cannot be undone.',
    'edit.credentials.sections.secret.flowSecretLabel': 'Flow secret',
    'edit.credentials.sections.secret.hints.clientSecret':
      'A confidential credential used with the Client ID to authenticate this application. Keep it secret.',
    'edit.credentials.sections.secret.hints.flowSecret':
      "A confidential credential used to authenticate this application's embedded sign-in flow. Keep it secret.",

    // General section
    'edit.general.sections.access': 'Access',
    'edit.general.sections.access.description': "Configure who can access this application, where it's hosted, etc.",
    'edit.general.sections.contacts': 'Contacts',
    'edit.general.sections.contacts.description': 'Contact email addresses for {{entity}} administrators.',
    'edit.general.labels.applicationId': 'Application ID',
    'edit.general.labels.clientId': 'Client ID',
    'edit.general.hints.applicationId':
      "ThunderID's internal identifier for this application. Use it when calling the Management API.",
    'edit.general.hints.clientId':
      'The public OAuth2 client identifier this application uses to authenticate as a client.',
    'edit.general.labels.allowedUserTypes': 'Allowed user types',
    'edit.general.labels.applicationUrl': 'Application URL',
    'edit.general.labels.contacts': 'Contacts',
    'edit.general.noUserTypes': 'No user types configured',
    'edit.general.contacts.placeholder': 'Type an email and press Enter',
    'edit.general.contacts.hint': 'Type a valid email address and press <0>Enter</0> to add it',
    'edit.general.contacts.error.invalid': 'Please enter a valid email address',
    'edit.general.redirectUris.title': 'Authorized redirect URIs',
    'edit.general.redirectUris.description': 'For use with requests from a web server',
    'edit.general.redirectUris.tooltip': 'OAuth2 redirect URIs where users will be redirected after authentication',
    'edit.general.redirectUris.addUri': 'Add URI',
    'edit.general.redirectUris.error.empty': 'Invalid redirect: URI must not be empty.',
    'edit.general.redirectUris.error.invalid': 'Invalid redirect: Please enter a valid URL.',
    'edit.general.postLogoutRedirectUris.title': 'Post-logout redirect URIs',
    'edit.general.postLogoutRedirectUris.description':
      'Allowed URIs to redirect to after logout. A post_logout_redirect_uri passed to the logout endpoint must match one of these.',
    'edit.general.postLogoutRedirectUris.addUri': 'Add URI',
    'edit.general.postLogoutRedirectUris.error.invalid': 'Invalid redirect: Please enter a valid URL.',
    'edit.general.postLogoutRedirectUris.sameAsRedirect.title': 'Use the same URLs for post-logout redirect',
    'edit.general.postLogoutRedirectUris.sameAsRedirect.description':
      'Reuse the redirect URIs above instead of maintaining a separate list',
    'edit.general.backchannelLogoutUri.title': 'Back-channel logout URI',
    'edit.general.backchannelLogoutUri.description':
      'Endpoint that receives a logout token when a session this client took part in ends. Leave empty to turn off notifications. By default the server refuses localhost and private network addresses.',
    'edit.general.backchannelLogoutUri.error.invalid':
      'Enter an absolute http or https URL with a host, and no user info, fragment, or wildcard.',
    'edit.general.backchannelLogoutUri.error.requiresHttps': 'A public client must use an https URL.',
    'edit.general.backchannelLogoutUri.error.privateHost':
      'The server refused the back-channel logout URI because it points to localhost or a private network address. Use a publicly reachable address.',
    'edit.general.backchannelLogoutUri.error.serverRequiresHttps':
      'The server refused the back-channel logout URI because a public client must use an https URL.',
    'edit.general.backchannelLogoutUri.error.serverInvalid':
      'The server refused the back-channel logout URI. Enter an absolute http or https URL with a host, and no user info, fragment, or wildcard.',
    'onboarding.configure.details.devServer.banner': 'Using {{label}}? Its dev server runs on',
    'onboarding.configure.details.devServer.byDefault': 'by default.',
    'onboarding.configure.details.devServer.addToRedirectAndCors': 'Add it to redirect URIs & CORS origins',
    'onboarding.configure.details.devServer.addToRedirect': 'Add it to redirect URIs',
    'onboarding.configure.details.corsOrigins.title': 'CORS allowed origins',
    'onboarding.configure.details.corsOrigins.description':
      'Origins allowed to make cross-origin requests to the token and userinfo endpoints. Each entry is either an exact origin or a regular expression.',
    'onboarding.configure.details.corsOrigins.placeholder': 'https://example.com',
    'onboarding.configure.details.corsOrigins.regexPlaceholder': '^https://[a-z0-9-]+\\.example\\.com$',
    'onboarding.configure.details.corsOrigins.addOrigin': 'Add origin',
    'onboarding.configure.details.corsOrigins.removeOrigin': 'Remove origin',
    'onboarding.configure.details.corsOrigins.saveError':
      'The application was created, but its allowed origins were not saved. Add them under Settings, CORS.',
    'edit.general.allowedUserTypes.placeholder': 'Select user types',
    'edit.general.allowedUserTypes.hint': 'Users of these types can sign up through this application',
    'edit.general.applicationUrl.hint': 'The homepage URL of your application',
    'edit.general.sections.dangerZone.title': 'Danger zone',
    'edit.general.sections.dangerZone.description': 'Actions in this section are irreversible. Proceed with caution.',
    'edit.general.sections.dangerZone.regenerateSecret.title': 'Regenerate client secret',
    'edit.general.sections.dangerZone.regenerateSecret.description':
      'Regenerating the client secret will immediately invalidate the current client secret and cannot be undone.',
    'edit.general.sections.dangerZone.regenerateSecret.button': 'Regenerate client secret',
    'edit.general.sections.dangerZone.regenerateFlowSecret.title': 'Regenerate flow secret',
    'edit.general.sections.dangerZone.regenerateFlowSecret.description':
      'Regenerating the flow secret immediately invalidates the current one. Server-side flow initiation will fail until the new secret is deployed.',
    'edit.general.sections.dangerZone.regenerateFlowSecret.button': 'Regenerate flow secret',
    'edit.general.sections.dangerZone.deleteApplication.title': 'Delete application',
    'edit.general.sections.dangerZone.deleteApplication.description':
      'Permanently delete this application and all associated data. This action cannot be undone.',
    'edit.general.sections.dangerZone.deleteApplication.button': 'Delete application',

    // Flows section
    'edit.flows.labels.authFlow': 'Sign-in flow',
    'edit.flows.labels.authFlow.description': 'Choose the flow that handles user login and authentication.',
    'edit.flows.labels.registrationFlow': 'Sign-up flow',
    'edit.flows.labels.registrationFlow.description': 'Choose the flow that handles user sign-up and account creation.',
    'edit.flows.authFlow.placeholder': 'Select an authentication flow',
    'edit.flows.authFlow.hint': 'Select the flow that handles user sign-in for this {{entity}}.',
    'edit.flows.authFlow.alert':
      'To modify the selected flow, <0>open the flow builder</0>. To create a new flow, visit the <1>Flows page</1>.',
    'edit.flows.registrationFlow.placeholder': 'Select a registration flow',
    'edit.flows.registrationFlow.hint': 'Select the flow that handles user registration for this {{entity}}.',
    'edit.flows.registrationFlow.alert':
      'To modify the selected flow, <0>open the flow builder</0>. To create a new flow, visit the <1>Flows page</1>.',
    'edit.flows.labels.recoveryFlow': 'Recovery flow',
    'edit.flows.labels.recoveryFlow.description': 'Choose the flow that handles password and account recovery.',
    'edit.flows.recoveryFlow.placeholder': 'Select a recovery flow',
    'edit.flows.recoveryFlow.hint': 'Select the flow that handles account recovery for this {{entity}}.',
    'edit.flows.recoveryFlow.alert':
      'To modify the selected flow, <0>open the flow builder</0>. To create a new flow, visit the <1>Flows page</1>.',
    'edit.flows.labels.signOutFlow': 'Sign out flow',
    'edit.flows.labels.signOutFlow.description': 'Choose the flow that handles user sign-out and session termination.',
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
    'edit.customization.labels.tosUri': 'Terms of service URI',
    'edit.customization.labels.policyUri': 'Privacy policy URI',
    'edit.customization.theme.placeholder': 'Select a theme',
    'edit.customization.theme.hint':
      'Choose a theme to customize authentication pages. Select the default theme (shared across all applications) or pick an app-specific theme.',
    'edit.customization.tosUri.placeholder': 'https://example.com/terms',
    'edit.customization.policyUri.placeholder': 'https://example.com/privacy',

    // Token section
    'edit.token.seconds': 'seconds',
    'edit.token.labels.token_validity': 'Token validity',
    'edit.token.labels.access_token_validity': 'Access token validity',
    'edit.token.loading_attributes': 'Loading user attributes...',
    'edit.token.no_user_attributes': 'No user attributes available. Configure allowed user types for this {{entity}}.',
    'edit.token.no_user_attributes.allUserTypes':
      'No user type defines any attributes yet. Add attributes to a user type to select them here.',
    'edit.token.user_attributes_load_error': 'Failed to load the user attributes.',
    'edit.token.click_to_add': 'Click to add',
    'edit.token.click_to_remove': 'Click to remove',
    'edit.token.configure_attributes': 'Add or remove attributes',
    'edit.token.configure_attributes.hint': 'Click on user attributes to add them to the token.',
    'edit.token.legacy_user_type_warning': 'userType is a legacy claim. Use userTypeHandle and userTypeName instead',
    'edit.token.token_preview.title': 'Decoded payload',
    'edit.token.validity.hint': 'Token validity period in seconds (e.g., 3600 for 1 hour)',
    'edit.token.validity.error': 'Validity period must be at least 1 second',
    'edit.token.token_profile_card.title': 'Token attributes & response',
    'edit.token.token_profile_card.title.native': 'Token attributes',
    'edit.token.token_profile_card.title.accessTokenOnly': 'Access token attributes',
    'edit.token.token_profile_card.title.accessTokenWithoutAttributes': 'Access token',
    'edit.token.token_profile_card.description':
      'Choose the attributes in each token and the user info response, and how each is returned.',
    'edit.token.token_profile_card.description.noUserInfo':
      'Choose the attributes in each token issued to this {{entity}}, and how each is returned.',
    'edit.token.token_profile_card.description.native':
      'Choose the attributes included in the token issued to this {{entity}}.',
    'edit.token.token_profile_card.description.accessTokenOnly':
      'The user attributes in the access token issued to this {{entity}} on behalf of a user.',
    'edit.token.token_profile_card.description.accessTokenWithoutAttributes':
      'The access token issued to this {{entity}} on behalf of a user.',
    'edit.token.tabs.access_token': 'Access token',
    'edit.token.tabs.id_token': 'ID token',
    'edit.token.tabs.refresh_token': 'Refresh token',
    'edit.token.tabs.user_info_endpoint': 'User info endpoint',
    'edit.token.token_validation.title': 'Token validity',
    'edit.token.token_validation.description': 'Configure how long tokens remain valid before expiration',
    'edit.token.inherit_from_id_token': 'Use same attributes as ID token',
    'edit.token.user_info.inherit_info': 'User info response will use the same attributes as the ID token',
    'edit.token.scopes_card.title': 'Scopes & user attribute mappings',
    'edit.token.scopes_card.description': 'Configure the OAuth2 scopes and the user attributes exposed for each scope',
    'edit.token.scopes_card.description.scopesOnly':
      'A custom scope listed here is kept as an OpenID Connect scope instead of being treated as a resource server permission. Its mapped attributes are not added to the access token.',
    'edit.token.scopes.title': 'Available scopes',
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
    'edit.token.tabs.application': 'Application',
    'edit.token.tabs.user': 'User',
    'edit.token.audience.title': 'Issued to',
    'edit.token.audience.application.description': 'M2M access token',
    'edit.token.audience.user.description': 'Tokens for a signed-in user',
    'edit.token.audience.footnote': 'Attribute sets are configured independently for each audience.',
    'edit.token.clientLock.message':
      'This application does not receive tokens for itself, so there is nothing to configure here.',
    'edit.token.userLock.message':
      'This application does not receive tokens for signed-in users, so there is nothing to configure here.',
    'edit.token.client.attributes.title': 'Access token attributes',
    'edit.token.client.attributes.description':
      'Extra attributes to add to the access token this application receives for itself.',
    'edit.token.client.attributes.label': 'Add or remove attributes',
    'edit.token.client.attributes.hint': "Click on attributes to add them to this application's access token.",
    'edit.token.client.attributes.empty': 'No attributes available.',
    'edit.token.client.validity.title': 'Token validity',
    'edit.token.client.validity.description': 'How long this access token remains valid before expiration.',
    'edit.token.client.validity.label': 'Token validity',
    'edit.token.client.validity.hint': 'Token validity period in seconds (e.g., 3600 for 1 hour).',
    'edit.token.client.validity.error': 'Enter a validity period of at least 1 second.',
    'edit.advanced.audience.title': 'Default audience',
    'edit.advanced.audience.description':
      "The default aud for access tokens that don't target a resource server (OIDC only or scopeless).",
    'edit.advanced.audience.label': 'Default audience (aud)',
    'edit.advanced.audience.placeholder': 'e.g. https://api.example.com',
    'edit.advanced.audience.hint': 'Leave empty to use the {{entity}} client ID.',
    'edit.token.scope_mapper.title': 'User attribute mapping',
    'edit.token.scope_mapper.hint':
      'Select a scope to configure which user attributes are exposed when it is requested.',
    'edit.token.scope_mapper.no_scopes': 'Add at least one scope above to start mapping attributes.',
    'edit.token.scope_mapper.mapped_label': 'Mapped attributes',
    'edit.token.scope_mapper.available_label': 'Available attributes',
    'edit.token.scope_mapper.no_mapped': 'No attributes mapped yet — click an attribute below to add it',
    'edit.token.scope_mapper.all_mapped': 'All available attributes are already mapped to this scope',
    'edit.token.scope_mapper.loading': 'Loading available attributes...',
    'edit.token.id_token.response_type_placeholder': 'Select response type',
    'edit.token.id_token.encryption_alg_placeholder': 'Select encryption algorithm',
    'edit.token.id_token.encryption_enc_placeholder': 'Select content encryption',
    'edit.token.user_info.response_type_placeholder': 'Select response type',
    'edit.token.user_info.encryption_alg_placeholder': 'Select encryption algorithm',
    'edit.token.user_info.encryption_enc_placeholder': 'Select content encryption',
    'edit.token.encryption_requires_certificate':
      'Encrypted formats require an OAuth client certificate (JWKS or JWKS URI) configured under the {{location}} tab.',
    'edit.token.signed_with': 'Signed with {{alg}}.',
    'edit.token.id_token.response_type_options.JWT.label': 'Signed (JWS)',
    'edit.token.id_token.response_type_options.JWT.description': 'Signed with the server key. Recommended default.',
    'edit.token.id_token.response_type_options.JWE.label': 'Encrypted (JWE)',
    'edit.token.id_token.response_type_options.JWE.description': 'Encrypted to the client certificate.',
    'edit.token.id_token.response_type_options.NESTED_JWT.label': 'Signed then encrypted (nested JWT)',
    'edit.token.id_token.response_type_options.NESTED_JWT.description':
      'Signed with the server key, then encrypted to the client certificate.',
    'edit.token.user_info.response_type_options.JSON.label': 'Plain JSON',
    'edit.token.user_info.response_type_options.JSON.description': 'Unsigned JSON response. Recommended default.',
    'edit.token.user_info.response_type_options.JWS.label': 'Signed (JWS)',
    'edit.token.user_info.response_type_options.JWS.description': 'Signed with the server key.',
    'edit.token.user_info.response_type_options.JWE.label': 'Encrypted (JWE)',
    'edit.token.user_info.response_type_options.JWE.description': 'Encrypted to the client certificate.',
    'edit.token.user_info.response_type_options.NESTED_JWT.label': 'Signed then encrypted (nested JWT)',
    'edit.token.user_info.response_type_options.NESTED_JWT.description':
      'Signed with the server key, then encrypted to the client certificate.',

    // Advanced section
    'edit.advanced.labels.oauth2Config': 'OAuth 2 configuration',
    'edit.advanced.labels.redirectUris': 'Redirect URIs',
    'edit.advanced.labels.grantTypes': 'Grant types',
    'edit.advanced.labels.responseTypes': 'Response types',
    'edit.advanced.labels.publicClient': 'Public client',
    'edit.advanced.labels.pkceRequired': 'PKCE required',
    'edit.advanced.labels.requirePAR': 'Require pushed authorization requests',
    'edit.advanced.par.hint': 'Require the client to use the PAR endpoint before authorization.',
    'edit.advanced.par.requiresAuthorizationCode': 'Available only when the authorization code grant is enabled.',
    'edit.advanced.labels.tokenEndpointAuthMethod': 'Client authentication method',
    'edit.advanced.labels.certificate': 'Certificate',
    'edit.advanced.labels.certificateType': 'Certificate type',
    'edit.advanced.labels.metadata': 'Metadata',
    'edit.advanced.labels.createdAt': 'Created at',
    'edit.advanced.labels.updatedAt': 'Updated at',
    'edit.advanced.publicClient.yes': 'Yes',
    'edit.advanced.publicClient.no': 'No',
    'edit.advanced.pkce.yes': 'Yes',
    'edit.advanced.pkce.no': 'No',
    'edit.advanced.certificate.placeholder.jwksUri': 'https://example.com/.well-known/jwks',
    'edit.advanced.certificate.placeholder.jwks': 'Enter JWKS JSON',
    'edit.advanced.certificate.hint.jwksUri': 'URL to the JWKS endpoint',
    'edit.advanced.certificate.hint.jwks': 'JSON Web Key Set',
    'edit.advanced.certificate.error.required': 'A certificate is required for private_key_jwt authentication.',
    'edit.advanced.certificate.error.valueRequired': 'Please enter a value for the selected certificate type.',
    'edit.advanced.certificate.error.encryptionDependsOnCert':
      'This certificate is used to encrypt the ID token or UserInfo response. Change those formats to a non-encrypted type before removing the certificate.',
    'edit.advanced.labels.acrValues': 'ACR values',
    'edit.advanced.acrValues.intro': 'Authentication context classes permitted for this application.',
    'edit.advanced.acrValues.placeholder': 'Select ACR values',
    'edit.advanced.acrValues.hint':
      'When acr_values is included in the authorization request, only values configured here are accepted.',
    'edit.advanced.grantTypes.labels.ciba': 'CIBA (Client-Initiated Backchannel Authentication)',
    'edit.advanced.grantTypes.labels.tokenExchange': 'Token exchange',
    'edit.advanced.grantTypes.labels.jwtBearer': 'JWT bearer',
    'edit.advanced.grantTypes.requiredByIdJag': 'Required by identity assertions (ID-JAG)',
    'edit.advanced.grantTypes.requiredByMetadataDocument': 'Required for a client registered from a metadata document',
    'edit.advanced.grantTypes.requiresPrivateKeyJwt':
      'Requires a metadata document that declares private key JWT authentication',
    'edit.advanced.grantTypes.followsIdJag': 'Turned on and off with identity assertions (ID-JAG)',
    'edit.advanced.grantTypes.metadataDocumentHint':
      'Only the grant types a metadata document client can hold are offered. Token exchange and JWT bearer need a document that declares private key JWT authentication, and token exchange comes with identity assertions (ID-JAG).',
    'edit.advanced.idJag.title': 'Identity assertions (ID-JAG)',
    'edit.advanced.idJag.description':
      "Issue signed assertions of the signed-in user's identity that external services accept for token issuance.",
    'edit.advanced.idJag.publicClientGuard':
      'Identity assertions require a confidential client. Turn off public client to enable.',
    'edit.advanced.idJag.metadataDocumentPublicClientGuard':
      "Identity assertions require a confidential client. This client's metadata document declares it as a public client.",
    'edit.advanced.idJag.labels.allowedAudiences': 'Allowed audiences',
    'edit.advanced.idJag.allowedAudiences.placeholder': 'Type an audience and press Enter',
    'edit.advanced.idJag.allowedAudiences.error': 'Add at least one audience.',
    'edit.advanced.idJag.allowedAudiences.hint': 'Each assertion targets exactly one of these audiences.',
    'edit.advanced.idJag.labels.validityPeriod': 'Assertion validity',
    'edit.advanced.idJag.labels.seconds': 'seconds',
    'edit.advanced.idJag.validityPeriod.error': 'Enter a value of at least 1 second.',
    'edit.advanced.idJag.validityPeriod.hint': 'How long an issued assertion stays valid. Default 300.',
    'edit.advanced.idJag.grantTypeHint': 'The token exchange grant type is enabled together with this feature.',
    'edit.advanced.idJag.tokenExchangeUnavailable':
      'This server does not allow the token exchange grant, so identity assertions cannot be issued.',
    'create.success': 'Application created successfully.',
    'create.error': 'Failed to create application. Please try again.',
    'create.flowGenerationError': 'Failed to generate the authentication flow. Please try again.',
    'update.success': 'Application updated successfully.',
    'update.error': 'Failed to update application. Please try again.',
    'delete.success': 'Application deleted successfully.',
    'delete.error': 'Failed to delete application. Please try again.',
    'regenerateSecret.snackbar.success': 'Client secret regenerated successfully.',
    'errors.APP-1001': 'This application no longer exists. It may have been deleted elsewhere.',
    'errors.APP-1002': 'The application identifier is not valid.',
    'errors.APP-1012': 'One or more redirect URIs are not valid.',
    'errors.APP-1015': 'The certificate value is invalid. Check the JWKS content and try again.',
    'errors.APP-1016': 'The JWKS URL is not valid.',
    'errors.APP-1020': 'An application with this name already exists. Choose a different name.',
    'errors.APP-1030': 'This application is managed declaratively and cannot be edited or deleted.',
    'errors.APP-1035': 'One or more user attributes are not valid for the selected user types.',
    'errors.APP-1047': 'A client secret is not applicable to this application: it authenticates without one.',
    'errors.APP-1048':
      'This application cannot be deleted because {{dependencies}} depend on it. Remove or reassign them first.',
    'errors.APP-1039':
      "The {{sourceFlowType}} references a different {{flowType}} than the one configured for this application. Update the {{sourceFlowType}} so it calls the same {{flowType}}, or change the application's {{flowType}} configuration.",
    'errors.APP-1050':
      "This client doesn't meet the metadata document rules. Check its redirect URIs, grant types, and authentication settings.",
    'errors.APP-1051':
      "This organization access can't be saved. Check the granted units and exceptions, then try again.",
    'errors.APP-1052':
      'This organization access no longer exists. Reset to load the current access, then make your changes again.',
    'errors.APP-1053':
      'Organization access was set up elsewhere meanwhile. Reset to load it, then make your changes again.',
    'errors.APP-1054':
      "Someone else changed this application's organization access. Reset to load the latest version, then make your changes again.",
    'errors.APP-1055': "This organization access is declared in a resource file and can't be changed here.",
    'cimd.document.quickPicks': 'Known clients:',
    'cimd.document.subtitle':
      "The client's URL is its client ID. Its name and sign-in details come from its vendor, with no setup in the client itself.",
    'cimd.document.title': "Enter the client's metadata document",
    'cimd.document.url.fetch': 'Fetch document',
    'cimd.document.url.label': 'Metadata document URL',
    'cimd.edit.badge': 'CIMD',
    'cimd.edit.description':
      "These values come from the client's metadata document. Re-fetch it to apply the vendor's changes.",
    'cimd.edit.device': 'Device',
    'cimd.edit.documentUrl': 'Document URL',
    'cimd.edit.refetch': 'Re-fetch document',
    'cimd.edit.title': 'CIMD',
    'cimd.errors.generic': "This metadata document can't be used.",
    'cimd.identity.document.description':
      "Enter the client's metadata document URL. Its identity and sign-in details come from its vendor.",
    'cimd.identity.document.title': 'Another client with a CIMD',
    'cimd.identity.knownClients': 'Known clients',
    'cimd.identity.manual.description': 'For your own MCP clients. You set the redirect URIs and credentials.',
    'cimd.identity.manual.title': "I'll configure it myself",
    'cimd.identity.otherClients': 'Other clients',
    'cimd.identity.subtitle': 'Known clients register in one step from the metadata document their vendor publishes.',
    'cimd.identity.title': 'Which MCP client are you connecting?',
    'cimd.preview.confidential': 'Private key JWT, signed by the vendor',
    'cimd.preview.grants': 'Grants',
    'cimd.preview.inlineKeys': 'Included in the document',
    'cimd.preview.keys': 'Keys',
    'cimd.preview.loopbackWarning':
      "This client only returns to the user's device. Any program on the device can present this identity.",
    'cimd.preview.public': 'Public client, no secret',
    'cimd.preview.publishedBy': 'Published by {{host}}',
    'cimd.preview.redirectUris': 'Redirect URIs',
    'cimd.preview.signInType': 'Sign-in type',
    'cimd.refetch.added': 'Added',
    'cimd.refetch.apply': 'Apply',
    'cimd.refetch.authChanged': 'The sign-in type changes from {{from}} to {{to}}.',
    'cimd.refetch.changed': 'The document has changed. Review the differences before applying them.',
    'cimd.refetch.idJagTurnedOff':
      'Identity assertions (ID-JAG) will be turned off because the document no longer declares private key JWT authentication.',
    'cimd.refetch.noChanges': 'The document matches what this application uses.',
    'cimd.refetch.removed': 'Removed',
    'cimd.refetch.title': 'Re-fetch metadata document',
    'cimd.refetch.unchanged': 'The application is unchanged.',
    'cimd.refetch.updateDetails': 'Also update the name and links from the document',
    'edit.page.tabs.metadataDocument': 'Metadata document',
    'onboarding.steps.clientIdentity': 'Client identity',
    'onboarding.steps.metadataDocument': 'Metadata document',
    'errors.CIMD-1002': 'Enter an HTTPS URL with a host name and a path, for example https://example.com/client.json.',
    'errors.CIMD-1003': "Couldn't retrieve a metadata document from this URL. Check the address and try again.",
    'errors.CIMD-1004':
      'This document describes a different client. Its client_id must equal the URL it is served from.',
    'errors.CIMD-1005':
      "Each redirect URI must be on the client's own domain or the user's device, and at least one is required.",
    'errors.CIMD-1006':
      'Only public clients, and private key JWT clients with exactly one key source, can register from a metadata document.',
    'errors.CIMD-1007': "This document contains a client secret, which a metadata document can't carry safely.",
    'errors.CIMD-1008': 'This document must declare the authorization code grant, and this deployment must allow it.',
    'errors.CIMD-1010': "The request couldn't be read. Please try again.",
    'errors.CIMD-1011': "The document's jwks_uri must be an HTTPS URL on a public address.",
    'errors.CIMD-1013': "The document's inline key set is too large. Publish it at a jwks_uri instead.",
    'errors.CIMD-1014':
      "The document's inline key set must hold only public keys. A metadata document is public, so it can't carry private or symmetric keys.",
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

    'export.page.title': 'Export configuration',
    'export.page.loading': 'Loading export configuration...',
    'export.page.loadError': 'Failed to load export configuration',

    'upload.breadcrumb.openProject': 'Import configuration',
    'upload.title': 'Import configuration',
    'upload.subtitle': 'Upload your {{configFileName}} configuration file to import',
    'upload.tabs.uploadFile': 'Upload file',
    'upload.tabs.fromUrl': 'From URL',
    'upload.actions.changeFile': 'Change file',
    'upload.dropConfig': 'Drop your configuration file here',
    'upload.orClickBrowse': 'or click to browse',
    'upload.supportsYaml': 'Supports YAML files only ({{configFileName}})',
    'upload.url.label': 'Configuration URL',
    'upload.url.placeholder': 'https://example.com/{{configFileName}}',
    'upload.url.helperText': 'Enter a URL pointing to your {{configFileName}} configuration file',
    'upload.env.title': 'Environment variables',
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
    'validate.title': 'Validating configuration',
    'validate.subtitle': 'Please wait while we validate your configuration',
    'validate.progress': 'Progress',
    'validate.parseErrors.invalidSections': 'Configuration file contains {{count}} invalid section(s)',
    'validate.parseErrors.summary': 'Successfully parsed {{successCount}} sections, {{failCount}} sections failed',
    'validate.parseErrors.title': 'Parse errors:',
    'validate.parseErrors.unknownFile': 'Unknown file',
    'validate.actions.uploadDifferentFile': 'Upload different file',

    'envViewer.title': 'Environment variables',
    'envViewer.variableCount': '{{count}} variables detected',
    'envViewer.modified': '• Modified',
    'envViewer.download': 'Download .env',
    'envViewer.placeholderWarning':
      'Some environment variables contain placeholder values. Edit them below before importing.',

    'fileViewer.download': 'Download {{fileName}}',

    'table.item': 'Item',
    'table.resourceType': 'Resource type',
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
    'configureExport.labels.organizationUnits': 'Organization units',
    'configureExport.labels.projects': 'Projects',
    'configureExport.labels.connections': 'Connections',
    'configureExport.labels.userTypes': 'User types',
    'configureExport.labels.translations': 'Translations',
    'configureExport.labels.layouts': 'Layouts',
    'configureExport.labels.selfRegistration': 'Self registration',
    'configureExport.labels.resourceServers': 'Resource servers',
    'configureExport.labels.roles': 'Roles',
    'configureExport.labels.groups': 'Groups',
    'configureExport.labels.credentialConfigurations': 'Credential configurations',
    'configureExport.labels.presentationDefinitions': 'Presentation definitions',
    'configureExport.fallback.unnamedApplication': 'Unnamed application',
    'configureExport.fallback.unnamedProvider': 'Unnamed provider',
    'configureExport.fallback.unnamedFlow': 'Unnamed flow',
    'configureExport.fallback.unnamedTheme': 'Unnamed theme',
    'configureExport.fallback.unnamedOrganization': 'Unnamed organization',
    'configureExport.fallback.unnamedProject': 'Unnamed project',
    'configureExport.fallback.unnamedSchema': 'Unnamed schema',
    'configureExport.fallback.unnamedTranslation': 'Unnamed translation',
    'configureExport.fallback.unnamedLayout': 'Unnamed layout',
    'configureExport.fallback.unnamedResourceServer': 'Unnamed resource server',
    'configureExport.fallback.unnamedRole': 'Unnamed role',
    'configureExport.fallback.unnamedGroup': 'Unnamed group',
    'configureExport.fallback.unnamedPresentationDefinition': 'Unnamed presentation definition',
    'configureExport.fallback.unnamedCredentialConfiguration': 'Unnamed credential configuration',
    'configureExport.labels.agents': 'Agents',
    'configureExport.fallback.unnamedAgent': 'Unnamed agent',
    'configureExport.fallback.unnamedAgentType': 'Unnamed agent type',
    'configureExport.labels.agentTypes': 'Agent types',
    'configureExport.labels.serverConfigs': 'Server configurations',
    'configureExport.fallback.unnamedServerConfig': 'Unnamed server configuration',
    'configureExport.fallback.unnamedUser': 'User {{index}}',
    'configureExport.labels.projectDetails': 'Project details',
    'configureExport.labels.totalResources': 'Total resources',
    'configureExport.labels.clientId': 'Client ID',
    'configureExport.labels.configurationResources': 'Configuration resources',
    'configureExport.labels.downloadConfig': 'Download your {{fileName}} configuration file',
    'configureExport.actions.exportConfiguration': 'Export configuration',
    'configureExport.runProduct.title': 'Run {{productName}}',
    'configureExport.runProduct.subtitle':
      'Use the following command to start {{productName}} with your configuration:',
    'configureExport.runProduct.nextStepsTitle': 'What happens next:',

    'summary.breadcrumb': 'Summary',
    'summary.title': 'Configuration summary',
    'summary.valid': 'Valid',
    'summary.subtitle': 'Review the imported configuration before proceeding',
    'summary.projectDetails': 'Project details',
    'summary.totalResources': 'Total resources',
    'summary.preImportValidation': 'Pre-import validation',
    'summary.labels.presentationDefinitions': 'Presentation definitions',
    'summary.labels.credentialConfigurations': 'Credential configurations',
    'summary.actions.reuploadEnv': 'Re-upload .env file',
    'summary.env.editInfo':
      'You can edit the environment variables below to fix missing or placeholder values. The validation will update automatically.',
    'summary.fallback.flow': 'Flow {{index}}',
    'summary.fallback.theme': 'Theme {{index}}',
    'summary.fallback.user': 'User {{index}}',
    'summary.fallback.schema': 'Schema {{index}}',
    'summary.fallback.agentType': 'Agent type {{index}}',
    'summary.precheck.readyNoEnvRequired': 'Ready to proceed. No environment values are required for this import.',
    'summary.precheck.readyAllEnvAvailable':
      'Ready to proceed. All {{count}} referenced environment values are available.',
    'summary.precheck.missingEnvValues': '{{count}} environment value(s) are missing. Add them before importing.',
    'summary.precheck.availableEnvValues':
      '{{resolved}} of {{total}} referenced environment values are already available.',
    'summary.precheck.missingVariables': 'Missing variables',
    'summary.importTest.status': 'Import test status',
    'summary.importTest.runToValidate': 'Run import test to validate behavior.',
    'summary.importTest.configUnavailable': 'Configuration content is unavailable. Re-upload the configuration file.',
    'summary.importTest.fixMissingThenRun': 'Fix missing environment values, then run test.',
    'summary.importTest.running': 'Running pre-flight dry-run...',
    'summary.importTest.runningShort': 'Running...',
    'summary.importTest.test': 'Test',
    'summary.importTest.retry': 'Retry',
    'summary.importTest.passed': 'Import test passed. {{imported}} of {{totalDocuments}} resources validated.',
    'summary.importTest.failedCount_one': 'Import test failed for {{count}} resource',
    'summary.importTest.failedCount_other': 'Import test failed for {{count}} resources',
    'summary.importTest.failedWithMessage': 'Import test failed: {{message}}',
    'summary.importTest.failures': 'Import test failures',
    'summary.import.tooltip.missingVariables_one':
      'Cannot import: {{count}} environment variable is missing. Edit the environment variables above to fix.',
    'summary.import.tooltip.missingVariables_other':
      'Cannot import: {{count}} environment variables are missing. Edit the environment variables above to fix.',
    'summary.import.tooltip.configUnavailable':
      'Cannot import: configuration content is unavailable. Re-upload the configuration file.',
    'summary.import.tooltip.runTestFirst': 'Cannot import: run pre-flight dry-run and ensure it passes.',
    'summary.import.action': 'Import configuration',
    'summary.import.importing': 'Importing...',
    'summary.import.completedWithFailures_one': 'Import completed with {{count}} failed resource.',
    'summary.import.completedWithFailures_other': 'Import completed with {{count}} failed resources.',
    'summary.import.completedSuccessfully_one': 'Import completed successfully. {{count}} resource imported.',
    'summary.import.completedSuccessfully_other': 'Import completed successfully. {{count}} resources imported.',
    'summary.import.failedRetry': 'Import failed. Please try again.',
    'summary.importTest.itemFailedGeneric': 'This resource failed to import.',

    // Backend error code translations (per importer/export service error envelopes).
    'errors.IMP-1001': 'The import request is missing required fields or is malformed.',
    'errors.IMP-1002': 'The uploaded YAML content could not be parsed.',
    'errors.IMP-1003': 'One or more template variables could not be resolved.',
    'errors.IMP-1004': 'The resource adapter required for this import is not configured.',
    'errors.EXP-1001': 'The export request is invalid or malformed.',
    'errors.EXP-1002': 'No resources were found for export.',
  },

  // ============================================================================
  // Sign In - Sign In page translations
  // ============================================================================
  signin: {
    'errors.signin.failed.message': 'Error',
    'errors.signin.failed.description': 'We are sorry, something has gone wrong here. Please try again.',
    'errors.signin.timeout': 'Time allowed to complete the step has expired.',
    'errors.signin.session.expired': 'Your session has expired. Please return to the application and sign in again.',
    'errors.signin.returnToApplication': 'Return to application',
    'errors.signin.returnToApplicationUnavailable': 'Please return to the application and try again.',
    'errors.passkey.failed': 'Passkey authentication failed. Please try again.',
    'redirect.to.signup': "Don't have an account? <1>Sign up</1>",
    heading: 'Sign in',
    // Passkey authentication
    'passkey.button.use': 'Sign in with passkey',
    'passkey.signin.heading': 'Sign in with passkey',
    'passkey.signin.description': 'Use your passkey to securely sign in to your account without a password.',
    'passkey.register.heading': 'Register passkey',
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
    heading: 'Sign up',
    // Passkey registration translations
    'passkey.setup.heading': 'Set up passkey',
    'passkey.setup.description': 'Create a passkey to securely sign in to your account without a password.',
    'passkey.button.create': 'Create passkey',
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
    'fields.first_name.label': 'First name',
    'fields.first_name.placeholder': 'Enter your first name',
    'fields.last_name.label': 'Last name',
    'fields.last_name.placeholder': 'Enter your last name',
    'fields.email.label': 'Email',
    'fields.email.placeholder': 'Enter your email address',
    'fields.mobile.label': 'Mobile number',
    'fields.mobile.placeholder': 'Enter your mobile number',
    'fields.password.label': 'Password',
    'fields.password.placeholder': 'Enter your password',
    'fields.username.label': 'Username',
    'fields.username.placeholder': 'Enter your username',
    'fields.usertype.label': 'User type',
    'fields.usertype.placeholder': 'Select user type',

    // Emoji picker
    'emoji_picker.search.placeholder': 'Search emojis...',
    'emoji_picker.search.label': 'Search emojis',
    'emoji_picker.empty_state.message': 'No emojis found for "{{search}}"',
    'emoji_picker.categories.smileys_emotion': 'Smileys & emotion',
    'emoji_picker.categories.people_body': 'People & body',
    'emoji_picker.categories.animals_nature': 'Animals & nature',
    'emoji_picker.categories.food_drink': 'Food & drink',
    'emoji_picker.categories.travel_places': 'Travel & places',
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
    'logo_picker.content_type.text_avatar': 'Text avatar',
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

    // Name suggestion
    'name_suggestion.prefix': 'Need inspiration? How about',
    'name_suggestion.suffix': '?',
    'name_suggestion.shuffle_aria_label': 'Try another suggestion',

    // Resource logo dialog
    'resource_logo_dialog.title': 'Choose a logo',
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
    'field.email.invalid': 'Please enter a valid email address.',
    'validation.pattern.invalid': '{{field}} format is invalid.',
    'validation.minLength.invalid': '{{field}} must be at least {{minLength}} characters.',
    'validation.maxLength.invalid': '{{field}} must not exceed {{maxLength}} characters.',
  },

  // ============================================================================
  // Flows - Flow builder feature translations
  // ============================================================================
  flows: {
    // Flow listing page
    'listing.title': 'Flows',
    'listing.subtitle': 'Create and manage authentication and registration flows for your applications',
    'listing.addFlow': 'Create new flow',
    'listing.columns.name': 'Name',
    'listing.columns.flowType': 'Type',
    'listing.columns.updatedAt': 'Last updated',
    'listing.columns.actions': 'Actions',
    'listing.error.title': 'Failed to load flows',
    'listing.error.unknown': 'An unknown error occurred',
    'delete.title': 'Delete flow',
    'delete.message': 'Are you sure you want to delete this flow? This action cannot be undone.',
    'delete.disclaimer': 'Warning: All associated configurations will be permanently removed.',
    'delete.error': 'Failed to delete flow. Please try again.',
    'delete.usages.loading': 'Checking affected resources…',
    'delete.usages.none': 'No applications or agents are currently using this flow.',
    'delete.usages.title': 'The following resources will revert to the default flow:',
    'delete.usages.more': '+{{count}} more',

    // Create flow wizard
    'create.steps.organizationUnit': 'Organization unit',
    'create.steps.type': 'Flow type',
    'create.steps.template': 'Template',
    'create.steps.configure': 'Details',
    'create.organizationUnit.title': 'Where should this flow belong?',
    'create.organizationUnit.subtitle':
      "Choose the organization unit that will own this flow. You can't change this once created.",
    'create.organizationUnit.required': 'Organization unit is required',
    'create.configure.title': "Let's collect some details about your flow",
    'create.configure.name.label': 'Flow name',
    'create.configure.name.placeholder': 'e.g. Customer sign-in',
    'create.configure.handle.label': 'Handle',
    'create.configure.handle.placeholder': 'e.g. customer-sign-in',
    'create.configure.handle.hint': 'Lowercase letters, numbers, and hyphens only',
    'create.type.title': 'What kind of flow do you want to create?',
    'create.type.signin.label': 'Sign-in',
    'create.type.signin.description': 'Authenticate users with passwords, passkeys, or social providers',
    'create.type.signup.label': 'Self sign-up',
    'create.type.signup.description': 'Let users register themselves with your application',
    'create.type.recovery.label': 'Password recovery',
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
    'core.labels.enableFlow': 'Enable flow',
    'core.labels.disableFlow': 'Disable flow',
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
    'core.executions.names.oidc': 'OIDC auth',
    'core.executions.names.PasskeyAuthentication': 'Passkey authentication',
    'core.executions.names.magicLink': 'Magic link',
    'core.executions.names.sendSMS': 'Send SMS',
    'core.executions.names.verifySMSOTP': 'Verify SMS OTP',
    'core.executions.names.default': 'Execution',
    'core.executions.names.ouResolver': 'Resolve OU',
    'core.executions.names.invite': 'Invite',
    'core.executions.names.email': 'Send email',
    'core.executions.names.sms': 'Send SMS',
    'core.executions.names.credentialSetter': 'Set credentials',
    'core.executions.names.attributeUniqueness': 'Validate attribute uniqueness',
    'core.executions.names.permissionValidator': 'Validate permission',
    'core.executions.names.provisioning': 'Provisioning',
    'core.executions.names.httpRequest': 'HTTP request',
    'core.executions.names.ouCreation': 'OU creation',
    'core.executions.names.userTypeResolver': 'User type resolver',
    'core.executions.names.agentTypeResolver': 'Agent type resolver',

    // OTP executor
    'core.executions.otp.description': 'Configure the OTP executor settings.',
    'core.executions.otp.otpLength.label': 'OTP length',
    'core.executions.otp.otpLength.placeholder': 'e.g., 6',
    'core.executions.otp.otpLength.hint': 'Number of characters in the generated OTP (4-10).',
    'core.executions.otp.otpUseNumericOnly.label': 'Numeric only',
    'core.executions.otp.otpUseNumericOnly.hint': 'When enabled, OTP will contain only numeric characters.',
    'core.executions.otp.otpValidityPeriodSeconds.label': 'Validity period (seconds)',
    'core.executions.otp.otpValidityPeriodSeconds.placeholder': 'e.g., 120',
    'core.executions.otp.otpValidityPeriodSeconds.hint': 'Time in seconds before the OTP expires (30-600).',
    'core.executions.otp.maxAttempts.label': 'Maximum attempts',
    'core.executions.otp.maxAttempts.placeholder': 'e.g., 3',
    'core.executions.otp.maxAttempts.hint': 'The maximum number of OTP verification attempts before the flow fails.',

    // SMS OTP executor modes
    'core.executions.smsOtp.mode.send': 'Send OTP',
    'core.executions.smsOtp.mode.verify': 'Verify OTP',
    'core.executions.smsOtp.mode.label': 'Mode',
    'core.executions.smsOtp.mode.placeholder': 'Select a mode',
    'core.executions.smsOtp.description': 'Configure the SMS OTP executor settings.',

    // SMS OTP sender selection
    'core.executions.smsOtp.sender.label': 'Notification sender',
    'core.executions.smsOtp.sender.placeholder': 'Select a notification sender',
    'core.executions.smsOtp.sender.required': 'Notification sender is required and must be selected.',
    'core.executions.smsOtp.sender.noSenders':
      'No notification senders available. Please create a notification sender first.',

    // Consent executor
    'core.executions.consent.description': 'Configure the consent executor settings.',
    'core.executions.consent.timeout.label': 'Consent timeout (seconds)',
    'core.executions.consent.timeout.placeholder': '0',
    'core.executions.consent.timeout.hint': 'Time in seconds before the consent request expires. Use 0 for no timeout.',
    'core.executions.consent.failOnDeny.label': 'Fail flow when user denies consent',
    'core.executions.consent.failOnDeny.hint':
      'When enabled, the flow fails if the user denies the consent prompt or lets it time out, even if all requested attributes and permissions are optional.',

    // Identifying executor modes
    'core.executions.identifying.mode.identify': 'Identify',
    'core.executions.identifying.mode.resolve': 'Resolve (disambiguation)',
    'core.executions.identifying.mode.label': 'Mode',
    'core.executions.identifying.mode.placeholder': 'Select a mode',
    'core.executions.identifying.description':
      'Configure the identifying executor mode. Use "Resolve" to enable user disambiguation when multiple users match.',

    // Administrative flow pre executor
    'core.executions.preDelete.mode.revokeAll': 'Revoke all grants',
    'core.executions.preDelete.mode.label': 'Revocation mode',
    'core.executions.preDelete.mode.placeholder': 'Select a revocation mode',
    'core.executions.preDelete.mode.hint':
      'Applies to every grant the subject holds. The executors that follow act on this setting, so it cannot be changed per request.',
    'core.executions.preDelete.description':
      'Validate the target of an administrative operation and plan the revocation carried out by the executors that follow.',

    // Criteria revocation executor modes
    'core.executions.criteriaRevocation.mode.revoke': 'Revoke tokens',
    'core.executions.criteriaRevocation.mode.advanceCutoff': 'Advance revocation cutoff',
    'core.executions.criteriaRevocation.mode.label': 'Mode',
    'core.executions.criteriaRevocation.mode.placeholder': 'Select a mode',
    'core.executions.criteriaRevocation.mode.hint':
      'Revoke runs before the change. Advance revocation cutoff runs after the change commits, closing the window a token could be minted in. A node without a mode revokes.',
    'core.executions.criteriaRevocation.description': 'Persist every token criterion from the trusted revocation plan.',

    // Access change validator executor modes
    'core.executions.accessChangeValidator.mode.roleAssignmentRemoval': 'Validate and plan assignment removal',
    'core.executions.accessChangeValidator.mode.roleDeletion': 'Validate and plan role deletion',
    'core.executions.accessChangeValidator.mode.rolePermissionRemoval': 'Validate and plan permission change',
    'core.executions.accessChangeValidator.mode.groupDeletion': 'Validate and plan group deletion',
    'core.executions.accessChangeValidator.mode.groupMembershipRemoval': 'Validate and plan membership removal',
    'core.executions.accessChangeValidator.mode.actionDeletion': 'Validate and plan action deletion',
    'core.executions.accessChangeValidator.mode.label': 'Mode',
    'core.executions.accessChangeValidator.mode.placeholder': 'Select a mode',
    'core.executions.accessChangeValidator.description':
      'Validate an access change and plan the revocation of the scopes it takes away, carried out by the executors that follow.',

    // Role executor modes
    'core.executions.roleExecutor.mode.roleAssignmentRemoval': 'Remove role assignment',
    'core.executions.roleExecutor.mode.roleDeletion': 'Delete role',
    'core.executions.roleExecutor.mode.rolePermissionRemoval': 'Change role permissions',
    'core.executions.roleExecutor.mode.label': 'Mode',
    'core.executions.roleExecutor.mode.placeholder': 'Select a mode',
    'core.executions.roleExecutor.mode.hint':
      'Must match the mode of the access change validator earlier in the flow. A mismatched pair is refused at runtime.',
    'core.executions.roleExecutor.description': 'Apply the role change named by the trusted revocation plan.',

    // Group executor modes
    'core.executions.groupExecutor.mode.groupDeletion': 'Delete group',
    'core.executions.groupExecutor.mode.groupMembershipRemoval': 'Remove group member',
    'core.executions.groupExecutor.mode.label': 'Mode',
    'core.executions.groupExecutor.mode.placeholder': 'Select a mode',
    'core.executions.groupExecutor.mode.hint':
      'Must match the mode of the access change validator earlier in the flow. A mismatched pair is refused at runtime.',
    'core.executions.groupExecutor.description': 'Apply the group change named by the trusted revocation plan.',

    // Identity governance executor operations
    'core.executions.identityGovernance.mode.suspend': 'Suspend',
    'core.executions.identityGovernance.mode.unsuspend': 'Unsuspend',
    'core.executions.identityGovernance.mode.unlock': 'Unlock',
    'core.executions.identityGovernance.mode.label': 'Operation',
    'core.executions.identityGovernance.mode.placeholder': 'Select an operation',
    'core.executions.identityGovernance.mode.hint':
      'Suspend and unlock act on different holds, so releasing one leaves the other in place. Suspend acts on the identity a Pre Suspend Executor planned.',
    'core.executions.identityGovernance.description': 'Run one lifecycle operation on the identity the flow resolved.',

    // Passkey executor modes
    'core.executions.passkey.mode.challenge': 'Challenge',
    'core.executions.passkey.mode.verify': 'Verify',
    'core.executions.passkey.mode.registerStart': 'Start registration',
    'core.executions.passkey.mode.registerFinish': 'Finish registration',
    'core.executions.passkey.mode.label': 'Mode',
    'core.executions.passkey.mode.placeholder': 'Select a mode',
    'core.executions.passkey.description': 'Configure the Passkey executor settings.',

    // Passkey relying party configuration
    'core.executions.passkey.relyingPartyId.label': 'Relying party ID',
    'core.executions.passkey.relyingPartyId.placeholder': 'e.g., localhost or example.com',
    'core.executions.passkey.relyingPartyId.hint':
      'The domain identifier for passkey registration (typically your domain name)',
    'core.executions.passkey.relyingPartyName.label': 'Relying party name',
    'core.executions.passkey.relyingPartyName.placeholder': 'e.g., my application',
    'core.executions.passkey.relyingPartyName.hint': 'A human-readable name shown to users during passkey registration',

    // OU Resolver executor
    'core.executions.ouResolver.description': 'Configure the OU resolution strategy.',
    'core.executions.ouResolver.resolveFrom.label': 'Resolve from',
    'core.executions.ouResolver.resolveFrom.placeholder': 'Select a resolution strategy',
    'core.executions.ouResolver.resolveFrom.caller': 'Caller',
    'core.executions.ouResolver.resolveFrom.prompt': 'Prompt',
    'core.executions.ouResolver.resolveFrom.promptAll': 'Prompt all',
    'core.executions.ouResolver.parentOuId.label': 'Parent OU ID',
    'core.executions.ouResolver.parentOuId.placeholder': 'OU ID',
    'core.executions.ouResolver.parentOuId.hint': 'Users select an OU below this one.',
    'core.executions.ouResolver.directChildrenOnly.label': 'Limit to direct children',
    'core.executions.ouResolver.directChildrenOnly.hint':
      'Only direct children of the parent OU can be selected. Declare an ouHandle input below to select by handle.',

    // Invite executor
    'core.executions.invite.description': 'Configure the invite executor mode.',
    'core.executions.invite.mode.label': 'Mode',
    'core.executions.invite.mode.placeholder': 'Select a mode',
    'core.executions.invite.mode.generate': 'Generate',
    'core.executions.invite.mode.verify': 'Verify',

    // Email executor
    'core.executions.email.description': 'Configure the email executor settings.',
    'core.executions.email.emailTemplate.label': 'Email template',
    'core.executions.email.emailTemplate.placeholder': 'Select an email template',
    'core.executions.email.emailTemplate.hint': 'The email template to use when sending the email.',
    'core.executions.email.emailTemplate.noTemplates':
      'No email templates available. Please create an email template first.',
    'core.executions.email.emailTemplate.loadError': 'Failed to load email templates. Please try again.',
    'core.executions.email.sender.label': 'Email provider',
    'core.executions.email.sender.placeholder': 'Select an email provider',
    'core.executions.email.sender.hint': 'The email provider used to deliver this email.',
    'core.executions.email.sender.noSenders': 'No email providers available. Please create an email provider first.',
    'core.executions.email.sender.loadError': 'Failed to load email providers',

    // SMS executor
    'core.executions.sms.description': 'Configure the SMS executor settings.',
    'core.executions.sms.smsTemplate.label': 'SMS template',
    'core.executions.sms.smsTemplate.placeholder': 'Select an SMS template',
    'core.executions.sms.smsTemplate.hint': 'The SMS template to use when sending the message.',
    'core.executions.sms.smsTemplate.noTemplates': 'No SMS templates available. Please create an SMS template first.',
    'core.executions.sms.smsTemplate.loadError': 'Failed to load SMS templates. Please try again.',

    // OpenID4VP verifier executor
    'core.executions.openid4vp.description':
      'Select the presentation definition this executor requests from the wallet.',
    'core.executions.openid4vp.allowAuthenticationWithoutLocalUser.label': 'Allow authentication without a local user',
    'core.executions.openid4vp.allowAuthenticationWithoutLocalUser.hint':
      'When enabled, a holder with no matching local user is provisioned just-in-time. When disabled, login requires an existing matching user.',

    // Permission validator executor
    'core.executions.permissionValidator.description': 'Configure required permission scopes.',
    'core.executions.permissionValidator.requiredScopes.label': 'Required scopes',
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
    'core.executions.federation.allowAuthenticationWithoutLocalUser.label': 'Allow authentication without local user',
    'core.executions.federation.allowAuthenticationWithoutLocalUser.hint':
      'Allow users to authenticate even when no matching local user exists.',
    'core.executions.federation.allowRegistrationWithExistingUser.label': 'Allow registration with existing user',
    'core.executions.federation.allowRegistrationWithExistingUser.hint':
      'Allow existing users to proceed through registration flows.',
    'core.executions.federation.allowCrossOUProvisioning.label': 'Allow cross-OU provisioning',
    'core.executions.federation.allowCrossOUProvisioning.hint':
      'Allow creating a user in a different organizational unit.',

    // Provisioning executor
    'core.executions.provisioning.description': 'Configure the provisioning executor settings.',
    'core.executions.provisioning.delegated.label': 'Delegated mode',
    'core.executions.provisioning.delegated.hint':
      'When enabled, agents created by this node act on behalf of a signed-in user. A value collected by the flow takes precedence.',
    'core.executions.provisioning.includeOptional.label': 'Allow optional non-credential attributes',
    'core.executions.provisioning.includeOptional.hint':
      'Prompt for optional non-credential attributes during dynamic input collection.',
    'core.executions.provisioning.includeOptionalCredentials.label': 'Include optional credentials',
    'core.executions.provisioning.includeOptionalCredentials.hint':
      'Prompt for optional credential attributes during dynamic input collection.',
    'core.executions.provisioning.maxPerPrompt.label': 'Max per prompt',
    'core.executions.provisioning.maxPerPrompt.placeholder': '0',
    'core.executions.provisioning.maxPerPrompt.hint':
      'Number of dynamic inputs to show per prompt when connected to this provisioning executor.',
    'core.executions.provisioning.assignGroup.label': 'Assign group',
    'core.executions.provisioning.assignGroup.placeholder': 'Comma-separated group IDs to assign',
    'core.executions.provisioning.assignRole.label': 'Assign role',
    'core.executions.provisioning.assignRole.placeholder': 'Comma-separated role IDs to assign',
    'core.executions.provisioning.seedGroupsFromMapping.label': 'Seed groups from authorization mapping',
    'core.executions.provisioning.seedGroupsFromMapping.hint':
      "Assign groups the connection's authorization mapping resolved for this subject.",
    'core.executions.provisioning.seedRolesFromMapping.label': 'Seed roles from authorization mapping',
    'core.executions.provisioning.seedRolesFromMapping.hint':
      "Assign roles the connection's authorization mapping resolved for this subject.",

    // Session sign out executor
    'core.executions.sessionSignOut.description': 'Configure the session sign out executor settings.',
    'core.executions.sessionSignOut.promptOnSignOut.label': 'Prompt for confirmation',
    'core.executions.sessionSignOut.promptOnSignOut.hint':
      'Ask the user to confirm before signing out when the logout request has no valid ID token hint.',
    'core.placeholders.dynamicInputPlaceholder.title': 'Dynamic input',
    'core.placeholders.dynamicInputPlaceholder.hint': 'Resolves input fields passed from runtime.',

    // OU executor
    'core.executions.ouExecutor.description': 'Configure the OU creation executor settings.',
    'core.executions.ouExecutor.parentOuId.label': 'Parent OU ID',
    'core.executions.ouExecutor.parentOuId.placeholder': 'Override the default parent OU',
    'core.executions.ouExecutor.parentOuId.hint': 'Overrides the default OU for new OU creation.',

    // User Type Resolver executor
    'core.executions.userTypeResolver.description': 'Configure the user type resolver settings.',
    'core.executions.userTypeResolver.allowedUserTypes.label': 'Allowed user types',
    'core.executions.userTypeResolver.allowedUserTypes.placeholder': 'e.g., employee, customer',
    'core.executions.userTypeResolver.allowedUserTypes.hint':
      'Comma-separated list of allowed user type handles to filter available types.',

    // Agent type resolver executor
    'core.executions.agentTypeResolver.description': 'Configure the agent type resolver settings.',
    'core.executions.agentTypeResolver.allowedAgentTypes.label': 'Allowed agent types',
    'core.executions.agentTypeResolver.allowedAgentTypes.placeholder': 'e.g., default',
    'core.executions.agentTypeResolver.allowedAgentTypes.hint':
      'Comma-separated list of allowed agent type handles to filter available types.',

    // HTTP Request executor
    'core.executions.httpRequest.description': 'Configure the HTTP request executor settings.',
    'core.executions.httpRequest.url.label': 'URL',
    'core.executions.httpRequest.url.placeholder': 'https://api.example.com/endpoint',
    'core.executions.httpRequest.method.label': 'Method',
    'core.executions.httpRequest.method.placeholder': 'Select HTTP method',
    'core.executions.httpRequest.headers.label': 'Headers',
    'core.executions.httpRequest.headers.keyLabel': 'Name',
    'core.executions.httpRequest.headers.valueLabel': 'Value',
    'core.executions.httpRequest.headers.keyPlaceholder': 'Header name',
    'core.executions.httpRequest.headers.valuePlaceholder': 'Header value',
    'core.executions.httpRequest.headers.addLabel': 'Add header',
    'core.executions.httpRequest.body.label': 'Request body',
    'core.executions.httpRequest.body.placeholder': 'Enter JSON request body',
    'core.executions.httpRequest.timeout.label': 'Timeout (seconds)',
    'core.executions.httpRequest.timeout.placeholder': '10',
    'core.executions.httpRequest.timeout.hint': 'Request timeout in seconds (max 20).',
    'core.executions.httpRequest.responseMapping.label': 'Response mapping',
    'core.executions.httpRequest.responseMapping.keyLabel': 'Runtime data key',
    'core.executions.httpRequest.responseMapping.valueLabel': 'Response path',
    'core.executions.httpRequest.responseMapping.keyPlaceholder': 'Runtime data key',
    'core.executions.httpRequest.responseMapping.valuePlaceholder': 'Response path (e.g., data.userId)',
    'core.executions.httpRequest.responseMapping.addLabel': 'Add mapping',
    'core.executions.httpRequest.errorHandling.label': 'Error handling',
    'core.executions.httpRequest.errorHandling.failOnError.label': 'Fail on error',
    'core.executions.httpRequest.errorHandling.retryCount.label': 'Retry count',
    'core.executions.httpRequest.errorHandling.retryCount.placeholder': '0',
    'core.executions.httpRequest.errorHandling.retryCount.hint': 'Max retry attempts (max 5).',
    'core.executions.httpRequest.errorHandling.retryDelay.label': 'Retry delay (ms)',
    'core.executions.httpRequest.errorHandling.retryDelay.placeholder': '0',
    'core.executions.httpRequest.errorHandling.retryDelay.hint': 'Delay between retries in milliseconds (max 5000).',

    // Extension executor
    'core.executions.extension.nodeIdPlaceholder': '<node ID>',
    'core.executions.extension.description':
      'Calls your service through a service endpoint connection and routes the flow on its answer.',
    'core.executions.extension.connection.label': 'Connection',
    'core.executions.extension.connection.placeholder': 'Select a service endpoint connection',
    'core.executions.extension.connection.hint':
      'The URL, headers, timeout and authentication come from the connection.',
    'core.executions.extension.connection.noConnections':
      'No service endpoint connections exist. Create one under Connections, then select it here.',
    'core.executions.extension.connection.loadError': 'Failed to load service endpoint connections.',
    'core.executions.extension.connection.dangling': 'This connection no longer exists. Select another connection.',
    'core.executions.extension.inputKeys.label': 'Inputs to send',
    'core.executions.extension.inputKeys.placeholder': 'username',
    'core.executions.extension.inputKeys.hint':
      'User inputs to send to the service. None are sent by default. Add a password or code only when the service needs it.',
    'core.executions.extension.runtimeDataKeys.label': 'Runtime data to send',
    'core.executions.extension.runtimeDataKeys.placeholder': 'extension.risk_check.riskScore',
    'core.executions.extension.runtimeDataKeys.hint':
      'Runtime data keys to send: userID, or data an earlier extension stored, named by its full key such as extension.risk_check.riskScore. When empty, only the user ID is sent.',
    'core.executions.extension.runtimeDataKeys.invalid':
      'Only userID and keys that start with "extension." can be sent. Remove {{keys}}.',
    'core.executions.extension.responseKeys.label': 'Stored response fields',
    'core.executions.extension.responseKeys.placeholder': 'riskScore',
    'core.executions.extension.responseKeys.hint':
      'Top-level response fields to store for later steps, stored under {{prefix}}. Later steps refer to a field by its full key, such as {{prefix}}riskScore. When empty, every top-level field except actionStatus, failureReason, failureDescription, redirectUrl and null values is stored.',
    'core.executions.extension.maxAttempts.label': 'Max attempts',
    'core.executions.extension.maxAttempts.hint':
      'How many FAILED or INCOMPLETE answers this step accepts in one flow execution. After that, the step fails without calling the service, and the flow continues at onFailure, or ends if none is set.',
    'core.executions.extension.contract.hint':
      'The service must answer with actionStatus SUCCESS, FAILED, INCOMPLETE or ERROR.',

    // Executor input configuration
    'core.executions.inputs.title': 'Executor inputs',
    'core.executions.inputs.typeLabel': 'Type',
    'core.executions.inputs.typePlaceholder': 'Select type',
    'core.executions.inputs.identifierLabel': 'Identifier',
    'core.executions.inputs.identifierPlaceholder': 'e.g., username, email',
    'core.executions.inputs.required': 'Required',
    'core.executions.inputs.add': 'Add input',
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
    'core.executions.stack.expandHint': 'Click to expand',
    'core.executions.stack.restack': 'Restack executors',
    'core.executions.handles.success': 'onSuccess',
    'core.executions.handles.failure': 'onFailure',
    'core.executions.handles.incomplete': 'onIncomplete',

    // Canvas hints and tips
    'core.canvas.hints.autoLayout': 'Tip: Use auto-layout to organize your flow',
    'core.canvas.buttons.autoLayout': 'Auto layout',

    // Steps - end
    'core.steps.end.flowCompletionProperties': 'Flow completion properties',

    // Validation messages - input fields
    'core.validation.fields.input.general':
      'Required fields are not properly configured for the input field with ID <code>{{id}}</code>.',
    'core.validation.fields.input.idpName': 'Identity provider name is required',
    'core.validation.fields.input.idpId': 'Connection is required',
    'core.validation.fields.input.senderId': 'Notification sender is required',
    'core.validation.fields.input.connectionId': 'Connection is required',
    'core.validation.fields.input.presentationDefinitionId': 'Presentation definition is required',
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

    // Validation messages - email
    'core.validation.fields.email.noSender':
      'Email executor <code>{{id}}</code> has no email provider selected. Select an email provider so the step can send.',

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

    // Validation messages - sign out
    'core.validation.signOut.confirmNotConnected':
      'Sign-out button <code>{{id}}</code> is not connected, so it will not sign the user out. Connect it to the session sign-out step, or change its action.',
    'core.validation.signOut.confirmInvalidTarget':
      'Sign-out button <code>{{id}}</code> does not lead to a session sign-out step, so it will not sign the user out. Connect it to one, or change its action.',

    // Validation messages - account linking
    'core.validation.linking.rejectNotConnected':
      'Reject button <code>{{id}}</code> is not connected, so the account linking step will never see the refusal. Connect it to that step, or change its action.',
    'core.validation.linking.rejectInvalidTarget':
      'Reject button <code>{{id}}</code> does not lead back to the account linking step, so that step will never see the refusal. Connect it to the linking step, or change its action.',
    'core.validation.linking.incompleteNotConnected':
      'Account linking step <code>{{id}}</code> has no incomplete path, so it cannot show the linking prompt. Connect its incomplete handle to the prompt view.',
    'core.validation.linking.failureLoop':
      'The failure path of account linking step <code>{{id}}</code> leads back to it without a federated sign-in, so the flow will be rejected on save. Route the failure path through a federated sign-in first.',

    // Validation messages - rich text links
    'core.validation.richText.actionNotConnected':
      'Link <code>{{id}}</code> is not connected to a step, so clicking it will fail at runtime. Connect it to the step it should open, or turn off "Use as an interactive link".',

    // Elements - runtime data bound
    'core.elements.dataSource.noSourceBound': 'No source bound',

    // Elements - rich text
    'core.elements.richText.placeholder': 'Enter text here...',
    'core.elements.richText.resolvedI18nValue': 'Resolved i18n value',
    'core.elements.richText.linkEditor.urlTypeLabel': 'URL type',
    'core.elements.richText.linkEditor.placeholder': 'Type or paste a link',
    'core.elements.richText.linkEditor.textPlaceholder': 'Text',
    'core.elements.richText.linkEditor.apply': 'Apply',
    'core.elements.richText.linkEditor.editLink': 'Edit link',
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
    'core.elements.classesPropertyField.label': 'CSS classes',
    'core.elements.classesPropertyField.placeholder': 'e.g. btn-primary',
    'core.elements.classesPropertyField.addClass': 'Add class',

    // Elements - text property field
    'core.elements.textPropertyField.placeholder': 'Enter {{propertyName}}',
    'core.elements.textPropertyField.tooltip.configureTranslation': 'Configure translation',
    'core.elements.textPropertyField.tooltip.configureDynamicValue': 'Insert dynamic value',
    'core.elements.textPropertyField.i18nKey': 'Translation key',
    'core.elements.textPropertyField.resolvedValue': 'Resolved value',

    // Elements - dynamic value popover
    'core.elements.textPropertyField.dynamicValuePopover.title': 'Dynamic value for {{field}}',
    'core.elements.textPropertyField.dynamicValuePopover.tabs.translation': 'Translation',
    'core.elements.textPropertyField.dynamicValuePopover.tabs.variables': 'Variables',

    // Elements - meta card
    'core.elements.textPropertyField.metaCard.title': 'Variable for {{field}}',
    'core.elements.textPropertyField.metaCard.variablePath': 'Variable path',
    'core.elements.textPropertyField.metaCard.variablePathPlaceholder': 'e.g. application.name',
    'core.elements.textPropertyField.metaCard.variablePathHint': 'Select a common variable or type a custom path',
    'core.elements.textPropertyField.metaCard.formattedValue': 'Formatted value',

    // Elements - i18n card
    'core.elements.textPropertyField.i18nCard.title': 'Translation for {{field}}',
    'core.elements.textPropertyField.i18nCard.createTitle': 'Create translation',
    'core.elements.textPropertyField.i18nCard.updateTitle': 'Update translation',
    'core.elements.textPropertyField.i18nCard.i18nKey': 'Translation key',
    'core.elements.textPropertyField.i18nCard.i18nKeyInputPlaceholder': 'Enter a unique translation key',
    'core.elements.textPropertyField.i18nCard.i18nKeyInputHint': 'Use format: screen.{{key}}',
    'core.elements.textPropertyField.i18nCard.selectI18nKey': 'Select an existing key',
    'core.elements.textPropertyField.i18nCard.language': 'Language',
    'core.elements.textPropertyField.i18nCard.languageText': 'Translation text',
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
    'core.dialogs.formRequiresView.formOnCanvas.title': 'Form requires a view',
    'core.dialogs.formRequiresView.formOnCanvas.description':
      'Form components cannot be placed directly on the canvas. They must be inside a View component.',
    'core.dialogs.formRequiresView.formOnCanvas.alertMessage':
      'Would you like to create a View and add the Form inside it?',
    'core.dialogs.formRequiresView.formOnCanvas.confirmButton': 'Add view with form',
    'core.dialogs.formRequiresView.inputOnCanvas.title': 'Input requires a form and view',
    'core.dialogs.formRequiresView.inputOnCanvas.description':
      'Input components cannot be placed directly on the canvas. They must be inside a Form, which must be inside a View.',
    'core.dialogs.formRequiresView.inputOnCanvas.alertMessage':
      'Would you like to create a View with a Form and add the Input inside it?',
    'core.dialogs.formRequiresView.inputOnCanvas.confirmButton': 'Add view, form and input',
    'core.dialogs.formRequiresView.inputOnView.title': 'Input requires a form',
    'core.dialogs.formRequiresView.inputOnView.description':
      'Input components cannot be placed directly inside a View. They must be inside a Form component.',
    'core.dialogs.formRequiresView.inputOnView.alertMessage':
      'Would you like to create a Form and add the Input inside it?',
    'core.dialogs.formRequiresView.inputOnView.confirmButton': 'Add form with input',
    'core.dialogs.formRequiresView.widgetOnCanvas.title': 'Widget requires a view',
    'core.dialogs.formRequiresView.widgetOnCanvas.description':
      'Widgets cannot be placed directly on the canvas. They must be inside a View component.',
    'core.dialogs.formRequiresView.widgetOnCanvas.alertMessage':
      'Would you like to create a View and add the Widget inside it?',
    'core.dialogs.formRequiresView.widgetOnCanvas.confirmButton': 'Add view with widget',
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

    // Discard unsaved changes dialog
    'core.dialogs.discardChanges.title': 'Discard unsaved changes?',
    'core.dialogs.discardChanges.description':
      'You have unsaved changes to this flow. If you leave now, your changes will be lost.',
    'core.dialogs.discardChanges.cancelButton': 'Keep editing',
    'core.dialogs.discardChanges.confirmButton': 'Discard changes',

    // Form adapter
    'core.adapters.form.badgeLabel': 'Form',
    'core.adapters.form.placeholder': 'DROP FORM COMPONENTS HERE',

    // Header panel
    'core.headerPanel.goBack': 'Go back to flows',
    'core.headerPanel.autoLayout': 'Auto layout',
    'core.headerPanel.save': 'Save',
    'core.headerPanel.unsavedChanges': 'You have unsaved changes',
    'core.headerPanel.undo': 'Undo',
    'core.headerPanel.undoTooltip': 'Undo (Ctrl+Z / Cmd+Z)',
    'core.headerPanel.redo': 'Redo',
    'core.headerPanel.redoTooltip': 'Redo (Ctrl+Shift+Z / Cmd+Shift+Z)',
    'core.headerPanel.editTitle': 'Edit flow name',
    'core.headerPanel.saveTitle': 'Save flow name',
    'core.headerPanel.cancelEdit': 'Cancel',
    'core.headerPanel.edgeStyleTooltip': 'Change edge style',
    'core.headerPanel.compactView': 'Compact view',
    'core.headerPanel.compactViewTooltip': 'Switch to compact view',
    'core.headerPanel.detailedViewTooltip': 'Switch to detailed view',
    'core.headerPanel.simulate': 'Preview',
    'core.headerPanel.stopSimulation': 'Stop preview',
    'core.headerPanel.saveDisabledDuringPreview': 'Stop the preview before saving',
    'core.headerPanel.edgeStyles.bezier': 'Bezier',
    'core.headerPanel.edgeStyles.smoothStep': 'Smooth step',
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
    'core.resourcePanel.showResources': 'Show resources',
    'core.resourcePanel.hideResources': 'Hide resources',
    'core.resourcePanel.starterTemplates.title': 'Starter templates',
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
    'core.steps.view.addComponent': 'Add component',
    'core.steps.view.addField': 'Add field',
    'core.steps.view.configure': 'Configure',
    'core.steps.view.preview': 'Preview this screen',
    'core.steps.view.remove': 'Remove',
    'core.steps.view.noComponentsAvailable': 'No components available',

    // Rule
    'core.rule.conditionalRule': 'Conditional rule',
    'core.rule.remove': 'Remove',

    // Field extended properties
    'core.fieldExtendedProperties.attribute': 'Attribute',
    'core.fieldExtendedProperties.selectAttribute': 'Select an attribute',

    // Button extended properties
    'core.buttonExtendedProperties.action.label': 'Action',
    'core.buttonExtendedProperties.action.submit': 'Submit form',
    'core.buttonExtendedProperties.action.trigger': 'Trigger action',
    'core.buttonExtendedProperties.action.confirm': 'Confirm action',
    'core.buttonExtendedProperties.action.reject': 'Reject action',
    'core.buttonExtendedProperties.action.hint': 'What happens when the button is activated',
    'core.buttonExtendedProperties.startIcon.label': 'Start icon',
    'core.buttonExtendedProperties.startIcon.placeholder': 'Enter icon path (e.g., assets/images/icons/icon.svg)',
    'core.buttonExtendedProperties.startIcon.hint': 'Optional icon displayed before the button label',
    'core.buttonExtendedProperties.endIcon.label': 'End icon',
    'core.buttonExtendedProperties.endIcon.placeholder': 'Enter icon path (e.g., assets/images/icons/icon.svg)',
    'core.buttonExtendedProperties.endIcon.hint': 'Optional icon displayed after the button label',

    // Rules properties
    'core.rulesProperties.description': 'Define a rule to how conditionally proceed to next steps in the flow',

    // Login flow builder
    'core.loginFlowBuilder.form': 'Form',
    'core.loginFlowBuilder.errors.validationRequired': 'Please fix all validation errors before saving.',
    'core.loginFlowBuilder.errors.structureValidationFailed': 'Flow structure validation failed: {{error}}',
    'core.loginFlowBuilder.errors.saveFailed': 'Failed to save flow. Please try again.',
    'core.loginFlowBuilder.errors.loadFailed': 'Failed to load flow',
    'core.loginFlowBuilder.success.flowCreated': 'Flow created successfully.',
    'core.loginFlowBuilder.success.flowUpdated': 'Flow updated successfully.',

    // I18n configuration card
    'core.elements.textPropertyField.i18nCard.createError': 'Failed to create translation. Please try again.',

    // Backend error code translations (per flow management service error envelope).
    'errors.FLM-1001': 'The request body is malformed or contains invalid data.',
    'errors.FLM-1002': 'The flow ID must be provided.',
    'errors.FLM-1003': 'This flow no longer exists. It may have already been deleted.',
    'errors.FLM-1004': 'The specified flow type is invalid.',
    'errors.FLM-1005': 'The flow definition contains invalid data.',
    'errors.FLM-1006': 'The limit parameter must be a positive integer.',
    'errors.FLM-1007': 'The offset parameter must be a non-negative integer.',
    'errors.FLM-1010': 'The flow handle must be provided.',
    'errors.FLM-1011': 'The flow name must be provided.',
    'errors.FLM-1012': 'The flow type cannot be changed once created.',
    'errors.FLM-1013': 'A flow with this handle already exists for the given flow type.',
    'errors.FLM-1014': 'The flow handle cannot be modified after creation.',
    'errors.FLM-1015': 'The flow handle must be lowercase, alphanumeric, and can only contain underscores or dashes.',
    'errors.FLM-1017': 'This flow is managed declaratively and cannot be modified or deleted.',
    'errors.FLM-1018': 'The flow ID must be a valid UUID.',
    'errors.FLM-1019': 'A flow with the specified ID already exists.',
    'errors.FLM-1020':
      'The flow definition has structural issues, such as a missing or duplicate start or end node, or an unreachable node.',
    'errors.FLM-1021': 'One or more nodes have an invalid configuration.',
    'errors.FLM-1022': 'A node or interceptor references a node that does not exist.',
    'errors.FLM-1023': 'One or more executor configurations are invalid.',
    'errors.FLM-1024': 'One or more input configurations are invalid.',
    'errors.FLM-1025': 'A CALL node references a flow that does not exist.',
    'errors.FLM-1026':
      'This update was rejected because a resource that references this flow would be left in an inconsistent state.',
    'errors.FLM-1027': 'The organization unit for this flow does not exist.',
    'errors.FLM-1028': 'The organization unit of a flow cannot be changed once the flow is created.',
    'errors.FLM-1029': 'A flow needs an organization unit.',
  },

  // ============================================================================
  // Onboarding namespace - User onboarding flows
  // ============================================================================
  onboarding: {
    'forms.onboarding_mode.actions.create.description': 'Create user account immediately with all details',
    'forms.onboarding_mode.actions.invite.description': 'Send invitation for user to complete their profile',
  },

  /**
   * Appearance namespace - Theme and layout related translations
   */
  appearance: {
    'theme.defaultTheme': 'Default theme',
    'theme.appTheme.displayName': '{{appName}} theme',
  },

  // ============================================================================
  // Translations namespace - Text & Translations feature
  // ============================================================================
  translations: {
    'page.title': 'Translations',
    'page.subtitle': 'Manage and customize UI text and translations for your application.',

    'editor.back': 'Back to translations',

    'listing.addLanguage': 'Add language',
    'listing.columns.language': 'Language',
    'listing.columns.actions': 'Actions',
    'listing.error': 'Failed to load languages',

    'page.loadErrorTitle': 'Failed to load translations',
    'page.loadError': 'Failed to load translations',

    'language.selectPlaceholder': 'Select a language',
    'language.addOption': 'Add new language...',

    'language.create.steps.country': 'Country',
    'language.create.steps.language': 'Language',
    'language.create.steps.localeCode': 'Locale code',

    'language.create.country.title': 'Choose a country',
    'language.create.country.subtitle': 'Select the country for the language you want to add.',
    'language.create.countryLabel': 'Country',
    'language.create.country.placeholder': 'Select a country',
    'language.create.country.helperText':
      'Country name will be used to derive a BCP 47 compliant locale code for the language.',

    'language.create.language.title': 'Choose a language',
    'language.create.language.subtitle': 'Select the language variant spoken in {{country}}.',
    'language.create.language.label': 'Language',
    'language.create.language.placeholder': 'Select a language',
    'language.create.language.helperText':
      'Language picked here together with the country selection will determine the BCP 47 compliant locale code.',

    'language.create.localeCode.title': 'Review locale code',
    'language.create.localeCode.subtitle':
      'The locale code was derived from your selection. Override it here if you need a different tag.',

    'language.create.createButton': 'Create language',

    'language.add.dialogTitle': 'Add new language',
    'language.add.code.label': 'Language code',
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

    'editor.panelHeader': 'Edit translations',
    'editor.fieldsTab': 'Fields',
    'editor.jsonTab': 'JSON',
    'editor.searchPlaceholder': 'Search by key or value...',
    'editor.noResults': 'No matching translations.',
    'editor.noLanguageSelected': 'Select a language to start editing.',
    'editor.loading': 'Loading translations...',
    'editor.fieldSaveSuccess': 'Saved.',
    'editor.fieldSaveError': 'Failed to save.',
    'editor.jsonSaveAll': 'Save all',
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
    'editor.addKey': 'Add key',
    'editor.addKey.keyLabel': 'Key',
    'editor.addKey.valueLabel': 'Value',
    'editor.addKey.keyPlaceholder': 'e.g. my.translation.key',
    'editor.addKey.valuePlaceholder': 'Translation value',
    'editor.addKey.submit': 'Add',
    'editor.addKey.cancel': 'Cancel',
    'editor.addKey.duplicateKey': 'This key already exists.',
    'editor.readOnlyKeys': 'Keys are fixed in this namespace. Only values can be edited.',

    'actions.saveChanges': 'Save changes',
    'actions.discardChanges': 'Discard changes',
    'actions.resetToDefault': 'Reset to default',
    'preview.noTheme': 'No themes configured. Preview unavailable.',

    'delete.title': 'Delete language',
    'delete.message':
      'Are you sure you want to delete all custom translations for "{{language}}"? This action cannot be undone.',
    'delete.disclaimer': 'All custom translations for this language will be permanently removed and reset to defaults.',
    'delete.error': 'Failed to delete translations. Please try again.',

    // Backend error code translations (per i18n service error envelope).
    'errors.I18N-1001': 'The language code must be a valid BCP 47 locale tag (e.g. en, es, fr-CA).',
    'errors.I18N-1003': 'The key can only contain letters, numbers, dots, underscores, and hyphens.',
    'errors.I18N-1005': 'A translation value is required for every key.',
  },

  design: {
    'page.title': 'Design',
    'page.subtitle': 'Create, customize, and manage visual themes & layouts for your applications.',
    'notificationTemplatePreview.subjectLabel': 'Subject',
    'notificationTemplatePreview.frameTitle': 'Email preview',
    'notificationTemplatePreview.warnings.unsupported':
      '"{{token}}" is not a supported token and will cause the template to fail to send.',
    'notificationTemplatePreview.warnings.designNotAllowed':
      'Design token "{{token}}" is only allowed in the email body, not here.',
    'notificationTemplatePreview.warnings.designUnresolved':
      'Design token "{{token}}" is not a recognized design token; it may not render as expected.',
    'notificationTemplatePreview.warnings.missingTranslation':
      'No translation for "{{token}}" in any language; the template will fail to send until it is added.',
    'notificationTemplatePreview.warnings.unknownContext':
      '"{{token}}" is not a recognized context value and will not be substituted when the template is sent.',
    'themes.section.title': 'Themes',
    'themes.actions.add.label': 'Add theme',
    'themes.empty_state.message': 'No themes yet',
    'themes.show_more.label': 'Show {{count}} more',
    'themes.builder.errors.load.title': 'Failed to load theme',
    'themes.builder.actions.delete.label': 'Delete',
    'themes.builder.actions.save.label': 'Save',
    'themes.builder.actions.revert.label': 'Revert',
    'themes.builder.tooltips.show_sections': 'Show sections',
    'themes.builder.tooltips.hide_sections': 'Hide sections',
    'themes.builder.config.label': 'Config',
    'themes.builder.preview.label': 'Preview',
    'themes.builder.preview.iframe_title': 'Gate preview',
    'themes.builder.actions.back_to_design.label': 'Back to design',
    'themes.builder.sections.colors.label': 'Colors',
    'themes.builder.sections.colors.description': 'Light & dark color schemes',
    'themes.builder.sections.shape.label': 'Shape',
    'themes.builder.sections.shape.description': 'Border radius & corner styles',
    'themes.builder.sections.typography.label': 'Typography',
    'themes.builder.sections.typography.description': 'Font family & type scale',
    'themes.config.select_theme.message': 'Select a theme to view configuration',
    'themes.config.errors.load.message': 'Failed to load theme configuration.',
    'themes.config.errors.save.message': 'Failed to save theme. Please try again.',
    'themes.createWizard.steps.name': 'Details',
    'themes.forms.configure_name.title': "Let's collect some details about your theme",
    'themes.forms.configure_name.fieldLabel': 'Theme name',
    'themes.forms.configure_name.placeholder': 'e.g. Solarized Light',
    'themes.forms.configure_color.title': 'Primary color',
    'themes.forms.configure_color.actions.back.label': 'Back',
    'themes.forms.configure_color.actions.continue.label': 'Continue',
    'themes.forms.configure_color.actions.create.label': 'Create theme',
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
    'themes.forms.color_builder.borders.title': 'Borders & dividers',
    'themes.forms.color_builder.fields.main.label': 'Main',
    'themes.forms.color_builder.fields.light.label': 'Light',
    'themes.forms.color_builder.fields.dark.label': 'Dark',
    'themes.forms.color_builder.fields.contrast_text.label': 'Contrast text',
    'themes.forms.color_builder.fields.default.label': 'Default',
    'themes.forms.color_builder.fields.surface.label': 'Surface',
    'themes.forms.color_builder.fields.acrylic.label': 'Acrylic',
    'themes.forms.color_builder.fields.primary.label': 'Primary',
    'themes.forms.color_builder.fields.secondary.label': 'Secondary',
    'themes.forms.color_builder.fields.disabled.label': 'Disabled',
    'themes.forms.color_builder.fields.black.label': 'Black',
    'themes.forms.color_builder.fields.white.label': 'White',
    'themes.forms.color_builder.fields.background.label': 'Background',
    'themes.forms.color_builder.fields.on_background.label': 'On background',
    'themes.forms.color_builder.fields.divider.label': 'Divider',
    'themes.forms.shape_builder.border_radius.title': 'Border radius',
    'themes.forms.shape_builder.fields.radius.label': 'Radius',
    'themes.forms.shape_builder.border_style.title': 'Border style',
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
    'themes.forms.typography_builder.font_family.title': 'Font family',
    'themes.forms.typography_builder.font_family.modes.web_safe': 'Use a web-safe font',
    'themes.forms.typography_builder.font_family.modes.import': 'Use a custom font',
    'themes.forms.typography_builder.fields.font_family.placeholder': 'Select a font',
    'themes.forms.typography_builder.fields.font_family.helper_text': 'Choose from the available web-safe fonts',
    'themes.forms.typography_builder.fields.font_import_url.label': 'Font import URL',
    'themes.forms.typography_builder.fields.font_import_url.placeholder':
      'E.g., https://fonts.googleapis.com/css2?family=Poppins',
    'themes.forms.typography_builder.fields.font_import_url.helper_text':
      'Enter a URL to import a custom font from a font service.',
    'themes.forms.typography_builder.fields.font_family_input.label': 'Font family',
    'themes.forms.typography_builder.fields.font_family_input.placeholder': 'E.g. Poppins',
    'themes.forms.typography_builder.fields.font_family_input.helper_text':
      'Enter the font family name documented by the font service above.',
    'themes.forms.typography_builder.fields.preview.label': 'Preview',
    'themes.forms.typography_builder.font_weights.title': 'Font weights',
    'themes.forms.typography_builder.fields.light.label': 'Light',
    'themes.forms.typography_builder.fields.regular.label': 'Regular',
    'themes.forms.typography_builder.fields.medium.label': 'Medium',
    'themes.forms.typography_builder.fields.bold.label': 'Bold',
    'themes.forms.typography_builder.base_sizes.title': 'Base sizes',
    'themes.forms.typography_builder.fields.base_font_size.label': 'Base font size',
    'themes.forms.typography_builder.fields.html_font_size.label': 'HTML font size',
    'themes.forms.typography_builder.type_scale.title': 'Type scale',
    'themes.forms.typography_builder.fields.type_scale.placeholder': 'e.g. 1.5rem',
    'themes.forms.typography_builder.actions.reset.label': 'Reset',
    'themes.forms.general_builder.internationalization.title': 'Internationalization',
    'themes.forms.general_builder.fields.text_direction.label': 'Text direction',
    'themes.forms.general_builder.fields.text_direction.options.ltr.label': 'LTR',
    'themes.forms.general_builder.fields.text_direction.options.rtl.label': 'RTL',
    'themes.forms.settings.heading': 'Settings',
    'themes.forms.settings.fields.default_color_scheme.label': 'Default color scheme',
    'themes.forms.settings.fields.default_color_scheme.helper_text':
      'Select whether you want a light, dark or system color scheme as the default.',
    'themes.forms.settings.fields.default_text_direction.label': 'Default text direction',
    'themes.forms.settings.fields.default_text_direction.helper_text':
      'Select the default text direction for your theme. This will affect the layout and alignment of components.',
    'themes.forms.settings.fields.default_text_direction.options.ltr.label': 'Left-to-right (LTR)',
    'themes.forms.settings.fields.default_text_direction.options.rtl.label': 'Right-to-left (RTL)',
    'themes.delete.title': 'Delete theme',
    'themes.delete.message': 'Are you sure you want to delete "{{name}}"? This action cannot be undone.',
    'themes.delete.messageUnnamed': 'Are you sure you want to delete this theme? This action cannot be undone.',
    'themes.delete.disclaimer': 'Deleting this theme may affect applications using it.',
    'themes.delete.error': 'Failed to delete theme. Please try again.',
    'themes.delete.usages.loading': 'Checking affected resources…',
    'themes.delete.usages.none': 'No applications are currently using this theme.',
    'themes.delete.usages.title': 'The following applications will revert to the default theme:',
    'themes.delete.usages.more': '+{{count}} more',
    'layouts.section.title': 'Layouts',
    'layouts.errors.create_failed.message': 'Failed to create layout. Please try again.',
    'layouts.presets.centered.label': 'Centered',
    'layouts.presets.split_screen.label': 'Split screen',
    'layouts.presets.full_screen.label': 'Full screen',
    'layouts.presets.popup.label': 'Popup',
    'layouts.badges.coming_soon.label': 'Coming soon',
    'layouts.config.select_layout.message': 'Select a layout to view constraints',
    'layouts.config.errors.load.message': 'Failed to load layout configuration.',
    'layouts.config.errors.save.message': 'Failed to save layout. Please try again.',
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
    'layouts.builder.errors.load.title': 'Failed to load layout',
    'layouts.builder.actions.back_to_design.tooltip': 'Back to design',
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
    'common.preview.toolbar.fields.color_scheme.label': 'Color scheme',
    'common.preview.toolbar.viewports.mobile.label': 'Mobile (390px)',
    'common.preview.toolbar.viewports.tablet.label': 'Tablet (768px)',
    'common.preview.toolbar.viewports.desktop.label': 'Desktop (1440px)',
    'common.preview.toolbar.actions.zoom_out.tooltip': 'Zoom out',
    'common.preview.toolbar.actions.zoom_in.tooltip': 'Zoom in',
    'common.preview.toolbar.actions.cycle_color_scheme.tooltip': 'Toggle color scheme',
    'common.item_card.actions.open_in_builder.label': 'Open in builder',
    'common.section_header.badges.coming_soon.label': 'COMING SOON',

    // Backend error code translations (per theme and layout management service error envelope).
    'errors.THM-1001': 'The provided theme data is invalid.',
    'errors.THM-1002': 'The provided theme ID is invalid.',
    'errors.THM-1003': 'The requested theme was not found.',
    'errors.THM-1005': 'The theme display name is required.',
    'errors.THM-1006': 'The theme configuration is required.',
    'errors.THM-1007': 'The theme must be a valid JSON object.',
    'errors.THM-1008': 'The limit parameter must be within the valid range.',
    'errors.THM-1009': 'The offset parameter must be non-negative.',
    'errors.THM-1010': 'The limit parameter must be a valid integer.',
    'errors.THM-1011': 'The offset parameter must be a valid integer.',
    'errors.THM-1014': 'This theme is declarative and cannot be modified or deleted.',
    'errors.THM-1015': 'A theme with the same handle already exists.',
    'errors.THM-1016': 'The theme handle is required.',
    'errors.THM-1017': 'The theme handle cannot be changed after creation.',
    'errors.LAY-1001': 'The provided layout data is invalid.',
    'errors.LAY-1002': 'The provided layout ID is invalid.',
    'errors.LAY-1003': 'The requested layout was not found.',
    'errors.LAY-1004': 'A layout with the same ID already exists.',
    'errors.LAY-1005': 'The layout display name is required.',
    'errors.LAY-1006': 'The layout configuration is required.',
    'errors.LAY-1007': 'The layout must be a valid JSON object.',
    'errors.LAY-1009': 'The limit parameter must be within the valid range.',
    'errors.LAY-1010': 'The offset parameter must be non-negative.',
    'errors.LAY-1011': 'The limit parameter must be a valid integer.',
    'errors.LAY-1012': 'The offset parameter must be a valid integer.',
    'errors.LAY-1015': 'This layout is declarative and cannot be modified or deleted.',
    'errors.LAY-1016': 'A layout with the same handle already exists.',
    'errors.LAY-1017': 'The layout handle is required.',
    'errors.LAY-1018': 'The layout handle cannot be changed after creation.',
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
    'start_building.hero.eyebrow': 'Get started',
    'start_building.hero.title': 'Integrate {{product}} into your application',
    'start_building.hero.description':
      'Add secure sign-in, token management, and user sessions to your app in minutes.',
    'start_building.hero.actions.create.label': 'Create application',
    'start_building.frameworks.label': 'Start with a template',

    // Next Steps section
    'next_steps.section.title': 'Quick links',

    // Invite Members card
    'next_steps.invite_members.title': 'Add users',
    'next_steps.invite_members.description': 'Add or invite collaborators to help manage your organization.',
    'next_steps.invite_members.actions.primary.label': 'Add user',
    'next_steps.invite_members.status.empty': 'No members yet',

    // Login Box card
    'next_steps.login_box.title': 'Sign-in box',
    'next_steps.login_box.description':
      'Build themes and attach them to your applications to personalise the sign-in experience.',
    'next_steps.login_box.actions.primary.label': 'Open design studio',

    // Connections card
    'next_steps.connections.title': 'Connections',
    'next_steps.connections.description':
      'Manage the external services {{product}} connects to for social login, enterprise OIDC, SMS delivery, and more.',
    'next_steps.connections.actions.primary.label': 'Manage connections',

    // Multi-factor Authentication card
    'next_steps.mfa.title': 'Multi-factor authentication',
    'next_steps.mfa.description': 'Protect users by enabling an additional verification factor to the sign-in process.',
    'next_steps.mfa.actions.primary.label': 'Configure flows',

    // Start Building dynamic
    'start_building.hero.status.app_count': '{{count}} application',
    'start_building.hero.status.app_count_other': '{{count}} applications',
    'start_building.hero.actions.view_apps.label': 'Create applications',

    // Feature status labels
    'feature_status.new': 'New',
    'feature_status.coming_soon': 'Coming soon',
  },

  // ============================================================================
  // How Solution Works Illustration - Shared illustration translations
  // ============================================================================
  consumerAppIllustration: {
    step1Title: 'Get the Wayfinder sample',
    step1Sub: 'Download the sample distribution',
    step2Title: 'Register application',
    step2Line1: 'Set redirect URIs',
    step2Line2: 'and client credentials',
    step2Line3: 'in the console',
    step2Sub: 'Creates the app in the console',
    step3Title: 'Run the sample',
    step3Sub: 'Linux / macOS or Windows',
  },
  aiAgentsIllustration: {
    consumers: 'Consumers',
    johnDoe: 'John Doe',
    janeSmith: 'Jane Smith',
    use: 'Use',
    wayfinderWeb: 'Wayfinder web',
    wayfinderWebSub: 'Browser SPA with chat widget',
    wayfinderWebDetail: 'Book travel, chat with the agent',
    identityAuthority: 'Identity authority',
    managesIdentities: 'Manages identities',
    issuesTokens: 'and issues tokens',
    aiAgent: 'AI agent',
    aiAgentSub: 'Wayfinder Concierge',
    drivesConversation: 'Drives the conversation',
    wayfinderServer: 'Wayfinder server',
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
    externalMcpClient: 'External MCP client',
    mcpInspector: 'MCP Inspector',
    discoversSignsIn: 'Discovers, signs in, calls MCP tools',
    identityAuthority: 'Identity authority',
    managesIdentities: 'Manages identities',
    issuesTokens: 'and issues tokens',
    wayfinderServer: 'Wayfinder server',
    wayfinderServerSub: 'Booking API + MCP tools',
    holdsData: 'Holds flights, hotels, bookings',
    signIn: 'Sign in',
    issueTokens: 'Issue tokens',
    callMcpTools: 'Call MCP tools',
    validateTokens: 'Validate tokens',
  },
  howSolutionWorksIllustration: {
    validateTest: 'Validate / test',
    configureProject: 'Configure project',
    run: 'Run',
    console: '{{productName}} Console',
    runtimeLocal: '{{productName}} runtime local',
    projectEnvConfigs: 'Project + ENV configs',
    runtimeHosted: '{{productName}} runtime hosted',
    saveExport: 'Save & export',
    import: 'Attach',
    runInProduction: 'Run {{productName}} in production',
    runtimeComponentsOnly: '(with required runtime components only)',
    designConfigure: 'Configure {{productName}}',
    designComponents: '(with design components)',
    commandProduction: './start.sh project-foo.yml --env production.env',
    commandStart: './start.sh',
    adminApp: 'Admin app',
    loginApp: 'Login app',
  },
  notificationTemplates: {
    'listing.title.email': 'Email templates',
    'listing.title.sms': 'SMS templates',
    'listing.subtitle.email': 'Create, customize, and localize the email notifications your applications send.',
    'listing.subtitle.sms': 'Create, customize, and localize the SMS notifications your applications send.',
    'listing.addTemplate': 'New template',
    'listing.error': 'Failed to load templates',
    'listing.columns.name': 'Name',
    'listing.columns.actions': 'Actions',
    'editor.back': 'Notification templates',
    'editor.save': 'Save changes',
    'editor.reset': 'Reset',
    'editor.unsavedChanges': 'You have unsaved changes',
    'editor.loadError': 'Failed to load template',
    'editor.description.placeholder': 'Add a description',
    'editor.description.empty': 'No description',
    'editor.subject': 'Subject',
    'editor.body': 'Body',
    'editor.design.title': 'Design',
    'editor.design.help': 'Choose the color theme for this email. The preview uses the selected design theme.',
    'editor.design.colorScheme': 'Color scheme',
    'editor.design.light': 'Light',
    'editor.design.dark': 'Dark',
    'editor.tokens.hint': 'Select a token to insert it into the template',
    'editor.tokens.collapse': 'Hide available tokens',
    'editor.tokens.expand': 'Show available tokens',
    'editor.tokens.searchPlaceholder': 'Search available tokens...',
    'editor.tokens.design': 'Design tokens',
    'editor.tokens.context': 'Context tokens',
    'editor.tokens.translation': 'Translation tokens',
    'editor.tokens.noTranslation': 'No translation keys match your search.',
    'editor.tokens.add.title': 'Add translation key',
    'editor.tokens.add.keyPlaceholder': 'Key (e.g. otp.subject)',
    'editor.tokens.add.valuePlaceholder': 'Translation value',
    'editor.tokens.add.submit': 'Add',
    'editor.tokens.add.error': 'Failed to add translation key.',
    'editor.tokens.add.invalidKey': 'Use only letters, numbers, dots, hyphens and underscores.',
    'editor.tokens.add.duplicateKey': 'This key already exists; select it from the list instead.',
    'editor.tokens.add.keysNotLoaded': 'Translation keys are still loading. Try again in a moment.',
    'editor.tokens.labels.design.primary': 'Primary color',
    'editor.tokens.labels.design.primaryContrast': 'Primary contrast text',
    'editor.tokens.labels.design.secondary': 'Secondary color',
    'editor.tokens.labels.design.textPrimary': 'Text primary',
    'editor.tokens.labels.design.backgroundDefault': 'Background default',
    'editor.tokens.labels.design.backgroundPaper': 'Background paper',
    'editor.tokens.labels.context.appName': 'App name',
    'editor.tokens.labels.context.otpCode': 'OTP code',
    'editor.tokens.labels.context.expiryTime': 'Expiry time',
    'editor.tokens.labels.context.magicLink': 'Magic link',
    'editor.tokens.labels.context.inviteLink': 'Invite link',
    'editor.tokens.labels.context.lockedAuthenticationMethod': 'Locked sign-in method',
    'editor.tokens.labels.context.lockDuration': 'Lock duration',
    'editor.tokens.labels.context.unlockTime': 'Unlock time',
    'editor.validation.emailRequired': 'A subject and body are required.',
    'editor.validation.bodyRequired': 'A body is required.',
    'editor.tokens.noDesign': 'No design tokens match your search.',
    'editor.tokens.noContext': 'No context placeholders match your search.',
    'editor.preview.title': 'Preview',
    'editor.preview.language': 'Language',
    'editor.preview.languageDefault': 'English (US)',
    'editor.preview.designTheme': 'Design theme',
    'editor.preview.designThemeNone': 'No Theme',
    'update.success': 'Template updated successfully.',
    'update.error': 'Failed to save template. Please try again.',
    'delete.title': 'Delete template',
    'delete.message': 'Are you sure you want to delete this template? This action cannot be undone.',
    'delete.messageNamed': 'Are you sure you want to delete "{{name}}"? This action cannot be undone.',
    'delete.success': 'Template deleted successfully.',
    'delete.error': 'Failed to delete template. Please try again.',
    'create.steps.organizationUnit': 'Organization unit',
    'create.steps.template': 'Template',
    'create.steps.identity': 'Details',
    'create.steps.content': 'Content',
    'create.organizationUnit.fieldLabel': 'Organization unit',
    'create.organizationUnit.title': 'Where should this template belong?',
    'create.organizationUnit.subtitle':
      "Choose the organization unit that will own this template. You can't change this once created.",
    'create.template.title': 'Choose a starting template',
    'create.template.blank.title': 'Start from scratch',
    'create.template.blank.description': 'Build the template from an empty subject and body.',
    'create.template.search': 'Search templates...',
    'create.template.noResults': 'No templates match your search.',
    'create.submit': 'Create',
    'create.error': 'Failed to create template. Please try again.',
    'create.details.titleEmail': "Let's collect some details about your email template",
    'create.details.titleSms': "Let's collect some details about your SMS template",
    'create.displayName.label': 'Display name',
    'create.displayName.placeholder': 'e.g. OTP verification',
    'create.handle.label': 'Handle',
    'create.handle.placeholder': 'e.g. otp-verification',
    'create.handle.helperText': 'A unique, URL-friendly identifier. Cannot be changed later.',
    'create.handle.duplicate': 'A template with this handle already exists.',
    'create.handle.invalid': 'Use lowercase letters, numbers, hyphens and underscores.',
    'errors.NTM-1009': 'A template with this handle already exists in this channel.',
    'errors.NTM-1010': 'The template cannot be deleted because {{usages}} depend on it. Remove or reassign them first.',
    'errors.NTM-1011': 'The handle must be a lowercase kebab-case identifier.',
    'errors.NTM-1012': 'A subject cannot be set for this channel.',
    'errors.NTM-1013': 'A design cannot be set for this channel.',
    'errors.NTM-1014': 'The display name is too long.',
    'errors.NTM-1015': 'The description is too long.',
    'errors.NTM-1016': 'The handle is too long.',
    'errors.NTM-1017': 'The color scheme must be light or dark.',
    'errors.NTM-1020': 'The content contains an unsupported placeholder. Use the ctx, t, or design token syntax.',
    'errors.NTM-1022': 'Select a valid organization unit.',
  },
  resourceServers: {
    'listing.title': 'Resource servers',
    'listing.subtitle': 'Define resource servers and their resources to manage access control.',
    'listing.addResourceServer': 'Add resource server',
    'listing.columns.name': 'Name',
    'listing.columns.type': 'Type',
    'listing.columns.identifier': 'Identifier',
    'listing.columns.actions': 'Actions',
    'listing.actions.more': 'More actions',
    'listing.systemResourceServer': 'System resource server',
    'listing.default': 'Default',
    'listing.error': 'Failed to load resource servers',
    'actions.setAsDefault': 'Set as default',
    'setDefault.title': 'Set default resource server',
    'setDefault.message': '<bold>{{name}}</bold> will become the default resource server.',
    'setDefault.unavailableReason': 'Checking the current default resource server.',
    'setDefault.lockedReason': 'The default resource server is fixed by the deployment configuration.',
    'setDefault.alreadyDefaultReason': 'This is already the default resource server.',
    'setDefault.ineligibleTypeReason': 'Only API and custom resource servers can be the default.',
    'setDefault.explanation':
      'When an application requests a token without naming a resource server, its permissions come from this one. Only one resource server can be the default at a time.',
    'setDefault.action': 'Make default resource server',
    'setDefault.confirm': 'Set as default',
    'setDefault.setting': 'Setting…',
    'setDefault.success': '{{name}} is now the default resource server.',
    'setDefault.error': 'Failed to set the default resource server.',
    'delete.title': 'Delete resource server',
    'delete.message': 'Are you sure you want to delete this resource server? This action cannot be undone.',
    'delete.disclaimer':
      'Warning: All associated resources, actions, and permission strings will be permanently removed.',
    'delete.error': 'Failed to delete resource server. Please try again.',
    'create.steps.type': 'Type',
    'create.steps.name': 'Details',
    'create.steps.separator': 'Permission delimiter',
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
    'edit.settings.general.organizationUnit.label': 'Organization unit',
    'edit.settings.save': 'Save changes',
    'edit.settings.saved': 'Settings saved successfully.',
    'edit.settings.error': 'Failed to save settings.',
    'edit.resources.title': 'Resources & actions',
    'edit.resources.subtitle': 'Define the resources and actions for this resource server.',
    'edit.resources.addResource': 'Add resource',
    'edit.resources.addAction': 'Add action',
    'edit.resources.noResources': 'No resources defined yet.',
    'edit.resources.deleteResource.title': 'Delete resource',
    'edit.resources.deleteResource.message': 'Are you sure? All actions under this resource will also be removed.',
    'edit.resources.deleteAction.title': 'Delete action',
    'edit.resources.deleteAction.message': 'Are you sure you want to delete this action?',
    'edit.resources.form.name.label': 'Name',
    'edit.resources.form.name.placeholder': 'Enter resource name',
    'edit.resources.form.name.required': 'Name is required.',
    'edit.resources.form.handle.label': 'Handle',
    'edit.resources.form.handle.placeholder': 'resource-handle',
    'edit.resources.form.description.label': 'Description',
    'edit.resources.form.description.placeholder': 'Enter description',
    'edit.resources.detail.title': 'Resource details',
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
    'create.name.title': "Let's collect some details about your resource server",
    'create.name.titleMcp': "Let's collect some details about your MCP server",
    'create.name.nameLabel': 'Resource server name',
    'create.name.nameLabelMcp': 'MCP server name',
    'create.name.namePlaceholder': 'e.g. Payments API',
    'create.name.identifierLabel': 'Identifier',
    'create.name.identifierPlaceholder': 'https://api.example.com',
    'create.name.identifierPlaceholderMcp': 'https://mcp.example.com/mcp',
    'create.name.identifierHint':
      'A unique identifier for this resource server. When set as an absolute URI, it becomes the token audience for RFC 8707 resource indicators.',
    'create.name.identifierHintMcp':
      'The URL MCP clients use to reach this server, such as https://mcp.example.com/mcp. MCP clients send it as the resource parameter, and it becomes the token audience.',
    'create.name.identifierInvalidMcp':
      'Enter an http or https URL with no fragment, such as https://mcp.example.com/mcp.',
    'create.name.makeDefaultLabel': 'Make this the default resource server',
    'create.setDefaultError': 'Resource server created, but it could not be made the default.',
    'create.separator.title': 'Choose your permission delimiter',
    'create.separator.subtitle':
      'The delimiter character joins parts of a permission string. This cannot be changed after creation.',
    'create.separator.label': 'Permission delimiter',
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
    'create.error': 'Failed to create resource server. Please try again.',
    'create.creating': 'Creating…',
    'create.submit': 'Create resource server',
    'create.submitMcp': 'Create MCP server',
    'edit.tab.resources': 'Resources',
    'edit.tab.advanced': 'Advanced',
    'edit.defaultBadge': 'Default resource server',
    'edit.defaultBadgeManaged': 'Managed by server configuration.',
    'edit.back': 'Back to resource servers',
    'edit.identifierRequired': 'Identifier is required.',
    'edit.errorTitle': 'Failed to load resource server',
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
    'edit.advanced.identifier.label': 'Identifier (audience)',
    'edit.advanced.identifier.hint':
      'A unique value that identifies this resource server. When set as an URI, enables RFC 8707 resource indicator support in OAuth2 authorization requests.',
    'edit.advanced.identifier.hintMcp':
      'The URL MCP clients use to reach this server, such as https://mcp.example.com/mcp. MCP clients send it as the resource parameter, and it becomes the token audience.',
    'edit.advanced.identifier.invalidMcp':
      'Enter an http or https URL with no fragment, such as https://mcp.example.com/mcp.',
    'edit.advanced.identifier.placeholder': 'https://api.example.com',
    'edit.advanced.identifier.placeholderMcp': 'https://mcp.example.com/mcp',
    'edit.advanced.identifier.saved': 'Identifier saved.',
    'edit.advanced.identifier.saveError': 'Failed to save identifier.',
    'edit.advanced.authorizationEngine.label': 'Authorization engine',
    'edit.advanced.authorizationEngine.option.rbac': 'Local - role based access control',
    'edit.advanced.authorizationEngine.option.selectedPDP': 'Selected PDP',
    'edit.advanced.authorizationEngine.loadPDPError': 'Failed to load AuthZEN PDP connections.',
    'edit.advanced.authorizationEngine.noPDPConnections': 'No external PDP connections are available.',
    'edit.advanced.authorizationEngine.createPDPConnection': 'Create a PDP connection',
    'edit.advanced.authorizationEngine.hint':
      'Choose Local - role based access control or a configured PDP connection for this resource server.',
    'edit.dangerZone.title': 'Danger zone',
    'edit.dangerZone.description': 'Irreversible actions for this resource server.',
    'edit.dangerZone.descriptionMcp': 'Irreversible actions for this MCP server.',
    'edit.dangerZone.deleteServer': 'Delete resource server',
    'edit.dangerZone.deleteServerMcp': 'Delete MCP server',
    'tree.title': 'Resource hierarchy',
    'detail.identifierRequired': 'Identifier is required.',
    'tree.add': 'Add',
    'tree.addResource.title': 'Add resource',
    'tree.addServerAction': 'Add server-level action',
    'tree.empty': 'No resources yet. Add a resource or action to get started.',
    'tree.emptyReadOnly': 'No resources are defined for this resource server.',
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

    // Backend error code translations (per resource server service error envelope).
    'errors.RES-1001': 'The request body is malformed or contains invalid data.',
    'errors.RES-1002': 'An ID is required for this request.',
    'errors.RES-1003': 'This resource server no longer exists. It may have already been deleted.',
    'errors.RES-1004': 'A resource server with the same name already exists.',
    'errors.RES-1005': 'The specified parent resource does not exist.',
    'errors.RES-1006': 'This cannot be deleted because it still has resources or actions. Remove them first.',
    'errors.RES-1008': 'This resource no longer exists. It may have already been deleted.',
    'errors.RES-1009': 'This action no longer exists. It may have already been deleted.',
    'errors.RES-1010': 'The specified organization unit does not exist.',
    'errors.RES-1011': 'The limit parameter must be a positive integer.',
    'errors.RES-1012': 'The offset parameter must be a non-negative integer.',
    'errors.RES-1013': 'A resource server with the same identifier already exists.',
    'errors.RES-1014': 'A resource or action with the same handle already exists under this parent.',
    'errors.RES-1015': 'The delimiter must be a single valid character: . _ : - /',
    'errors.RES-1016': 'The handle must be under 100 characters and contain only letters, numbers, and . _ : - /',
    'errors.RES-1017': 'The handle cannot contain the delimiter character.',
    'errors.RES-1018': 'This resource server is managed declaratively and cannot be modified.',
    'errors.RES-1019': 'This resource is managed declaratively and cannot be modified.',
    'errors.RES-1020': 'This action is managed declaratively and cannot be modified.',
    'errors.RES-1021': 'The number of records exceeds the maximum allowed in composite mode.',
    'errors.RES-1023': 'A resource server with the specified ID already exists.',
    'errors.RES-1024':
      'An MCP server identifier must be an http or https URL with no fragment, such as https://mcp.example.com/mcp.',
  },

  // ============================================================================
  // Verifiable Presentations namespace - OpenID4VP presentation definitions
  // ============================================================================
  'verifiable-presentations': {
    // List page
    'listing.title': 'Presentation definitions',
    'listing.subtitle': 'Define which verifiable credentials your verifier requests from wallets via OpenID4VP.',
    'listing.add': 'Add definition',
    'listing.error': 'Failed to load presentation definitions',
    'listing.columns.name': 'Name',
    'listing.columns.organizationUnit': 'Organization unit',
    'listing.columns.actions': 'Actions',
    'listing.verify': 'Verify',

    // Verification dialog
    'verify.title': 'Verify presentation',
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
    'form.tabs.issuerTrust': 'Issuer trust',
    'form.tabs.advanced': 'Advanced',
    'form.quickCopy.title': 'Quick copy',
    'form.quickCopy.description': "Copy the definition's identifiers for use in flows and API calls.",
    'form.quickCopy.idHint': 'Use this ID to reference the presentation definition in API calls.',
    'form.id.label': 'Definition ID',
    'form.copyId': 'Copy definition ID',
    'form.handle.label': 'Handle',
    'form.handle.hint': 'The handle identifies this presentation definition in flow steps and DCQL queries.',
    'form.name.label': 'Name',
    'form.name.placeholder': 'EUDI Wallet PID',
    'form.name.hint': 'A memorable label for this presentation definition, shown only in the console.',
    'form.organizationUnit.title': 'Organization unit',
    'form.organizationUnit.description': 'The organization unit this presentation definition belongs to.',
    'form.organizationUnit.label': 'Organization unit',
    'form.organizationUnit.pickerHint': 'Select the organization unit this presentation definition will belong to.',
    'form.organizationUnit.handleLabel': 'Handle',
    'form.organizationUnit.handleHint': 'Use this handle to reference the organization unit in configuration files.',
    'form.organizationUnit.idLabel': 'ID',
    'form.organizationUnit.idHint': 'Use this ID to reference the organization unit in API calls.',
    'form.protocol.title': 'Settings',
    'form.protocol.description': 'The credential type and format this presentation definition requests.',
    'form.vct.label': 'Credential type (VCT)',
    'form.vct.hint': 'The credential type (vct) wallets must present to satisfy this request.',
    'form.format.label': 'Format',
    'form.format.sdJwt': 'SD-JWT VC (dc+sd-jwt)',
    'form.format.hint': 'Only SD-JWT VC is currently supported.',
    'form.dangerZone.title': 'Danger zone',
    'form.dangerZone.description': 'Irreversible actions for this presentation definition.',
    'form.dangerZone.delete': 'Delete presentation definition',
    'form.dangerZone.deleteDescription':
      'Login flows referencing this presentation definition will stop working, and wallets will no longer be able to complete this verification request. Verifications already completed are not affected.',
    'form.unsavedChanges': 'You have unsaved changes',

    // Form - Issuer Trust tab
    'form.issuerTrust.title': 'Issuer trust',
    'form.issuerTrust.description': 'Control which credential issuers are accepted for this presentation definition.',
    'form.issuerTrust.enforce.label': 'Enforce trusted issuer',
    'form.issuerTrust.enforce.hint': 'When enabled, only credentials issued by a trusted issuer are accepted.',
    'form.issuerTrust.enforce.noAnchorsHint':
      'No trust anchors are configured, so issuer trust cannot be enforced. Configure trust anchors first.',
    'form.issuerTrust.authorities.label': 'Trusted issuers',
    'form.issuerTrust.authorities.hint': 'Leave empty to accept any registered trust anchor.',
    'form.issuerTrust.authorities.optionSecondary': '{{subject}} · expires {{notAfter}}',

    // Claims editor
    'claims.empty': 'No claims yet. Add the claims this definition should request from the wallet.',
    'claims.remove': 'Remove claim',
    'claims.name': 'Claim',
    'claims.nameHint':
      'The claim path to request from the wallet, and whether it must be disclosed (mandatory) or may be withheld (optional).',
    'claims.errors.duplicate': 'This claim is already requested. Each claim can be listed only once.',
    'claims.requirement': 'Requirement',
    'claims.mandatory': 'Mandatory',
    'claims.optional': 'Optional',
    'claims.values': 'Allowed values',
    'claims.valuesPlaceholder': 'Leave empty to allow any value',
    'claims.valuesHint':
      'If set, the disclosed value must match one of these (compared as text). Enforced at verification.',
    'claims.add': 'Add claim',

    // Create wizard
    'createWizard.steps.name': 'Details',
    'createWizard.name.title': "Let's collect some details about your verifiable presentation",
    'create.title': 'New presentation definition',
    'create.subtitle': 'Configure the credential type and the claims to request from the wallet.',
    'create.steps.organizationUnit': 'Organization unit',
    'create.steps.details': 'Type',
    'create.steps.claims': 'Claims',
    'create.organizationUnit.title': 'Where should this verifiable presentation belong?',
    'create.organizationUnit.subtitle':
      "Choose the organization unit that will own this verifiable presentation. You can't change this once created.",
    'create.organizationUnit.fieldLabel': 'Organization unit',
    'create.claims.help': 'Add each claim once and set whether it is mandatory and restrict its allowed values.',
    'create.success': 'Presentation definition created',
    'create.error': 'Failed to create presentation definition',

    // Edit page
    'edit.back': 'Back to presentation definitions',
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

    // Verification failure (generic - the backend does not yet expose a per-reason code)
    'verify.failedGeneric': 'The wallet could not complete verification. Please try again.',

    // Backend error code translations (per presentation-definition service error envelope).
    'errors.VP-2002': 'This presentation definition no longer exists. It may have already been deleted.',
    'errors.VP-2003': 'A presentation definition with the same handle already exists.',
    'errors.VP-2005': 'This presentation definition is managed declaratively and cannot be modified or deleted.',
    'errors.VP-2006':
      'The number of presentation definitions exceeds the supported limit. Use search to narrow the results.',
    'errors.VP-2007': 'The specified organization unit is not valid.',
    'errors.EUDI-1002': 'This verification request is no longer active. Start a new verification.',
    'errors.EUDI-1004': 'This presentation definition is not registered for verification.',
  },
  'verifiable-credentials': {
    // List page
    'listing.title': 'Credential templates',
    'listing.subtitle': 'Define the credentials your issuer can issue to wallets via OpenID4VCI.',
    'listing.add': 'Add template',
    'listing.error': 'Failed to load credential templates',
    'listing.offer': 'Generate offer',
    'listing.columns.name': 'Name',
    'listing.columns.organizationUnit': 'Organization unit',
    'listing.columns.actions': 'Actions',

    // Create page
    'createWizard.steps.name': 'Details',
    'createWizard.name.title': "Let's collect some details about your verifiable credential",
    'create.title': 'New credential template',
    'create.subtitle': 'Define the credential type, claims and display shown in wallets.',
    'create.steps.organizationUnit': 'Organization unit',
    'create.steps.details': 'Type',
    'create.steps.claims': 'Claims',
    'create.organizationUnit.title': 'Where should this verifiable credential belong?',
    'create.organizationUnit.subtitle':
      "Choose the organization unit that will own this verifiable credential. You can't change this once created.",
    'create.organizationUnit.fieldLabel': 'Organization unit',
    'create.claims.help':
      'Add each claim once and set the attribute name as it appears in the user profile and how it should be displayed in the wallet.',
    'create.success': 'Credential template created',
    'create.error': 'Failed to create credential template',

    // Form
    'form.tabs.general': 'General',
    'form.tabs.protocolSettings': 'Settings',
    'form.tabs.claims': 'Claims',
    'form.tabs.advanced': 'Advanced',
    'form.quickCopy.title': 'Quick copy',
    'form.quickCopy.description': "Copy the credential template's identifiers for use in flows and API calls.",
    'form.quickCopy.idHint': 'Use this ID to reference the credential template in API calls.',
    'form.id.label': 'Template ID',
    'form.copyId': 'Copy template ID',
    'form.handle.label': 'Handle',
    'form.handle.hint': 'The handle is the credential identifier and OAuth scope wallets request.',
    'form.name.label': 'Name',
    'form.name.placeholder': 'EUDI Wallet PID',
    'form.name.hint': 'A memorable label for this credential template, shown only in the console.',
    'form.organizationUnit.title': 'Organization unit',
    'form.organizationUnit.description': 'The organization unit this credential template belongs to.',
    'form.organizationUnit.label': 'Organization unit',
    'form.organizationUnit.pickerHint': 'Select the organization unit this credential template will belong to.',
    'form.organizationUnit.handleLabel': 'Handle',
    'form.organizationUnit.handleHint': 'Use this handle to reference the organization unit in configuration files.',
    'form.organizationUnit.idLabel': 'ID',
    'form.organizationUnit.idHint': 'Use this ID to reference the organization unit in API calls.',
    'form.protocol.title': 'Settings',
    'form.protocol.description': 'The OpenID4VCI credential type, format, and wallet display for this credential.',
    'form.vct.label': 'Credential type (VCT)',
    'form.vct.hint': 'The SD-JWT VC credential type wallets and verifiers use to identify this credential.',
    'form.format.label': 'Format',
    'form.format.sdJwt': 'SD-JWT VC (dc+sd-jwt)',
    'form.format.hint': 'Only SD-JWT VC is currently supported.',
    'form.display.locale': 'Locale',
    'form.display.localeHint': 'The BCP 47 language tag for the credential name shown in the wallet, e.g. en-US.',
    'form.display.logo': 'Logo URI',
    'form.display.logoHint': "A hosted image URL shown as the credential's logo in the wallet.",
    'form.dangerZone.title': 'Danger zone',
    'form.dangerZone.description': 'Irreversible actions for this credential template.',
    'form.dangerZone.delete': 'Delete credential template',
    'form.dangerZone.deleteDescription':
      'Wallets will no longer be able to request this credential. Credentials already issued are not revoked and remain valid until they expire.',
    'form.unsavedChanges': 'You have unsaved changes',

    // Claims editor
    'claims.empty': 'No claims yet. Add the attributes this credential should disclose.',
    'claims.add': 'Add claim',
    'claims.remove': 'Remove claim',
    'claims.name': 'Attribute name',
    'claims.displayName': 'Display name',
    'claims.nameHint': 'Must match a user profile attribute name; the value is sourced from the user.',
    'claims.errors.duplicate': 'This attribute is already added. Each attribute can be listed only once.',
    'claims.errors.reserved': 'This name is reserved by the credential format and cannot be used as an attribute.',

    // Offer dialog
    'offer.title': 'Credential offer',
    'offer.openInWallet': 'Open in wallet',
    'offer.copy': 'Copy offer link',
    'offer.notConfigured':
      'Credential issuance is not enabled. Configure an issuer signing key to generate credential offers.',

    // Edit page
    'edit.back': 'Back to credential templates',
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

    // Backend error code translations (per credential-configuration service error envelope).
    'errors.VCI-2002': 'This credential template no longer exists. It may have already been deleted.',
    'errors.VCI-2003': 'A credential template with the same handle already exists.',
    'errors.VCI-2005': 'This credential template is managed declaratively and cannot be modified or deleted.',
    'errors.VCI-2006':
      'The number of credential templates exceeds the supported limit. Use search to narrow the results.',
    'errors.VCI-2007': 'The specified organization unit is not valid.',
  },

  // ============================================================================
  // Gateways namespace - Gateway management translations
  // ============================================================================
  gateways: {
    default: 'Default',
    'delete.confirm': 'Remove the registration of {{name}}?',
    'delete.disclaimer': 'The gateway itself is not changed. It keeps running with the configuration it already holds.',
    'delete.error': 'The gateway could not be removed. Please try again.',
    'delete.submit': 'Remove',
    'delete.submitting': 'Removing...',
    'delete.success': 'Gateway removed.',
    'delete.title': 'Remove gateway',
    'detail.back': 'Back to gateways',
    'detail.connection.description': 'Where this gateway answers and how it is trusted.',
    'detail.connection.title': 'Connection',
    'detail.dangerZone.description': 'Removing the registration stops this deployment administering the gateway.',
    'detail.dangerZone.title': 'Danger zone',
    'detail.error': 'Failed to load the gateway',
    'detail.key.description':
      'The key presented to this gateway is never shown after it is set. Rotate it to replace it.',
    'detail.key.hint':
      'After rotating, configure the gateway with the new key so it keeps accepting configuration from here.',
    'detail.key.title': 'Key',
    'edit.error': 'The gateway could not be updated.',
    'edit.success': 'Gateway updated.',
    'edit.unsavedChanges': 'You have unsaved changes.',
    'form.baseUrl.hint': 'Where the gateway answers. Each gateway registers once.',
    'form.baseUrl.label': 'Base URL',
    'form.baseUrl.placeholder': 'https://gateway.example.com:8090',
    'form.caCertificate.hint':
      'A PEM certificate to trust when calling this gateway, for one serving a certificate no public authority signed.',
    'form.caCertificate.label': 'CA certificate (optional)',
    'form.isDefault.label': 'Make this the default gateway, whose base URL the console shows for runtime endpoints',
    'form.key.hint': 'Leave empty to generate one. Supply a key only when the gateway already holds one.',
    'form.key.label': 'Key (optional)',
    'form.name.label': 'Name',
    'form.name.placeholder': 'e.g. production',
    'list.columns.actions': 'Actions',
    'list.columns.baseUrl': 'Base URL',
    'list.columns.createdAt': 'Created',
    'list.columns.name': 'Name',
    'list.empty': 'No gateways are registered yet.',
    'list.error': 'Failed to load gateways',
    'list.register': 'Register gateway',
    'list.subtitle': 'Register the gateways this deployment administers and how it reaches them.',
    'list.title': 'Gateway management',
    'register.copyKey': 'Copy key',
    'register.description':
      'Record where a gateway answers. The gateway is not contacted until a configuration is applied to it.',
    'register.done': 'I have saved the key',
    'register.error': 'The gateway could not be registered. Please try again.',
    'register.keyDescription': 'Configure {{name}} with this key so it accepts configuration from here.',
    'register.keyLabel': 'Gateway key',
    'register.keyTitle': 'Save the gateway key',
    'register.keyWarning':
      'This key will not be shown again. Copy it now. If it is lost, rotate the key from the gateway page.',
    'register.submit': 'Register',
    'register.submitting': 'Registering...',
    'register.title': 'Register a gateway',
    'rotateKey.action': 'Rotate key',
    'rotateKey.copyLabel': 'Key to give the gateway',
    'rotateKey.description':
      'Replace the key presented to {{name}}. Give the gateway the same key, or it will refuse configuration from here.',
    'rotateKey.error': 'The key could not be rotated. Please try again.',
    'rotateKey.generate': 'Generate',
    'rotateKey.label': 'New key',
    'rotateKey.savedDescription':
      'The key was rotated. Configure {{name}} with this key so it keeps accepting configuration from here.',
    'rotateKey.savedTitle': 'Save the new key',
    'rotateKey.submit': 'Rotate key',
    'rotateKey.submitting': 'Rotating...',
    'rotateKey.success': 'The key was rotated.',
    'rotateKey.title': 'Rotate key',
    'rotateKey.warning': 'This key will not be shown again once the dialog is closed. Copy it now.',

    // Backend error code translations (per gateway service error envelope).
    'errors.GTW-1001': 'The gateway ID is not valid.',
    'errors.GTW-1002': 'The gateway was not found. It may have been removed.',
    'errors.GTW-1003': 'A gateway name is required.',
    'errors.GTW-1004': 'Another gateway is already registered under that name.',
    'errors.GTW-1005': 'A base URL is required to reach the gateway.',
    'errors.GTW-1006': 'This deployment already administers as many gateways as it is configured to allow.',
    'errors.GTW-1007': 'The request could not be read. Please try again.',
    'errors.GTW-1008': 'A gateway is already registered at that base URL.',
    'errors.GTW-1009':
      'The base URL is not valid. Enter an absolute URL, for example https://gateway.example.com:8090.',
    'errors.GTW-1010': 'A key cannot be empty.',
    'errors.GTW-1011': 'Nothing to change. Edit the name, base URL, CA certificate or key.',
    'errors.GTW-1012':
      'This gateway is declared in a file, so it cannot be changed or removed here. Change the file and restart instead.',
    'errors.GTW-1013': 'That configuration version was not found.',
    'errors.GTW-1014': 'A version is a number or latest.',
    'errors.GTW-1015': 'No configuration version has been captured yet. Capture one first.',
    'errors.GTW-1016': 'This gateway holds no earlier version to revert to.',
    'errors.GTW-1017':
      'This gateway is missing variables or secrets the version needs. Set them on the gateway, then try again.',
    'errors.GTW-1018': 'The version was removed or replaced while it was deployed. Deploy a newer version.',
    'errors.GTW-1019': 'Another deploy to this gateway finished first. Check what it runs now, then try again.',
    'errors.GTW-1029': 'Another deploy to this gateway is running. Try again once it finishes.',
    'errors.GTW-5001': 'The gateway could not apply the configuration.',
    'errors.GTW-5003': 'The deploy took too long to prepare and sent nothing to the gateway. Try again.',
    'errors.VAR-1002':
      'A name can use only letters, digits and underscores, must start with a letter or an underscore, and can be at most 255 characters.',
    'errors.VAR-1003': 'A value is required and can be at most 8192 characters.',
    'errors.VAR-1004': 'The gateway holds nothing by that name. It may have been deleted.',
    'errors.VAR-1008': 'A description can be at most 1000 characters.',
    'errors.VAR-1009': 'The gateway already holds something by that name.',
  },

  // ============================================================================
  // Settings namespace - Server-wide settings translations
  // ============================================================================
  deploy: {
    'applyPanel.cancel': 'Cancel',
    'applyPanel.checkFailed': 'What the gateway would write could not be read.',
    'applyPanel.checking': 'Working out what {{gateway}} would write...',
    'applyPanel.confirmRemovals':
      'Deploying removes {{count}} resources from {{gateway}}. Check them in the list, then deploy again to go ahead.',
    'applyPanel.done': 'Deployed to {{gateway}}.',
    'applyPanel.doneWithFailures': 'Deployed to {{gateway}}; {{count}} resources were refused.',
    'applyPanel.failed': 'The version could not be deployed.',
    'applyPanel.forbidden': 'Your account may not deploy to this gateway.',
    'applyPanel.gateway': 'Gateway',
    'applyPanel.submit': 'Deploy',
    'applyPanel.submitConfirm': 'Deploy and remove {{count}}',
    'applyPanel.title': 'Deploy {{label}}',
    'applyPanel.to': 'to',
    'changed.hint': 'These values take effect with this deploy, though no version carries them.',
    'changed.secret': 'Secret',
    'changed.title': 'Values set since the last deploy',
    'changed.usedBy': 'used by {{names}}',
    'changed.variable': 'Variable',
    'changes.added': 'New',
    'changes.deleted': 'Removed',
    'changes.heldBack': 'Held back',
    'changes.heldBackOn': 'Held back on {{target}}',
    'changes.none': 'The gateway already runs this version as it is.',
    'changes.selectAll': '{{count}} resources to write',
    'changes.selectOne': 'Write {{name}}',
    'changes.sourceHeldBackHint': '{{source}} does not run this change, so it cannot be promoted.',
    'changes.unchanged': 'Unchanged',
    'changes.unchangedCount': '{{count}} resources are already as the version has them.',
    'changes.updated': 'Changed',
    'diff.hide': 'Hide changes',
    'diff.hideLabel': 'Hide the changes to {{name}}',
    'diff.show': 'Show changes',
    'diff.showLabel': 'Show the changes to {{name}}',
    'diff.unchangedLines': '{{count}} unchanged lines',
    'explorer.edges.agentIn': 'Agent in {{name}}',
    'explorer.edges.allowsAgentType': 'Allows agent type {{name}}',
    'explorer.edges.allowsUserType': 'Allows user type {{name}}',
    'explorer.edges.applicationIn': 'Application in {{name}}',
    'explorer.edges.assignedTo': 'Assigned to {{name}}',
    'explorer.edges.grants': 'Grants {{scopes}}',
    'explorer.edges.groupIn': 'Group in {{name}}',
    'explorer.edges.memberOf': 'Member of {{name}}',
    'explorer.edges.provisionsAs': 'Provisions users as {{name}}',
    'explorer.edges.recovers': 'Recovers passwords with {{name}}',
    'explorer.edges.registers': 'Registers users with {{name}}',
    'explorer.edges.resourceServerIn': 'Resource server in {{name}}',
    'explorer.edges.roleIn': 'Role in {{name}}',
    'explorer.edges.signsIn': 'Signs users in with {{name}}',
    'explorer.edges.signsOut': 'Signs users out with {{name}}',
    'explorer.edges.step': 'Step {{step}}: {{name}}',
    'explorer.edges.userIn': 'User in {{name}}',
    'explorer.edges.userOfType': 'User of type {{name}}',
    'explorer.edges.usersStoredIn': 'Users stored in {{name}}',
    'explorer.endpoints.authorize': 'Authorization',
    'explorer.endpoints.discovery': 'Discovery',
    'explorer.endpoints.introspect': 'Introspection',
    'explorer.endpoints.jwks': 'JWKS',
    'explorer.endpoints.logout': 'Logout',
    'explorer.endpoints.redirect': 'Redirect URI',
    'explorer.endpoints.revoke': 'Revocation',
    'explorer.endpoints.token': 'Token',
    'explorer.endpoints.userinfo': 'User info',
    'explorer.heldBack': 'Held back',
    'explorer.kinds.agent': 'Agent',
    'explorer.kinds.agent_type': 'Agent type',
    'explorer.kinds.application': 'Application',
    'explorer.kinds.connection': 'Connection',
    'explorer.kinds.flow': 'Flow',
    'explorer.kinds.group': 'Group',
    'explorer.kinds.layout': 'Layout',
    'explorer.kinds.organization_unit': 'Organization unit',
    'explorer.kinds.resource_server': 'Resource server',
    'explorer.kinds.role': 'Role',
    'explorer.kinds.theme': 'Theme',
    'explorer.kinds.user': 'User',
    'explorer.kinds.user_type': 'User type',
    'explorer.lanes.agents': 'Agents',
    'explorer.lanes.applications': 'Applications',
    'explorer.lanes.connections': 'Connections',
    'explorer.lanes.design': 'Design',
    'explorer.lanes.flows': 'Flows',
    'explorer.lanes.identities': 'Users and groups',
    'explorer.lanes.organizationUnits': 'Organization units',
    'explorer.lanes.resourceServers': 'Resource servers',
    'explorer.lanes.roles': 'Roles',
    'explorer.lanes.userTypes': 'User types',
    'explorer.noGateway': 'No gateway is registered under this id.',
    'explorer.nothingDeployed': 'Nothing has been deployed to {{gateway}} yet.',
    'explorer.panel.clientEndpointsHint': 'Where {{name}} signs users in and gets tokens on {{gateway}}.',
    'explorer.panel.close': 'Close',
    'explorer.panel.components': 'Components',
    'explorer.panel.deployedVersion': 'Deployed version',
    'explorer.panel.discard': 'Discard',
    'explorer.panel.edited': 'Edited',
    'explorer.panel.endpoints': 'Endpoints',
    'explorer.panel.environment': 'Gateway',
    'explorer.panel.gatewayEndpointsHint': 'Where clients discover {{gateway}} and verify its tokens.',
    'explorer.panel.hide': 'Hide',
    'explorer.panel.laneShared': '{{count}} · shared',
    'explorer.panel.laneValues': '{{count}} · {{values}} values',
    'explorer.panel.listHint': 'One per line.',
    'explorer.panel.noValues':
      'This resource has no gateway-specific values. Its configuration is the same on every gateway and changes through a captured version.',
    'explorer.panel.notSet': 'Not set',
    'explorer.panel.redirectHint': 'Register this with the provider so it returns users to {{gateway}}.',
    'explorer.panel.relationships': 'Relationships · {{count}}',
    'explorer.panel.reveal': 'Show',
    'explorer.panel.save': 'Save changes',
    'explorer.panel.saving': 'Saving...',
    'explorer.panel.secretPlaceholder': 'Enter a new value to replace',
    'explorer.panel.secretStored': 'Stored',
    'explorer.panel.secretUpdated': 'Updated, not captured',
    'explorer.panel.secrets': 'Secrets',
    'explorer.panel.selectHint': 'Select a resource on the map to configure the values {{gateway}} holds for it.',
    'explorer.panel.values': 'Values',
    'explorer.panel.valuesHint': 'Editable. Changes apply to {{gateway}} only.',
    'explorer.panel.valuesTitle': 'Values for {{gateway}}',
    'explorer.panel.variables': 'Variables',
    'explorer.readFailed': 'What {{gateway}} runs could not be read.',
    'explorer.referencesFailed': 'Which values {{gateway}} refers to could not be read, so none are shown.',
    'explorer.saveFailed': 'The values could not be saved.',
    'explorer.saved': 'Saved {{count}} values to {{gateway}}.',
    'explorer.savedSome': 'Saved {{saved}} of {{count}} values to {{gateway}}. {{reason}}',
    'explorer.switch': 'Gateway',
    'explorer.fullscreen.enter': 'Full screen',
    'explorer.fullscreen.exit': 'Exit full screen',
    'explorer.zoom.fit': 'Fit to width',
    'explorer.zoom.in': 'Zoom in',
    'explorer.zoom.out': 'Zoom out',
    'header.exit': 'Exit',
    'header.title': 'Deploy',
    'missing.allSet': 'All values set',
    'missing.listHint': 'One per line.',
    'missing.needed': 'Needed',
    'missing.progress': '{{filled}} of {{total}} set',
    'missing.secretPlaceholder': 'Secret value',
    'missing.title': 'Values this gateway needs',
    'missing.variablePlaceholder': 'Variable value',
    'page.description':
      'Deploy any version of the configuration to any gateway. The newest is kept up to date as the configuration changes.',
    'page.gatewaysFailed': 'The gateways could not be read.',
    'page.versionsFailed': 'The configuration versions could not be read.',
    'page.noGateways': 'Register a gateway to deploy a version to it.',
    'page.title': 'Deploy',
    'revertPanel.cancel': 'Cancel',
    'revertPanel.changes': 'Rolling back writes {{count}} resources as the earlier version has them.',
    'revertPanel.checking': 'Working out what rolling back would change...',
    'revertPanel.confirmRemovals':
      'Rolling back removes {{count}} resources from {{gateway}}. Roll back again to go ahead.',
    'revertPanel.done': '{{gateway}} runs its earlier version again.',
    'revertPanel.doneWithFailures': '{{gateway}} runs its earlier version again; {{count}} resources were refused.',
    'revertPanel.failed': 'The gateway could not be rolled back.',
    'revertPanel.forbidden': 'Your account may not roll this gateway back.',
    'revertPanel.noChanges': 'Rolling back changes nothing the gateway runs.',
    'revertPanel.submit': 'Roll back',
    'revertPanel.submitConfirm': 'Roll back and remove {{count}}',
    'revertPanel.title': 'Roll back {{gateway}} to {{label}}',
    'revertPanel.titleUnknown': 'Roll back {{gateway}}',
    'revertPanel.what':
      'A rollback goes back one step, to {{label}}, the version {{gateway}} ran before. To go further back, deploy an older version.',
    'revertPanel.whatUnknown':
      'A rollback goes back one step, to the version {{gateway}} ran before. To go further back, deploy an older version.',
    'stage.configure': 'Configure',
    'stage.deployedAt': 'Deployed',
    'stage.deployedUnknown': 'Not recorded',
    'stage.deployedVersion': 'Deployed version',
    'stage.explore': 'Explore',
    'stage.exploreLabel': 'Explore {{name}}',
    'stage.heldBack': '{{count}} held back, not running',
    'stage.nothingDeployed': 'Nothing deployed yet',
    'stage.changed_one': '{{count}} value changed since deploy',
    'stage.changed_other': '{{count}} values changed since deploy',
    'stage.redeploy': 'Redeploy',
    'stage.redeployHint': 'Deploy the version {{name}} runs again, so {{values}} take effect.',
    'stage.redeployNone': 'Nothing to redeploy: no value has changed since {{name}} was deployed.',
    'stage.changedCheckFailed': 'Could not check whether any value changed since {{name}} was deployed.',
    'stage.checkAgain': 'Check again',
    'stage.revert': 'Rollback',
    'stage.revertHint': 'Go back one step, to the version {{name}} ran before.',
    'stage.revertNone':
      'Nothing to roll back to. A rollback goes back one step, once; deploy a version to {{name}} to roll back again.',
    'stage.url': 'URL',
    'stage.valuesSummary': '{{variables}} variables, {{secrets}} secrets',
    'time.utc': '{{time}} UTC',
    'url.copied': 'Copied',
    'url.copy': 'Copy the URL',
    'values.cancel': 'Cancel',
    'values.delete': 'Remove',
    'values.deleteFailed': 'The value could not be removed.',
    'values.deleteLabel': 'Remove {{name}}',
    'values.description':
      'The variables and secrets this gateway holds. The configuration refers to them by name; each gateway holds its own.',
    'values.edit': 'Edit',
    'values.empty': '(empty)',
    'values.listHint': 'One per line.',
    'values.missing': 'The version {{gateway}} runs refers to {{count}} values it does not hold.',
    'values.missingTitle': 'Missing values',
    'values.more': '+{{count}} more',
    'values.name': 'Name',
    'values.noSecrets': 'This gateway holds no secrets.',
    'values.noVariables': 'This gateway holds no variables.',
    'values.notSet': 'Not set',
    'values.other': 'Not used by the version this gateway runs',
    'values.readFailed': 'The values {{gateway}} holds could not be read.',
    'values.referencesFailed': 'Which values the version {{gateway}} runs refers to could not be read.',
    'values.replace': 'Replace',
    'values.save': 'Save',
    'values.saveFailed': 'The value could not be saved.',
    'values.secretPlaceholder': 'Enter a new value to replace',
    'values.secretsTab': 'Secrets ({{count}})',
    'values.set': 'Set',
    'values.setLabel': 'Set {{name}}',
    'values.stored': 'Stored',
    'values.title': 'Configure {{gateway}}',
    'values.types.agent': 'Agents',
    'values.types.agent_type': 'Agent types',
    'values.types.application': 'Applications',
    'values.types.connection': 'Connections',
    'values.types.flow': 'Flows',
    'values.types.group': 'Groups',
    'values.types.layout': 'Layouts',
    'values.types.organization_unit': 'Organization units',
    'values.types.resource_server': 'Resource servers',
    'values.types.role': 'Roles',
    'values.types.server_config': 'Server configuration',
    'values.types.theme': 'Themes',
    'values.types.translation': 'Translations',
    'values.types.user': 'Users',
    'values.types.user_type': 'User types',
    'values.value': 'Value',
    'values.variablesTab': 'Variables ({{count}})',
    'version.hash': 'Hash {{hash}}',
    'versions.deployRow': 'Deploy',
    'versions.alreadyDeployed': 'Already deployed to {{targets}}',
    'versions.description': 'Captured from the configuration, newest first.',
    'versions.details': 'Details of v{{number}}',
    'versions.empty': 'No version has been captured yet.',
    'versions.latest': 'Latest',
    'versions.showFewer': 'Show fewer',
    'versions.showOlder': 'Show {{count}} older',
    'versions.title': 'Configuration versions',
  },
  settings: {
    'page.title': 'Settings',
    'page.subtitle': 'Settings that apply across your entire ThunderID deployment.',
    'tabs.ariaLabel': 'Settings sections',
    'tabs.cors': 'CORS',
    'tabs.accountSecurity': 'Account security',
    'cors.card.title': 'Allowed origins',
    'cors.card.description':
      'Manage which origins are allowed to access your APIs. Each entry is either an exact origin or a regular expression.',
    'cors.readOnlyHint': "Some origins are read-only because they're managed declaratively.",
    'cors.addOrigin': 'Add origin',
    'cors.originPlaceholder': 'https://app.example.com',
    'cors.regexPlaceholder': '^https://[a-z0-9-]+\\.example\\.com$',
    'cors.removeOrigin': 'Remove origin',
    'cors.lockedOrigin': "Managed declaratively and can't be edited here.",
    'cors.type.label': 'Entry type',
    'cors.type.origin': 'Origin',
    'cors.type.regex': 'Regex',
    'cors.validation.invalidOrigin':
      'Enter a valid origin, e.g. https://app.example.com. Paths, query strings, and fragments are not allowed.',
    'cors.validation.invalidRegex': 'Enter a valid regular expression.',
    'cors.validation.unanchoredRegex':
      'This pattern is not anchored with ^ and $, so it also matches any origin that merely contains it.',
    'cors.validation.duplicate': 'This entry is already in the list.',
    'cors.unsavedChanges': 'You have unsaved changes',
    'cors.reset': 'Reset',
    'cors.save': 'Save changes',
    'cors.saving': 'Saving...',
    'cors.load.error': 'Failed to load allowed origins.',
    'cors.save.success': 'Allowed origins updated.',
    'cors.save.error': 'Failed to update allowed origins.',

    // Backend error code translations (per server-config service error envelope).
    'errors.SCF-1003': 'One or more origins are not valid. Enter a valid URL or regular expression.',

    // Automatic account lock
    'accountLock.section.title': 'Automatic account lock',
    'accountLock.section.description':
      "Temporarily block sign-in after too many failed attempts. Lockouts prevent new sign-ins but don't affect existing sessions or tokens. To revoke existing access, suspend the account.",

    'accountLock.toggle.ariaLabel': 'Automatic lock for {{title}}',
    'accountLock.user.title': 'Users',
    'accountLock.user.description': 'Temporarily block sign-in after repeated failed attempts.',
    'accountLock.agent.title': 'Agents',
    'accountLock.agent.description':
      'Temporarily block agent authentication after repeated failed attempts. Client secret authentication is excluded from lockouts.',

    'accountLock.summary.off.user': 'Automatic locking is off. Failed sign-in attempts do not lock users.',
    'accountLock.summary.off.agent': 'Automatic locking is off. Failed authentication attempts do not lock agents.',
    'accountLock.summary.trigger.authentication_method.user':
      'After {{count}} failed attempts with no more than {{window}} between them, the affected sign-in method is locked.',
    'accountLock.summary.trigger.authentication_method.agent':
      'After {{count}} failed attempts with no more than {{window}} between them, the affected authentication method is locked.',
    'accountLock.summary.trigger.account.user':
      'After {{count}} failed attempts with no more than {{window}} between them, all sign-in methods are locked.',
    'accountLock.summary.trigger.account.agent':
      'After {{count}} failed attempts with no more than {{window}} between them, all authentication methods are locked.',
    'accountLock.summary.triggerNoWindow.authentication_method.user':
      'After {{count}} failed attempts without a successful sign-in, the affected sign-in method is locked.',
    'accountLock.summary.triggerNoWindow.authentication_method.agent':
      'After {{count}} failed attempts without a successful authentication, the affected authentication method is locked.',
    'accountLock.summary.triggerNoWindow.account.user':
      'After {{count}} failed attempts without a successful sign-in, all sign-in methods are locked.',
    'accountLock.summary.triggerNoWindow.account.agent':
      'After {{count}} failed attempts without a successful authentication, all authentication methods are locked.',
    'accountLock.summary.duration.single': 'Each lockout lasts {{duration}}.',
    'accountLock.summary.duration.range':
      'The first lockout lasts {{from}}, increasing to {{to}} for subsequent lockouts.',
    'accountLock.summary.duration.permanentOnly.user':
      'Each lockout lasts until account recovery or administrator intervention.',
    'accountLock.summary.duration.permanentOnly.agent': 'Each lockout lasts until administrator intervention.',
    'accountLock.summary.duration.permanentRange.user':
      'The first lockout lasts {{from}}. Subsequent lockouts last until account recovery or administrator intervention.',
    'accountLock.summary.duration.permanentRange.agent':
      'The first lockout lasts {{from}}. Subsequent lockouts last until administrator intervention.',
    'accountLock.summary.duration.rangeThenPermanent.user':
      'The first {{count}} lockouts end on their own, starting at {{from}}. Subsequent lockouts last until account recovery or administrator intervention.',
    'accountLock.summary.duration.rangeThenPermanent.agent':
      'The first {{count}} lockouts end on their own, starting at {{from}}. Subsequent lockouts last until administrator intervention.',
    'accountLock.summary.duration.mixed.user':
      'Some lockouts last until account recovery or administrator intervention, and the others end on their own.',
    'accountLock.summary.duration.mixed.agent':
      'Some lockouts last until administrator intervention, and the others end on their own.',

    'accountLock.granularity.label': 'Lockout applies to',
    'accountLock.granularity.authenticationMethod.user': 'Only the affected sign-in method',
    'accountLock.granularity.authenticationMethod.agent': 'Only the affected authentication method',
    'accountLock.granularity.authenticationMethodHint.user': 'Users can still sign in using other methods.',
    'accountLock.granularity.authenticationMethodHint.agent': 'Agents can still authenticate using other methods.',
    'accountLock.granularity.account.user': 'All sign-in methods',
    'accountLock.granularity.account.agent': 'All authentication methods',
    'accountLock.granularity.accountHint.user':
      'Prevent users from signing in using any method until the lockout ends.',
    'accountLock.granularity.accountHint.agent':
      'Prevent agents from authenticating using any method until the lockout ends.',

    'accountLock.threshold.label': 'Lock after',
    'accountLock.threshold.ariaLabel': 'Failed attempts before lock',
    'accountLock.threshold.within': 'failed attempts within',
    'accountLock.window.ariaLabel': 'Longest gap allowed between failed attempts',
    'accountLock.window.hint': 'A longer gap between two failed attempts starts the count again.',
    'accountLock.window.never': 'No time limit',

    'accountLock.advanced.label': 'Advanced settings',

    'accountLock.ladder.label': 'Lockout duration',
    'accountLock.ladder.hint': 'Each successive lockout uses the next duration in the list.',
    'accountLock.ladder.hintSingle': 'How long a lockout lasts before it ends on its own.',
    'accountLock.ladder.position_ordinal_one': '{{count}}st',
    'accountLock.ladder.position_ordinal_two': '{{count}}nd',
    'accountLock.ladder.position_ordinal_few': '{{count}}rd',
    'accountLock.ladder.position_ordinal_other': '{{count}}th',
    'accountLock.ladder.row': '{{position}} lockout',
    'accountLock.ladder.rowLast': '{{position}} and later',
    'accountLock.ladder.add': 'Add duration',
    'accountLock.ladder.addFirstEscalation': 'Add a longer duration for subsequent lockouts',
    'accountLock.ladder.remove': 'Remove the {{position}} duration',
    'accountLock.ladder.untilUnlocked.user': 'Until account recovery or an administrator unlocks',
    'accountLock.ladder.untilUnlocked.agent': 'Until an administrator unlocks',
    'accountLock.ladder.end.label': 'After the final duration',
    'accountLock.ladder.end.repeat': 'Continue using the final duration',
    'accountLock.ladder.end.repeatHint': 'Apply the same duration to all subsequent lockouts.',
    'accountLock.ladder.end.untilUnlocked.user': 'Require account recovery or administrator intervention',
    'accountLock.ladder.end.untilUnlocked.agent': 'Require administrator intervention',
    'accountLock.ladder.end.untilUnlockedHint.user':
      'Keep the lockout in place until the user completes account recovery or an administrator unlocks the account.',
    'accountLock.ladder.end.untilUnlockedHint.agent':
      'Keep the lockout in place until an administrator unlocks the agent.',

    'accountLock.notify.label': 'Notify user by email',
    'accountLock.notify.hint':
      "Send an email to the user's primary email address when a sign-in method is locked. Users without a primary email address won't receive notifications.",

    'accountLock.decay.label': 'Reset lockout progression',
    'accountLock.decay.hint':
      'Restart the lockout sequence if no new lockout occurs within this period after the last one ends.',
    'accountLock.decay.ariaLabel': 'Period without a lockout before the sequence restarts',
    'accountLock.decay.never': 'Never reset',

    'accountLock.ceiling.label': 'Consecutive failure limit',
    'accountLock.ceiling.hint.user':
      'Require account recovery or administrator intervention when the number of consecutive failed sign-in attempts reaches this limit.',
    'accountLock.ceiling.hint.agent':
      'Require administrator intervention when the number of consecutive failed authentication attempts reaches this limit.',
    'accountLock.ceiling.ariaLabel': 'Consecutive failure limit',

    'accountLock.units.label': 'Unit',
    'accountLock.units.seconds': 'Seconds',
    'accountLock.units.minutes': 'Minutes',
    'accountLock.units.hours': 'Hours',
    'accountLock.units.days': 'Days',
    'accountLock.units.second': 'Second',
    'accountLock.units.minute': 'Minute',
    'accountLock.units.hour': 'Hour',
    'accountLock.units.day': 'Day',
    'accountLock.units.short.second': '{{count}} second',
    'accountLock.units.short.seconds': '{{count}} seconds',
    'accountLock.units.short.minute': '{{count}} minute',
    'accountLock.units.short.minutes': '{{count}} minutes',
    'accountLock.units.short.hour': '{{count}} hour',
    'accountLock.units.short.hours': '{{count}} hours',
    'accountLock.units.short.day': '{{count}} day',
    'accountLock.units.short.days': '{{count}} days',

    'accountLock.validation.threshold': 'Enter how many failed attempts should lock the account.',
    'accountLock.validation.ladder': 'Add at least one lock duration.',
    'accountLock.validation.ceiling':
      'Enter 0 for no limit, or at least the number of failed attempts that locks the account.',

    'accountLock.readOnlyHint':
      "These settings are read-only because they're managed declaratively. Change them in the declarative configuration.",
    'accountLock.unsavedChanges': 'You have unsaved changes',
    'accountLock.reset': 'Reset',
    'accountLock.save': 'Save changes',
    'accountLock.saving': 'Saving...',
    'accountLock.load.error': 'Failed to load the account security settings.',
    'accountLock.save.success': 'Account security settings updated.',
    'accountLock.save.error': 'Failed to update the account security settings.',
    'accountLock.errors.SCF-1003':
      'One or more account security settings are not valid. Check the values and try again.',

    // Lock and suspension flows
    'accountSuspension.title': 'Lock and suspension flows',
    'accountSuspension.description':
      'Manage the flows that suspend and unsuspend users, agents, and applications, and unlock users and agents. Suspension invalidates existing sessions and tokens and blocks access until it is lifted.',
    'accountSuspension.purpose.userSuspendFlow': 'Suspend users',
    'accountSuspension.purpose.userUnsuspendFlow': 'Unsuspend users',
    'accountSuspension.purpose.userUnlockFlow': 'Unlock users',
    'accountSuspension.purpose.agentSuspendFlow': 'Suspend agents',
    'accountSuspension.purpose.agentUnsuspendFlow': 'Unsuspend agents',
    'accountSuspension.purpose.agentUnlockFlow': 'Unlock agents',
    'accountSuspension.purpose.applicationSuspendFlow': 'Suspend applications',
    'accountSuspension.purpose.applicationUnsuspendFlow': 'Unsuspend applications',
    'accountSuspension.notConfigured': '{{operation}}: Not configured.',
    'accountSuspension.openFlows': 'Open flows',
    'accountSuspension.noFlowOperations':
      'Suspend, unsuspend, and unlock do not run through flows in this console, so there are no flows to manage.',
    'accountSuspension.load.error': 'Failed to load the account lifecycle flows.',
  },
} as const;

export default translations;
