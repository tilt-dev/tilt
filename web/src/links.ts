// Helper functions for displaying links

export function displayURL(li: Proto.webviewLink): string {
  let url = li.url?.replace(/^(http:\/\/)/, "")
  url = url?.replace(/^(https:\/\/)/, "")
  url = url?.replace(/^(www\.)/, "")
  return url || ""
}
