// From https://github.com/DefinitelyTyped/DefinitelyTyped/tree/master/types/react-table

import {
  UsePaginationInstanceProps,
  UsePaginationOptions,
  UsePaginationState,
  UseSortByColumnOptions,
  UseSortByColumnProps,
  UseSortByHooks,
  UseSortByInstanceProps,
  UseSortByOptions,
  UseSortByState,
} from "react-table"

declare module "react-table" {
  // take this file as-is, or comment out the sections that don't apply to your plugin configuration

  export interface TableOptions<D extends Record<string, unknown>>
    extends UseSortByOptions<D>,
      UsePaginationOptions<D> {
    // UseExpandedOptions<D>,
    // UseFiltersOptions<D>,
    // UseGlobalFiltersOptions<D>,
    // UseGroupByOptions<D>,
    // UseResizeColumnsOptions<D>,
    // UseRowSelectOptions<D>,
    // UseRowStateOptions<D>,
    // note that having Record here allows you to add anything to the options, this matches the spirit of the
    // underlying js library, but might be cleaner if it's replaced by a more specific type that matches your
    // feature set, this is a safe default.
    // Record<string, any>
  }

  export interface Hooks<
    D extends Record<string, unknown> = Record<string, unknown>
  > extends UseSortByHooks<D> {
    // UseExpandedHooks<D>,
    // UseGroupByHooks<D>,
    // UseRowSelectHooks<D>,
    // UseSortByHooks<D>
  }

  export interface TableInstance<
    D extends Record<string, unknown> = Record<string, unknown>
  > extends UseSortByInstanceProps<D>,
      UsePaginationInstanceProps<D> {
    // UseColumnOrderInstanceProps<D>,
    // UseExpandedInstanceProps<D>,
    // UseFiltersInstanceProps<D>,
    // UseGlobalFiltersInstanceProps<D>,
    // UseGroupByInstanceProps<D>,
    // UseRowSelectInstanceProps<D>,
    // UseRowStateInstanceProps<D>,
    // UseSortByInstanceProps<D>
  }

  export interface TableState<
    D extends Record<string, unknown> = Record<string, unknown>
  > extends UseSortByState<D>,
      UsePaginationState<D> {
    // UseColumnOrderState<D>,
    // UseExpandedState<D>,
    // UseFiltersState<D>,
    // UseGlobalFiltersState<D>,
    // UseGroupByState<D>,
    // UseResizeColumnsState<D>,
    // UseRowSelectState<D>,
    // UseRowStateState<D>,
  }

  export interface ColumnInterface<
    D extends Record<string, unknown> = Record<string, unknown>
  > extends UseSortByColumnOptions<D> {
    // UseFiltersColumnOptions<D>,
    // UseGlobalFiltersColumnOptions<D>,
    // UseGroupByColumnOptions<D>,
    // UseResizeColumnsColumnOptions<D>,
  }

  export interface ColumnInstance<
    D extends Record<string, unknown> = Record<string, unknown>
  > extends UseSortByColumnProps<D> {
    // UseFiltersColumnProps<D>,
    // UseGroupByColumnProps<D>,
    // UseResizeColumnsColumnProps<D>,
  }

  // export interface Cell<D extends Record<string, unknown> = Record<string, unknown>, V = any>
  //   extends UseGroupByCellProps<D>,
  //     UseRowStateCellProps<D> {}

  // export interface Row<D extends Record<string, unknown> = Record<string, unknown>>
  //   extends UseExpandedRowProps<D>,
  //     UseGroupByRowProps<D>,
  //     UseRowSelectRowProps<D>,
  //     UseRowStateRowProps<D> {}
}
