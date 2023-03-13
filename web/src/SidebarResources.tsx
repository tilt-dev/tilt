import {
  Accordion,
  AccordionDetails,
  AccordionSummary,
} from "@material-ui/core"
import React, { ChangeEvent, useCallback, useState } from "react"
import { Link } from "react-router-dom"
import styled from "styled-components"
import { AnalyticsType, Tags } from "./analytics"
import {
  DEFAULT_RESOURCE_LIST_LIMIT,
  RESOURCE_LIST_MULTIPLIER,
} from "./constants"
import { FeaturesContext, Flag, useFeatures } from "./feature"
import {
  GroupByLabelView,
  orderLabels,
  TILTFILE_LABEL,
  UNLABELED_LABEL,
} from "./labels"
import { OverviewSidebarOptions } from "./OverviewSidebarOptions"
import PathBuilder from "./PathBuilder"
import {
  AccordionDetailsStyleResetMixin,
  AccordionStyleResetMixin,
  AccordionSummaryStyleResetMixin,
  ResourceGroupsInfoTip,
  ResourceGroupSummaryIcon,
  ResourceGroupSummaryMixin,
} from "./ResourceGroups"
import { useResourceGroups } from "./ResourceGroupsContext"
import { ResourceListOptions } from "./ResourceListOptionsContext"
import { matchesResourceName } from "./ResourceNameFilter"
import { SidebarGroupStatusSummary } from "./ResourceStatusSummary"
import { ShowMoreButton } from "./ShowMoreButton"
import SidebarItem from "./SidebarItem"
import SidebarItemView, {
  sidebarItemIsDisabled,
  SidebarItemRoot,
} from "./SidebarItemView"
import SidebarKeyboardShortcuts from "./SidebarKeyboardShortcuts"
import { AnimDuration, Color, Font, FontSize, SizeUnit } from "./style-helpers"
import { startBuild } from "./trigger"
import { ResourceName, ResourceStatus, ResourceView } from "./types"
import { useStarredResources } from "./StarredResourcesContext"

export type SidebarProps = {
  items: SidebarItem[]
  selected: string
  resourceView: ResourceView
  pathBuilder: PathBuilder
  resourceListOptions: ResourceListOptions
}

type SidebarGroupedByProps = SidebarProps & {
  onStartBuild: () => void
}

type SidebarSectionProps = {
  sectionName?: string
  groupView?: boolean
} & SidebarProps

export let SidebarResourcesRoot = styled.nav`
  flex: 1 0 auto;

  &.isOverview {
    overflow: auto;
    flex-shrink: 1;
  }
`

let SidebarResourcesContent = styled.div`
  margin-bottom: ${SizeUnit(1.75)};
`

let SidebarListSectionName = styled.div`
  margin-top: ${SizeUnit(0.5)};
  margin-left: ${SizeUnit(0.5)};
  text-transform: uppercase;
  color: ${Color.gray50};
  font-size: ${FontSize.small};
`

const BuiltinResourceLinkRoot = styled(Link)`
  background-color: ${Color.gray40};
  border: 1px solid ${Color.gray50};
  border-radius: ${SizeUnit(1 / 8)};
  color: ${Color.white};
  display: block;
  font-family: ${Font.sansSerif};
  font-size: ${FontSize.smallest};
  font-weight: normal;
  margin: ${SizeUnit(1 / 3)} ${SizeUnit(1 / 2)};
  padding: ${SizeUnit(1 / 5)} ${SizeUnit(1 / 3)};
  text-decoration: none;
  transition: all ${AnimDuration.default} ease;

  &:is(:hover, :focus, :active) {
    background-color: ${Color.gray30};
  }

  &.isSelected {
    background-color: ${Color.gray70};
    color: ${Color.gray30};
    font-weight: 600;
  }
`

export const SidebarListSectionItemsRoot = styled.ul`
  margin-top: ${SizeUnit(0.25)};
  list-style: none;
`

export const SidebarDisabledSectionList = styled.li`
  color: ${Color.gray60};
  font-family: ${Font.sansSerif};
  font-size: ${FontSize.small};
`

export const SidebarDisabledSectionTitle = styled.span`
  display: inline-block;
  margin-bottom: ${SizeUnit(1 / 12)};
  margin-top: ${SizeUnit(1 / 3)};
  padding-left: ${SizeUnit(3 / 4)};
`

const NoMatchesFound = styled.li`
  margin-left: ${SizeUnit(0.5)};
  color: ${Color.grayLightest};
`

const SidebarLabelSection = styled(Accordion)`
  ${AccordionStyleResetMixin}

  /* Set specific margins for sidebar */
  &.MuiAccordion-root,
  &.MuiAccordion-root.Mui-expanded {
    margin: ${SizeUnit(1 / 3)} ${SizeUnit(1 / 2)};
  }
`

const SidebarGroupSummary = styled(AccordionSummary)`
  ${AccordionSummaryStyleResetMixin}
  ${ResourceGroupSummaryMixin}

  /* Set specific background and borders for sidebar */
  .MuiAccordionSummary-content {
    background-color: ${Color.gray40};
    border: 1px solid ${Color.gray50};
    border-radius: ${SizeUnit(1 / 8)};
    font-size: ${FontSize.small};
  }
`

export const SidebarGroupName = styled.span`
  margin-right: auto;
  overflow: hidden;
  text-overflow: ellipsis;
  width: 100%;
`

const SidebarGroupDetails = styled(AccordionDetails)`
  ${AccordionDetailsStyleResetMixin}

  &.MuiAccordionDetails-root {
    ${SidebarItemRoot} {
      margin-right: unset;
    }
  }
`

const GROUP_INFO_TOOLTIP_ID = "sidebar-groups-info"

function onlyEnabledItems(items: SidebarItem[]): SidebarItem[] {
  return items.filter((item) => !sidebarItemIsDisabled(item))
}
function onlyDisabledItems(items: SidebarItem[]): SidebarItem[] {
  return items.filter((item) => sidebarItemIsDisabled(item))
}
function enabledItemsFirst(items: SidebarItem[]): SidebarItem[] {
  let result = onlyEnabledItems(items)
  result.push(...onlyDisabledItems(items))
  return result
}

function AllResourcesLink(props: {
  pathBuilder: PathBuilder
  selected: string
}) {
  const isSelectedClass = props.selected === "" ? "isSelected" : ""
  return (
    <BuiltinResourceLinkRoot
      className={isSelectedClass}
      aria-label="View all resource logs"
      to={props.pathBuilder.encpath`/r/(all)/overview`}
    >
      All Resources
    </BuiltinResourceLinkRoot>
  )
}

function StarredResourcesLink(props: {
  pathBuilder: PathBuilder
  selected: string
}) {
  const starContext = useStarredResources()
  if (!starContext.starredResources.length) {
    return null
  }
  const isSelectedClass =
    props.selected === ResourceName.starred ? "isSelected" : ""
  return (
    <BuiltinResourceLinkRoot
      className={isSelectedClass}
      aria-label="View starred resource logs"
      to={props.pathBuilder.encpath`/r/(starred)/overview`}
    >
      Starred Resources
    </BuiltinResourceLinkRoot>
  )
}

export function SidebarListSection(props: SidebarSectionProps): JSX.Element {
  const features = useFeatures()
  const sectionName = props.sectionName ? (
    <SidebarListSectionName>{props.sectionName}</SidebarListSectionName>
  ) : null

  const resourceNameFilterApplied =
    props.resourceListOptions.resourceNameFilter.length > 0
  if (props.items.length === 0 && resourceNameFilterApplied) {
    return (
      <>
        {sectionName}
        <SidebarListSectionItemsRoot>
          <NoMatchesFound>No matching resources</NoMatchesFound>
        </SidebarListSectionItemsRoot>
      </>
    )
  }

  // TODO(nick): Figure out how to memoize filters effectively.
  const enabledItems = onlyEnabledItems(props.items)
  const disabledItems = onlyDisabledItems(props.items)

  const displayDisabledResources = disabledItems.length > 0

  return (
    <>
      {sectionName}
      <SidebarListSectionItemsRoot>
        <SidebarListSectionItems {...props} items={enabledItems} />

        {displayDisabledResources && (
          <SidebarDisabledSectionList aria-label="Disabled resources">
            <SidebarDisabledSectionTitle>Disabled</SidebarDisabledSectionTitle>
            <ul>
              <SidebarListSectionItems {...props} items={disabledItems} />
            </ul>
          </SidebarDisabledSectionList>
        )}
      </SidebarListSectionItemsRoot>
    </>
  )
}

const ShowMoreRow = styled.li`
  margin: ${SizeUnit(0.5)} ${SizeUnit(0.5)} 0 ${SizeUnit(0.5)};
  display: flex;
  align-items: center;
  justify-content: right;
`
const DETAIL_TYPE_TAGS: Tags = { type: AnalyticsType.Detail }

function SidebarListSectionItems(props: SidebarSectionProps) {
  let [maxItems, setMaxItems] = useState(DEFAULT_RESOURCE_LIST_LIMIT)
  let displayItems = props.items
  let moreItems = Math.max(displayItems.length - maxItems, 0)
  if (moreItems) {
    displayItems = displayItems.slice(0, maxItems)
  }

  let showMore = useCallback(() => {
    setMaxItems(maxItems * RESOURCE_LIST_MULTIPLIER)
  }, [maxItems, setMaxItems])

  let showMoreItemsButton = null
  if (moreItems > 0) {
    showMoreItemsButton = (
      <ShowMoreRow>
        <ShowMoreButton
          onClick={showMore}
          analyticsTags={DETAIL_TYPE_TAGS}
          currentListSize={maxItems}
          itemCount={props.items.length}
        />
      </ShowMoreRow>
    )
  }

  return (
    <>
      {displayItems.map((item) => (
        <SidebarItemView
          key={"sidebarItem-" + item.name}
          groupView={props.groupView}
          item={item}
          selected={props.selected === item.name}
          pathBuilder={props.pathBuilder}
          resourceView={props.resourceView}
        />
      ))}
      {showMoreItemsButton}
    </>
  )
}

function SidebarGroupListSection(props: { label: string } & SidebarProps) {
  if (props.items.length === 0) {
    return null
  }

  const formattedLabel =
    props.label === UNLABELED_LABEL ? <em>{props.label}</em> : props.label
  const labelNameId = `sidebarItem-${props.label}`

  const { getGroup, toggleGroupExpanded } = useResourceGroups()
  let { expanded } = getGroup(props.label)

  let isSelected = props.items.some((item) => item.name == props.selected)

  if (isSelected) {
    // If an item in the group is selected, expand the group
    // without writing it back to persistent state.
    //
    // This creates a nice interaction, where if you're keyboard-navigating
    // through sidebar items, we expand the group you navigate into and expand
    // it when you navigate out again.
    expanded = true
  }

  const handleChange = (_e: ChangeEvent<{}>) =>
    toggleGroupExpanded(props.label, AnalyticsType.Detail)

  // TODO (lizz): Improve the accessibility interface for accordion feature by adding focus styles
  // according to https://www.w3.org/TR/wai-aria-practices-1.1/examples/accordion/accordion.html
  return (
    <SidebarLabelSection expanded={expanded} onChange={handleChange}>
      <SidebarGroupSummary id={labelNameId}>
        <ResourceGroupSummaryIcon role="presentation" />
        <SidebarGroupName>{formattedLabel}</SidebarGroupName>
        <SidebarGroupStatusSummary
          labelText={`Status summary for ${props.label} group`}
          resources={props.items}
        />
      </SidebarGroupSummary>
      <SidebarGroupDetails aria-labelledby={labelNameId}>
        <SidebarListSection {...props} />
      </SidebarGroupDetails>
    </SidebarLabelSection>
  )
}

function resourcesLabelView(
  items: SidebarItem[]
): GroupByLabelView<SidebarItem> {
  const labelsToResources: { [key: string]: SidebarItem[] } = {}
  const unlabeled: SidebarItem[] = []
  const tiltfile: SidebarItem[] = []

  items.forEach((item) => {
    if (item.labels.length) {
      item.labels.forEach((label) => {
        if (!labelsToResources.hasOwnProperty(label)) {
          labelsToResources[label] = []
        }

        labelsToResources[label].push(item)
      })
    } else if (item.isTiltfile) {
      tiltfile.push(item)
    } else {
      unlabeled.push(item)
    }
  })

  // Labels are always displayed in sorted order
  const labels = orderLabels(Object.keys(labelsToResources))

  return { labels, labelsToResources, tiltfile, unlabeled }
}

function SidebarGroupedByLabels(props: SidebarGroupedByProps) {
  const { labels, labelsToResources, tiltfile, unlabeled } = resourcesLabelView(
    props.items
  )

  // NOTE(nick): We need the visual order of the items to pass
  // to the keyboard navigation component. The problem is that
  // each section component does its own ordering. So we cheat
  // here and replicate the logic for determining the order.
  let totalOrder: SidebarItem[] = []
  labels.map((label) => {
    totalOrder.push(...enabledItemsFirst(labelsToResources[label]))
  })
  totalOrder.push(...enabledItemsFirst(unlabeled))
  totalOrder.push(...enabledItemsFirst(tiltfile))

  return (
    <>
      {labels.map((label) => (
        <SidebarGroupListSection
          {...props}
          key={`sidebarItem-${label}`}
          label={label}
          items={labelsToResources[label]}
        />
      ))}
      <SidebarGroupListSection
        {...props}
        label={UNLABELED_LABEL}
        items={unlabeled}
      />
      <SidebarListSection
        {...props}
        sectionName={TILTFILE_LABEL}
        items={tiltfile}
        groupView={true}
      />
      <SidebarKeyboardShortcuts
        selected={props.selected}
        items={totalOrder}
        onStartBuild={props.onStartBuild}
        resourceView={props.resourceView}
      />
    </>
  )
}

function hasAlerts(item: SidebarItem): boolean {
  return item.buildAlertCount > 0 || item.runtimeAlertCount > 0
}

function sortByHasAlerts(itemA: SidebarItem, itemB: SidebarItem): number {
  return Number(hasAlerts(itemB)) - Number(hasAlerts(itemA))
}

function applyOptionsToItems(
  items: SidebarItem[],
  options: ResourceListOptions
): SidebarItem[] {
  let itemsToDisplay: SidebarItem[] = [...items]

  const itemsShouldBeFiltered =
    options.resourceNameFilter.length > 0 || !options.showDisabledResources

  if (itemsShouldBeFiltered) {
    itemsToDisplay = itemsToDisplay.filter((item) => {
      const itemIsDisabled = item.runtimeStatus === ResourceStatus.Disabled
      if (!options.showDisabledResources && itemIsDisabled) {
        return false
      }

      if (options.resourceNameFilter) {
        return matchesResourceName(item.name, options.resourceNameFilter)
      }

      return true
    })
  }

  if (options.alertsOnTop) {
    itemsToDisplay.sort(sortByHasAlerts)
  }

  return itemsToDisplay
}

export class SidebarResources extends React.Component<SidebarProps> {
  constructor(props: SidebarProps) {
    super(props)
    this.startBuildOnSelected = this.startBuildOnSelected.bind(this)
  }

  static contextType = FeaturesContext

  startBuildOnSelected() {
    if (this.props.selected) {
      startBuild(this.props.selected)
    }
  }

  render() {
    const filteredItems = applyOptionsToItems(
      this.props.items,
      this.props.resourceListOptions
    )

    // only say no matches if there were actually items that got filtered out
    // otherwise, there might just be 0 resources because there are 0 resources
    // (though technically there's probably always at least a Tiltfile resource)
    const resourceFilterApplied =
      this.props.resourceListOptions.resourceNameFilter.length > 0
    const sidebarName = resourceFilterApplied
      ? `${filteredItems.length} result${filteredItems.length === 1 ? "" : "s"}`
      : "resources"

    let isOverviewClass =
      this.props.resourceView === ResourceView.OverviewDetail
        ? "isOverview"
        : ""

    const labelsEnabled: boolean = this.context.isEnabled(Flag.Labels)
    const resourcesHaveLabels = this.props.items.some(
      (item) => item.labels.length > 0
    )

    // The label group tip is only displayed if labels are enabled but not used
    const displayLabelGroupsTip = labelsEnabled && !resourcesHaveLabels
    // The label group view does not display if a resource name filter is applied
    const displayLabelGroups =
      !resourceFilterApplied && labelsEnabled && resourcesHaveLabels

    return (
      <SidebarResourcesRoot
        aria-label="Resource logs"
        className={`Sidebar-resources ${isOverviewClass}`}
      >
        {displayLabelGroupsTip && (
          <ResourceGroupsInfoTip idForIcon={GROUP_INFO_TOOLTIP_ID} />
        )}
        <SidebarResourcesContent
          aria-describedby={
            displayLabelGroupsTip ? GROUP_INFO_TOOLTIP_ID : undefined
          }
        >
          <OverviewSidebarOptions items={filteredItems} />
          <AllResourcesLink
            pathBuilder={this.props.pathBuilder}
            selected={this.props.selected}
          />
          <StarredResourcesLink
            pathBuilder={this.props.pathBuilder}
            selected={this.props.selected}
          />
          {displayLabelGroups ? (
            <SidebarGroupedByLabels
              {...this.props}
              items={filteredItems}
              onStartBuild={this.startBuildOnSelected}
            />
          ) : (
            <SidebarListSection
              {...this.props}
              sectionName={sidebarName}
              items={filteredItems}
            />
          )}
        </SidebarResourcesContent>
        {/* The label groups display handles the keyboard shortcuts separately. */}
        {displayLabelGroups ? null : (
          <SidebarKeyboardShortcuts
            selected={this.props.selected}
            items={enabledItemsFirst(filteredItems)}
            onStartBuild={this.startBuildOnSelected}
            resourceView={this.props.resourceView}
          />
        )}
      </SidebarResourcesRoot>
    )
  }
}

export default SidebarResources
