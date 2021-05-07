import { showUpdate } from "./UpdateDialog"

it("compares versions correctly", () => {
  function v(current: string, suggested: string): Proto.webviewView {
    return {
      uiSession: {
        status: {
          runningTiltBuild: { version: current },
          suggestedTiltVersion: suggested,
          versionSettings: { checkUpdates: true },
        },
      },
    }
  }

  expect(showUpdate(v("1.2.3", "1.2.3"))).toEqual(false)
  expect(showUpdate(v("1.2.3", "1.2.2"))).toEqual(false)
  expect(showUpdate(v("1.2.3", "1.2.4"))).toEqual(true)
  expect(showUpdate(v("1.2.3", "1.1.3"))).toEqual(false)
  expect(showUpdate(v("1.2.3", "1.3.3"))).toEqual(true)
  expect(showUpdate(v("1.2.3", "0.2.3"))).toEqual(false)
  expect(showUpdate(v("1.2.3", "2.2.3"))).toEqual(true)
  expect(showUpdate(v("1.2.3", "2.0.0"))).toEqual(true)
  expect(showUpdate(v("1.2.3", "1.1.4"))).toEqual(false)
  expect(showUpdate(v("1.2.3", "1.4.1"))).toEqual(true)
})
