import { ShowFatalErrorModal, SnapshotHighlight, SocketState } from "./types"
import LogStore from "./LogStore"

type HudState = {
  view: Proto.webviewView
  isSidebarClosed: boolean
  snapshotLink: string
  showSnapshotModal: boolean
  showFatalErrorModal: ShowFatalErrorModal
  snapshotHighlight: SnapshotHighlight | undefined
  socketState: SocketState
  logStore?: LogStore
}
export default HudState
