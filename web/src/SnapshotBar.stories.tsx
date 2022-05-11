import React from "react"
import { MemoryRouter } from "react-router"
import Features, { FeaturesTestProvider, Flag } from "./feature"
import OverviewTablePane from "./OverviewTablePane"
import PathBuilder, { PathBuilderProvider } from "./PathBuilder"
import { ResourceGroupsContextProvider } from "./ResourceGroupsContext"
import { ResourceListOptionsProvider } from "./ResourceListOptionsContext"
import { ResourceSelectionProvider } from "./ResourceSelectionContext"
import { TiltSnackbarProvider } from "./Snackbar"
import { SnapshotActionProvider } from "./snapshot"
import { nResourceView } from "./testdata"
import { Snapshot } from "./types"

const snapshotPb = PathBuilder.forTesting("localhost", "/snapshot/local")
const FAKE_SNAPSHOT: Snapshot = {
  view: nResourceView(10),
  createdAt: new Date().toISOString(),
}

export default {
  title: "New UI/Shared/SnapshotBar",
  decorators: [
    (Story: any) => {
      const { view, createdAt } = FAKE_SNAPSHOT
      const features = new Features({ [Flag.DisableResources]: true })
      return (
        <MemoryRouter initialEntries={["/snapshot/local"]}>
          <PathBuilderProvider value={snapshotPb}>
            <TiltSnackbarProvider>
              <FeaturesTestProvider value={features}>
                <SnapshotActionProvider
                  openModal={() => {}}
                  currentSnapshotTime={{
                    createdAt,
                    tiltUpTime: view?.tiltStartTime,
                  }}
                >
                  <ResourceGroupsContextProvider>
                    <ResourceListOptionsProvider>
                      <ResourceSelectionProvider>
                        <div style={{ margin: "-1rem" }}>
                          <Story />
                        </div>
                      </ResourceSelectionProvider>
                    </ResourceListOptionsProvider>
                  </ResourceGroupsContextProvider>
                </SnapshotActionProvider>
              </FeaturesTestProvider>
            </TiltSnackbarProvider>
          </PathBuilderProvider>
        </MemoryRouter>
      )
    },
  ],
}

export const OnTableView = () => (
  <OverviewTablePane view={FAKE_SNAPSHOT.view!} isSocketConnected={false} />
)
