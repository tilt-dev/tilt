import React from "react"
import styled from "styled-components"
import FloatDialog from "./FloatDialog"

type props = {
  open: boolean
  onClose: () => void
  anchorEl: Element | null
}

let ShortcutRow = styled.div`
  display: flex;
  flex-direction: row;
  font-size: 16px;
  align-items: center;
`

let ShortcutKey = styled.div`
  text-align: right;
  flex-grow: 1;
  margin-right: 24px;
`

let ShortcutBox = styled.div`
  display: inline-block;
  background: rgba(204, 218, 222, 0.4);
  border-radius: 2px;
  padding: 0px 6px;
  margin: 4px 0;
`

let ShortcutLabel = styled.div`
  text-align: left;
`

function Shortcut(props: React.PropsWithChildren<{ label: string }>) {
  return (
    <ShortcutRow>
      <ShortcutLabel>{props.label}</ShortcutLabel>
      <ShortcutKey>{props.children}</ShortcutKey>
    </ShortcutRow>
  )
}

function cmdOrCtrlShortcut(key: string) {
  // OS detection is inherently fragile on the web; thankfully, we really only
  // care about macOS vs "everything else" and as of macOS 11 (Big Sur), this
  // works reliably
  const isMac = navigator.platform.indexOf("Mac") != -1

  if (isMac) {
    return (
      <>
        <ShortcutBox>&#8984;</ShortcutBox> + <ShortcutBox>{key}</ShortcutBox>
      </>
    )
  }
  return (
    <>
      <ShortcutBox>Ctrl</ShortcutBox> + <ShortcutBox>{key}</ShortcutBox>
    </>
  )
}

export default function KeyboardShortcutsDialog(props: props) {
  return (
    <FloatDialog id="shortcuts" title="Keyboard Shortcuts" {...props}>
      <Shortcut label="Navigate Resource">
        <ShortcutBox>j</ShortcutBox> or <ShortcutBox>k</ShortcutBox>
      </Shortcut>
      <Shortcut label="Trigger rebuild for a resource">
        <ShortcutBox>r</ShortcutBox>
      </Shortcut>
      <Shortcut label="Open Endpoint">
        <ShortcutBox>Shift</ShortcutBox> + <ShortcutBox>1</ShortcutBox>,{" "}
        <ShortcutBox>2</ShortcutBox> …
      </Shortcut>
      <Shortcut label="Select Resource Row">
        <ShortcutBox>x</ShortcutBox>
      </Shortcut>
      <Shortcut label="Clear Logs">{cmdOrCtrlShortcut("Backspace")}</Shortcut>
      <Shortcut label="Make Snapshot">
        <ShortcutBox>s</ShortcutBox>
      </Shortcut>
      <Shortcut label="Focus Resource Filter">
        <ShortcutBox>/</ShortcutBox>
      </Shortcut>
      <Shortcut label="Keyboard Shortcuts">
        <ShortcutBox>?</ShortcutBox>
      </Shortcut>
    </FloatDialog>
  )
}
