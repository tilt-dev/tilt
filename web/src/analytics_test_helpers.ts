import fetchMock from "fetch-mock"
import { Tags } from "./analytics"

export function mockAnalyticsCalls() {
  fetchMock.mock("//localhost/api/analytics", JSON.stringify({}))
}
export function cleanupMockAnalyticsCalls() {
  fetchMock.reset()
}

export function expectIncrs(...incrs: { name: string; tags: Tags }[]) {
  const expectedRequestBodies = incrs.map((i) => [
    {
      verb: "incr",
      name: i.name,
      tags: i.tags,
    },
  ])
  const incrCalls = fetchMock
    .calls()
    .filter((e) => e[0]?.toString().endsWith("/api/analytics"))
  const actualRequestBodies = incrCalls.map((e) =>
    JSON.parse(e[1]?.body?.toString() ?? "")
  )
  expect(actualRequestBodies).toEqual(expectedRequestBodies)
}
