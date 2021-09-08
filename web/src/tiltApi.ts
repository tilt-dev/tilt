import { Moment } from "moment"

// apiserver's date format time is _extremely_ strict to the point that it requires the full
// six-decimal place microsecond precision, e.g. .000Z will be rejected, it must be .000000Z
// so use an explicit RFC3339 moment format to ensure it passes
export function apiTimeFormat(moment: Moment): string {
  return moment.format("YYYY-MM-DDTHH:mm:ss.SSSSSSZ")
}

export async function tiltApiPut<T extends { metadata?: Proto.v1ObjectMeta }>(
  kindPlural: string,
  subResource: string,
  obj: T
) {
  if (!obj.metadata?.name) {
    throw "object has no name"
  }
  const url = `/proxy/apis/tilt.dev/v1alpha1/${kindPlural}/${obj.metadata.name}/${subResource}`
  const resp = await fetch(url, {
    method: "PUT",
    headers: {
      Accept: "application/json",
      "Content-Type": "application/json",
    },
    body: JSON.stringify(obj),
  })
  if (resp && resp.status !== 200) {
    const body = await resp.text()
    throw `error updating object in api: ${body}`
  }
}
