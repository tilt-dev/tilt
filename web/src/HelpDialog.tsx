import React from "react"
import styled from "styled-components"
import { ReactComponent as GithubSvg } from "./assets/svg/github.svg"
import { ReactComponent as SlackSvg } from "./assets/svg/slack.svg"
import FloatDialog, { HR } from "./FloatDialog"
import { HelpSearchBar } from "./HelpSearchBar"
import { AnimDuration, Color } from "./style-helpers"

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

let HelpLink = styled.a`
  display: flex;
  align-items: center;
  text-decoration: none;
  transform: color ${AnimDuration.default} ease;

  &:hover {
    color: ${Color.gray50};
  }
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

export default function HelpDialog(props: props) {
  return (
    <FloatDialog id="shortcuts" title="Help" {...props}>
      <ShortcutRow>
        <HelpSearchBar />
      </ShortcutRow>
      <HR />
      <ShortcutRow style={{ marginBottom: "24px" }}>
        <HelpLink
          href="http://slack.k8s.io/"
          target="_blank"
          rel="noopener noreferrer"
        >
          <SlackSvg style={{ marginRight: "8px" }} />
          Connect in #tilt on Kubernetes Slack
        </HelpLink>
      </ShortcutRow>
      <ShortcutRow style={{ marginBottom: "8px" }}>
        <HelpLink
          href="https://github.com/tilt-dev/tilt/issues/new/choose"
          target="_blank"
          rel="noopener noreferrer"
        >
          <GithubSvg style={{ marginRight: "8px" }} />
          File an Issue
        </HelpLink>
        <HR />
      </ShortcutRow>
      <HR />
      <ShortcutRow
        style={{
          textDecoration: "underline",
          fontSize: "18px",
          marginBottom: "8px",
        }}
      >
        Keyboard shortcuts
      </ShortcutRow>
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
      <Shortcut label="Help">
        <ShortcutBox>?</ShortcutBox>
      </Shortcut>
    </FloatDialog>
  )
}
