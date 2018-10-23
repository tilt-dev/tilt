package rty

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/mattn/go-runewidth"
)

// this is all copied from tview.

var (
	colorPattern  = regexp.MustCompile(`\[([a-zA-Z]+|#[0-9a-zA-Z]{6}|\-)?(:([a-zA-Z]+|#[0-9a-zA-Z]{6}|\-)?(:([lbdru]+|\-)?)?)?\]`)
	escapePattern = regexp.MustCompile(`\[([a-zA-Z0-9_,;: \-\."#]+)\[(\[*)\]`)
)

// decomposeString returns information about a string which may contain color
// tags. It returns the indices of the color tags (as returned by
// re.FindAllStringIndex()), the color tags themselves (as returned by
// re.FindAllStringSubmatch()), the indices of an escaped tags, the string
// stripped by any color tags and escaped, and the screen width of the stripped
// string.
func decomposeString(text string) (colorIndices [][]int, colors [][]string, escapeIndices [][]int, stripped string, width int) {
	// Get positions of color and escape tags.
	colorIndices = colorPattern.FindAllStringIndex(text, -1)
	colors = colorPattern.FindAllStringSubmatch(text, -1)
	escapeIndices = escapePattern.FindAllStringIndex(text, -1)

	// Because the color pattern detects empty tags, we need to filter them out.
	for i := len(colorIndices) - 1; i >= 0; i-- {
		if colorIndices[i][1]-colorIndices[i][0] == 2 {
			colorIndices = append(colorIndices[:i], colorIndices[i+1:]...)
			colors = append(colors[:i], colors[i+1:]...)
		}
	}

	// Remove the color tags from the original string.
	var from int
	buf := make([]byte, 0, len(text))
	for _, indices := range colorIndices {
		buf = append(buf, []byte(text[from:indices[0]])...)
		from = indices[1]
	}
	buf = append(buf, text[from:]...)

	// Escape string.
	stripped = string(escapePattern.ReplaceAll(buf, []byte("[$1$2]")))

	// Get the width of the stripped string.
	width = runewidth.StringWidth(stripped)

	return
}

// Positions of substrings in regular expressions.
const (
	colorForegroundPos = 1
	colorBackgroundPos = 3
	colorFlagPos       = 5
)

// styleFromTag takes the given style, defined by a foreground color (fgColor),
// a background color (bgColor), and style attributes, and modifies it based on
// the substrings (tagSubstrings) extracted by the regular expression for color
// tags. The new colors and attributes are returned where empty strings mean
// "don't modify" and a dash ("-") means "reset to default".
func styleFromTag(fgColor, bgColor, attributes string, tagSubstrings []string) (newFgColor, newBgColor, newAttributes string) {
	if tagSubstrings[colorForegroundPos] != "" {
		color := tagSubstrings[colorForegroundPos]
		if color == "-" {
			fgColor = "-"
		} else if color != "" {
			fgColor = color
		}
	}

	if tagSubstrings[colorBackgroundPos-1] != "" {
		color := tagSubstrings[colorBackgroundPos]
		if color == "-" {
			bgColor = "-"
		} else if color != "" {
			bgColor = color
		}
	}

	if tagSubstrings[colorFlagPos-1] != "" {
		flags := tagSubstrings[colorFlagPos]
		if flags == "-" {
			attributes = "-"
		} else if flags != "" {
			attributes = flags
		}
	}

	return fgColor, bgColor, attributes
}

// The states of the ANSI escape code parser.
const (
	ansiText = iota
	ansiEscape
	ansiSubstring
	ansiControlSequence
)

// ansi is a io.Writer which translates ANSI escape codes into tview color
// tags.
type ansi struct {
	io.Writer

	// Reusable buffers.
	buffer                        *bytes.Buffer // The entire output text of one Write().
	csiParameter, csiIntermediate *bytes.Buffer // Partial CSI strings.

	// The current state of the parser. One of the ansi constants.
	state int
}

// ANSIWriter returns an io.Writer which translates any ANSI escape codes
// written to it into tview color tags. Other escape codes don't have an effect
// and are simply removed. The translated text is written to the provided
// writer.
func ANSIWriter(writer io.Writer) io.Writer {
	return &ansi{
		Writer:          writer,
		buffer:          new(bytes.Buffer),
		csiParameter:    new(bytes.Buffer),
		csiIntermediate: new(bytes.Buffer),
		state:           ansiText,
	}
}

// Write parses the given text as a string of runes, translates ANSI escape
// codes to color tags and writes them to the output writer.
func (a *ansi) Write(text []byte) (int, error) {
	defer func() {
		a.buffer.Reset()
	}()

	for _, r := range string(text) {
		switch a.state {

		// We just entered an escape sequence.
		case ansiEscape:
			switch r {
			case '[': // Control Sequence Introducer.
				a.csiParameter.Reset()
				a.csiIntermediate.Reset()
				a.state = ansiControlSequence
			case 'c': // Reset.
				fmt.Fprint(a.buffer, "[-:-:-]")
				a.state = ansiText
			case 'P', ']', 'X', '^', '_': // Substrings and commands.
				a.state = ansiSubstring
			default: // Ignore.
				a.state = ansiText
			}

			// CSI Sequences.
		case ansiControlSequence:
			switch {
			case r >= 0x30 && r <= 0x3f: // Parameter bytes.
				if _, err := a.csiParameter.WriteRune(r); err != nil {
					return 0, err
				}
			case r >= 0x20 && r <= 0x2f: // Intermediate bytes.
				if _, err := a.csiIntermediate.WriteRune(r); err != nil {
					return 0, err
				}
			case r >= 0x40 && r <= 0x7e: // Final byte.
				switch r {
				case 'E': // Next line.
					count, _ := strconv.Atoi(a.csiParameter.String())
					if count == 0 {
						count = 1
					}
					fmt.Fprint(a.buffer, strings.Repeat("\n", count))
				case 'm': // Select Graphic Rendition.
					var (
						background, foreground, attributes string
						clearAttributes                    bool
					)
					fields := strings.Split(a.csiParameter.String(), ";")
					if len(fields) == 0 || len(fields) == 1 && fields[0] == "0" {
						// Reset.
						if _, err := a.buffer.WriteString("[-:-:-]"); err != nil {
							return 0, err
						}
						break
					}
					lookupColor := func(colorNumber int, bright bool) string {
						if colorNumber < 0 || colorNumber > 7 {
							return "black"
						}
						if bright {
							colorNumber += 8
						}
						return [...]string{
							"black",
							"red",
							"green",
							"yellow",
							"blue",
							"darkmagenta",
							"darkcyan",
							"white",
							"#7f7f7f",
							"#ff0000",
							"#00ff00",
							"#ffff00",
							"#5c5cff",
							"#ff00ff",
							"#00ffff",
							"#ffffff",
						}[colorNumber]
					}
					for index, field := range fields {
						switch field {
						case "1", "01":
							attributes += "b"
						case "2", "02":
							attributes += "d"
						case "4", "04":
							attributes += "u"
						case "5", "05":
							attributes += "l"
						case "7", "07":
							attributes += "7"
						case "22", "24", "25", "27":
							clearAttributes = true
						case "30", "31", "32", "33", "34", "35", "36", "37":
							colorNumber, _ := strconv.Atoi(field)
							foreground = lookupColor(colorNumber-30, false)
						case "40", "41", "42", "43", "44", "45", "46", "47":
							colorNumber, _ := strconv.Atoi(field)
							background = lookupColor(colorNumber-40, false)
						case "90", "91", "92", "93", "94", "95", "96", "97":
							colorNumber, _ := strconv.Atoi(field)
							foreground = lookupColor(colorNumber-90, true)
						case "100", "101", "102", "103", "104", "105", "106", "107":
							colorNumber, _ := strconv.Atoi(field)
							background = lookupColor(colorNumber-100, true)
						case "38", "48":
							var color string
							if len(fields) > index+1 {
								if fields[index+1] == "5" && len(fields) > index+2 { // 8-bit colors.
									colorNumber, _ := strconv.Atoi(fields[index+2])
									if colorNumber <= 7 {
										color = lookupColor(colorNumber, false)
									} else if colorNumber <= 15 {
										color = lookupColor(colorNumber, true)
									} else if colorNumber <= 231 {
										red := (colorNumber - 16) / 36
										green := ((colorNumber - 16) / 6) % 6
										blue := (colorNumber - 16) % 6
										color = fmt.Sprintf("#%02x%02x%02x", 255*red/5, 255*green/5, 255*blue/5)
									} else if colorNumber <= 255 {
										grey := 255 * (colorNumber - 232) / 23
										color = fmt.Sprintf("#%02x%02x%02x", grey, grey, grey)
									}
								} else if fields[index+1] == "2" && len(fields) > index+4 { // 24-bit colors.
									red, _ := strconv.Atoi(fields[index+2])
									green, _ := strconv.Atoi(fields[index+3])
									blue, _ := strconv.Atoi(fields[index+4])
									color = fmt.Sprintf("#%02x%02x%02x", red, green, blue)
								}
							}
							if len(color) > 0 {
								if field == "38" {
									foreground = color
								} else {
									background = color
								}
							}
						}
					}
					if len(attributes) > 0 || clearAttributes {
						attributes = ":" + attributes
					}
					if len(foreground) > 0 || len(background) > 0 || len(attributes) > 0 {
						fmt.Fprintf(a.buffer, "[%s:%s%s]", foreground, background, attributes)
					}
				}
				a.state = ansiText
			default: // Undefined byte.
				a.state = ansiText // Abort CSI.
			}

			// We just entered a substring/command sequence.
		case ansiSubstring:
			if r == 27 { // Most likely the end of the substring.
				a.state = ansiEscape
			} // Ignore all other characters.

			// "ansiText" and all others.
		default:
			if r == 27 {
				// This is the start of an escape sequence.
				a.state = ansiEscape
			} else {
				// Just a regular rune. Send to buffer.
				if _, err := a.buffer.WriteRune(r); err != nil {
					return 0, err
				}
			}
		}
	}

	// Write buffer to target writer.
	n, err := a.buffer.WriteTo(a.Writer)
	if err != nil {
		return int(n), err
	}
	return len(text), nil
}

// TranslateANSI replaces ANSI escape sequences found in the provided string
// with tview's color tags and returns the resulting string.
func TranslateANSI(text string) string {
	var buffer bytes.Buffer
	writer := ANSIWriter(&buffer)
	writer.Write([]byte(text))
	return buffer.String()
}
