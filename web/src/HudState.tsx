import {
  ShowFatalErrorModal,
  SnapshotHighlight,
  SocketState,
  ShowErrorModal,
} from "./types"
import LogStore from "./LogStore"

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
