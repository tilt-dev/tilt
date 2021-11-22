import {
  Accordion,
  AccordionDetails,
  AccordionSummary,
} from "@material-ui/core"
import React, { ChangeEvent, useMemo } from "react"
import styled from "styled-components"
import { AnalyticsType } from "./analytics"
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
import SidebarItem from "./SidebarItem"
import SidebarItemView, {
  sidebarItemIsDisabled,
  SidebarItemRoot,
  triggerUpdate,
} from "./SidebarItemView"
import SidebarKeyboardShortcuts from "./SidebarKeyboardShortcuts"
import SrOnly from "./SrOnly"
import { Color, Font, FontSize, SizeUnit } from "./style-helpers"
import { ResourceView } from "./types"

export type SidebarProps = {
  items: SidebarItem[]
  selected: string
  resourceView: ResourceView
  pathBuilder: PathBuilder
  resourceListOptions: ResourceListOptions
}

type SidebarSectionProps = {
  sectionName?: string
  groupView?: boolean
} & SidebarProps

let SidebarResourcesRoot = styled.nav`
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
  color: ${Color.grayLight};
  font-size: ${FontSize.small};
`

const SidebarListSectionItemsRoot = styled.ul`
  margin-top: ${SizeUnit(0.25)};
  list-style: none;
`

export const SidebarDisabledSectionList = styled.li`
  color: ${Color.gray6};
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
    background-color: ${Color.grayLighter};
    border: 1px solid ${Color.grayLight};
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

  const [enabledItems, disabledItems] = useMemo(() => {
    const enabledItems: SidebarItem[] = []
    const disabledItems: SidebarItem[] = []

    props.items.forEach((item) => {
      if (sidebarItemIsDisabled(item)) {
        disabledItems.push(item)
      } else {
        enabledItems.push(item)
      }
    })

    return [enabledItems, disabledItems]
  }, props.items)

  // The title for the disabled resource list is semantically important,
  // but should only be visible when there's no filter term
  const disableTitle = (
    <SidebarDisabledSectionTitle>Disabled</SidebarDisabledSectionTitle>
  )
  const disableSectionTitle = resourceNameFilterApplied ? (
    <SrOnly>{disableTitle}</SrOnly>
  ) : (
    disableTitle
  )

  const displayDisabledResources =
    features.isEnabled(Flag.DisableResources) && disabledItems.length > 0
  return (
    <>
      {sectionName}
      <SidebarListSectionItemsRoot>
        <SidebarListSectionItems {...props} items={enabledItems} />
        {displayDisabledResources && (
          <SidebarDisabledSectionList>
            {disableSectionTitle}
            <ul>
              <SidebarListSectionItems {...props} items={disabledItems} />
            </ul>
          </SidebarDisabledSectionList>
        )}
      </SidebarListSectionItemsRoot>
    </>
  )
}

function SidebarListSectionItems(props: SidebarSectionProps) {
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

function SidebarGroupListSection(props: { label: string } & SidebarProps) {
  if (props.items.length === 0) {
    return null
  }

  // If all resources in this group are disabled, but the disable resources
  // flag isn't enabled, don't display any group information
  const features = useFeatures()
  const showDisabledResources = features.isEnabled(Flag.DisableResources)
  const allResourcesDisabled = props.items.every((item) =>
    sidebarItemIsDisabled(item)
  )

  if (!showDisabledResources && allResourcesDisabled) {
    return null
  }

  const formattedLabel =
    props.label === UNLABELED_LABEL ? <em>{props.label}</em> : props.label
  const labelNameId = `sidebarItem-${props.label}`

  const { getGroup, toggleGroupExpanded } = useResourceGroups()
  const { expanded } = getGroup(props.label)
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
          aria-label={`Status summary for ${props.label} group`}
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

function SidebarGroupedByLabels(props: SidebarProps) {
  const { labels, labelsToResources, tiltfile, unlabeled } = resourcesLabelView(
    props.items
  )

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
  if (options.resourceNameFilter) {
    itemsToDisplay = itemsToDisplay.filter((item) =>
      matchesResourceName(item.name, options.resourceNameFilter)
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
      <SidebarResourcesRoot className={`Sidebar-resources ${isOverviewClass}`}>
        {displayLabelGroupsTip && (
          <ResourceGroupsInfoTip idForIcon={GROUP_INFO_TOOLTIP_ID} />
        )}
        <SidebarResourcesContent
          aria-describedby={
            displayLabelGroupsTip ? GROUP_INFO_TOOLTIP_ID : undefined
          }
        >
          <OverviewSidebarOptions />
          {displayLabelGroups ? (
            <SidebarGroupedByLabels {...this.props} items={filteredItems} />
          ) : (
            <SidebarListSection
              {...this.props}
              sectionName={sidebarName}
              items={filteredItems}
            />
          )}
        </SidebarResourcesContent>
        <SidebarKeyboardShortcuts
          selected={this.props.selected}
          items={filteredItems}
          onTrigger={this.triggerSelected}
          resourceView={this.props.resourceView}
        />
      </SidebarResourcesRoot>
    )
  }
}

export default SidebarResources
