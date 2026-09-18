package emit

import (
	"bufio"
	"io"
	"strings"
)

// dotenvSpecial lists the characters whose presence forces a value to be quoted,
// so an unquoted line never carries a separator, comment, expansion, or
// whitespace a dotenv reader would misinterpret.
const dotenvSpecial = " \t\n\r\"'#$=`\\"

// renderDotenv writes KEY=value lines. A value that is empty or contains a
// character a dotenv reader would treat specially is double-quoted with backslash
// escapes, so the output round-trips through a standard dotenv parser.
func renderDotenv(w io.Writer, entries []Entry) error {
	bw := bufio.NewWriter(w)
	for _, e := range entries {
		if _, err := bw.WriteString(e.Key + "=" + dotenvValue(e.Value) + "\n"); err != nil {
			return err
		}
	}
	return bw.Flush()
}

// dotenvValue renders one value for a dotenv line, quoting only when needed. An
// empty value is quoted so the assignment stays explicit; any other value with no
// special character is written bare.
func dotenvValue(value string) string {
	if value != "" && !strings.ContainsAny(value, dotenvSpecial) {
		return value
	}
	replacer := strings.NewReplacer(
		`\`, `\\`,
		`"`, `\"`,
		"\n", `\n`,
		"\r", `\r`,
		"\t", `\t`,
	)
	return `"` + replacer.Replace(value) + `"`
}
