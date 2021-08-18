import Features, { Flag } from "./feature"
import { getResourceLabels, resourcesHaveLabels } from "./labels"
import { nResourceView, nResourceWithLabelsView } from "./testdata"

describe("Resource label helpers", () => {
  describe("resourcesHaveLabels", () => {
    it("returns `false` if labels feature is not enabled", () => {
      const { uiResources } = nResourceWithLabelsView(5)
      const features = new Features({ [Flag.Labels]: false })

      expect(
        resourcesHaveLabels(features, uiResources, getResourceLabels)
      ).toBe(false)
    })

    it("returns `false` if labels feature is enabled but no resources have labels", () => {
      const { uiResources } = nResourceView(5)
      const features = new Features({ [Flag.Labels]: true })

      expect(
        resourcesHaveLabels(features, uiResources, getResourceLabels)
      ).toBe(false)
    })

    it("returns `true` if labels feature is enabled and at least one resource has labels", () => {
      const { uiResources } = nResourceWithLabelsView(2)
      const features = new Features({ [Flag.Labels]: true })

      expect(
        resourcesHaveLabels(features, uiResources, getResourceLabels)
      ).toBe(true)
    })
  })

  describe("getResourceLabels", () => {
    describe("when a resource doesn't have labels", () => {
      it("returns an empty list if there are no labels", () => {
        const resource = nResourceView(1).uiResources[0]
        expect(getResourceLabels(resource)).toStrictEqual([])
      })

      it("returns an empty list if metadata is undefined", () => {
        const resource = nResourceView(1).uiResources[0]
        resource.metadata = undefined
        expect(getResourceLabels(resource)).toStrictEqual([])
      })

      it("returns an empty list if labels are undefined", () => {
        const resource = nResourceView(1).uiResources[0]
        resource.metadata!.labels = undefined
        expect(getResourceLabels(resource)).toStrictEqual([])
      })
    })

    describe("when a resource has labels", () => {
      it("returns a list of labels", () => {
        const resource = nResourceView(1).uiResources[0]
        resource.metadata!.labels = {
          testLabel: "testLabel",
          anotherLabel: "anotherLabel",
        }
        expect(getResourceLabels(resource)).toEqual([
          "testLabel",
          "anotherLabel",
        ])
      })

      it("does not return prefixed labels", () => {
        const resource = nResourceView(1).uiResources[0]
        resource.metadata!.labels = {
          "prefixed/label": "prefixed/label",
          anotherLabel: "anotherLabel",
        }
        expect(getResourceLabels(resource)).toEqual(["anotherLabel"])
      })
    })
  })
})
