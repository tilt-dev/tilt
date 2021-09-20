import { mount, ReactWrapper } from "enzyme"
import { SnackbarProvider } from "notistack"
import React from "react"
import { MemoryRouter } from "react-router"
import { HeaderGroup } from "react-table"
import {
  cleanupMockAnalyticsCalls,
  mockAnalyticsCalls,
} from "./analytics_test_helpers"
import { ApiButton } from "./ApiButton"
import Features, { FeaturesProvider, Flag } from "./feature"
import { GroupByLabelView, TILTFILE_LABEL, UNLABELED_LABEL } from "./labels"
import LogStore from "./LogStore"
import OverviewTable, {
  labeledResourcesToTableCells,
  OverviewGroup,
  OverviewGroupName,
  OverviewGroupSummary,
  OverviewTableProps,
  ResourceTableData,
  ResourceTableHeader,
  ResourceTableHeaderSortTriangle,
  ResourceTableHeadRow,
  ResourceTableRow,
  RowValues,
  Table,
  TableGroupedByLabels,
  TableNameColumn,
  TableNoMatchesFound,
  TableResourceResultCount,
  TableWithoutGroups,
} from "./OverviewTable"
import { ResourceGroupsInfoTip } from "./ResourceGroups"
import {
  DEFAULT_GROUP_STATE,
  GroupsState,
  ResourceGroupsContextProvider,
} from "./ResourceGroupsContext"
import {
  ResourceListOptions,
  ResourceListOptionsProvider,
} from "./ResourceListOptionsContext"
import {
  matchesResourceName,
  ResourceNameFilterTextField,
} from "./ResourceNameFilter"
import { TableGroupStatusSummary } from "./ResourceStatusSummary"
import {
  nResourceView,
  nResourceWithLabelsView,
  oneButton,
  TestDataView,
} from "./testdata"
import { RuntimeStatus, UpdateStatus } from "./types"

// Helpers
const tableViewWithSettings = ({
  view,
  labelsEnabled,
  resourceListOptions,
}: {
  view: TestDataView
  labelsEnabled?: boolean
  resourceListOptions?: ResourceListOptions
}) => {
  const features = new Features({ [Flag.Labels]: labelsEnabled ?? true })
  return (
    <MemoryRouter initialEntries={["/"]}>
      <SnackbarProvider>
        <FeaturesProvider value={features}>
          <ResourceGroupsContextProvider>
            <ResourceListOptionsProvider
              initialValuesForTesting={resourceListOptions}
            >
              <OverviewTable view={view} />
            </ResourceListOptionsProvider>
          </ResourceGroupsContextProvider>
        </FeaturesProvider>
      </SnackbarProvider>
    </MemoryRouter>
  )
}

const findTableHeaderByName = (
  wrapper: ReactWrapper<any>,
  columnName: string,
  sortable = true
): ReactWrapper<any, typeof ResourceTableHeader> => {
  const selector = sortable ? `Sort by ${columnName}` : columnName
  return wrapper.find(ResourceTableHeader).filter(`[title="${selector}"]`)
}

const findTableColumnByName = (
  wrapper: ReactWrapper<any>,
  columnName: string
): HeaderGroup<RowValues>[] => {
  const matchingColumns = wrapper
    .find(ResourceTableHeadRow)
    .reduce((columns: HeaderGroup<RowValues>[], row) => {
      const specificColumn = row
        .prop("headerGroup")
        .headers.filter((column) => column.Header === columnName)
      return [...columns, ...specificColumn]
    }, [])

  return matchingColumns
}
// End helpers

afterEach(() => {
  localStorage.clear()
})

it("shows buttons on the appropriate resources", () => {
  let view = nResourceView(3)
  // one resource with one button, one with multiple, and one with none
  view.uiButtons = [
    oneButton(0, view.uiResources[0].metadata?.name!),
    oneButton(1, view.uiResources[1].metadata?.name!),
    oneButton(2, view.uiResources[1].metadata?.name!),
  ]

  const root = mount(tableViewWithSettings({ view }))

  // buttons expected to be on each row, in order
  const expectedButtons = [["button1"], ["button2", "button3"], []]
  // first row is headers, so skip it
  const rows = root.find(ResourceTableRow).slice(1)
  const actualButtons = rows.map((row) =>
    row.find(ApiButton).map((e) => e.prop("uiButton").metadata?.name)
  )

  expect(actualButtons).toEqual(expectedButtons)
})

it("sorts by status", () => {
  let view = nResourceView(10)
  view.uiResources[3].status!.updateStatus = UpdateStatus.Error
  view.uiResources[7].status!.runtimeStatus = RuntimeStatus.Error
  const root = mount(tableViewWithSettings({ view }))

  const statusHeader = root
    .find(ResourceTableHeader)
    .filterWhere((r) => r.text() === "Status")
  statusHeader.simulate("click")
  root.update()

  const rows = root.find(ResourceTableRow).slice(1) // skip the header
  const actualResources = rows.map((row) =>
    row.find(ResourceTableData).at(3).text()
  )
  // 3 and 7 go first because they're failing, then it's alpha
  const expectedResources = [
    "_3",
    "_7",
    "(Tiltfile)",
    "_1",
    "_2",
    "_4",
    "_5",
    "_6",
    "_8",
    "_9",
  ]
  expect(expectedResources).toEqual(actualResources)
})

describe("resource name filter", () => {
  describe("when a filter is applied", () => {
    let view: TestDataView
    let wrapper: ReactWrapper<OverviewTableProps, typeof OverviewTable>

    beforeEach(() => {
      view = nResourceView(100)
      wrapper = mount(
        tableViewWithSettings({
          view,
          resourceListOptions: { resourceNameFilter: "1", alertsOnTop: false },
        })
      )
    })

    it("displays an accurate result count", () => {
      const resultCount = wrapper.find(TableResourceResultCount)
      expect(resultCount).toBeDefined()
      // Expect 19 results because test data names resources with their index number
      expect(resultCount.text()).toMatch(/19/)
    })

    it("displays only matching resources if there are matches", () => {
      const matchingResources =
        wrapper.find(TableWithoutGroups).prop("resources") || []

      // Expect 19 results because test data names resources with their index number
      expect(matchingResources.length).toBe(19)

      const displayedResourceNames = wrapper
        .find(TableNameColumn)
        .map((nameCell) => nameCell.text())
      const everyNameMatchesFilterTerm = displayedResourceNames.every((name) =>
        matchesResourceName(name, "1")
      )

      expect(everyNameMatchesFilterTerm).toBe(true)
    })

    it("displays a `no matches` message if there are no matches", () => {
      wrapper
        .find(`${ResourceNameFilterTextField} input`)
        .simulate("change", { target: { value: "eek no matches!" } })
      wrapper.update()

      expect(wrapper.find(TableNoMatchesFound)).toBeDefined()
    })
  })

  describe("when a filter is NOT applied", () => {
    it("displays all resources", () => {
      const wrapper = mount(
        tableViewWithSettings({
          view: nResourceView(10),
          resourceListOptions: { resourceNameFilter: "", alertsOnTop: false },
        })
      )

      const resourceListProp =
        wrapper.find(TableWithoutGroups).prop("resources") || []
      expect(resourceListProp.length).toBe(10)
      expect(wrapper.find(`tbody ${ResourceTableRow}`).length).toBe(10)
    })
  })
})

describe("when labels feature is enabled", () => {
  it("it displays tables grouped by labels if resources have labels", () => {
    const wrapper = mount(
      tableViewWithSettings({
        view: nResourceWithLabelsView(5),
        labelsEnabled: true,
      })
    )
    expect(wrapper.find(TableGroupedByLabels).length).toBeGreaterThan(0)
    expect(wrapper.find(TableWithoutGroups).length).toBe(0)
  })

  it("it displays a single table if no resources have labels", () => {
    const wrapper = mount(
      tableViewWithSettings({ view: nResourceView(5), labelsEnabled: true })
    )
    expect(wrapper.find(TableWithoutGroups).length).toBe(1)
    expect(wrapper.find(TableGroupedByLabels).length).toBe(0)
  })

  it("it displays the resource grouping tooltip if no resources have labels", () => {
    const wrapper = mount(
      tableViewWithSettings({ view: nResourceView(5), labelsEnabled: true })
    )
    expect(wrapper.find(ResourceGroupsInfoTip).length).toBe(1)
  })
})

describe("when labels feature is NOT enabled", () => {
  let wrapper: ReactWrapper<OverviewTableProps, typeof OverviewTable>

  beforeEach(() => {
    wrapper = mount(
      tableViewWithSettings({
        view: nResourceWithLabelsView(5),
        labelsEnabled: false,
      })
    )
  })

  it("it displays a single table", () => {
    expect(wrapper.find(TableWithoutGroups).length).toBe(1)
    expect(wrapper.find(TableGroupedByLabels).length).toBe(0)
  })

  it("it does not display the resource grouping tooltip", () => {
    expect(wrapper.find(ResourceGroupsInfoTip).length).toBe(0)
  })
})

describe("overview table without groups", () => {
  let view: TestDataView
  let wrapper: ReactWrapper<OverviewTableProps, typeof OverviewTable>

  beforeEach(() => {
    view = nResourceView(8)
    wrapper = mount(tableViewWithSettings({ view, labelsEnabled: true }))
  })

  describe("sorting", () => {
    it("table sorts when a column header is clicked", () => {
      findTableHeaderByName(wrapper, "Pod ID").simulate("click")
      const [podIdColumn] = findTableColumnByName(wrapper, "Pod ID")

      expect(podIdColumn.isSorted).toBe(true)
    })

    it("table column header displays ascending arrow when sorted ascending", () => {
      findTableHeaderByName(wrapper, "Pod ID").simulate("click")
      const arrowIcon = findTableHeaderByName(wrapper, "Pod ID").find(
        ResourceTableHeaderSortTriangle
      )

      expect(arrowIcon.hasClass("is-sorted-asc")).toBe(true)
    })

    it("table column header displays descending arrow when sorted descending", () => {
      findTableHeaderByName(wrapper, "Pod ID")
        .simulate("click")
        .simulate("click")
      const arrowIcon = findTableHeaderByName(wrapper, "Pod ID").find(
        ResourceTableHeaderSortTriangle
      )

      expect(arrowIcon.hasClass("is-sorted-desc")).toBe(true)
    })
  })
})

describe("overview table with groups", () => {
  let view: TestDataView
  let wrapper: ReactWrapper<OverviewTableProps, typeof OverviewTable>
  let resources: GroupByLabelView<RowValues>

  beforeEach(() => {
    view = nResourceWithLabelsView(8)
    wrapper = mount(tableViewWithSettings({ view, labelsEnabled: true }))
    resources = labeledResourcesToTableCells(
      view.uiResources,
      view.uiButtons,
      new LogStore()
    )

    mockAnalyticsCalls()
    localStorage.clear()
  })

  afterEach(() => {
    cleanupMockAnalyticsCalls()
    localStorage.clear()
  })

  describe("display", () => {
    it("does not show the resource groups tooltip", () => {
      expect(wrapper.find(ResourceGroupsInfoTip).length).toBe(0)
    })

    it("renders each label group in order", () => {
      const { labels: sortedLabels } = resources
      const groupNames = wrapper.find(OverviewGroupName)

      // Loop through the sorted labels (which includes every label
      // attached to a resource, but not unlabeled or tiltfile "labels")
      sortedLabels.forEach((label, idx) => {
        const groupName = groupNames.at(idx)
        expect(groupName.text()).toBe(label)
      })
    })

    // Note: the sample data generated in the test helper `nResourcesWithLabels`
    // always includes unlabeled resources
    it("renders a resource group for unlabeled resources and for Tiltfiles", () => {
      const groupNames = wrapper.find(OverviewGroupName)

      const unlabeledLabel = groupNames.filterWhere(
        (name) => name.text() == UNLABELED_LABEL
      )
      expect(unlabeledLabel.length).toBe(1)

      const tiltfileLabel = groupNames.filterWhere(
        (name) => name.text() == TILTFILE_LABEL
      )
      expect(tiltfileLabel.length).toBe(1)
    })

    it("renders a table for each resource group", () => {
      const tables = wrapper.find(Table)
      const totalLabelCount =
        resources.labels.length +
        (resources.tiltfile.length ? 1 : 0) +
        (resources.unlabeled.length ? 1 : 0)

      expect(tables.length).toBe(totalLabelCount)
    })

    it("renders the correct resources in each label group", () => {
      const { labelsToResources, unlabeled, tiltfile } = resources
      const resourceGroups = wrapper.find(OverviewGroup)

      const actualResourcesFromTable: { [key: string]: string[] } = {}
      const expectedResourcesFromLabelGroups: { [key: string]: string[] } = {}

      // Create a dictionary of labels to a list of resource names
      // based on the view
      Object.keys(labelsToResources).forEach((label) => {
        const resourceNames = labelsToResources[label].map((r) => r.name)
        expectedResourcesFromLabelGroups[label] = resourceNames
      })

      expectedResourcesFromLabelGroups[UNLABELED_LABEL] = unlabeled.map(
        (r) => r.name
      )
      expectedResourcesFromLabelGroups[TILTFILE_LABEL] = tiltfile.map(
        (r) => r.name
      )

      // Create a dictionary of labels to a list of resource names
      // based on what's rendered in each group table
      resourceGroups.forEach((group) => {
        // Find the label group name
        const groupName = group.find(OverviewGroupName).text()
        // Find the resource list displayed in the table
        const table = group.find(Table)
        const resourcesInTable = table
          .find(TableNameColumn)
          .map((resourceName) => resourceName.text())

        actualResourcesFromTable[groupName] = resourcesInTable
      })

      expect(actualResourcesFromTable).toEqual(expectedResourcesFromLabelGroups)
    })
  })

  describe("resource status summary", () => {
    it("renders summaries for each label group", () => {
      const summaries = wrapper.find(TableGroupStatusSummary)
      const totalLabelCount =
        resources.labels.length +
        (resources.tiltfile.length ? 1 : 0) +
        (resources.unlabeled.length ? 1 : 0)

      expect(summaries.length).toBe(totalLabelCount)
    })
  })

  describe("expand and collapse", () => {
    let groups: ReactWrapper<any, typeof OverviewGroup>

    // Helpers
    const getResourceGroups = () => wrapper.find(OverviewGroup)

    beforeEach(() => {
      groups = getResourceGroups()
    })

    it("displays as expanded or collapsed based on the ResourceGroupContext", () => {
      // Create an existing randomized group state from the labels
      const { labels } = resources
      const testData: GroupsState = [
        ...labels,
        UNLABELED_LABEL,
        TILTFILE_LABEL,
      ].reduce((groupsState: GroupsState, label) => {
        const randomLabelState = Math.random() > 0.5
        groupsState[label] = {
          ...DEFAULT_GROUP_STATE,
          expanded: randomLabelState,
        }

        return groupsState
      }, {})

      // Re-mount the component with the initial groups context values
      wrapper = mount(
        <ResourceGroupsContextProvider initialValuesForTesting={testData}>
          <TableGroupedByLabels
            resources={view.uiResources}
            buttons={view.uiButtons}
          />
        </ResourceGroupsContextProvider>
      )

      // Loop through each resource group and expect that its expanded state
      // matches with the hardcoded test data
      const actualExpandedState: GroupsState = {}
      wrapper.find(OverviewGroup).forEach((group) => {
        const groupName = group.find(OverviewGroupName).text()
        actualExpandedState[groupName] = { expanded: group.props().expanded }
      })

      expect(actualExpandedState).toEqual(testData)
    })

    it("is collapsed when an expanded resource group summary is clicked on", () => {
      const group = groups.first()
      expect(group.props().expanded).toBe(true)

      group.find(OverviewGroupSummary).simulate("click")

      // Manually refresh the test component tree
      wrapper.update()
      groups = getResourceGroups()

      const updatedGroup = groups.first()
      expect(updatedGroup.props().expanded).toBe(false)
    })

    it("is expanded when a collapsed resource group summary is clicked on", () => {
      // Because groups are expanded by default, click on it once to get it
      // into a collapsed state for testing
      const initialGroup = groups.first()
      expect(initialGroup.props().expanded).toBe(true)

      initialGroup.find(OverviewGroupSummary).simulate("click")

      // Manually refresh the test component tree
      wrapper.update()

      const group = getResourceGroups().first()
      expect(group.props().expanded).toBe(false)

      group.find(OverviewGroupSummary).simulate("click")

      // Manually refresh the test component tree
      wrapper.update()

      const updatedGroup = getResourceGroups().first()
      expect(updatedGroup.props().expanded).toBe(true)
    })
  })

  describe("sorting", () => {
    let firstTableNameColumn: ReactWrapper<any, typeof ResourceTableHeader>

    beforeEach(() => {
      // Find and click the "Resource Name" column on the first table group
      firstTableNameColumn = wrapper
        .find(ResourceTableHeader)
        .filter('[title="Sort by Resource Name"]')
        .first()
      firstTableNameColumn.simulate("click")
    })

    it("all resource group tables are sorted by the same column when one table is sorted", () => {
      const allTables = wrapper.find(Table)
      const allNameColumns = findTableColumnByName(wrapper, "Resource Name")

      // As a safeguard, make sure that the number of "Resource Name" columns
      // matches the number of tables being rendered
      expect(allNameColumns.length).toBe(allTables.length)
      // Expect that every "Resource Name" column is sorted
      expect(allNameColumns.every((column) => column.isSorted)).toBe(true)
    })

    it("tables sort by ascending values when clicked once", () => {
      // Use the fourth resource group table, since it has multiple resources in the test data generator
      const ascendingNames = wrapper.find(Table).at(3).find(TableNameColumn)
      const expectedNames = ["_1", "_3", "_5", "_7", "a_failed_build"]
      const actualNames = ascendingNames.map((name) => name.text())

      expect(actualNames).toStrictEqual(expectedNames)
    })

    it("tables sort by descending values when clicked twice", () => {
      firstTableNameColumn.simulate("click")

      // Use the fourth resource group table, since it has multiple resources in the test data generator
      const descendingNames = wrapper.find(Table).at(3).find(TableNameColumn)
      const expectedNames = ["a_failed_build", "_7", "_5", "_3", "_1"]
      const actualNames = descendingNames.map((name) => name.text())

      expect(actualNames).toStrictEqual(expectedNames)
    })

    it("tables un-sort when clicked thrice", () => {
      firstTableNameColumn.simulate("click")
      firstTableNameColumn.simulate("click")

      // Use the fourth resource group table, since it has multiple resources in the test data generator
      const unsortedNames = wrapper.find(Table).at(3).find(TableNameColumn)
      const expectedNames = ["_1", "_3", "_5", "_7", "a_failed_build"]
      const actualNames = unsortedNames.map((name) => name.text())

      expect(actualNames).toStrictEqual(expectedNames)
    })
  })

  describe("resource name filter", () => {
    it("does not display tables in groups when a resource filter is applied", () => {
      expect(wrapper.find(TableGroupedByLabels).length).toBeGreaterThan(0)

      wrapper
        .find(`${ResourceNameFilterTextField} input`)
        .simulate("change", { target: { value: "filtering!" } })
      wrapper.update()

      expect(wrapper.find(TableGroupedByLabels).length).toBe(0)
    })
  })
})
