import React from "react"
import { MemoryRouter } from "react-router"
import styled from "styled-components"
import LogDetailsDrawer from "./LogDetailsDrawer"
import { EMPTY_FILTER_TERM, FilterLevel, FilterSource } from "./logfilters"
import { useSelectedLogLine } from "./logselection"
import LogStore, { LogStoreProvider } from "./LogStore"
import OverviewLogPane from "./OverviewLogPane"
import { appendLines } from "./testlogs"

export default {
  title: "New UI/Overview/LogDetailsDrawer",
}

const defaultFilter = {
  source: FilterSource.all,
  level: FilterLevel.all,
  term: EMPTY_FILTER_TERM,
}

const Container = styled.div`
  position: relative;
  display: flex;
  margin: -1rem;
  height: 80vh;
  width: 90vw;
  border: thin solid #ccc;
`

function mixedLogStore() {
  let logStore = new LogStore()
  appendLines(
    logStore,
    "api",
    "Starting api server...\n",
    `{"level":"info","ts":"2024-05-01T12:00:00Z","msg":"listening","addr":":8080"}\n`,
    `{"level":"debug","msg":"request","method":"GET","path":"/healthz","status":200,"duration_ms":0.42}\n`,
    `time=2024-05-01T12:00:01Z level=warn msg="slow query" table=users ms=1250\n`,
    `INFO\tgrpc/server.go:42\tfinished call\t{"grpc.code":"OK","grpc.method":"Get","peer":{"addr":"10.0.0.3"}}\n`,
    {
      text: `{"level":"error","msg":"failed to connect","error":"dial tcp 10.0.0.5:5432: connection refused","retry":{"attempt":3,"backoff":"2s"},"tags":["db","postgres"]}\n`,
      // appendLines copies extra keys onto the segment
      level: "ERROR",
    } as any,
    "\x1b[32m✓\x1b[0m ready in 1.2s, see http://localhost:8080\n"
  )
  return logStore
}

function Harness(props: { logStore: LogStore }) {
  let [selected, setSelected] = useSelectedLogLine()
  return (
    <LogStoreProvider value={props.logStore}>
      <Container>
        <OverviewLogPane manifestName="api" filterSet={defaultFilter} />
        {selected !== null && (
          <LogDetailsDrawer
            logStore={props.logStore}
            selectedLineIndex={selected}
            onClose={() => setSelected(null)}
          />
        )}
      </Container>
    </LogStoreProvider>
  )
}

export const MixedLines = () => (
  <MemoryRouter initialEntries={["/r/api/overview"]}>
    <Harness logStore={mixedLogStore()} />
  </MemoryRouter>
)

export const JsonLineSelected = () => (
  <MemoryRouter initialEntries={["/r/api/overview?line=5"]}>
    <Harness logStore={mixedLogStore()} />
  </MemoryRouter>
)

export const PlainLineSelected = () => (
  <MemoryRouter initialEntries={["/r/api/overview?line=6"]}>
    <Harness logStore={mixedLogStore()} />
  </MemoryRouter>
)
