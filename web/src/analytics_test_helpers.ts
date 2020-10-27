import { Tags } from "./analytics"
import fetchMock from "jest-fetch-mock"

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
