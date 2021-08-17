import { mount, ReactWrapper } from "enzyme"
import React from "react"
import { MemoryRouter } from "react-router"
import {
  cleanupMockAnalyticsCalls,
  mockAnalyticsCalls,
} from "./analytics_test_helpers"
import { GroupByLabelView, TILTFILE_LABEL, UNLABELED_LABEL } from "./labels"
import LogStore from "./LogStore"
import { CustomActionButton } from "./OverviewButton"
import OverviewTable, {
  OverviewGroup,
  OverviewGroupName,
  OverviewGroupSummary,
  OverviewTableProps,
  resourcesToTableCells,
  ResourceTableRow,
  RowValues,
  Table,
  TableGroupedByLabels,
  TableNameColumn,
} from "./OverviewTable"
import {
  DEFAULT_GROUP_STATE,
  GroupsState,
  ResourceGroupsContextProvider,
} from "./ResourceGroupsContext"
import { nResourceView, nResourceWithLabelsView, oneButton } from "./testdata"

it("shows buttons on the appropriate resources", () => {
  let view = nResourceView(3)
  // one resource with one button, one with multiple, and one with none
  view.uiButtons = [
    oneButton(0, view.uiResources[0].metadata?.name!),
    oneButton(1, view.uiResources[1].metadata?.name!),
    oneButton(2, view.uiResources[1].metadata?.name!),
  ]

  const root = mount(
    <MemoryRouter initialEntries={["/"]}>
      <OverviewTable view={view} />
    </MemoryRouter>
  )

  // buttons expected to be on each row, in order
  const expectedButtons = [["button1"], ["button2", "button3"], []]
  // first row is headers, so skip it
  const rows = root.find(ResourceTableRow).slice(1)
  const actualButtons = rows.map((row) =>
    row.find(CustomActionButton).map((e) => e.prop("button").metadata.name)
  )

  expect(actualButtons).toEqual(expectedButtons)
})

describe("overview table with groups", () => {
  let view: Proto.webviewView
  let wrapper: ReactWrapper<OverviewTableProps, typeof TableGroupedByLabels>
  let resources: GroupByLabelView<RowValues>

  beforeEach(() => {
    view = nResourceWithLabelsView(5)
    wrapper = mount(
      <ResourceGroupsContextProvider>
        <TableGroupedByLabels view={view} />
      </ResourceGroupsContextProvider>
    )
    resources = resourcesToTableCells(
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

  // TODO: When resource groups are live in table view, add tests for:
  //       If no resources have labels, it renders a single table view
  //       If labels are not enabled, it renders a single table view
  //       If there are labels and feature is enabled, it renders table with groups

  describe("display", () => {
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
          <TableGroupedByLabels view={view} />
        </ResourceGroupsContextProvider>
      )

      // Loop through each resource group and expect that its expanded state
      // matches with the hardcoded test data
      const resourceGroups = wrapper.find(OverviewGroup)
      resourceGroups.forEach((group) => {
        const groupName = group.find(OverviewGroupName).text()
        expect(group.props().expanded).toEqual(testData[groupName].expanded)
      })
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
})
