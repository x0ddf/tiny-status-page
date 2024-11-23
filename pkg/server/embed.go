package server

import "embed"

//go:embed templates/index.html
var content embed.FS
