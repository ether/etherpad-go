package io

import "github.com/pdfcpu/pdfcpu/pkg/api"

// pdfcpu keeps its configuration in a directory it creates under the
// user's config directory ($HOME/.config/pdfcpu). The Docker image runs as
// an unprivileged user without a home, where creating /.config fails and
// pdfcpu panics; the PDF export and import only attach and extract files
// and need none of that configuration, so pdfcpu works from memory.
func init() {
	api.DisableConfigDir()
}
