import { Formatter, Suffix, Unit } from "react-timeago"
// @ts-ignore
import enStrings from "react-timeago/lib/language-strings/en-short.js"
// @ts-ignore
import buildFormatter from "react-timeago/lib/formatters/buildFormatter"

const minutePlusFormatter = buildFormatter(enStrings)

const timeAgoFormatter = (
  value: number,
  unit: Unit,
  suffix: Suffix,
  epochMilliseconds: Number,
  _nextFormatter?: Formatter,
  now?: any
) => {
  let str = ""
  if (unit === "second") {
    for (let threshold of [5, 15, 30, 45]) {
      if (value < threshold) {
        str = `<${threshold}s`
        break
      } else {
        str = "<1m"
      }
    }
  } else {
    str = minutePlusFormatter(
      value,
      unit,
      suffix,
      epochMilliseconds,
      _nextFormatter,
      now
    )
  }
  return `${str} ago`
}

export { timeAgoFormatter }
