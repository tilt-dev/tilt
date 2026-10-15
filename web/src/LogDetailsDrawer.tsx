import { Close as CloseIcon } from "@material-ui/icons"
import moment from "moment"
import React, { useEffect, useMemo, useRef, useState } from "react"
import styled, { keyframes } from "styled-components"
import { InstrumentedButton } from "./instrumentedComponents"
import JsonView from "./JsonView"
import {
  flattenObject,
  formatValueCompact,
  JSONValue,
  parseStructured,
  StructuredKind,
} from "./logparse"
import { formatLogTime, isBuildSpanId, stripAnsiCodes } from "./logs"
import LogStore, { LogLineDetails } from "./LogStore"
import {
  AnimDuration,
  Color,
  ColorRGBA,
  Font,
  FontSize,
  mixinResetButtonStyle,
  SizeUnit,
} from "./style-helpers"
import Anser from "./third-party/anser/index.js"

const anser = new Anser()

const slideIn = keyframes`
  from { transform: translateX(24px); opacity: 0; }
  to { transform: translateX(0); opacity: 1; }
`

const DrawerRoot = styled.aside`
  position: absolute;
  top: 0;
  right: 0;
  bottom: 0;
  z-index: 2;
  width: min(640px, 45%);
  min-width: 360px;
  max-width: 100%;
  display: flex;
  flex-direction: column;
  box-sizing: border-box;
  background-color: ${Color.gray20};
  border-left: 1px solid ${Color.gray40};
  box-shadow: -8px 0 24px ${ColorRGBA(Color.black, 0.4)};
  color: ${Color.offWhite};
  font-family: ${Font.sansSerif};
  font-size: ${FontSize.smallest};
  animation: ${slideIn} ${AnimDuration.short} ease-out;

  @media screen and (max-width: 720px) {
    width: 100%;
    min-width: 0;
  }
`

const DrawerHeader = styled.header`
  display: flex;
  align-items: center;
  gap: ${SizeUnit(0.25)};
  padding: ${SizeUnit(0.25)} ${SizeUnit(0.5)};
  border-bottom: 1px solid ${Color.gray40};
  flex-wrap: wrap;
`

const DrawerTitle = styled.h2`
  margin: 0;
  font-size: ${FontSize.small};
  font-weight: normal;
  flex-grow: 1;
`

const DrawerBody = styled.div`
  flex: 1;
  overflow-y: auto;
  padding: ${SizeUnit(0.5)};
  display: flex;
  flex-direction: column;
  gap: ${SizeUnit(0.5)};
`

const IconButton = styled(InstrumentedButton)`
  ${mixinResetButtonStyle};
  color: ${Color.gray60};
  display: flex;
  &:hover {
    color: ${Color.blue};
  }
`

const ActionButton = styled(InstrumentedButton)`
  ${mixinResetButtonStyle};
  color: ${Color.gray70};
  border: 1px solid ${Color.gray40};
  border-radius: 4px;
  padding: 2px 8px;
  transition: color ${AnimDuration.default} ease,
    border-color ${AnimDuration.default} ease;

  &:hover {
    color: ${Color.blue};
    border-color: ${Color.blue};
  }
`

const LevelPill = styled.span<{ $color: string }>`
  font-family: ${Font.monospace};
  font-weight: bold;
  padding: 1px 8px;
  border-radius: 999px;
  color: ${(p) => p.$color};
  background-color: ${(p) => ColorRGBA(p.$color, 0.15)};
  border: 1px solid ${(p) => ColorRGBA(p.$color, 0.4)};
`

const Section = styled.section`
  display: flex;
  flex-direction: column;
  gap: ${SizeUnit(0.125)};
`

const SectionTitle = styled.h3`
  margin: 0;
  font-size: ${FontSize.smallest};
  font-weight: normal;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: ${Color.gray60};
`

const CodeBox = styled.div`
  font-family: ${Font.monospace};
  background-color: ${Color.gray10};
  border: 1px solid ${Color.gray30};
  border-radius: 4px;
  padding: ${SizeUnit(0.25)};
  max-height: 50vh;
  overflow: auto;
`

const RawText = styled.code`
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  font-family: ${Font.monospace};
`

const MetaGrid = styled.dl`
  display: grid;
  grid-template-columns: max-content 1fr;
  gap: 4px ${SizeUnit(0.5)};
  margin: 0;

  dt {
    color: ${Color.gray60};
  }
  dd {
    margin: 0;
    overflow-wrap: anywhere;
  }
`

const FieldsTable = styled.table`
  border-collapse: collapse;
  width: 100%;
  font-family: ${Font.monospace};

  td {
    padding: 3px 6px;
    border-bottom: 1px solid ${Color.gray30};
    vertical-align: top;
    overflow-wrap: anywhere;
  }
  td.field-key {
    color: ${Color.blueLight};
    width: 35%;
  }
  td.field-copy {
    width: 1px;
    white-space: nowrap;
  }
  tr:hover td {
    background-color: ${ColorRGBA(Color.white, 0.04)};
  }
`

const EmptyState = styled.div`
  color: ${Color.gray60};
  padding: ${SizeUnit(0.5)};
`

function levelColor(level: string): string {
  switch (level) {
    case "ERROR":
      return Color.red
    case "WARN":
      return Color.yellow
    default:
      return Color.blue
  }
}

// Copies text to the clipboard. Returns whether it succeeded.
//
// navigator.clipboard only exists in secure contexts (https or localhost),
// so fall back to execCommand when Tilt is accessed over plain http
// (e.g., with --host 0.0.0.0).
export async function copyText(text: string): Promise<boolean> {
  if (navigator.clipboard?.writeText) {
    try {
      await navigator.clipboard.writeText(text)
      return true
    } catch (e) {
      // fall through to the fallback
    }
  }

  let textarea = document.createElement("textarea")
  textarea.value = text
  textarea.setAttribute("readonly", "")
  textarea.style.position = "fixed"
  textarea.style.top = "-1000px"
  textarea.style.opacity = "0"
  document.body.appendChild(textarea)
  let activeEl = document.activeElement as HTMLElement | null
  try {
    textarea.focus()
    textarea.select()
    return !!document.execCommand?.("copy")
  } catch (e) {
    return false
  } finally {
    document.body.removeChild(textarea)
    activeEl?.focus?.()
  }
}

type CopyStatus = { id: string; ok: boolean } | null

// Shows whether the copy worked for a moment after copying something.
function useCopy(): [CopyStatus, (id: string, text: string) => void] {
  let [status, setStatus] = useState<CopyStatus>(null)
  let timer = useRef<ReturnType<typeof setTimeout>>()
  let mounted = useRef(true)
  useEffect(
    () => () => {
      mounted.current = false
      clearTimeout(timer.current)
    },
    []
  )

  let copy = async (id: string, text: string) => {
    let ok = await copyText(text)
    if (!mounted.current) {
      return
    }
    setStatus({ id, ok })
    clearTimeout(timer.current)
    timer.current = setTimeout(() => setStatus(null), 1500)
  }
  return [status, copy]
}

function copyLabel(status: CopyStatus, id: string, label: string) {
  if (status?.id !== id) {
    return label
  }
  return status.ok ? "Copied!" : "Copy failed"
}

// Re-renders when the log store changes, so that progress lines that update
// in place stay current.
function useLineDetails(
  logStore: LogStore,
  index: number
): LogLineDetails | null {
  let [, setVersion] = useState(0)
  useEffect(() => {
    let onUpdate = () => setVersion((v) => v + 1)
    logStore.addUpdateListener(onUpdate)
    return () => logStore.removeUpdateListener(onUpdate)
  }, [logStore])
  return logStore.lineDetails(index)
}

type LogDetailsDrawerProps = {
  logStore: LogStore
  selectedLineIndex: number
  onClose: () => void
}

export default function LogDetailsDrawer(props: LogDetailsDrawerProps) {
  let { logStore, selectedLineIndex, onClose } = props
  let line = useLineDetails(logStore, selectedLineIndex)
  let [copied, copy] = useCopy()

  let text = line?.text ?? ""
  let parsed = useMemo(() => parseStructured(text), [text])
  let rawHtml = useMemo(
    () =>
      anser.linkify(
        anser.ansiToHtml(anser.escapeForHtml(text), { use_classes: false })
      ),
    [text]
  )

  let header = (
    <DrawerHeader>
      <DrawerTitle>Log details</DrawerTitle>
      <IconButton aria-label="Close log details" onClick={onClose}>
        <CloseIcon fontSize="small" />
      </IconButton>
    </DrawerHeader>
  )

  if (!line) {
    return (
      <DrawerRoot aria-label="Log details">
        {header}
        <EmptyState>This log line is no longer available.</EmptyState>
      </DrawerRoot>
    )
  }

  let structured = parsed.kind !== StructuredKind.Text ? parsed : null
  let fields = structured ? flattenObject(structured.data) : []
  let plainText = stripAnsiCodes(line.text)
  let tiltFields = Object.entries(line.fields).filter(
    ([key]) => key !== "buildEvent"
  )
  let time = line.time ? moment(new Date(line.time)) : null
  let timeValid = !!time?.isValid()

  return (
    <DrawerRoot aria-label="Log details">
      {header}
      <DrawerBody>
        <Section>
          <MetaGrid>
            <dt>Level</dt>
            <dd>
              <LevelPill $color={levelColor(line.level)}>
                {line.level}
              </LevelPill>
            </dd>
            <dt>Time</dt>
            <dd>
              {timeValid ? (
                <>
                  {time!.format("YYYY-MM-DD")} {formatLogTime(line.time)}{" "}
                  <span style={{ color: Color.gray60 }}>
                    ({time!.fromNow()})
                  </span>
                </>
              ) : (
                "unknown"
              )}
            </dd>
            <dt>Resource</dt>
            <dd>{line.manifestName || "(global)"}</dd>
            <dt>Source</dt>
            <dd>{isBuildSpanId(line.spanId) ? "Build" : "Runtime"}</dd>
            {structured && (
              <>
                <dt>Format</dt>
                <dd>
                  {structured.kind === StructuredKind.JSON ? "JSON" : "logfmt"}
                </dd>
              </>
            )}
            {tiltFields.map(([key, value]) => (
              <React.Fragment key={key}>
                <dt>{key}</dt>
                <dd>{value}</dd>
              </React.Fragment>
            ))}
          </MetaGrid>
        </Section>

        <Section>
          <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
            <ActionButton onClick={() => copy("line", plainText)}>
              {copyLabel(copied, "line", "Copy line")}
            </ActionButton>
            {structured && (
              <ActionButton
                onClick={() =>
                  copy("json", JSON.stringify(structured!.data, null, 2))
                }
              >
                {copyLabel(copied, "json", "Copy JSON")}
              </ActionButton>
            )}
            <ActionButton onClick={() => copy("link", window.location.href)}>
              {copyLabel(copied, "link", "Copy link")}
            </ActionButton>
          </div>
        </Section>

        {structured && (
          <Section>
            <SectionTitle>JSON</SectionTitle>
            <CodeBox>
              <JsonView value={structured.data} />
            </CodeBox>
          </Section>
        )}

        {structured && (
          <Section>
            <SectionTitle>Fields</SectionTitle>
            <FieldsTable aria-label="Log fields">
              <tbody>
                {structured.prefix && (
                  <tr>
                    <td className="field-key">(prefix)</td>
                    <td>{structured.prefix}</td>
                    <td className="field-copy" />
                  </tr>
                )}
                {fields.map(([key, value]) => (
                  <FieldRow
                    key={key}
                    fieldKey={key}
                    value={value}
                    status={copied?.id === `field:${key}` ? copied.ok : null}
                    onCopy={() =>
                      copy(`field:${key}`, formatValueCompact(value))
                    }
                  />
                ))}
              </tbody>
            </FieldsTable>
          </Section>
        )}

        <Section>
          <SectionTitle>Raw</SectionTitle>
          <CodeBox>
            <RawText dangerouslySetInnerHTML={{ __html: rawHtml }} />
          </CodeBox>
        </Section>
      </DrawerBody>
    </DrawerRoot>
  )
}

function FieldRow(props: {
  fieldKey: string
  value: JSONValue
  // null if the field hasn't just been copied
  status: boolean | null
  onCopy: () => void
}) {
  return (
    <tr>
      <td className="field-key">{props.fieldKey}</td>
      <td>{formatValueCompact(props.value)}</td>
      <td className="field-copy">
        <IconButton
          aria-label={`Copy ${props.fieldKey}`}
          onClick={props.onCopy}
        >
          {props.status === null ? "copy" : props.status ? "✓" : "✗"}
        </IconButton>
      </td>
    </tr>
  )
}
