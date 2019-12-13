const zeroTime = "0001-01-01T00:00:00Z"

function isZeroTime(time: string | undefined) {
  return !time || time === zeroTime
}

export { isZeroTime, zeroTime }
