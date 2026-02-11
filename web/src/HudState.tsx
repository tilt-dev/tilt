import LogStore from "./LogStore"
import {
  ShowErrorModal,
  ShowFatalErrorModal,
  SnapshotHighlight,
  SocketState,
} from "./types"
import type { View } from "./webview"

type HudState = {
  view: View
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
