import { fireEvent, render, screen, waitFor } from "@testing-library/react"
import { MemoryRouter, useLocation } from "react-router"
import LogDetailsDrawer from "./LogDetailsDrawer"
import { EMPTY_FILTER_TERM, FilterLevel, FilterSource } from "./logfilters"
import { useSelectedLogLine } from "./logselection"
import LogStore, { LogStoreProvider } from "./LogStore"
import OverviewLogPane from "./OverviewLogPane"
import { SyncRafProvider } from "./raf"
import { appendLines } from "./testlogs"

const defaultFilter = {
  source: FilterSource.all,
  level: FilterLevel.all,
  term: EMPTY_FILTER_TERM,
}

let currentSearch = ""
function LocationSpy() {
  currentSearch = useLocation().search
  return null
}

// Mirrors how OverviewResourceDetails wires the pane and the drawer.
function Harness(props: { logStore: LogStore }) {
  let [selected, setSelected] = useSelectedLogLine()
  return (
    <LogStoreProvider value={props.logStore}>
      <OverviewLogPane manifestName="fe" filterSet={defaultFilter} />
      {selected !== null && (
        <LogDetailsDrawer
          logStore={props.logStore}
          selectedLineIndex={selected}
          onClose={() => setSelected(null)}
        />
      )}
      <LocationSpy />
    </LogStoreProvider>
  )
}

function renderHarness(logStore: LogStore, initialEntry = "/r/fe/overview") {
  return render(
    <MemoryRouter
      initialEntries={[initialEntry]}
      future={{ v7_startTransition: true, v7_relativeSplatPath: true }}
    >
      <SyncRafProvider>
        <Harness logStore={logStore} />
      </SyncRafProvider>
    </MemoryRouter>
  )
}

function newLogStore() {
  let logStore = new LogStore()
  appendLines(
    logStore,
    "fe",
    "plain line\n",
    `{"level":"info","msg":"request done","status":200,"user":{"id":7}}\n`,
    `time=now level=warn msg="slow query" ms=1200\n`
  )
  return logStore
}

function lineEls(container: HTMLElement) {
  return Array.from(container.querySelectorAll(".LogLine"))
}

describe("log line details", () => {
  beforeEach(() => {
    currentSearch = ""
    // jsdom doesn't implement scrolling
    Element.prototype.scrollIntoView = jest.fn()
  })

  it("renders structured lines as key=value summaries", () => {
    const { container } = renderHarness(newLogStore())
    let lines = lineEls(container)
    expect(lines).toHaveLength(3)

    expect(lines[0].querySelector(".is-structured")).toBeNull()

    let json = lines[1].querySelector(".is-structured")!
    expect(json.querySelector(".LogLine-structLevel")).toHaveTextContent("INFO")
    expect(json.querySelector(".LogLine-structMsg")).toHaveTextContent(
      "request done"
    )
    let keys = Array.from(json.querySelectorAll(".LogLine-key")).map(
      (el) => el.textContent
    )
    expect(keys).toEqual(["status", "user"])

    let logfmt = lines[2].querySelector(".is-structured")!
    expect(logfmt.querySelector(".LogLine-structMsg")).toHaveTextContent(
      "slow query"
    )
  })

  it("does not interpret html in structured values", () => {
    let logStore = new LogStore()
    appendLines(logStore, "fe", `{"msg":"<b>bold</b>","x":"<img>"}\n`)
    const { container } = renderHarness(logStore)
    expect(container.querySelector("b")).toBeNull()
    expect(container.querySelector("img")).toBeNull()
    expect(container.querySelector(".LogLine-structMsg")).toHaveTextContent(
      "<b>bold</b>"
    )
  })

  it("opens the drawer when a line is clicked", () => {
    const { container } = renderHarness(newLogStore())
    expect(screen.queryByLabelText("Log details")).toBeNull()

    fireEvent.click(lineEls(container)[1])

    expect(currentSearch).toEqual("?line=1")
    expect(lineEls(container)[1]).toHaveClass("is-highlighted")
    expect(screen.getByLabelText("Log details")).toBeInTheDocument()
    expect(screen.getByLabelText("Log fields")).toHaveTextContent("user.id7")
    expect(screen.getByLabelText("Formatted JSON")).toHaveTextContent(
      `"status": 200`
    )
  })

  it("toggles the selection when the same line is clicked again", () => {
    const { container } = renderHarness(newLogStore())
    fireEvent.click(lineEls(container)[0])
    expect(currentSearch).toEqual("?line=0")
    fireEvent.click(lineEls(container)[0])
    expect(currentSearch).toEqual("")
    expect(lineEls(container)[0]).not.toHaveClass("is-highlighted")
  })

  it("shows plain lines without a fields table", () => {
    const { container } = renderHarness(newLogStore())
    fireEvent.click(lineEls(container)[0])
    expect(screen.queryByLabelText("Log fields")).toBeNull()
    expect(screen.getByText("Raw")).toBeInTheDocument()
  })

  it("does not select a line when the user is selecting text", () => {
    const { container } = renderHarness(newLogStore())
    let spy = jest.spyOn(window, "getSelection").mockReturnValue({
      isCollapsed: false,
      toString: () => "some text",
    } as any)
    try {
      fireEvent.click(lineEls(container)[0])
    } finally {
      spy.mockRestore()
    }
    expect(currentSearch).toEqual("")
  })

  it("moves the selection with arrow keys and closes with escape", () => {
    const { container } = renderHarness(newLogStore(), "/r/fe/overview?line=0")
    expect(screen.getByLabelText("Log details")).toBeInTheDocument()
    expect(lineEls(container)[0]).toHaveClass("is-highlighted")

    fireEvent.keyDown(document.body, { key: "ArrowDown" })
    expect(currentSearch).toEqual("?line=1")
    expect(lineEls(container)[1]).toHaveClass("is-highlighted")
    expect(lineEls(container)[0]).not.toHaveClass("is-highlighted")

    fireEvent.keyDown(document.body, { key: "ArrowDown" })
    fireEvent.keyDown(document.body, { key: "ArrowDown" })
    expect(currentSearch).toEqual("?line=2")

    fireEvent.keyDown(document.body, { key: "ArrowUp" })
    expect(currentSearch).toEqual("?line=1")

    fireEvent.keyDown(document.body, { key: "Escape" })
    expect(currentSearch).toEqual("")
    expect(screen.queryByLabelText("Log details")).toBeNull()
  })

  it("closes the drawer with the close button", () => {
    renderHarness(newLogStore(), "/r/fe/overview?line=2")
    fireEvent.click(screen.getByLabelText("Close log details"))
    expect(screen.queryByLabelText("Log details")).toBeNull()
  })

  describe("copy buttons", () => {
    let originalClipboard = navigator.clipboard
    let originalExecCommand = document.execCommand
    afterEach(() => {
      Object.assign(navigator, { clipboard: originalClipboard })
      document.execCommand = originalExecCommand
    })

    it("copies the line and the json", async () => {
      let writeText = jest.fn().mockResolvedValue(undefined)
      Object.assign(navigator, { clipboard: { writeText } })
      renderHarness(newLogStore(), "/r/fe/overview?line=1")

      fireEvent.click(screen.getByText("Copy JSON"))
      expect(await screen.findByText("Copied!")).toBeInTheDocument()
      expect(JSON.parse(writeText.mock.calls[0][0])).toEqual({
        level: "info",
        msg: "request done",
        status: 200,
        user: { id: 7 },
      })

      fireEvent.click(screen.getByText("Copy line"))
      await waitFor(() => expect(writeText).toHaveBeenCalledTimes(2))
      expect(writeText.mock.calls[1][0]).toEqual(
        `{"level":"info","msg":"request done","status":200,"user":{"id":7}}`
      )
    })

    it("falls back to execCommand without the clipboard api (plain http)", async () => {
      Object.assign(navigator, { clipboard: undefined })
      let copied = ""
      document.execCommand = jest.fn(() => {
        copied = (document.activeElement as HTMLTextAreaElement).value
        return true
      })
      renderHarness(newLogStore(), "/r/fe/overview?line=0")

      fireEvent.click(screen.getByText("Copy line"))
      expect(await screen.findByText("Copied!")).toBeInTheDocument()
      expect(document.execCommand).toHaveBeenCalledWith("copy")
      expect(copied).toEqual("plain line")
      expect(document.querySelector("textarea")).toBeNull()
    })

    it("says when copying failed", async () => {
      Object.assign(navigator, {
        clipboard: { writeText: jest.fn().mockRejectedValue(new Error("no")) },
      })
      document.execCommand = jest.fn(() => false)
      renderHarness(newLogStore(), "/r/fe/overview?line=0")

      fireEvent.click(screen.getByText("Copy link"))
      expect(await screen.findByText("Copy failed")).toBeInTheDocument()
    })
  })

  it("shows a message when the selected line no longer exists", () => {
    renderHarness(newLogStore(), "/r/fe/overview?line=99")
    expect(
      screen.getByText("This log line is no longer available.")
    ).toBeInTheDocument()
  })

  it("keeps the selection when new logs arrive", async () => {
    let logStore = newLogStore()
    const { container } = renderHarness(logStore, "/r/fe/overview?line=0")
    appendLines(logStore, "fe", "another line\n")
    // LogStore notifies listeners on the next animation frame.
    await waitFor(() => expect(lineEls(container)).toHaveLength(4))
    expect(lineEls(container)[0]).toHaveClass("is-highlighted")
  })
})
