import React, { useCallback } from "react"
import styled from "styled-components"
import { ButtonSet } from "./ApiButton"
import { ReactComponent as CloseSvg } from "./assets/svg/close.svg"
import { ReactComponent as LinkSvg } from "./assets/svg/link.svg"
import { InstrumentedButton } from "./instrumentedComponents"
import { displayURL, resolveURL } from "./links"
import { useLogAlertIndex } from "./LogStore"
import {
  CopyButton,
  DisableButtonSection,
  OverviewWidgets,
} from "./OverviewActionBar"
import { uiResourceToCell } from "./OverviewTable"
import { OverviewTableBuildButton } from "./OverviewTableBuildButton"
import OverviewTableStatus from "./OverviewTableStatus"
import OverviewTableTriggerModeToggle from "./OverviewTableTriggerModeToggle"
import {
  Color,
  ColorAlpha,
  ColorRGBA,
  Font,
  FontSize,
  SizeUnit,
} from "./style-helpers"
import { startBuild } from "./trigger"
import { ResourceName, ResourceStatus, UIButton, UIResource } from "./types"

type OverviewResourceHeaderProps = {
  // Shown even when the resource doesn't exist (e.g., a stale link).
  name: string
  resource?: UIResource
  buttons?: ButtonSet
  allButtons?: UIButton[]
  onClose: () => void
}

const HeaderRoot = styled.header`
  padding: ${SizeUnit(0.35)} ${SizeUnit(0.5)};
  background-color: ${ColorRGBA(Color.blue, ColorAlpha.almostTransparent)};
  border-bottom: 1px solid ${Color.gray40};
  font-family: ${Font.monospace};
  color: ${Color.gray70};
`

// Line 1: name, status, and the resource's main actions.
// When space is tight, the status wraps under the name; the actions stay put.
const TitleRow = styled.div`
  display: flex;
  align-items: center;
  gap: ${SizeUnit(0.5)};
`

const TitleAndStatus = styled.div`
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  flex: 1 1 auto;
  min-width: 0;
  gap: ${SizeUnit(0.15)} ${SizeUnit(0.5)};
`

const Title = styled.h2`
  margin: 0;
  font-size: ${FontSize.default};
  font-weight: normal;
  color: ${Color.offWhite};
  overflow-wrap: anywhere;

  &.has-error {
    color: ${Color.red};
  }
`

const StatusGroup = styled.div`
  display: flex;
  align-items: center;
  gap: ${SizeUnit(0.5)};

  /* The table stacks these two statuses; here they sit side by side */
  & > a {
    width: auto;
  }
  && > a + a {
    margin-top: 0;
  }
`

const Actions = styled.div`
  display: flex;
  align-items: center;
  gap: ${SizeUnit(0.25)};
  flex-shrink: 0;
  margin-left: auto;
`

const CloseButton = styled(InstrumentedButton)`
  min-width: 0;
  padding: ${SizeUnit(0.25)};

  svg {
    fill: ${Color.gray60};
    width: 16px;
    height: 16px;
  }

  &:hover svg {
    fill: ${Color.offWhite};
  }
`

// Line 2: small labeled facts about the resource
const Fields = styled.dl`
  display: flex;
  flex-wrap: wrap;
  align-items: flex-start;
  gap: ${SizeUnit(0.25)} ${SizeUnit(1)};
  margin: ${SizeUnit(0.25)} 0 0 0;
`

const Field = styled.div`
  display: flex;
  flex-direction: column;
  min-width: 0;
  max-width: 100%;
`

const FieldLabel = styled.dt`
  font-size: ${FontSize.smallester};
  text-transform: uppercase;
  letter-spacing: 0.08em;
  color: ${Color.gray60};
  margin-bottom: ${SizeUnit(0.1)};
`

const FieldValue = styled.dd`
  margin: 0;
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: ${SizeUnit(0.1)} ${SizeUnit(0.35)};
  min-height: 28px;
  font-size: ${FontSize.smallest};
  color: ${Color.offWhite};
`

const Endpoint = styled.a`
  display: flex;
  align-items: center;
  max-width: 260px;
  color: ${Color.offWhite};

  svg {
    fill: ${Color.gray60};
    flex-shrink: 0;
    margin-right: ${SizeUnit(0.15)};
  }

  span {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
`

export default function OverviewResourceHeader(
  props: OverviewResourceHeaderProps
) {
  const { name, resource, buttons, allButtons, onClose } = props
  const alertIndex = useLogAlertIndex()
  const onStartBuild = useCallback(() => startBuild(name), [name])

  const close = (
    <CloseButton
      onClick={onClose}
      aria-label="Close details (Esc)"
      title="Close details (Esc)"
    >
      <CloseSvg role="presentation" />
    </CloseButton>
  )

  // The combined logs, or a stale link to a resource that doesn't exist:
  // just the title and close.
  if (!resource) {
    let title = name
    if (name === ResourceName.all) {
      title = "All Resources"
    } else if (name === ResourceName.starred) {
      title = "Starred Resources"
    }
    return (
      <HeaderRoot aria-label={`${title} details`}>
        <TitleRow>
          <Title>{title}</Title>
          <Actions>{close}</Actions>
        </TitleRow>
      </HeaderRoot>
    )
  }

  // Reuse the table's view of this resource, so the header and the
  // table row always agree.
  const cell = uiResourceToCell(resource, allButtons, alertIndex)
  const status = cell.statusLine
  const isDisabled = status.runtimeStatus === ResourceStatus.Disabled
  const isTiltfile = name === ResourceName.tiltfile
  const hasError =
    status.buildStatus === ResourceStatus.Unhealthy ||
    status.runtimeStatus === ResourceStatus.Unhealthy

  // Only show fields that have something in them.
  const fields: { label: string; value: React.ReactNode }[] = []
  if (!isDisabled && cell.endpoints.length) {
    fields.push({
      label: "Endpoints",
      value: cell.endpoints.map((ep) => {
        const url = resolveURL(ep.url || "")
        const text = ep.name || displayURL(url)
        // We use the url as the target, so that clicking the link re-uses the tab.
        return (
          <Endpoint href={url} target={url} key={url} title={url}>
            <LinkSvg role="presentation" />
            <span>{text}</span>
          </Endpoint>
        )
      }),
    })
  }
  if (!isDisabled && cell.podId) {
    fields.push({ label: "Pod ID", value: <CopyButton podId={cell.podId} /> })
  }
  if (cell.resourceTypeLabel) {
    fields.push({ label: "Type", value: cell.resourceTypeLabel })
  }
  if (!isDisabled && !isTiltfile) {
    fields.push({
      label: "Mode",
      value: (
        <OverviewTableTriggerModeToggle
          resourceName={name}
          triggerMode={cell.mode}
        />
      ),
    })
  }
  if (!isDisabled && buttons?.default.length) {
    fields.push({
      label: "Widgets",
      value: <OverviewWidgets buttons={buttons.default} />,
    })
  }

  return (
    <HeaderRoot aria-label={`${name} details`}>
      <TitleRow>
        <TitleAndStatus>
          <Title className={hasError ? "has-error" : ""}>{name}</Title>
          <StatusGroup>
            {!isDisabled && (
              <OverviewTableStatus
                status={status.buildStatus}
                lastBuildDur={status.lastBuildDur}
                isBuild={true}
                resourceName={name}
                hold={status.hold}
              />
            )}
            <OverviewTableStatus
              status={status.runtimeStatus}
              resourceName={name}
            />
          </StatusGroup>
        </TitleAndStatus>
        <Actions>
          {!isDisabled && (
            <OverviewTableBuildButton
              hasPendingChanges={cell.trigger.hasPendingChanges}
              hasBuilt={cell.trigger.hasBuilt}
              isBuilding={cell.trigger.isBuilding}
              triggerMode={cell.mode}
              isQueued={cell.trigger.isQueued}
              onStartBuild={onStartBuild}
              stopBuildButton={cell.buttons.stopBuild}
            />
          )}
          <DisableButtonSection button={buttons?.toggleDisable} />
          {close}
        </Actions>
      </TitleRow>
      {fields.length > 0 && (
        <Fields>
          {fields.map((f) => (
            <Field key={f.label}>
              <FieldLabel>{f.label}</FieldLabel>
              <FieldValue>{f.value}</FieldValue>
            </Field>
          ))}
        </Fields>
      )}
    </HeaderRoot>
  )
}
