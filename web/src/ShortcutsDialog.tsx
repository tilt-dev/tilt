import React from "react"
import styled from "styled-components"
import FloatDialog from "./FloatDialog"

type props = {
  isOpen: boolean
  onRequestClose: () => void
  style?: any
}

let ShortcutRoot = styled.div`
  display: flex;
  flex-direction: row;
  font-size: 16px;
`

let ShortcutKey = styled.div`
  text-align: right;
  flex: 2;
  margin-right: 24px;
`

let ShortcutLabel = styled.div`
  text-align: left;
  flex: 5;
`

function Shortcut(props: { keys: string; label: string }) {
  return (
    <ShortcutRoot>
      <ShortcutKey>{props.keys}</ShortcutKey>
      <ShortcutLabel>{props.label}</ShortcutLabel>
    </ShortcutRoot>
  )
}

export default function ShortcutsDialog(props: props) {
  return (
    <FloatDialog title="Keyboard Shortcuts" {...props}>
      <Shortcut keys="j, k" label="Navigate Resource" />
      <Shortcut keys="Shift+1, 2..." label="Open Endpoint" />
      <Shortcut keys="1" label="View Logs" />
      <Shortcut keys="2" label="View Alerts" />
      <Shortcut keys="s" label="Create Snapshot" />
      <Shortcut keys="?" label="Help" />
    </FloatDialog>
  )
}
