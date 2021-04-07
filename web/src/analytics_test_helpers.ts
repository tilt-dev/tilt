import fetchMock from "jest-fetch-mock"
import { Tags } from "./analytics"

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
  const expectedRequestBodies = incrs.map((i) =>
    JSON.stringify([
      {
        verb: "incr",
        name: i.name,
        tags: i.tags,
      },
    ])
  )
  const actualRequestBodies = fetchMock.mock.calls.map((e) => e[1]?.body)
  expect(actualRequestBodies).toEqual(expectedRequestBodies)
}
