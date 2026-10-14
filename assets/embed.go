package assets

import _ "embed"

//go:embed index.html
var IndexHTML []byte

//go:embed shapez-auto.js
var ModJS []byte

//go:embed icon.ico
var IconICO []byte
