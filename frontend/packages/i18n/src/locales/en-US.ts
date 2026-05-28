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
    'welcome.sections.start': 'Start',
    'welcome.sections.recent': 'Recent',
    'welcome.sections.tryoutProduct': 'Tryout',
    'welcome.tryoutProduct.b2c': 'Securing Consumer App (B2C)',
    'welcome.tryoutProduct.b2cDesc': 'Tryout user journeys for a consumer-facing app',
    'welcome.tryoutProduct.aiAgents': 'Securing AI Agents',
    'welcome.tryoutProduct.aiAgentsDesc': 'Tryout identity patterns for AI agents and tools',
    'welcome.tryoutProduct.mcp': 'Securing MCP',
    'welcome.tryoutProduct.mcpDesc': 'Try authorizing MCP clients to your MCP server',
    'welcome.start.newProject': 'New / Continue',
    'welcome.start.newProjectDesc': 'Configure a new project or edit existing',
    'welcome.start.openImport': 'Open',
    'welcome.start.openImportDesc': 'Import an existing {{productName}} configuration',
    'welcome.start.startSamples': 'Start with samples',
    'welcome.start.connectTo': 'Connect to \u2026',
    'welcome.noRecentItems': 'No recent projects found',
    'welcome.hero.titlePrefix': 'Welcome to',
    'welcome.hero.subtitle': 'Design and configure your Identity & Access Management project',
    'welcome.walkthrough.getStartedDesigner': 'Get started',
    'welcome.walkthrough.getStartedDesignerDesc': 'Learn how to design and customize your identity experience',
    'welcome.walkthrough.learnFundamentals': 'Learn the Fundamentals',
    'welcome.walkthrough.learnFundamentalsDesc': 'Understand core concepts and architecture',
    'welcome.createProject.breadcrumb': 'New',
    'welcome.createProject.title': "Let's Create Your Identity Project",
    'welcome.createProject.subtitle':
      'This wizard will guide you through minimal configuration for your project and generate the necessary configs to run {{productName}}.',
    'welcome.createProject.cards.configure.title': 'Configure Project',
    'welcome.createProject.cards.configure.description':
      "Set up your project's authentication flows, choose sign-in methods, and customize the user experience.",
    'welcome.createProject.cards.verify.title': 'Verify',
    'welcome.createProject.cards.verify.description':
      'Test your project configuration to ensure everything works as expected.',
    'welcome.createProject.cards.runServer.title': 'Run Server',
    'welcome.createProject.cards.runServer.description':
      '{{productName}} will run in immutable mode with the attached configurations.',
    'welcome.createProject.actions.getStarted': 'Get Started',
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
    'pages.home': 'Home',
    'pages.users': 'Users',
    'pages.userTypes': 'User Types',
    'pages.agentTypes': 'Agent Types',
    'pages.organizationUnits': 'Organization Units',
    'pages.groups': 'Groups',
    'pages.roles': 'Roles',
    'pages.integrations': 'Integrations',
    'pages.applications': 'Applications',
    'pages.dashboard': 'Dashboard',
    'pages.flows': 'Flows',
    'pages.design': 'Design',
    'pages.translations': 'Translations',
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
    'createWizard.name.fieldLabel': 'Agent name',
    'createWizard.name.placeholder': 'e.g. Billing Service',
    'createWizard.name.suggestions.label': 'Need inspiration? Pick one:',
    'createWizard.agentDetails.title': 'Agent attributes',
    'createWizard.agentDetails.subtitle': 'Provide values for the attributes defined by the agent schema.',
    'createWizard.owner.title': 'Owner',
    'createWizard.owner.subtitle': 'Choose the user that owns this agent.',
    'createWizard.owner.userLabel': 'Owner',
    'createWizard.owner.userPlaceholder': 'Select a user',

    // Client secret (creation)
    'clientSecret.saveTitle': 'Save your client secret',
    'clientSecret.saveSubtitle': "Copy your client secret and store it somewhere safe. It won't be shown again.",
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
    'edit.page.tabs.attributes': 'Attributes',
    'edit.page.unsavedChanges': 'You have unsaved changes',
    'edit.page.reset': 'Discard',
    'edit.page.save': 'Save',
    'edit.page.saving': 'Saving…',
    'update.success': 'Agent updated successfully.',
    'update.error': 'Failed to update agent. Please try again.',

    // Edit page — Attributes tab
    'edit.attributes.title': 'Attributes',
    'edit.attributes.description': 'View and manage agent attribute values.',
    'edit.attributes.empty': 'No attributes available.',
    'edit.attributes.noEditable': 'No editable attributes available.',

    // Edit page — General tab
    'edit.general.sections.quickCopy.title': 'Quick Copy',
    'edit.general.sections.quickCopy.description': 'Copy agent identifiers for use in your code.',
    'edit.general.labels.agentId': 'Agent ID',
    'edit.general.labels.ownerId': 'Owner ID',
    'edit.general.agentId.hint': 'Unique identifier for this agent',
    'edit.general.clientId.hint': 'OAuth2 client identifier used by this agent to obtain tokens',
    'edit.general.owner.hint': 'Identifier of the user that owns this agent',
    'edit.general.dangerZone.deleteAgent.title': 'Delete Agent',
    'edit.general.dangerZone.deleteAgent.description':
      'Permanently delete this agent and all associated data. This action cannot be undone.',
    'edit.general.dangerZone.deleteAgent.button': 'Delete Agent',

    // Edit page — Flows tab
    'edit.flows.allowedUserTypes.title': 'Allowed User Types',
    'edit.flows.allowedUserTypes.description':
      'Restrict which user types can authenticate or register through this agent.',
    'edit.flows.allowedUserTypes.label': 'User Types',
    'edit.flows.allowedUserTypes.placeholder': 'Select or add user types',
    'edit.flows.allowedUserTypes.hint': 'Leave empty to allow any user type.',

    // Edit page — Advanced tab
    'edit.advanced.redirectUris.title': 'Redirect URIs',
    'edit.advanced.redirectUris.description': 'Allowed redirect destinations for the authorization code grant.',
    'edit.advanced.redirectUris.empty': 'No redirect URIs configured.',
    'edit.advanced.redirectUris.addUri': 'Add Redirect URI',
    'edit.advanced.redirectUris.error.empty': 'URI cannot be empty',
    'edit.advanced.redirectUris.error.invalid': 'Enter a valid URL',
    'edit.advanced.redirectUris.required':
      'At least one valid redirect URI is required for the authorization code grant.',

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
    'createWizard.basicInfo.title': "Let's give a name to your role",
    'createWizard.basicInfo.suggestions.label': 'In a hurry? Pick a random name:',
    'createWizard.organizationUnit.title': 'Select an organization unit',
    'createWizard.organizationUnit.subtitle': 'Choose the organization unit this role will belong to.',

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
    'edit.permissions.description':
      'Select the permissions this role grants. Changes are saved when you click Save Changes.',
    'edit.permissions.resourcesLabel': 'Resources',
    'edit.permissions.actionsLabel': 'Actions',
    'edit.permissions.noPermissions': 'No permissions defined for this resource server.',
    'edit.permissions.noResourceServers': 'No resource servers found. Create a resource server first.',
    'edit.permissions.loadError': 'Failed to load permissions for this resource server.',
    'edit.permissions.loadResourceServersError': 'Failed to load resource servers.',
    'edit.permissions.selectedCount': '{{count}} selected',

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
  // Integrations namespace - Integrations feature translations
  // ============================================================================
  integrations: {
    title: 'Integrations',
    subtitle: 'Manage your integrations and connections',
    addIntegration: 'Add Integration',
    editIntegration: 'Edit Integration',
    deleteIntegration: 'Delete Integration',
    integrationDetails: 'Integration Details',
    provider: 'Provider',
    apiKey: 'API Key',
    endpoint: 'Endpoint',
    status: 'Status',
    connected: 'Connected',
    disconnected: 'Disconnected',
    testConnection: 'Test Connection',
    noIntegrations: 'No integrations found',
    comingSoon: 'Coming Soon',
    comingSoonDescription: 'Integrations management functionality will be available soon.',
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
    'onboarding.configure.details.title': 'Configuration',
    'onboarding.configure.details.description': 'Configure where your application is hosted and callback settings',
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
    'export.table.integrations': 'Integrations',
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
    'clientSecret.copied': 'Copied to clipboard',
    'clientSecret.copySecret': 'Copy Secret',
    'clientSecret.securityReminder.title': 'Security Reminder',
    'clientSecret.securityReminder.description':
      'Your client secret is a confidential key used to authenticate your application. It should be treated with the same level of security as a password. Never expose it in browser console, version control, or logs.',
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
    'edit.general.allowedUserTypes.placeholder': 'Select user types',
    'edit.general.allowedUserTypes.hint': 'Users of these types can authenticate with this application',
    'edit.general.applicationUrl.hint': 'The homepage URL of your application',
    'edit.general.sections.dangerZone.title': 'Danger Zone',
    'edit.general.sections.dangerZone.description': 'Actions in this section are irreversible. Proceed with caution.',
    'edit.general.sections.dangerZone.regenerateSecret.title': 'Regenerate Client Secret',
    'edit.general.sections.dangerZone.regenerateSecret.description':
      'Regenerating the client secret will immediately invalidate the current client secret and cannot be undone.',
    'edit.general.sections.dangerZone.regenerateSecret.button': 'Regenerate Client Secret',
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
    'edit.token.user_attributes_card.title': 'User Attributes',
    'edit.token.user_attributes_card.description':
      'Configure the user attributes to include in your tokens & user info response',
    'edit.token.tabs.access_token': 'Access Token',
    'edit.token.tabs.id_token': 'ID Token',
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
    'edit.token.scope_mapper.title': 'User Attribute Mapping',
    'edit.token.scope_mapper.hint':
      'Select a scope to configure which user attributes are exposed when it is requested.',
    'edit.token.scope_mapper.no_scopes': 'Add at least one scope above to start mapping attributes.',
    'edit.token.scope_mapper.mapped_label': 'Mapped Attributes',
    'edit.token.scope_mapper.available_label': 'Available Attributes',
    'edit.token.scope_mapper.no_mapped': 'No attributes mapped yet — click an attribute below to add it',
    'edit.token.scope_mapper.all_mapped': 'All available attributes are already mapped to this scope',
    'edit.token.scope_mapper.loading': 'Loading available attributes...',

    // Advanced section
    'edit.advanced.labels.oauth2Config': 'OAuth2 Configuration',
    'edit.advanced.labels.redirectUris': 'Redirect URIs',
    'edit.advanced.labels.grantTypes': 'Grant Types',
    'edit.advanced.labels.responseTypes': 'Response Types',
    'edit.advanced.labels.publicClient': 'Public Client',
    'edit.advanced.labels.pkceRequired': 'PKCE Required',
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
    'errors.APP-5001': 'An unexpected error occurred while processing the request.',
    'errors.APP-5002': 'An error occurred while performing the certificate operation.',
  },

  // ============================================================================
  // Import / Export - Project import-export feature translations
  // ============================================================================
  importExport: {
    'export.page.title': 'Export Configuration',
    'export.page.loading': 'Loading export configuration...',
    'export.page.loadError': 'Failed to load export configuration: {{message}}',

    'upload.breadcrumb.openProject': 'Open Project',
    'upload.title': 'Open Project',
    'upload.subtitle': 'Upload your {{configFileName}} configuration file or provide a URL to import',
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
    'configureExport.nextSteps.resourcesAvailable': 'All applications, flows, and identity providers will be available',
    'configureExport.nextSteps.testFlows': 'You can test your authentication flows immediately',
    'configureExport.actions.showLess': 'Show less',
    'configureExport.actions.more': '+ {{count}} more',
    'configureExport.labels.themes': 'Themes',
    'configureExport.labels.users': 'Users',
    'configureExport.labels.organizationUnits': 'Organization Units',
    'configureExport.labels.notificationSenders': 'Notification Senders',
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
    'configureExport.fallback.unnamedSender': 'Unnamed Sender',
    'configureExport.fallback.unnamedSchema': 'Unnamed Schema',
    'configureExport.fallback.unnamedTranslation': 'Unnamed Translation',
    'configureExport.fallback.unnamedLayout': 'Unnamed Layout',
    'configureExport.fallback.unnamedResourceServer': 'Unnamed Resource Server',
    'configureExport.fallback.unnamedRole': 'Unnamed Role',
    'configureExport.fallback.unnamedGroup': 'Unnamed Group',
    'configureExport.labels.agents': 'Agents',
    'configureExport.fallback.unnamedAgent': 'Unnamed Agent',
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
    'summary.labels.identityProviders': 'Identity Providers',
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

    // Resource logo dialog
    'resource_logo_dialog.title': 'Choose a Logo',
    'resource_logo_dialog.divider.or': 'Or',
    'resource_logo_dialog.url_section.label': 'Use a custom image URL',
    'resource_logo_dialog.url_section.placeholder': 'https://example.com/logo.png',
    'resource_logo_dialog.url_section.helper_text': 'Enter a direct URL to a custom logo image',
    'resource_logo_dialog.actions.cancel': 'Cancel',
    'resource_logo_dialog.actions.select': 'Select',
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
    'listing.columns.version': 'Version',
    'listing.columns.updatedAt': 'Last Updated',
    'listing.columns.actions': 'Actions',
    'listing.error.title': 'Failed to load flows',
    'listing.error.unknown': 'An unknown error occurred',
    'delete.title': 'Delete Flow',
    'delete.message': 'Are you sure you want to delete this flow? This action cannot be undone.',
    'delete.disclaimer': 'Warning: All associated configurations will be permanently removed.',
    'delete.error': 'Failed to delete flow. Please try again.',

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
    'core.executions.provisioning.assignGroup.placeholder': 'Group ID to assign',
    'core.executions.provisioning.assignRole.label': 'Assign Role',
    'core.executions.provisioning.assignRole.placeholder': 'Role ID to assign',
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

    // Elements - rich text
    'core.elements.richText.placeholder': 'Enter text here...',
    'core.elements.richText.resolvedI18nValue': 'Resolved i18n value',
    'core.elements.richText.linkEditor.urlTypeLabel': 'URL Type',
    'core.elements.richText.linkEditor.placeholder': 'Type or paste a link',
    'core.elements.richText.linkEditor.textPlaceholder': 'Text',
    'core.elements.richText.linkEditor.apply': 'Apply',
    'core.elements.richText.linkEditor.editLink': 'Edit Link',
    'core.elements.richText.linkEditor.viewLink': 'Link',

    // Elements - text element
    'core.elements.text.align.label': 'Align',
    'core.elements.text.align.options.left': 'Left',
    'core.elements.text.align.options.center': 'Center',
    'core.elements.text.align.options.right': 'Right',
    'core.elements.text.align.options.justify': 'Justify',
    'core.elements.text.align.options.inherit': 'Inherit',

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
    'core.headerPanel.edgeStyles.bezier': 'Bezier',
    'core.headerPanel.edgeStyles.smoothStep': 'Smooth Step',
    'core.headerPanel.edgeStyles.step': 'Step',

    // Resource panel
    'core.resourcePanel.title': 'Resources',
    'core.resourcePanel.showResources': 'Show Resources',
    'core.resourcePanel.hideResources': 'Hide Resources',
    'core.resourcePanel.starterTemplates.title': 'Starter Templates',
    'core.resourcePanel.starterTemplates.description':
      'Choose one of these templates to start building registration experience',
    'core.resourcePanel.widgets.title': 'Widgets',
    'core.resourcePanel.widgets.description': 'Use these widgets to build up the flow using pre-created flow blocks',
    'core.resourcePanel.steps.title': 'Steps',
    'core.resourcePanel.steps.description': 'Use these as steps in your flow',
    'core.resourcePanel.components.title': 'Components',
    'core.resourcePanel.components.description': 'Use these components to build up your views',
    'core.resourcePanel.executors.title': 'Executors',
    'core.resourcePanel.executors.description': 'Add authentication executors to your flow',

    // View step
    'core.steps.view.addComponent': 'Add Component',
    'core.steps.view.configure': 'Configure',
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

    // Social Login card
    'next_steps.social_login.title': 'Social Integrations',
    'next_steps.social_login.description':
      'Let users sign in with their favourite identity providers — Google, GitHub, and more.',

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
    designConfigure: 'Design / Configure {{productName}} Project',
    designComponents: '(with design components)',
    commandProduction: './start.sh project-foo.yml --env production.env',
    commandStart: './start.sh',
    adminApp: 'Admin App',
    loginApp: 'Login App',
  },
} as const;

export default translations;
