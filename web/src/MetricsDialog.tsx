import React from "react"
import styled from "styled-components"
import ButtonInput from "./ButtonInput"
import FloatDialog from "./FloatDialog"
import { usePathBuilder } from "./PathBuilder"
import { AnimDuration, Color, Font, FontSize } from "./style-helpers"

type Serving = Proto.webviewMetricsServing

let MetricsContentsRoot = styled.div`
  display: flex;
  flex-direction: column;
  font-size: ${FontSize.small};
  font-family: ${Font.sansSerif};
`

let MetricsHeaderLink = styled.a`
  text-decoration: none;
  transition: color ${AnimDuration.default} ease;

  &:hover {
    color: ${Color.blue};
  }
`

let MetricsGraf = styled.div`
  margin-bottom: 8px;
`

let MetricsGraphRoot = styled.div`
  display: flex;
  flex-direction: column;
`

let MetricsButtonBlock = styled(ButtonInput)`
  width: auto;
  margin: 8px 0 8px auto;
`

let MetricsGraph = styled.iframe`
  display: block;
  width: 100%;
  height: 240px;
  background: transparent;
  border: none;
  margin: 8px 0;
`

function enableLocalMetrics(opt: string) {
  fetch(`/api/metrics_opt`, {
    method: "post",
    body: opt,
  })
}

function MetricsTeaser() {
  return (
    <MetricsContentsRoot>
      <MetricsGraf>
        Experimental: Enabling this pane deploys a small metrics stack to your
        cluster that monitors your build performance.
      </MetricsGraf>

      <MetricsGraf>
        These metrics are not sent outside your cluster.
      </MetricsGraf>

      <MetricsGraf>
        We would love to{" "}
        <a
          href="https://docs.tilt.dev/#community"
          target="_blank"
          rel="noopener noreferrer"
        >
          hear from you
        </a>{" "}
        on your thoughts on this feature.
      </MetricsGraf>

      <MetricsButtonBlock
        type="button"
        value="Enable Metrics"
        onClick={() => enableLocalMetrics("local")}
      />
    </MetricsContentsRoot>
  )
}

function MetricsLoading() {
  return (
    <MetricsContentsRoot>
      <MetricsGraf>Loading dashboards...</MetricsGraf>
      <MetricsButtonBlock
        type="button"
        value="Disable Metrics"
        onClick={() => enableLocalMetrics("disabled")}
      />
    </MetricsContentsRoot>
  )
}

type MetricsProps = {
  open: boolean
  onClose: () => void
  anchorEl: Element | null
  serving: Serving | null | undefined
}

function MetricsDialogContents(props: { serving: Serving | null | undefined }) {
  let pathBuilder = usePathBuilder()
  let serving = props.serving
  if (!serving || serving?.mode !== "local") {
    return <MetricsTeaser />
  }

  if (!serving.grafanaHost) {
    return <MetricsLoading />
  }

  let protocol = pathBuilder.isSecure() ? "https" : "http"
  let root = `${protocol}://${serving.grafanaHost}`

  let frames = [
    "/d-solo/nIq4P-TMz/tilt-local-metrics?orgId=1&refresh=5s&panelId=2",
    "/d-solo/nIq4P-TMz/tilt-local-metrics?orgId=1&refresh=5s&panelId=3",
  ]

  let graphs = frames.map((frame, i) => {
    return <MetricsGraph key={"graph" + i} src={root + frame}></MetricsGraph>
  })

  let link = `${root}/d/nIq4P-TMz/tilt-local-metrics?orgId=1&refresh=5s`

  return (
    <MetricsContentsRoot>
      <MetricsHeaderLink href={link} target="_blank" rel="noopener noreferrer">
        View Full Dashboard &gt;
      </MetricsHeaderLink>
      <MetricsGraphRoot>{graphs}</MetricsGraphRoot>
      <MetricsButtonBlock
        type="button"
        value="Disable Metrics"
        onClick={() => enableLocalMetrics("")}
      />
    </MetricsContentsRoot>
  )
}

export default function MetricsDialog(props: MetricsProps) {
  let { serving, ...others } = props
  return (
    <FloatDialog id="metrics" title="Metrics" {...others}>
      <MetricsDialogContents serving={serving} />
    </FloatDialog>
  )
}
