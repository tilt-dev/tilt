import Divider from "@material-ui/core/Divider"
import Menu from "@material-ui/core/Menu"
import MenuItem from "@material-ui/core/MenuItem"
import React, { KeyboardEvent } from "react"
import styled from "styled-components"
import { ReactComponent as GithubSvg } from "./assets/svg/github.svg"
import { ReactComponent as SlackSvg } from "./assets/svg/slack.svg"
import { ReactComponent as SnapshotSvg } from "./assets/svg/snapshot.svg"
import { HelpSearchBar } from "./HelpSearchBar"
import { useMenuItemStyles } from "./menuStyles"
import { SizeUnit } from "./style-helpers"
import { useTheme } from "./ThemeContext"

type props = {
  open: boolean
  onClose: () => void
  anchorEl: Element | null
  // Omit to hide the snapshot item.
  onSnapshot?: () => void
  onKeyboardShortcuts: () => void
}

// Padded like a MenuItem so the search box lines up with the items below.
const SearchItem = styled.div`
  padding: 6px ${SizeUnit(0.5)};
`

const SearchBar = styled(HelpSearchBar)`
  width: 100%;
  margin: 0;
`

const ItemIcon = styled.span`
  display: inline-flex;
  justify-content: center;
  align-items: center;
  width: 20px;
  height: 20px;
  margin-right: ${SizeUnit(0.25)};
`

// Keep typing in the search box from triggering the menu's
// arrow-key and type-to-select navigation.
function stopMenuNavigation(e: KeyboardEvent) {
  if (e.key !== "Escape" && e.key !== "Tab") {
    e.stopPropagation()
  }
}

// The gear menu in the global nav: docs search, snapshots, theme,
// community links, and keyboard shortcuts.
export default function SettingsDialog(props: props) {
  const classes = useMenuItemStyles()
  const { theme, toggleTheme } = useTheme()
  const nextTheme = theme === "dark" ? "light" : "dark"

  return (
    <Menu
      id="settings"
      anchorEl={props.anchorEl}
      open={props.open}
      onClose={props.onClose}
      disableScrollLock={true}
      anchorOrigin={{ vertical: "bottom", horizontal: "right" }}
      transformOrigin={{ vertical: "top", horizontal: "right" }}
      getContentAnchorEl={null}
    >
      <SearchItem onKeyDown={stopMenuNavigation}>
        <SearchBar />
      </SearchItem>
      <Divider />
      {props.onSnapshot && (
        <MenuItem classes={classes} onClick={props.onSnapshot}>
          <ItemIcon>
            <SnapshotSvg height="16" role="presentation" />
          </ItemIcon>
          Create a snapshot
        </MenuItem>
      )}
      <MenuItem classes={classes} onClick={toggleTheme}>
        <ItemIcon role="presentation">
          {nextTheme === "light" ? "\u2600" : "\u263D"}
        </ItemIcon>
        Switch to {nextTheme} theme
      </MenuItem>
      <Divider />
      <MenuItem
        classes={classes}
        component="a"
        href="http://slack.k8s.io/"
        target="_blank"
        rel="noopener noreferrer"
        onClick={props.onClose}
      >
        <ItemIcon>
          <SlackSvg role="presentation" />
        </ItemIcon>
        Connect in #tilt on Kubernetes Slack
      </MenuItem>
      <MenuItem
        classes={classes}
        component="a"
        href="https://github.com/tilt-dev/tilt/issues/new/choose"
        target="_blank"
        rel="noopener noreferrer"
        onClick={props.onClose}
      >
        <ItemIcon>
          <GithubSvg role="presentation" />
        </ItemIcon>
        File an Issue
      </MenuItem>
      <MenuItem classes={classes} onClick={props.onKeyboardShortcuts}>
        <ItemIcon role="presentation">?</ItemIcon>
        Keyboard shortcuts
      </MenuItem>
    </Menu>
  )
}
