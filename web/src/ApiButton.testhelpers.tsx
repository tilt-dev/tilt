import fetchMock, { MockCall } from "fetch-mock"
import { UIButton } from "./types"

export function mockUIButtonUpdates() {
  fetchMock.mock(
    (url) => url.startsWith("/proxy/apis/tilt.dev/v1alpha1/uibuttons"),
    JSON.stringify({})
  )
}

export function cleanupMockUIButtonUpdates() {
  fetchMock.reset()
}

export function getUIButtonDataFromCall(call: MockCall): UIButton | undefined {
  if (call.length < 2) {
    return
  }

  const callRequest = call[1]

  if (!callRequest?.body) {
    return
  }

  const buttonData = JSON.parse(String(callRequest?.body))

  return buttonData as UIButton
}
