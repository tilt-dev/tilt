import React from "react"
import { MemoryRouter } from "react-router"
import OverviewResourceDetails from "./OverviewResourceDetails"
import { SidebarPinMemoryProvider } from "./SidebarPin"
import { oneResource } from "./testdata"

type Resource = Proto.webviewResource

export default {
  title: "OverviewResourceDetails",
  decorators: [
    (Story: any) => (
      <MemoryRouter initialEntries={["/"]}>
        <SidebarPinMemoryProvider>
          <div style={{ margin: "-1rem", height: "80vh" }}>
            <Story />
          </div>
        </SidebarPinMemoryProvider>
      </MemoryRouter>
    ),
  ],
}

export const Vigoda = () => (
  <OverviewResourceDetails view={oneResource()} name="vigoda" />
)
