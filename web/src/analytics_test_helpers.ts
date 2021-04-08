import fetchMock from "jest-fetch-mock"
import { Tags } from "./analytics"

// at some point we might want to be clever about this and only mock the responses of calls to /api/analytics
export function mockAnalyticsCalls() {
  fetchMock.resetMocks()
  fetchMock.mockResponse(JSON.stringify({}))
}
export function cleanupMockAnalyticsCalls() {
  fetchMock.resetMocks()
}

// TODO(matt) migrate uses of this to `expectIncrs`
export function expectIncr(fetchMockIndex: number, name: string, tags: Tags) {
  expect(fetchMock.mock.calls.length).toBeGreaterThan(fetchMockIndex)
  expect(fetchMock.mock.calls[fetchMockIndex][0]).toEqual(
    "//localhost/api/analytics"
  )
  expect(fetchMock.mock.calls[fetchMockIndex][1]?.body).toEqual(
    JSON.stringify([
      {
        verb: "incr",
        name: name,
        tags: tags,
      },
    ])
  )
}

export function expectIncrs(...incrs: { name: string; tags: Tags }[]) {
  const expectedRequestBodies = incrs.map((i) => [
    {
      verb: "incr",
      name: i.name,
      tags: i.tags,
    },
  ])
  const incrCalls = fetchMock.mock.calls.filter((e) =>
    e[0]?.toString().endsWith("/api/analytics")
  )
  const actualRequestBodies = incrCalls.map((e) =>
    JSON.parse(e[1]?.body?.toString() ?? "")
  )
  expect(actualRequestBodies).toEqual(expectedRequestBodies)
}
