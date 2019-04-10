// Fire and forget all analytics events
const incr = (name: string, tags: Map<string, string> = new Map()): void => {
  let url = `http://${window.location.host}/api/analytics`
  let tagObj: { [key: string]: string } = {}
  tags.forEach((value, key) => {
    tagObj[key] = value
  })
  fetch(url, {
    method: "post",
    body: JSON.stringify([{ verb: "incr", name: name, tags: tagObj }]),
  })
}

export { incr }
