import React from "react"
import styled from "styled-components"
import { ReactComponent as HealthySvg } from "./assets/svg/checkmark-small.svg"
import { ReactComponent as UnhealthySvg } from "./assets/svg/close.svg"
import FloatDialog, {
  FloatDialogProps,
  Title as FloatDialogTitle,
} from "./FloatDialog"
import SrOnly from "./SrOnly"
import { Color, FontSize, SizeUnit } from "./style-helpers"
import { Cluster } from "./types"

export type ClusterStatusDialogProps = {
  clusterConnection?: Cluster
} & Pick<FloatDialogProps, "open" | "onClose" | "anchorEl">

export const DEFAULT_CLUSTER_NAME = "default"
export const CLUSTER_STATUS_HEALTHY = "Healthy"

const ClusterHeading = styled.figure`
  align-items: top;
  display: flex;
  flex-direction: row;
  justify-content: flex-start;
  margin: 0;
`

const statusIconMixin = `
flex-shrink: 0;
height: ${SizeUnit(0.4)};
padding-right: ${SizeUnit(0.4)};
padding-top: ${SizeUnit(1 / 4)};
width: ${SizeUnit(0.4)};
`

const HealthyIcon = styled(HealthySvg)`
  ${statusIconMixin}
  .fillStd {
    fill: ${Color.green};
  }
`

const UnhealthyIcon = styled(UnhealthySvg)`
  ${statusIconMixin}
  .fillStd {
    fill: ${Color.red};
  }
`

const ClusterText = styled(FloatDialogTitle)`
  line-height: unset;
`

const ClusterType = styled.span`
  display: block;
  text-decoration: underline;
  text-underline-position: under;
`

const ClusterStatus = styled.span`
  display: block;
  font-size: ${FontSize.small};
  text-decoration: none;
`

const ClusterPropertyName = styled.dt``

const ClusterPropertyDetail = styled.dd`
  margin-left: unset;
`

const ClusterPropertyList = styled.dl`
  margin: unset;

  ${ClusterPropertyName},
  ${ClusterPropertyDetail} {
    display: inline-block;
    width: 50%;
  }
`

export function getDefaultCluster(clusters?: Cluster[]): Cluster | undefined {
  if (!clusters || !clusters.length) {
    return
  }

  return clusters.find(
    (connection) => connection.metadata?.name === DEFAULT_CLUSTER_NAME
  )
}

function ClusterProperty({
  displayName,
  details,
}: {
  displayName: string
  details?: string
}) {
  if (!details) {
    return null
  }

  return (
    <>
      <ClusterPropertyName>{displayName}</ClusterPropertyName>
      <ClusterPropertyDetail>{details}</ClusterPropertyDetail>
    </>
  )
}

function K8sClusterProperties({
  clusterStatus,
}: {
  clusterStatus?: Cluster["status"]
}) {
  if (!clusterStatus) {
    return null
  }

  const k8sInfo = clusterStatus?.connection?.kubernetes
  return (
    <ClusterPropertyList>
      <ClusterProperty displayName="Product" details={k8sInfo?.product} />
      <ClusterProperty displayName="Context" details={k8sInfo?.context} />
      <ClusterProperty displayName="Namespace" details={k8sInfo?.namespace} />
      <ClusterProperty
        displayName="Architecture"
        details={clusterStatus?.arch}
      />
      <ClusterProperty displayName="Version" details={clusterStatus?.version} />
      {/* TODO (lizz): Decide how to display different registry info, ie fromContainerRuntime */}
      <ClusterProperty
        displayName="Local registry"
        details={clusterStatus?.registry?.host}
      />
    </ClusterPropertyList>
  )
}

export function ClusterStatusDialog(props: ClusterStatusDialogProps) {
  const { open, onClose, anchorEl, clusterConnection } = props

  if (!clusterConnection) {
    return null
  }

  const clusterStatus =
    clusterConnection.status?.error ?? CLUSTER_STATUS_HEALTHY
  const clusterStatusIcon =
    clusterStatus === CLUSTER_STATUS_HEALTHY ? (
      <HealthyIcon role="presentation" data-testid="healthy-icon" />
    ) : (
      <UnhealthyIcon role="presentation" data-testid="unhealthy-icon" />
    )

  const clusterTitle = (
    <ClusterHeading>
      {clusterStatusIcon}
      <ClusterText>
        <ClusterType>Kubernetes</ClusterType>
        <ClusterStatus>
          <SrOnly>Status:</SrOnly> {clusterStatus}
        </ClusterStatus>
      </ClusterText>
    </ClusterHeading>
  )

  return (
    <FloatDialog
      id="cluster"
      title={clusterTitle}
      open={open}
      onClose={onClose}
      anchorEl={anchorEl}
    >
      <K8sClusterProperties clusterStatus={clusterConnection.status} />
    </FloatDialog>
  )
}
