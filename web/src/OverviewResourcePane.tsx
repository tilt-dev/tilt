import React, { useEffect, useState } from "react"
import SplitPane from "react-split-pane"
import styled from "styled-components"
import { Alert, combinedAlerts } from "./alerts"
import { AnalyticsType } from "./analytics"
import { ApiButtonType, buttonsForComponent } from "./ApiButton"
import HeaderBar from "./HeaderBar"
import { LogUpdateAction, LogUpdateEvent, useLogStore } from "./LogStore"
import OverviewResourceDetails from "./OverviewResourceDetails"
import OverviewResourceSidebar from "./OverviewResourceSidebar"
import "./Resizer.scss"
import { useResourceNav } from "./ResourceNav"
import StarredResourceBar, {
  starredResourcePropsFromView,
} from "./StarredResourceBar"
import { Color, Width } from "./style-helpers"
import { ResourceName } from "./types"

type UIResource = Proto.v1alpha1UIResource
type OverviewResourcePaneProps = {
  view: Proto.webviewView
}

let OverviewResourcePaneRoot = styled.div`
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 100vh;
  background-color: ${Color.gray20};
  max-height: 100%;
`
let Main = styled.div`
  display: flex;
  width: 100%;
  // In Safari, flex-basis "auto" squishes OverviewTabBar + OverviewResourceBar
  flex: 1 1 100%;
  overflow: hidden;
  position: relative;

  .SplitPane {
    position: relative !important;
  }
  .Pane {
    display: flex;
  }
`

export default function OverviewResourcePane(props: OverviewResourcePaneProps) {
  let nav = useResourceNav()
  const logStore = useLogStore()
  let resources = props.view?.uiResources || []
  let name = nav.invalidResource || nav.selectedResource || ""
  let r: UIResource | undefined
  let all = name === "" || name === ResourceName.all
  if (!all) {
    r = resources.find((r) => r.metadata?.name === name)
  }
  let selectedTab = ""
  if (all) {
    selectedTab = ResourceName.all
  } else if (r?.metadata?.name) {
    selectedTab = r.metadata.name
  }

  const [truncateCount, setTruncateCount] = useState<number>(0)

  // add a listener to rebuild alerts whenever a truncation event occurs
  // truncateCount is a dummy state variable to trigger a re-render to
  // simplify logic vs reconciliation between logStore + props
  useEffect(() => {
    const rebuildAlertsOnLogClear = (e: LogUpdateEvent) => {
      if (e.action === LogUpdateAction.truncate) {
        setTruncateCount(truncateCount + 1)
      }
    }

    logStore.addUpdateListener(rebuildAlertsOnLogClear)
    return () => logStore.removeUpdateListener(rebuildAlertsOnLogClear)
  }, [truncateCount])

  let alerts: Alert[] = []
  if (r) {
    alerts = combinedAlerts(r, logStore)
  } else if (all) {
    resources.forEach((r) => alerts.push(...combinedAlerts(r, logStore)))
  }

  const buttons = buttonsForComponent(
    props.view.uiButtons,
    ApiButtonType.Resource,
    name
  )

  return (
    <OverviewResourcePaneRoot>
      <HeaderBar view={props.view} currentPage={AnalyticsType.Detail} />
      <StarredResourceBar
        {...starredResourcePropsFromView(props.view, selectedTab)}
      />
      <Main>
        <SplitPane
          split="vertical"
          minSize={Width.sidebarDefault}
          defaultSize={Width.sidebarDefault}
        >
          <OverviewResourceSidebar {...props} name={name} />
          <OverviewResourceDetails
            resource={r}
            name={name}
            alerts={alerts}
            buttons={buttons}
          />
        </SplitPane>
      </Main>
    </OverviewResourcePaneRoot>
  )
}
