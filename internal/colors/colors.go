package colors

import (
	"fmt"
	"regexp"
)

// ansiPattern matches ANSI SGR (color/style) escape sequences.
var ansiPattern = regexp.MustCompile("\x1b\\[[0-9;]*m")

// StripANSI removes ANSI color/style escape sequences from s. Used to honor
// NO_COLOR and to support hosts (e.g. some TUIs) that don't interpret raw ANSI.
func StripANSI(s string) string {
	return ansiPattern.ReplaceAllString(s, "")
}

// ANSI color codes - A full spectrum for Prism
const (
	Reset = "\033[0m"
	Dim   = "\033[2m"
	Bold  = "\033[1m"

	// Basic colors (8-color)
	Black   = "\033[30m"
	Red     = "\033[31m"
	Green   = "\033[32m"
	Yellow  = "\033[33m"
	Blue    = "\033[34m"
	Magenta = "\033[35m"
	Cyan    = "\033[36m"
	White   = "\033[37m"
	Gray    = "\033[90m"

	// Bright variants
	BrightRed     = "\033[91m"
	BrightGreen   = "\033[92m"
	BrightYellow  = "\033[93m"
	BrightBlue    = "\033[94m"
	BrightMagenta = "\033[95m"
	BrightCyan    = "\033[96m"
	BrightWhite   = "\033[97m"

	// Reds (256-color)
	Maroon    = "\033[38;5;52m"
	DarkRed   = "\033[38;5;88m"
	Crimson   = "\033[38;5;160m"
	Coral     = "\033[38;5;209m"
	Salmon    = "\033[38;5;210m"
	Rose      = "\033[38;5;211m"
	HotPink   = "\033[38;5;206m"
	DeepPink  = "\033[38;5;199m"
	Pink      = "\033[38;5;218m"
	LightPink = "\033[38;5;217m"

	// Oranges (256-color)
	DarkOrange  = "\033[38;5;208m"
	Orange      = "\033[38;5;214m"
	LightOrange = "\033[38;5;215m"
	Peach       = "\033[38;5;223m"
	Tangerine   = "\033[38;5;202m"

	// Yellows (256-color)
	Gold        = "\033[38;5;220m"
	LightYellow = "\033[38;5;229m"
	Khaki       = "\033[38;5;228m"
	Olive       = "\033[38;5;142m"

	// Greens (256-color)
	DarkGreen     = "\033[38;5;22m"
	ForestGreen   = "\033[38;5;28m"
	SeaGreen      = "\033[38;5;29m"
	Emerald       = "\033[38;5;35m"
	LimeGreen     = "\033[38;5;118m"
	Lime          = "\033[38;5;154m"
	SpringGreen   = "\033[38;5;48m"
	Mint          = "\033[38;5;121m"
	LightGreen    = "\033[38;5;119m"
	PaleGreen     = "\033[38;5;157m"
	SupabaseGreen = "\033[38;2;62;207;142m" // Supabase brand #3ECF8E

	// Teals & Cyans (256-color)
	Teal       = "\033[38;5;30m"
	DarkCyan   = "\033[38;5;36m"
	Turquoise  = "\033[38;5;45m"
	Aqua       = "\033[38;5;51m"
	SkyBlue    = "\033[38;5;117m"
	LightCyan  = "\033[38;5;159m"
	PowderBlue = "\033[38;5;152m"

	// Blues (256-color)
	Navy           = "\033[38;5;17m"
	DarkBlue       = "\033[38;5;19m"
	MediumBlue     = "\033[38;5;20m"
	RoyalBlue      = "\033[38;5;63m"
	DodgerBlue     = "\033[38;5;33m"
	CornflowerBlue = "\033[38;5;69m"
	SteelBlue      = "\033[38;5;67m"
	LightBlue      = "\033[38;5;153m"
	SlateBlue      = "\033[38;5;62m"

	// Purples (256-color)
	Indigo     = "\033[38;5;54m"
	DarkViolet = "\033[38;5;128m"
	Purple     = "\033[38;5;129m"
	Violet     = "\033[38;5;177m"
	Orchid     = "\033[38;5;170m"
	Plum       = "\033[38;5;182m"
	Lavender   = "\033[38;5;183m"
	Mauve      = "\033[38;5;139m"

	// Browns (256-color)
	Brown     = "\033[38;5;94m"
	Chocolate = "\033[38;5;130m"
	Sienna    = "\033[38;5;137m"
	Tan       = "\033[38;5;180m"
	Wheat     = "\033[38;5;223m"

	// Grays (256-color)
	DarkGray  = "\033[38;5;240m"
	DimGray   = "\033[38;5;242m"
	LightGray = "\033[38;5;248m"
	Silver    = "\033[38;5;7m"
)

// ColorMap returns all colors as a map for plugin input
func ColorMap() map[string]string {
	return map[string]string{
		// Basic
		"reset": Reset,
		"dim":   Dim,
		"bold":  Bold,
		"black": Black,
		"white": White,
		"gray":  Gray,

		// Standard colors
		"red":     Red,
		"green":   Green,
		"yellow":  Yellow,
		"blue":    Blue,
		"magenta": Magenta,
		"cyan":    Cyan,

		// Bright variants
		"bright_red":     BrightRed,
		"bright_green":   BrightGreen,
		"bright_yellow":  BrightYellow,
		"bright_blue":    BrightBlue,
		"bright_magenta": BrightMagenta,
		"bright_cyan":    BrightCyan,
		"bright_white":   BrightWhite,

		// Reds
		"maroon":     Maroon,
		"dark_red":   DarkRed,
		"crimson":    Crimson,
		"coral":      Coral,
		"salmon":     Salmon,
		"rose":       Rose,
		"hot_pink":   HotPink,
		"deep_pink":  DeepPink,
		"pink":       Pink,
		"light_pink": LightPink,

		// Oranges
		"dark_orange":  DarkOrange,
		"orange":       Orange,
		"light_orange": LightOrange,
		"peach":        Peach,
		"tangerine":    Tangerine,

		// Yellows
		"gold":         Gold,
		"light_yellow": LightYellow,
		"khaki":        Khaki,
		"olive":        Olive,

		// Greens
		"dark_green":     DarkGreen,
		"forest_green":   ForestGreen,
		"sea_green":      SeaGreen,
		"emerald":        Emerald,
		"lime_green":     LimeGreen,
		"lime":           Lime,
		"spring_green":   SpringGreen,
		"mint":           Mint,
		"light_green":    LightGreen,
		"pale_green":     PaleGreen,
		"supabase_green": SupabaseGreen,

		// Teals & Cyans
		"teal":        Teal,
		"dark_cyan":   DarkCyan,
		"turquoise":   Turquoise,
		"aqua":        Aqua,
		"sky_blue":    SkyBlue,
		"light_cyan":  LightCyan,
		"powder_blue": PowderBlue,

		// Blues
		"navy":            Navy,
		"dark_blue":       DarkBlue,
		"medium_blue":     MediumBlue,
		"royal_blue":      RoyalBlue,
		"dodger_blue":     DodgerBlue,
		"cornflower_blue": CornflowerBlue,
		"steel_blue":      SteelBlue,
		"light_blue":      LightBlue,
		"slate_blue":      SlateBlue,

		// Purples
		"indigo":      Indigo,
		"dark_violet": DarkViolet,
		"purple":      Purple,
		"violet":      Violet,
		"orchid":      Orchid,
		"plum":        Plum,
		"lavender":    Lavender,
		"mauve":       Mauve,

		// Browns
		"brown":     Brown,
		"chocolate": Chocolate,
		"sienna":    Sienna,
		"tan":       Tan,
		"wheat":     Wheat,

		// Grays
		"dark_gray":  DarkGray,
		"dim_gray":   DimGray,
		"light_gray": LightGray,
		"silver":     Silver,
	}
}

// Wrap wraps text in color codes
func Wrap(color, text string) string {
	return fmt.Sprintf("%s%s%s", color, text, Reset)
}

// Separator returns the status line separator
func Separator() string {
	return fmt.Sprintf(" %s·%s ", Dim, Reset)
}
