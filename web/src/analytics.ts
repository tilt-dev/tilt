// Fire and forget all analytics events
const incr = (name: string, tags: Map<string, string> = new Map()): void => {
  let url = `http://${window.location.host}/api/analytics`
  fetch(url, {
    method: "post",
    body: JSON.stringify([{ verb: "incr", name: name, tags: tags }]),
  })
}

export { incr }
