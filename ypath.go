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

// Access to IETF RFC yang definitions (as of 2023-12-29)
var InternalIetfRfcYPath = source.EmbedDir(internal, "yang/ietf-rfc")
