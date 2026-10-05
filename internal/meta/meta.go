// Package meta holds build-wide identity constants for SmartEYE.
package meta

// Version is the SmartEYE application version. It is also overridable at build
// time via -ldflags "-X .../internal/meta.Version=...".
var Version = "1.0.0"

// Product is the user-facing product name.
const Product = "SmartEYE"
