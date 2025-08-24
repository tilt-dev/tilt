import { render, screen } from "@testing-library/react"
import React from "react"
import {
  ClusterStatusDialog,
  ClusterStatusDialogProps,
  CLUSTER_STATUS_HEALTHY,
  getDefaultCluster,
} from "./ClusterStatusDialog"
import { clusterConnection } from "./testdata"

const TEST_BUTTON = document.createElement("button")
TEST_BUTTON.textContent = "Cluster status"

const DEFAULT_TEST_PROPS: ClusterStatusDialogProps = {
  open: true,
  onClose: () => console.log("closing dialog"),
  anchorEl: TEST_BUTTON,
}
const HEALTHY_CLUSTER = clusterConnection()
const UNHEALTHY_CLUSTER = clusterConnection("Yikes, something went wrong.")

describe("ClusterStatusDialog", () => {
  // Note: the MUI dialog component doesn't have good a11y markup,
  // so use the presence of the dialog close button to determine
  // whether or not the dialog has rendered.
  it("does NOT render if there is no cluster information", () => {
    render(<ClusterStatusDialog {...DEFAULT_TEST_PROPS} />)

    expect(screen.queryByLabelText("Close dialog")).toBeNull()
  })

  it("renders if there is cluster information", () => {
    render(
      <ClusterStatusDialog
        {...DEFAULT_TEST_PROPS}
        clusterConnection={HEALTHY_CLUSTER}
      />
    )

    expect(screen.queryByLabelText("Close dialog")).toBeTruthy()
  })

  it("renders Kubernetes connection information", () => {
    // The expected properties are hardcoded based on the test data
    // and order of properties in the property list component
    const expectedProperties = [
      "Product",
      "Context",
      "Namespace",
      "Architecture",
      "Local registry",
    ]
    const clusterStatus = HEALTHY_CLUSTER.status
    const k8sStatus = clusterStatus?.connection?.kubernetes
    const expectedDescriptions = [
      k8sStatus?.product,
      k8sStatus?.context,
      k8sStatus?.namespace,
      clusterStatus?.arch,
      clusterStatus?.registry?.host,
    ]

    render(
      <ClusterStatusDialog
        {...DEFAULT_TEST_PROPS}
        clusterConnection={HEALTHY_CLUSTER}
      />
    )

    const k8sProperties = screen
      .getAllByRole("term")
      .map((dt) => dt.textContent)
    const k8sDescriptions = screen
      .getAllByRole("definition")
      .map((dd) => dd.textContent)

    expect(k8sProperties).toStrictEqual(expectedProperties)
    expect(k8sDescriptions).toStrictEqual(expectedDescriptions)
  })

  it("displays `healthy` status with healthy icon if there is no error", () => {
    render(
      <ClusterStatusDialog
        {...DEFAULT_TEST_PROPS}
        clusterConnection={HEALTHY_CLUSTER}
      />
    )

    expect(screen.getByTestId("healthy-icon")).toBeTruthy()
    expect(screen.getByText(CLUSTER_STATUS_HEALTHY)).toBeTruthy()
    expect(screen.queryByTestId("unhealthy-icon")).toBeNull()
  })

  it("displays the error message with the unhealthy icon if there is an error", () => {
    render(
      <ClusterStatusDialog
        {...DEFAULT_TEST_PROPS}
        clusterConnection={UNHEALTHY_CLUSTER}
      />
    )

    expect(screen.getByTestId("unhealthy-icon")).toBeTruthy()
    expect(
      screen.getByText(UNHEALTHY_CLUSTER.status?.error as string)
    ).toBeTruthy()
    expect(screen.queryByTestId("healthy-icon")).toBeNull()
  })

  describe("getDefaultCluster", () => {
    const defaultCluster = clusterConnection()
    const nonDefaultClusterA = clusterConnection()
    nonDefaultClusterA.metadata!.name = "special"
    const nonDefaultClusterB = clusterConnection()
    nonDefaultClusterB.metadata!.name = "extra-special"

    it("returns the default cluster when it is present", () => {
      expect(
        getDefaultCluster([
          defaultCluster,
          nonDefaultClusterA,
          nonDefaultClusterB,
        ])
      ).toEqual(defaultCluster)
    })

    it("returns undefined when there are no clusters", () => {
      expect(getDefaultCluster()).toBe(undefined)
    })

    it("returns undefined when there is no default cluster", () => {
      expect(getDefaultCluster([nonDefaultClusterA, nonDefaultClusterB])).toBe(
        undefined
      )
    })
  })
})
