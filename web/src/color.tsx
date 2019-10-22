function rgba(hex: string, alpha: number) {
  let r = parseInt(hex.slice(1, 3), 16),
    g = parseInt(hex.slice(3, 5), 16),
    b = parseInt(hex.slice(5, 7), 16)

  return `rgba(${r}, ${g}, ${b}, ${alpha})`
}

export default {
  white: "#ffffff",
  offWhite: "#eef1f1",
  black: "#000000",
  grayLightest: "#93a1a1", // Solarized base1
  grayLight: "#586e75", // Solarized base01
  gray: "#073642", // Solarized base02
  grayDark: "#002b36", // Solarized base03
  grayDarkest: "#001b20",

  // Tilt Colors
  green: "#20ba31",
  greenLight: "#70d37b",
  blue: "#03c7d3",
  blueLight: "#5edbe3",
  purple: "#6378ba",
  purpleLight: "#9ba9d3",
  red: "#f6685c",
  redLight: "#f7aaa4",
  pink: "#ef5aa0",
  yellow: "#fcb41e",
  yellowLight: "#fdcf6f",

  rgba,
}
