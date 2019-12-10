import {
  WebView,
  ShowFatalErrorModal,
  SnapshotHighlight,
  SocketState,
} from "./types"
import LogStore from "./LogStore"

type HudState = {
  view: WebView
  isSidebarClosed: boolean
  snapshotLink: string
  showSnapshotModal: boolean
  showFatalErrorModal: ShowFatalErrorModal
  snapshotHighlight: SnapshotHighlight | undefined
  socketState: SocketState
  logStore?: LogStore
}
export default HudState
