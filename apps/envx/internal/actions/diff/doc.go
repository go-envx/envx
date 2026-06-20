// Package diff implements "envx diff <project> <env-a> <env-b>": resolve the
// same project under two environments and compare them. Values are masked by
// default; --reveal opts into plaintext. Its only export is NewCommand.
package diff
