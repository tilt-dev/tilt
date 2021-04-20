import React, { useEffect, useState } from "react"
import styled from "styled-components"
import { Alert, combinedAlerts } from "./alerts"
import HeaderBar from "./HeaderBar"
import { LogUpdateAction, LogUpdateEvent, useLogStore } from "./LogStore"
import OverviewResourceDetails from "./OverviewResourceDetails"
import OverviewResourceSidebar from "./OverviewResourceSidebar"
import { useResourceNav } from "./ResourceNav"
import StarredResourceBar, {
  starredResourcePropsFromView,
} from "./StarredResourceBar"
import { Color } from "./style-helpers"
import { ResourceName } from "./types"

type OverviewResourcePaneProps = {
  view: Proto.webviewView
}

let OverviewResourcePaneRoot = styled.div`
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 100vh;
  background-color: ${Color.grayDark};
  max-height: 100%;
`

let Main = styled.div`
  display: flex;
  width: 100%;
  // In Safari, flex-basis "auto" squishes OverviewTabBar + OverviewResourceBar
  flex: 1 1 100%;
  overflow: hidden;
`

export default function OverviewResourcePane(props: OverviewResourcePaneProps) {
  let nav = useResourceNav()
  const logStore = useLogStore()
  let resources = props.view?.resources || []
  let name = nav.invalidResource || nav.selectedResource || ""
  let r: Proto.webviewResource | undefined
  let all = name === "" || name === ResourceName.all
  if (!all) {
    r = resources.find((r) => r.name === name)
  }
  let selectedTab = ""
  if (all) {
    selectedTab = ResourceName.all
  } else if (r?.name) {
    selectedTab = r.name
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

  // Hide the HTML element scrollbars, since this pane does all scrolling internally.
  // TODO(nick): Remove this when the old UI is deleted.
  useEffect(() => {
    document.documentElement.style.overflow = "hidden"
  })

  return (
    <OverviewResourcePaneRoot>
      <HeaderBar view={props.view} />
      <StarredResourceBar
        {...starredResourcePropsFromView(props.view, selectedTab)}
      />
      <Main>
        <OverviewResourceSidebar {...props} name={name} />
        <OverviewResourceDetails resource={r} name={name} alerts={alerts} />
      </Main>
    </OverviewResourcePaneRoot>
  )
}
