import moment from "moment"
import { formatBuildDuration } from "./time"

it("format durations correctly", () => {
  function assertFormatted(expected: string, ms: number) {
    expect(formatBuildDuration(moment.duration(ms))).toEqual(expected)
  }

  let second = 1000
  assertFormatted("1.0s", second)
  assertFormatted("20s", 20 * second)
  assertFormatted("40s", 40 * second)
  assertFormatted("50s", 50 * second)
  assertFormatted("1m", 70 * second)
  assertFormatted("2m", 150 * second)
  assertFormatted("1h", 4000 * second)

  // there used to be a bug where the UI would flip from
  // "10.0s" to "10s", which looked weird.
  assertFormatted("9.9s", 10 * second - 100)
  assertFormatted("9.9s", 10 * second - 51)
  assertFormatted("10s", 10 * second - 50)
  assertFormatted("10s", 10 * second - 1)
  assertFormatted("10s", 10 * second)
  assertFormatted("10s", 10 * second + 1)
})
