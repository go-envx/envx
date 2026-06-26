// Package schema is the single source of truth for envx's declarative surface:
//   - FlagSpec catalog (flag name, shorthand, ENVX_* fallback, and usage) for each
//     declared setting.
//   - The Manifest schema, including the Settings, Environments, Projects, and
//     pure query methods to easily read the manifest.
//
// Package schema is a pure leaf importing only the standard library, allowing
// any other package to import it without coupling.
package schema
