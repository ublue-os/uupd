package percent

import (
	"math"

	"github.com/jedib0t/go-pretty/v6/text"
)

// Accent color portal return as of xdg-desktop-portal-gnome 47.1
type Accent struct {
	Type string `json:"type"`
	Data []struct {
		Type string     `json:"type"`
		Data [3]float64 `json:"data"`
	} `json:"data"`
}

// Colors taken straight from GNOME 47 accent colors using this command:
// busctl --user call org.freedesktop.portal.Desktop /org/freedesktop/portal/desktop org.freedesktop.portal.Settings ReadOne 'ss' 'org.freedesktop.appearance' 'accent-color'
// This is as close as we can map the colors as possible afaik - Pink and Magenta DO look a like, and thats kind of a problem
var colorMap = map[text.Color][3]float64{
	text.FgHiBlack:   {0, 0, 0},
	text.FgHiBlue:    {0.207843, 0.517647, 0.894118},
	text.FgHiCyan:    {0.129412, 0.564706, 0.643137},
	text.FgHiGreen:   {0.227451, 0.580392, 0.290196},
	text.FgHiYellow:  {0.784314, 0.533333, 0},
	text.FgHiRed:     {0.901961, 0.176471, 0.258824},
	text.FgHiMagenta: {0.568627, 0.254902, 0.67451},
	text.FgHiWhite:   {0.435294, 0.513726, 0.588235},
}

// Calculates the squared Euclidean distance between two colors
func colorDistance(c1, c2 [3]float64) float64 {
	distR := c1[0] - c2[0]
	distG := c1[1] - c2[1]
	distB := c1[2] - c2[2]
	return distR*distR +
		distG*distG +
		distB*distB
}

func findClosestColor(rgb [3]float64) (text.Color, text.Color) {
	var closestColor text.Color
	minDistance := math.MaxFloat64

	for color, predefinedRGB := range colorMap {
		distance := colorDistance(rgb, predefinedRGB)
		if distance < minDistance {
			minDistance = distance
			closestColor = color
		}
	}

	nonHiColor, isHiColor := hiToNonHiMap[closestColor]
	if isHiColor {
		return closestColor, nonHiColor
	}

	return closestColor, closestColor
}

var hiToNonHiMap = map[text.Color]text.Color{
	text.FgHiBlack:   text.FgBlack,
	text.FgHiRed:     text.FgRed,
	text.FgHiGreen:   text.FgGreen,
	text.FgHiYellow:  text.FgYellow,
	text.FgHiBlue:    text.FgBlue,
	text.FgHiMagenta: text.FgMagenta,
	text.FgHiCyan:    text.FgCyan,
	text.FgHiWhite:   text.FgWhite,
}
