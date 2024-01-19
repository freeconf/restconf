package restconf

import (
	"embed"

	"github.com/freeconf/yang"
	"github.com/freeconf/yang/source"
)

//go:embed yang/*.yang
var internal embed.FS

// Access to fc-yang and fc-doc yang definitions.
var InternalYPath = source.Any(yang.InternalYPath, source.EmbedDir(internal, "yang"))

//go:embed yang/ietf-rfc/*.yang
var internalIetf embed.FS

// Access to IETF RFC yang definitions (as of 2023-12-29)
var InternalIetfRfcYPath = source.EmbedDir(internalIetf, "yang/ietf-rfc")
