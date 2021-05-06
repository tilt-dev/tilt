// Helper functions for displaying links

type Link = Proto.v1alpha1UIResourceLink

export function displayURL(li: Link): string {
  let url = li.url?.replace(/^(http:\/\/)/, "")
  url = url?.replace(/^(https:\/\/)/, "")
  url = url?.replace(/^(www\.)/, "")
  return url || ""
}
