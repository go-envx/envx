package style

// ANSI control sequences used to open and close styling.
const (
	escape = "\033["
	reset  = "\033[0m"
)

// ANSI SGR parameter codes for the styles envx renders.
const (
	codeBold   = "1"
	codeRed    = "31"
	codeGreen  = "32"
	codeYellow = "33"
	codeCyan   = "36"
	codeGray   = "90"
)

// Color names an explicit foreground color for content whose meaning is not a
// resolution severity, such as diff signs. The zero value, ColorNone, leaves
// text unstyled.
type Color int

const (
	// ColorNone applies no color.
	ColorNone Color = iota
	// ColorRed renders text in red.
	ColorRed
	// ColorGreen renders text in green.
	ColorGreen
	// ColorYellow renders text in yellow.
	ColorYellow
	// ColorCyan renders text in cyan.
	ColorCyan
	// ColorMuted renders text in a dim gray.
	ColorMuted
)

// Severity classifies a value's resolution outcome so it can be colored
// consistently. The zero value, SeverityNone, leaves text unstyled.
type Severity int

const (
	// SeverityNone applies no color.
	SeverityNone Severity = iota
	// SeverityOK marks a successful outcome (cyan, chosen over green so it stays
	// distinguishable from SeverityError under red-green color blindness).
	SeverityOK
	// SeverityWarning marks a recoverable concern (yellow).
	SeverityWarning
	// SeverityError marks a failure (red).
	SeverityError
)

// Styler applies ANSI styling to strings, emitting escape codes only when
// enabled. The zero value is a disabled Styler that returns text unchanged.
type Styler struct {
	// enabled reports whether escape codes are emitted.
	enabled bool
}

// New returns a Styler that emits ANSI styling only when enabled is true.
func New(enabled bool) Styler {
	return Styler{enabled: enabled}
}

// Enabled reports whether the Styler emits ANSI styling.
func (s Styler) Enabled() bool {
	return s.enabled
}

// wrap surrounds text with the given SGR code and a reset, or returns text
// unchanged when styling is disabled.
func (s Styler) wrap(code, text string) string {
	if !s.enabled {
		return text
	}
	return escape + code + "m" + text + reset
}

// Bold renders text with increased weight.
func (s Styler) Bold(text string) string {
	return s.wrap(codeBold, text)
}

// Red renders text in red.
func (s Styler) Red(text string) string {
	return s.wrap(codeRed, text)
}

// Green renders text in green.
func (s Styler) Green(text string) string {
	return s.wrap(codeGreen, text)
}

// Cyan renders text in cyan.
func (s Styler) Cyan(text string) string {
	return s.wrap(codeCyan, text)
}

// Yellow renders text in yellow.
func (s Styler) Yellow(text string) string {
	return s.wrap(codeYellow, text)
}

// Muted renders text in a dim gray for secondary information.
func (s Styler) Muted(text string) string {
	return s.wrap(codeGray, text)
}

// Color renders text in the named color. ColorNone leaves it unstyled.
func (s Styler) Color(c Color, text string) string {
	switch c {
	case ColorRed:
		return s.Red(text)
	case ColorGreen:
		return s.Green(text)
	case ColorYellow:
		return s.Yellow(text)
	case ColorCyan:
		return s.Cyan(text)
	case ColorMuted:
		return s.Muted(text)
	case ColorNone:
		return text
	default:
		return text
	}
}

// Severity colors text according to sev. SeverityNone leaves it unstyled.
func (s Styler) Severity(sev Severity, text string) string {
	switch sev {
	case SeverityOK:
		return s.Cyan(text)
	case SeverityWarning:
		return s.Yellow(text)
	case SeverityError:
		return s.Red(text)
	case SeverityNone:
		return text
	default:
		return text
	}
}
