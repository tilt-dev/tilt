import Features, { Flag } from "./feature"
import {
  buildGroupTree,
  getResourceLabels,
  resourcesHaveLabels,
} from "./labels"
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

      it("does not return K8s system prefixed labels", () => {
        const resource = nResourceView(1).uiResources[0]
        resource.metadata!.labels = {
          "kubernetes.io/name": "kubernetes.io/name",
          "app.kubernetes.io/component": "app.kubernetes.io/component",
          "helm.sh/chart": "helm.sh/chart",
          "k8s.io/name": "k8s.io/name",
          anotherLabel: "anotherLabel",
        }
        expect(getResourceLabels(resource)).toEqual(["anotherLabel"])
      })

      it("returns custom hierarchical labels with .", () => {
        const resource = nResourceView(1).uiResources[0]
        resource.metadata!.labels = {
          customHierarchy: "env-1.apis",
          anotherHierarchy: "shared.storage",
        }
        expect(getResourceLabels(resource)).toEqual([
          "env-1.apis",
          "shared.storage",
        ])
      })
    })
  })

  describe("buildGroupTree", () => {
    type TestItem = { name: string; labels: string[]; isTiltfile: boolean }

    const getLabels = (item: TestItem) => item.labels
    const isTiltfile = (item: TestItem) => item.isTiltfile

    it("creates flat structure for labels without .", () => {
      const items: TestItem[] = [
        { name: "r1", labels: ["frontend"], isTiltfile: false },
        { name: "r2", labels: ["backend"], isTiltfile: false },
        { name: "r3", labels: ["frontend"], isTiltfile: false },
      ]

      const result = buildGroupTree(items, getLabels, isTiltfile)

      expect(result.roots.length).toBe(2)
      expect(result.roots[0].name).toBe("backend")
      expect(result.roots[0].path).toBe("backend")
      expect(result.roots[0].resources.length).toBe(1)
      expect(result.roots[0].children.length).toBe(0)

      expect(result.roots[1].name).toBe("frontend")
      expect(result.roots[1].resources.length).toBe(2)
    })

    it("creates nested structure for hierarchical labels", () => {
      const items: TestItem[] = [
        { name: "api", labels: ["env-1.apis"], isTiltfile: false },
        { name: "db", labels: ["env-1.dbs"], isTiltfile: false },
        { name: "api2", labels: ["env-2.apis"], isTiltfile: false },
      ]

      const result = buildGroupTree(items, getLabels, isTiltfile)

      expect(result.roots.length).toBe(2)

      // env-1 group
      const env1 = result.roots[0]
      expect(env1.name).toBe("env-1")
      expect(env1.path).toBe("env-1")
      expect(env1.resources.length).toBe(0) // No direct resources
      expect(env1.children.length).toBe(2) // apis and dbs

      // env-1.apis
      const env1Apis = env1.children[0]
      expect(env1Apis.name).toBe("apis")
      expect(env1Apis.path).toBe("env-1.apis")
      expect(env1Apis.resources.length).toBe(1)

      // env-1.dbs
      const env1Dbs = env1.children[1]
      expect(env1Dbs.name).toBe("dbs")
      expect(env1Dbs.path).toBe("env-1.dbs")
      expect(env1Dbs.resources.length).toBe(1)

      // env-2 group
      const env2 = result.roots[1]
      expect(env2.name).toBe("env-2")
      expect(env2.children.length).toBe(1) // apis only
    })

    it("aggregates resources from descendants", () => {
      const items: TestItem[] = [
        { name: "parent-resource", labels: ["parent"], isTiltfile: false },
        { name: "child-resource", labels: ["parent.child"], isTiltfile: false },
        {
          name: "grandchild-resource",
          labels: ["parent.child.grandchild"],
          isTiltfile: false,
        },
      ]

      const result = buildGroupTree(items, getLabels, isTiltfile)

      const parent = result.roots[0]
      expect(parent.name).toBe("parent")
      expect(parent.resources.length).toBe(1)
      expect(parent.aggregatedResources.length).toBe(3) // parent + child + grandchild

      const child = parent.children[0]
      expect(child.aggregatedResources.length).toBe(2) // child + grandchild

      const grandchild = child.children[0]
      expect(grandchild.aggregatedResources.length).toBe(1) // grandchild only
    })

    it("places resources with multiple labels in all matching groups", () => {
      const items: TestItem[] = [
        { name: "multi", labels: ["group-a", "group-b"], isTiltfile: false },
      ]

      const result = buildGroupTree(items, getLabels, isTiltfile)

      expect(result.roots.length).toBe(2)
      expect(result.roots[0].name).toBe("group-a")
      expect(result.roots[0].resources.length).toBe(1)
      expect(result.roots[0].resources[0].name).toBe("multi")

      expect(result.roots[1].name).toBe("group-b")
      expect(result.roots[1].resources.length).toBe(1)
      expect(result.roots[1].resources[0].name).toBe("multi")
    })

    it("deduplicates resources in aggregatedResources", () => {
      // A resource appears in multiple child groups under the same parent
      const items: TestItem[] = [
        {
          name: "shared",
          labels: ["parent.child-a", "parent.child-b"],
          isTiltfile: false,
        },
      ]

      const result = buildGroupTree(items, getLabels, isTiltfile)

      const parent = result.roots[0]
      // Should not have duplicates even though the resource appears in both children
      expect(parent.aggregatedResources.length).toBe(1)
    })

    it("handles deeply nested paths (e.g., a.b.c.d)", () => {
      const items: TestItem[] = [
        { name: "deep", labels: ["a.b.c.d"], isTiltfile: false },
      ]

      const result = buildGroupTree(items, getLabels, isTiltfile)

      const a = result.roots[0]
      expect(a.name).toBe("a")
      expect(a.resources.length).toBe(0)
      expect(a.aggregatedResources.length).toBe(1)

      const b = a.children[0]
      expect(b.name).toBe("b")
      expect(b.path).toBe("a.b")

      const c = b.children[0]
      expect(c.name).toBe("c")
      expect(c.path).toBe("a.b.c")

      const d = c.children[0]
      expect(d.name).toBe("d")
      expect(d.path).toBe("a.b.c.d")
      expect(d.resources.length).toBe(1)
    })

    it("separates tiltfile and unlabeled resources", () => {
      const items: TestItem[] = [
        { name: "labeled", labels: ["group"], isTiltfile: false },
        { name: "tiltfile", labels: [], isTiltfile: true },
        { name: "unlabeled", labels: [], isTiltfile: false },
      ]

      const result = buildGroupTree(items, getLabels, isTiltfile)

      expect(result.roots.length).toBe(1)
      expect(result.roots[0].resources.length).toBe(1)
      expect(result.tiltfile.length).toBe(1)
      expect(result.tiltfile[0].name).toBe("tiltfile")
      expect(result.unlabeled.length).toBe(1)
      expect(result.unlabeled[0].name).toBe("unlabeled")
    })

    it("returns allGroupPaths sorted alphabetically", () => {
      const items: TestItem[] = [
        { name: "r1", labels: ["z.a"], isTiltfile: false },
        { name: "r2", labels: ["a.z"], isTiltfile: false },
        { name: "r3", labels: ["m"], isTiltfile: false },
      ]

      const result = buildGroupTree(items, getLabels, isTiltfile)

      expect(result.allGroupPaths).toEqual(["a", "a.z", "m", "z", "z.a"])
    })

    it("sorts children alphabetically within each parent", () => {
      const items: TestItem[] = [
        { name: "r1", labels: ["parent.zulu"], isTiltfile: false },
        { name: "r2", labels: ["parent.alpha"], isTiltfile: false },
        { name: "r3", labels: ["parent.mike"], isTiltfile: false },
      ]

      const result = buildGroupTree(items, getLabels, isTiltfile)

      const parent = result.roots[0]
      expect(parent.children.map((c) => c.name)).toEqual([
        "alpha",
        "mike",
        "zulu",
      ])
    })
  })
})
