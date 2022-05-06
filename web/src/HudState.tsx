import LogStore from "./LogStore"
import {
  ShowErrorModal,
  ShowFatalErrorModal,
  SnapshotHighlight,
  SocketState,
} from "./types"

type HudState = {
  view: Proto.webviewView
  snapshotLink: string
  showSnapshotModal: boolean
  showCopySuccess: boolean
  showFatalErrorModal: ShowFatalErrorModal
  error: string | undefined
  showErrorModal: ShowErrorModal
  snapshotHighlight: SnapshotHighlight | undefined
  snapshotDialogAnchor: HTMLElement | null
  socketState: SocketState
  logStore?: LogStore
}

export default HudState
