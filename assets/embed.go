package assets

import _ "embed"

//go:embed index.html
var IndexHTML []byte

//go:embed shapez-auto.js
var ModJS []byte
