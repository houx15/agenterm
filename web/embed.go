package web

import "embed"

//go:embed index.html vendor/*
var Assets embed.FS
