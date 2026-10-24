// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, renderHook, screen, userEvent} from '@thunderid/test-utils';
import {useState} from 'react';
import {getI18n} from 'react-i18next';
import {describe, it, expect, vi} from 'vitest';
import useDataGridLocaleText from '../useDataGridLocaleText';

const {mockLogger} = vi.hoisted(() => ({
  mockLogger: {
    warn: vi.fn(),
    error: vi.fn(),
    info: vi.fn(),
    debug: vi.fn(),
  },
}));

vi.mock('@thunderid/logger', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/logger')>();

  return {
    ...actual,
    createLogger: () => mockLogger,
  };
});

// Unmock the hook for testing the actual implementation
vi.unmock('@/hooks/useDataGridLocaleText');

describe('useDataGridLocaleText', () => {
  it('should return localized text object for DataGrid', () => {
    const {result} = renderHook(() => useDataGridLocaleText());

    expect(result.current).toBeDefined();
    expect(typeof result.current).toBe('object');
  });

  it('should return root labels', () => {
    const {result} = renderHook(() => useDataGridLocaleText());

    expect(result.current.noRowsLabel).toBe('No rows');
    expect(result.current.noResultsOverlayLabel).toBe('No results found.');
    expect(result.current.noColumnsOverlayLabel).toBe('No columns');
    expect(result.current.noColumnsOverlayManageColumns).toBe('Manage columns');
  });

  it('should return toolbar density labels', () => {
    const {result} = renderHook(() => useDataGridLocaleText());

    expect(result.current.toolbarDensity).toBe('Density');
    expect(result.current.toolbarDensityLabel).toBe('Density');
    expect(result.current.toolbarDensityCompact).toBe('Compact');
    expect(result.current.toolbarDensityStandard).toBe('Standard');
    expect(result.current.toolbarDensityComfortable).toBe('Comfortable');
  });

  it('should return toolbar columns labels', () => {
    const {result} = renderHook(() => useDataGridLocaleText());

    expect(result.current.toolbarColumns).toBe('Columns');
    expect(result.current.toolbarColumnsLabel).toBe('Select columns');
  });

  it('should return toolbar filters labels', () => {
    const {result} = renderHook(() => useDataGridLocaleText());

    expect(result.current.toolbarFilters).toBe('Filters');
    expect(result.current.toolbarFiltersLabel).toBe('Show filters');
    expect(result.current.toolbarFiltersTooltipHide).toBe('Hide filters');
    expect(result.current.toolbarFiltersTooltipShow).toBe('Show filters');
  });

  it('should return toolbar filters tooltip active as a function', () => {
    const {result} = renderHook(() => useDataGridLocaleText());

    expect(typeof result.current.toolbarFiltersTooltipActive).toBe('function');
    expect(result.current.toolbarFiltersTooltipActive!(5)).toBe('5 active filters');
    expect(result.current.toolbarFiltersTooltipActive!(1)).toBe('1 active filter');
  });

  it('should return quick filter labels', () => {
    const {result} = renderHook(() => useDataGridLocaleText());

    expect(result.current.toolbarQuickFilterPlaceholder).toBe('Search…');
    expect(result.current.toolbarQuickFilterLabel).toBe('Search');
    expect(result.current.toolbarQuickFilterDeleteIconLabel).toBe('Clear');
  });

  it('should return export labels', () => {
    const {result} = renderHook(() => useDataGridLocaleText());

    expect(result.current.toolbarExport).toBe('Export');
    expect(result.current.toolbarExportLabel).toBe('Export');
    expect(result.current.toolbarExportCSV).toBe('Download as CSV');
    expect(result.current.toolbarExportPrint).toBe('Print');
  });

  it('should return columns management labels', () => {
    const {result} = renderHook(() => useDataGridLocaleText());

    expect(result.current.columnsManagementSearchTitle).toBe('Search');
    expect(result.current.columnsManagementNoColumns).toBe('No columns');
    expect(result.current.columnsManagementShowHideAllText).toBe('Show/hide all');
    expect(result.current.columnsManagementReset).toBe('Reset');
  });

  it('should return filter panel labels', () => {
    const {result} = renderHook(() => useDataGridLocaleText());

    expect(result.current.filterPanelAddFilter).toBe('Add filter');
    expect(result.current.filterPanelRemoveAll).toBe('Remove all');
    expect(result.current.filterPanelDeleteIconLabel).toBe('Delete');
    expect(result.current.filterPanelLogicOperator).toBe('Logic operator');
    expect(result.current.filterPanelOperator).toBe('Operator');
    expect(result.current.filterPanelOperatorAnd).toBe('And');
    expect(result.current.filterPanelOperatorOr).toBe('Or');
    expect(result.current.filterPanelColumns).toBe('Columns');
    expect(result.current.filterPanelInputLabel).toBe('Value');
    expect(result.current.filterPanelInputPlaceholder).toBe('Filter value');
  });

  it('should return filter operator labels', () => {
    const {result} = renderHook(() => useDataGridLocaleText());

    expect(result.current.filterOperatorContains).toBe('contains');
    expect(result.current.filterOperatorDoesNotContain).toBe('does not contain');
    expect(result.current.filterOperatorEquals).toBe('equals');
    expect(result.current.filterOperatorDoesNotEqual).toBe('does not equal');
    expect(result.current.filterOperatorStartsWith).toBe('starts with');
    expect(result.current.filterOperatorEndsWith).toBe('ends with');
    expect(result.current.filterOperatorIs).toBe('is');
    expect(result.current.filterOperatorNot).toBe('is not');
    expect(result.current.filterOperatorAfter).toBe('is after');
    expect(result.current.filterOperatorOnOrAfter).toBe('is on or after');
    expect(result.current.filterOperatorBefore).toBe('is before');
    expect(result.current.filterOperatorOnOrBefore).toBe('is on or before');
    expect(result.current.filterOperatorIsEmpty).toBe('is empty');
    expect(result.current.filterOperatorIsNotEmpty).toBe('is not empty');
    expect(result.current.filterOperatorIsAnyOf).toBe('is any of');
  });

  it('should return filter value labels', () => {
    const {result} = renderHook(() => useDataGridLocaleText());

    expect(result.current.filterValueAny).toBe('any');
    expect(result.current.filterValueTrue).toBe('true');
    expect(result.current.filterValueFalse).toBe('false');
  });

  it('should return column menu labels', () => {
    const {result} = renderHook(() => useDataGridLocaleText());

    expect(result.current.columnMenuLabel).toBe('Menu');
    expect(result.current.columnMenuShowColumns).toBe('Show columns');
    expect(result.current.columnMenuManageColumns).toBe('Manage columns');
    expect(result.current.columnMenuFilter).toBe('Filter');
    expect(result.current.columnMenuHideColumn).toBe('Hide column');
    expect(result.current.columnMenuUnsort).toBe('Unsort');
    expect(result.current.columnMenuSortAsc).toBe('Sort by ASC');
    expect(result.current.columnMenuSortDesc).toBe('Sort by DESC');
  });

  it('should return column header labels as functions', () => {
    const {result} = renderHook(() => useDataGridLocaleText());

    expect(typeof result.current.columnHeaderFiltersTooltipActive).toBe('function');
    expect(result.current.columnHeaderFiltersLabel).toBe('Show filters');
    expect(result.current.columnHeaderSortIconLabel).toBe('Sort');
    expect(result.current.columnHeaderFiltersTooltipActive!(3)).toBe('3 active filters');
    expect(result.current.columnHeaderFiltersTooltipActive!(1)).toBe('1 active filter');
  });

  it('should return footer labels as functions', () => {
    const {result} = renderHook(() => useDataGridLocaleText());

    expect(typeof result.current.footerRowSelected).toBe('function');
    expect(result.current.footerTotalRows).toBe('Total rows:');
    expect(typeof result.current.footerTotalVisibleRows).toBe('function');
    expect(result.current.footerRowSelected!(10)).toBe('10 rows selected');
    expect(result.current.footerRowSelected!(1)).toBe('1 row selected');
    expect(result.current.footerTotalVisibleRows!(50, 100)).toBe('50 of 100');
  });

  it('should return checkbox selection labels', () => {
    const {result} = renderHook(() => useDataGridLocaleText());

    expect(result.current.checkboxSelectionHeaderName).toBe('Checkbox selection');
    expect(result.current.checkboxSelectionSelectAllRows).toBe('Select all rows');
    expect(result.current.checkboxSelectionUnselectAllRows).toBe('Unselect all rows');
    expect(result.current.checkboxSelectionSelectRow).toBe('Select row');
    expect(result.current.checkboxSelectionUnselectRow).toBe('Unselect row');
  });

  it('should return boolean cell labels', () => {
    const {result} = renderHook(() => useDataGridLocaleText());

    expect(result.current.booleanCellTrueLabel).toBe('yes');
    expect(result.current.booleanCellFalseLabel).toBe('no');
  });

  it('should return actions cell label', () => {
    const {result} = renderHook(() => useDataGridLocaleText());

    expect(result.current.actionsCellMore).toBe('more');
  });

  it('should return column pinning labels', () => {
    const {result} = renderHook(() => useDataGridLocaleText());

    expect(result.current.pinToLeft).toBe('Pin to left');
    expect(result.current.pinToRight).toBe('Pin to right');
    expect(result.current.unpin).toBe('Unpin');
  });

  it('should return tree data labels', () => {
    const {result} = renderHook(() => useDataGridLocaleText());

    expect(result.current.treeDataGroupingHeaderName).toBe('Group');
    expect(result.current.treeDataExpand).toBe('see children');
    expect(result.current.treeDataCollapse).toBe('hide children');
  });

  it('should return grouping labels as functions', () => {
    const {result} = renderHook(() => useDataGridLocaleText());

    expect(result.current.groupingColumnHeaderName).toBe('Group');
    expect(typeof result.current.groupColumn).toBe('function');
    expect(typeof result.current.unGroupColumn).toBe('function');
    expect(result.current.groupColumn!('status')).toBe('Group by status');
    expect(result.current.unGroupColumn!('status')).toBe('Stop grouping by status');
  });

  it('should return master/detail labels', () => {
    const {result} = renderHook(() => useDataGridLocaleText());

    expect(result.current.detailPanelToggle).toBe('Detail panel toggle');
    expect(result.current.expandDetailPanel).toBe('Expand');
    expect(result.current.collapseDetailPanel).toBe('Collapse');
  });

  it('should return pagination labels', () => {
    const {result} = renderHook(() => useDataGridLocaleText());

    expect(result.current.paginationRowsPerPage).toBe('Rows per page:');
    expect(typeof result.current.paginationDisplayedRows).toBe('function');
    expect(result.current.paginationDisplayedRows!({from: 1, to: 10, count: 100, estimated: undefined})).toBe(
      '1–10 of 100',
    );
    expect(result.current.paginationDisplayedRows!({from: 1, to: 10, count: -1, estimated: undefined})).toBe(
      '1–10 of more than 10',
    );
  });

  it('should return row reordering label', () => {
    const {result} = renderHook(() => useDataGridLocaleText());

    expect(result.current.rowReorderingHeaderName).toBe('Row reordering');
  });

  it('should return aggregation labels', () => {
    const {result} = renderHook(() => useDataGridLocaleText());

    expect(result.current.aggregationMenuItemHeader).toBe('Aggregation');
    expect(result.current.aggregationFunctionLabelSum).toBe('sum');
    expect(result.current.aggregationFunctionLabelAvg).toBe('avg');
    expect(result.current.aggregationFunctionLabelMin).toBe('min');
    expect(result.current.aggregationFunctionLabelMax).toBe('max');
    expect(result.current.aggregationFunctionLabelSize).toBe('size');
  });

  it('should memoize the result', () => {
    const {result, rerender} = renderHook(() => useDataGridLocaleText());
    const firstResult = result.current;

    rerender();

    // The result should be the same object reference due to memoization
    expect(result.current).toBe(firstResult);
  });

  it('should return stable result across multiple re-renders via state updates', async () => {
    // Use a real component with state updates to trigger genuine re-renders
    // that exercise the React Compiler's memoization skip paths
    function TestComponent() {
      const [count, setCount] = useState(0);
      const localeText = useDataGridLocaleText();

      return (
        <div>
          <span data-testid="count">{count}</span>
          <span data-testid="label">{localeText.noRowsLabel}</span>
          <button type="button" onClick={() => setCount((c) => c + 1)}>
            increment
          </button>
        </div>
      );
    }

    const user = userEvent.setup();
    render(<TestComponent />);

    expect(screen.getByTestId('label')).toHaveTextContent('No rows');

    // Trigger re-renders via state updates to exercise compiler memoization
    await user.click(screen.getByText('increment'));
    expect(screen.getByTestId('count')).toHaveTextContent('1');
    expect(screen.getByTestId('label')).toHaveTextContent('No rows');

    await user.click(screen.getByText('increment'));
    expect(screen.getByTestId('count')).toHaveTextContent('2');
    expect(screen.getByTestId('label')).toHaveTextContent('No rows');
  });

  it('should return undefined for function keys when resource bundle has non-function values', () => {
    const i18n = getI18n();
    const originalGetResourceBundle = i18n.getResourceBundle.bind(i18n);

    vi.spyOn(i18n, 'getResourceBundle').mockImplementation((lng: string, ns: string) => {
      const bundle = originalGetResourceBundle(lng, ns) as Record<string, unknown>;
      return {
        ...bundle,
        'dataTable.toolbarFiltersTooltipActive': 'not-a-function',
      };
    });

    const {result} = renderHook(() => useDataGridLocaleText());

    expect(result.current.toolbarFiltersTooltipActive).toBeUndefined();
    expect(mockLogger.warn).toHaveBeenCalledWith(expect.stringContaining('dataTable.toolbarFiltersTooltipActive'));

    vi.restoreAllMocks();
  });

  it('should return undefined for function keys when key is missing from resource bundle', () => {
    const i18n = getI18n();

    vi.spyOn(i18n, 'getResourceBundle').mockImplementation(() => ({}));

    const {result} = renderHook(() => useDataGridLocaleText());

    // All function-type keys should be undefined
    expect(result.current.toolbarFiltersTooltipActive).toBeUndefined();
    expect(result.current.columnHeaderFiltersTooltipActive).toBeUndefined();
    expect(result.current.footerRowSelected).toBeUndefined();
    expect(result.current.footerTotalVisibleRows).toBeUndefined();
    expect(result.current.groupColumn).toBeUndefined();
    expect(result.current.unGroupColumn).toBeUndefined();
    expect(result.current.paginationDisplayedRows).toBeUndefined();

    vi.restoreAllMocks();
  });

  it('should handle getResourceBundle returning undefined', () => {
    const i18n = getI18n();

    vi.spyOn(i18n, 'getResourceBundle').mockImplementation(() => undefined as unknown as Record<string, unknown>);

    const {result} = renderHook(() => useDataGridLocaleText());

    // All function-type keys should be undefined when bundle is null/undefined
    expect(result.current.toolbarFiltersTooltipActive).toBeUndefined();
    expect(result.current.columnHeaderFiltersTooltipActive).toBeUndefined();
    expect(result.current.footerRowSelected).toBeUndefined();
    expect(result.current.footerTotalVisibleRows).toBeUndefined();
    expect(result.current.groupColumn).toBeUndefined();
    expect(result.current.unGroupColumn).toBeUndefined();
    expect(result.current.paginationDisplayedRows).toBeUndefined();

    // String keys should still work (they use t() not the bundle)
    expect(result.current.noRowsLabel).toBeDefined();

    vi.restoreAllMocks();
  });

  it('should handle getResourceBundle returning null', () => {
    const i18n = getI18n();

    vi.spyOn(i18n, 'getResourceBundle').mockImplementation(() => null as unknown as Record<string, unknown>);

    const {result} = renderHook(() => useDataGridLocaleText());

    // All function-type keys should be undefined when bundle is null
    expect(result.current.toolbarFiltersTooltipActive).toBeUndefined();
    expect(result.current.columnHeaderFiltersTooltipActive).toBeUndefined();
    expect(result.current.footerRowSelected).toBeUndefined();
    expect(result.current.footerTotalVisibleRows).toBeUndefined();
    expect(result.current.groupColumn).toBeUndefined();
    expect(result.current.unGroupColumn).toBeUndefined();
    expect(result.current.paginationDisplayedRows).toBeUndefined();

    vi.restoreAllMocks();
  });

  it('should handle non-function values for all function keys with DEV warnings', () => {
    const i18n = getI18n();

    vi.spyOn(i18n, 'getResourceBundle').mockReturnValue({
      'dataTable.toolbarFiltersTooltipActive': 'not-a-function',
      'dataTable.columnHeaderFiltersTooltipActive': 42,
      'dataTable.footerRowSelected': true,
      'dataTable.footerTotalVisibleRows': [],
      'dataTable.groupColumn': {},
      'dataTable.unGroupColumn': 'string-value',
      'dataTable.paginationDisplayedRows': 0,
    } as unknown as Record<string, unknown>);

    const {result} = renderHook(() => useDataGridLocaleText());

    // All function-type keys should be undefined since they are not functions
    expect(result.current.toolbarFiltersTooltipActive).toBeUndefined();
    expect(result.current.columnHeaderFiltersTooltipActive).toBeUndefined();
    expect(result.current.footerRowSelected).toBeUndefined();
    expect(result.current.footerTotalVisibleRows).toBeUndefined();
    expect(result.current.groupColumn).toBeUndefined();
    expect(result.current.unGroupColumn).toBeUndefined();
    expect(result.current.paginationDisplayedRows).toBeUndefined();

    // Should have logged warnings for non-function, non-undefined values (DEV is true in test env)
    expect(mockLogger.warn).toHaveBeenCalledWith(expect.stringContaining('dataTable.toolbarFiltersTooltipActive'));
    expect(mockLogger.warn).toHaveBeenCalledWith(expect.stringContaining('dataTable.columnHeaderFiltersTooltipActive'));
    expect(mockLogger.warn).toHaveBeenCalledWith(expect.stringContaining('dataTable.footerRowSelected'));
    expect(mockLogger.warn).toHaveBeenCalledWith(expect.stringContaining('dataTable.footerTotalVisibleRows'));
    expect(mockLogger.warn).toHaveBeenCalledWith(expect.stringContaining('dataTable.groupColumn'));
    expect(mockLogger.warn).toHaveBeenCalledWith(expect.stringContaining('dataTable.unGroupColumn'));
    expect(mockLogger.warn).toHaveBeenCalledWith(expect.stringContaining('dataTable.paginationDisplayedRows'));
  });
});
