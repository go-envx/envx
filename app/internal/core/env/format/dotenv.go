package format

import (
	"bufio"
	"io"
	"strings"
)

// dotenvSpecial lists characters that force a dotenv value to be quoted.
const dotenvSpecial = " \t\n\r\"'#$=`\\"

// renderDotenv writes KEY=value lines, quoting values a dotenv reader would
// otherwise misinterpret.
func renderDotenv(w io.Writer, items []item) error {
	bw := bufio.NewWriter(w)
	for _, it := range items {
		if _, err := bw.WriteString(it.key + "=" + dotenvValue(it.value) + "\n"); err != nil {
			return err
		}
	}
	return bw.Flush()
}

// dotenvValue quotes only when needed, preferring single quotes, which dotenv
// readers never expand; otherwise "$" is escaped inside double quotes.
func dotenvValue(value string) string {
	if value != "" && !strings.ContainsAny(value, dotenvSpecial) {
		return value
	}
	if !strings.ContainsAny(value, "'\n\r") {
		return "'" + value + "'"
	}
	replacer := strings.NewReplacer(
		`\`, `\\`,
		`"`, `\"`,
		`$`, `\$`,
		"\n", `\n`,
		"\r", `\r`,
		"\t", `\t`,
	)
	return `"` + replacer.Replace(value) + `"`
}
