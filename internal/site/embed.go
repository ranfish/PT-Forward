package site

import _ "embed"

//go:embed data/sites.json
var sitesJSON []byte

//go:embed supported_sites.json
var SupportedSitesJSON []byte
