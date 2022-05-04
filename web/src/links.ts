// Helper functions for displaying links

import { UILink } from "./types"

export function displayURL(li: UILink): string {
  let url = li.url?.replace(/^(http:\/\/)/, "")
  url = url?.replace(/^(https:\/\/)/, "")
  url = url?.replace(/^(www\.)/, "")
  return url || ""
}
