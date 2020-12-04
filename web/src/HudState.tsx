import LogStore from "./LogStore"
import {
  ShowErrorModal,
  ShowFatalErrorModal,
  SnapshotHighlight,
  SocketState,
} from "./types"

type HudState = {
  view: Proto.webviewView
  isSidebarClosed: boolean
  snapshotLink: string
  showSnapshotModal: boolean
  showFatalErrorModal: ShowFatalErrorModal
  error: string | undefined
  showErrorModal: ShowErrorModal
  snapshotHighlight: SnapshotHighlight | undefined
  socketState: SocketState
  logStore?: LogStore
}
export default HudState
