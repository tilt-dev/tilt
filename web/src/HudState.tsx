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
  snapshotHighlight: SnapshotHighlight | undefined
  snapshotDialogAnchor: HTMLElement | null
  snapshotStartTime: string | undefined
  showSnapshotModal: boolean
  showCopySuccess: boolean
  showFatalErrorModal: ShowFatalErrorModal
  error: string | undefined
  showErrorModal: ShowErrorModal
  socketState: SocketState
  logStore?: LogStore
}

export default HudState
