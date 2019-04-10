type Tags = { [key: string]: string }

// Fire and forget all analytics events
const incr = (name: string, tags: Tags = {}): void => {
  let url = `http://${window.location.host}/api/analytics`

  fetch(url, {
    method: "post",
    body: JSON.stringify([{ verb: "incr", name: name, tags: tags }]),
  })
}

export { incr }
