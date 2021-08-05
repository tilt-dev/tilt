import {
  Accordion,
  AccordionDetails,
  AccordionSummary,
} from "@material-ui/core"
import React, {
  ChangeEvent,
  Dispatch,
  PropsWithChildren,
  SetStateAction,
  useState,
} from "react"
import styled from "styled-components"
import { AnalyticsType, incr } from "./analytics"
import { ReactComponent as CaretSvg } from "./assets/svg/caret.svg"
import { ReactComponent as InfoSvg } from "./assets/svg/info.svg"
import Features, { FeaturesContext, Flag } from "./feature"
import { orderLabels } from "./labels"
import { PersistentStateProvider } from "./LocalStorage"
import { OverviewSidebarOptions } from "./OverviewSidebarOptions"
import PathBuilder from "./PathBuilder"
import { ResourceSidebarStatusSummary } from "./ResourceStatusSummary"
import SidebarItem from "./SidebarItem"
import SidebarItemView, {
  SidebarItemRoot,
  triggerUpdate,
} from "./SidebarItemView"
import SidebarKeyboardShortcuts from "./SidebarKeyboardShortcuts"
import { Color, FontSize, SizeUnit } from "./style-helpers"
import { TiltInfoTooltip } from "./Tooltip"
import { ResourceView, SidebarOptions } from "./types"

let SidebarResourcesRoot = styled.nav`
  flex: 1 0 auto;

  &.isOverview {
    overflow: auto;
    flex-shrink: 1;
  }
`

let SidebarList = styled.div`
  margin-bottom: ${SizeUnit(1.75)};
`

let SidebarListSectionName = styled.div`
  margin-top: ${SizeUnit(0.5)};
  margin-left: ${SizeUnit(0.5)};
  text-transform: uppercase;
  color: ${Color.grayLight};
  font-size: ${FontSize.small};
`
const SidebarListSectionItems = styled.ul`
  margin-top: ${SizeUnit(0.25)};
  list-style: none;
`

const NoMatchesFound = styled.li`
  margin-left: ${SizeUnit(0.5)};
  color: ${Color.grayLightest};
`

const SidebarLabelSection = styled(Accordion)`
  &.MuiPaper-root {
    background-color: unset;
  }

  &.MuiPaper-elevation1 {
    box-shadow: unset;
  }

  &.MuiAccordion-root,
  &.MuiAccordion-root.Mui-expanded {
    margin: ${SizeUnit(1 / 3)} ${SizeUnit(1 / 2)};
  }
`

const SidebarGroupInfo = styled.aside`
  background-color: ${Color.grayDark};
  bottom: 0;
  box-sizing: border-box;
  left: 0;
  padding: 10px 10px 5px 10px;
  position: absolute;
  width: 100%;
  z-index: 2;
`

const InfoIcon = styled(InfoSvg)`
  .fillStd {
    fill: ${Color.blueLight};
  }
`

const SummaryIcon = styled(CaretSvg)`
  flex-shrink: 0;
  padding: ${SizeUnit(1 / 4)};
  transition: transform 300ms cubic-bezier(0.4, 0, 0.2, 1) 0ms; /* Copied from MUI accordion */

  .fillStd {
    fill: ${Color.grayLight};
  }
`

const SidebarGroupSummary = styled(AccordionSummary)`
  &.MuiAccordionSummary-root,
  &.MuiAccordionSummary-root.Mui-expanded {
    min-height: unset;
    padding: unset;
  }

  .MuiAccordionSummary-content {
    align-items: center;
    background-color: ${Color.grayLighter};
    border: 1px solid ${Color.grayLight};
    border-radius: ${SizeUnit(1 / 8)};
    box-sizing: border-box;
    color: ${Color.white};
    display: flex;
    font-size: ${FontSize.small};
    margin: 0;
    padding: ${SizeUnit(1 / 8)};
    width: 100%;

    &.Mui-expanded {
      margin: 0;

      ${SummaryIcon} {
        transform: rotate(90deg);
      }
    }
  }
`

const SidebarGroupName = styled.span`
  margin-right: auto;
  overflow: hidden;
  text-overflow: ellipsis;
  width: 100%;
`

const SidebarGroupDetails = styled(AccordionDetails)`
  &.MuiAccordionDetails-root {
    display: unset;
    padding: unset;

    ${SidebarItemRoot} {
      margin-right: unset;
    }
  }
`

const GROUP_INFO_TOOLTIP_ID = "sidebar-groups-info"
function SidebarLabelInfo() {
  const tooltipInfo = (
    <>
      Resources can be grouped by adding custom labels.{" "}
      <a
        href="https://docs.tilt.dev/tiltfile_concepts.html#resource-groups"
        target="_blank"
      >
        See docs for more info
      </a>
      .
    </>
  )

  return (
    <SidebarGroupInfo>
      <TiltInfoTooltip title={tooltipInfo} />
    </SidebarGroupInfo>
  )
}

export function SidebarListSection(
  props: PropsWithChildren<{ name: string }>
): JSX.Element {
  return (
    <div>
      <SidebarListSectionName>{props.name}</SidebarListSectionName>
      <SidebarListSectionItems>{props.children}</SidebarListSectionItems>
    </div>
  )
}

function SidebarItemsView(props: SidebarProps & { groupView?: boolean }) {
  return (
    <>
      {props.items.map((item) => (
        <SidebarItemView
          key={"sidebarItem-" + item.name}
          groupView={props.groupView}
          item={item}
          selected={props.selected === item.name}
          pathBuilder={props.pathBuilder}
          resourceView={props.resourceView}
        />
      ))}
    </>
  )
}

function SidebarLabelListSection(props: { label: string } & SidebarProps) {
  if (props.items.length === 0) {
    return null
  }

  const formattedLabel =
    props.label === "unlabeled" ? <em>{props.label}</em> : props.label
  const labelNameId = `sidebarItem-${props.label}`

  // Track the expanded/collapsed state of a resource group manually
  // so analytics events are captured
  const [expanded, setExpanded] = useState(true)
  const handleChange = (_e: ChangeEvent<{}>) => {
    const action = expanded ? "collapse" : "expand"
    incr("ui.web.resourceGroup", { action, type: AnalyticsType.Detail })

    setExpanded(!expanded)
  }

  // TODO (lizz): Improve the accessibility interface for accordion feature by adding focus styles
  // according to https://www.w3.org/TR/wai-aria-practices-1.1/examples/accordion/accordion.html
  return (
    <SidebarLabelSection
      expanded={expanded}
      key={labelNameId}
      onChange={handleChange}
    >
      <SidebarGroupSummary id={labelNameId}>
        <SummaryIcon role="presentation" />
        <SidebarGroupName>{formattedLabel}</SidebarGroupName>
        <ResourceSidebarStatusSummary
          aria-label={`Status summary for ${props.label} group`}
          items={props.items}
        />
      </SidebarGroupSummary>
      <SidebarGroupDetails aria-labelledby={labelNameId}>
        <SidebarListSectionItems>
          <SidebarItemsView {...props} />
        </SidebarListSectionItems>
      </SidebarGroupDetails>
    </SidebarLabelSection>
  )
}

function SidebarGroupedByLabels(props: SidebarProps) {
  const labelsToResources: { [key: string]: SidebarItem[] } = {}
  const unlabeled: SidebarItem[] = []
  const tiltfile: SidebarItem[] = []

  props.items.forEach((item) => {
    if (item.labels.length) {
      item.labels.forEach((label) => {
        if (!labelsToResources.hasOwnProperty(label)) {
          labelsToResources[label] = []
        }

        labelsToResources[label].push(item)
      })
    } else if (!item.isTiltfile) {
      unlabeled.push(item)
    }

    // Display the Tiltfile outside of the label groups
    if (item.isTiltfile) {
      tiltfile.push(item)
    }
  })

  const labels = orderLabels(Object.keys(labelsToResources))

  return (
    <>
      {labels.map((label) => (
        <SidebarLabelListSection
          {...props}
          key={`sidebarItem-${label}`}
          label={label}
          items={labelsToResources[label]}
        />
      ))}
      <SidebarLabelListSection {...props} label="unlabeled" items={unlabeled} />
      <SidebarListSection name="Tiltfile">
        <SidebarItemsView {...props} items={tiltfile} groupView={true} />
      </SidebarListSection>
    </>
  )
}

type UIResource = Proto.v1alpha1UIResource
type Build = Proto.v1alpha1UIBuildTerminated

type SidebarProps = {
  items: SidebarItem[]
  selected: string
  resourceView: ResourceView
  pathBuilder: PathBuilder
}

export const defaultOptions: SidebarOptions = {
  alertsOnTop: false,
  resourceNameFilter: "",
}

function MaybeUpgradeSavedSidebarOptions(o: SidebarOptions) {
  // non-nullable fields added to SidebarOptions after its initial release need to have default values
  // filled in here
  return { ...o, resourceNameFilter: o.resourceNameFilter ?? "" }
}

function hasAlerts(item: SidebarItem): boolean {
  return item.buildAlertCount > 0 || item.runtimeAlertCount > 0
}

function sortByHasAlerts(itemA: SidebarItem, itemB: SidebarItem): number {
  return Number(hasAlerts(itemB)) - Number(hasAlerts(itemA))
}

function matchesResourceName(item: SidebarItem, filter: string): boolean {
  filter = filter.trim()
  // this is functionally redundant but probably an important enough case to make its own thing
  if (filter === "") {
    return true
  }
  // a resource matches the query if the resource name contains all tokens in the query
  return filter
    .split(" ")
    .every((token) => item.name.toLowerCase().includes(token.toLowerCase()))
}

function resourcesHaveLabels(features: Features, items: SidebarItem[]) {
  if (!features.isEnabled(Flag.Labels)) {
    return false
  }

  return items.some((item) => item.labels.length > 0)
}

function applyOptionsToItems(
  items: SidebarItem[],
  options: SidebarOptions
): SidebarItem[] {
  let itemsToDisplay: SidebarItem[] = [...items]
  if (options.resourceNameFilter) {
    itemsToDisplay = itemsToDisplay.filter((item) =>
      matchesResourceName(item, options.resourceNameFilter)
    )
  }

  if (options.alertsOnTop) {
    itemsToDisplay.sort(sortByHasAlerts)
  }

  return itemsToDisplay
}

export class SidebarResources extends React.Component<SidebarProps> {
  constructor(props: SidebarProps) {
    super(props)
    this.triggerSelected = this.triggerSelected.bind(this)
  }

  static contextType = FeaturesContext

  triggerSelected() {
    if (this.props.selected) {
      triggerUpdate(this.props.selected)
    }
  }

  renderWithOptions(
    options: SidebarOptions,
    setOptions: Dispatch<SetStateAction<SidebarOptions>>
  ) {
    const filteredItems = applyOptionsToItems(this.props.items, options)

    // only say no matches if there were actually items that got filtered out
    // otherwise, there might just be 0 resources because there are 0 resources
    // (though technically there's probably always at least a Tiltfile resource)
    const resourceFilterApplied = options.resourceNameFilter.length > 0
    const noResourcesMatchFilter =
      resourceFilterApplied && filteredItems.length === 0
    const listItems = noResourcesMatchFilter ? (
      <NoMatchesFound key={"no-matches"}>No matching resources</NoMatchesFound>
    ) : (
      <SidebarItemsView {...this.props} items={filteredItems} />
    )
    const sidebarName = resourceFilterApplied
      ? `${filteredItems.length} result${filteredItems.length === 1 ? "" : "s"}`
      : "resources"

    let isOverviewClass =
      this.props.resourceView === ResourceView.OverviewDetail
        ? "isOverview"
        : ""

    // Note: the label group view does not display if a resource name filter is applied
    const labelsEnabled = resourcesHaveLabels(this.context, this.props.items)
    const displayLabelGroups = !resourceFilterApplied && labelsEnabled

    return (
      <SidebarResourcesRoot className={`Sidebar-resources ${isOverviewClass}`}>
        <SidebarLabelInfo />
        <SidebarList aria-describedby={GROUP_INFO_TOOLTIP_ID}>
          <OverviewSidebarOptions options={options} setOptions={setOptions} />
          {displayLabelGroups ? (
            <SidebarGroupedByLabels {...this.props} items={filteredItems} />
          ) : (
            <SidebarListSection name={sidebarName}>
              {listItems}
            </SidebarListSection>
          )}
        </SidebarList>
        <SidebarKeyboardShortcuts
          selected={this.props.selected}
          items={filteredItems}
          onTrigger={this.triggerSelected}
          resourceView={this.props.resourceView}
        />
      </SidebarResourcesRoot>
    )
  }

  render() {
    return (
      <PersistentStateProvider
        defaultValue={defaultOptions}
        name={"sidebar_options"}
        maybeUpgradeSavedState={MaybeUpgradeSavedSidebarOptions}
      >
        {(value: SidebarOptions, set) => this.renderWithOptions(value, set)}
      </PersistentStateProvider>
    )
  }
}

export default SidebarResources
