package web

import "embed"

//go:embed frontend/dist
var Assets embed.FS
