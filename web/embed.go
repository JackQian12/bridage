package web

import "embed"

//go:embed admin/static user/static
var FS embed.FS
