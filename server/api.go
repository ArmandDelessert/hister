package server

import (
	"fmt"
)

const (
	// GET is HTTP GET request type
	GET string = "GET"
	// POST is HTTP POST request type
	POST string = "POST"
	// PUT is HTTP PUT request type
	PUT string = "PUT"
	// PATCH is HTTP PATCH request type
	PATCH string = "PATCH"
	// HEAD is HTTP HEAD request type
	HEAD string = "HEAD"
)

type endpointHandler func(*webContext)

// EndpointArg represents an API endpoint argument.
type EndpointArg struct {
	Name        string
	Type        string
	Required    bool
	Description string
}

// Endpoint represents an API endpoint definition.
type Endpoint struct {
	Name         string
	Path         string
	Method       string
	CSRFRequired bool
	Handler      endpointHandler `json:"-"`
	Description  string
	Args         []*EndpointArg
}

func (e *Endpoint) Pattern() string {
	return fmt.Sprintf("%s %s", e.Method, e.Path)
}

// Endpoints contains all registered API endpoints.
var Endpoints []*Endpoint

func init() {
	// TODO add Args
	Endpoints = []*Endpoint{
		&Endpoint{
			Name:         "Config",
			Path:         "/api/config",
			Method:       GET,
			CSRFRequired: true,
			Handler:      serveConfig,
			Description:  "Serve config",
		},
		&Endpoint{
			Name:         "Search",
			Path:         "/search",
			Method:       GET,
			CSRFRequired: false,
			Handler:      serveSearch,
			Description:  "Search websocket endpoint",
		},
		// tmp added for backward compatibility
		&Endpoint{
			Name:         "Add",
			Path:         "/api/add",
			Method:       GET,
			CSRFRequired: true,
			Handler:      serveAdd,
			Description:  "Add document form",
		},
		&Endpoint{
			Name:         "Add post",
			Path:         "/api/add",
			Method:       POST,
			CSRFRequired: true,
			Handler:      serveAdd,
			Description:  "Save added document",
		},
		// alias for /api/add - backward compatibility - use /api/add in the future
		&Endpoint{
			Name:         "Add post",
			Path:         "/add",
			Method:       POST,
			CSRFRequired: true,
			Handler:      serveAdd,
			Description:  "Save added document",
		},
		&Endpoint{
			Name:         "Get document",
			Path:         "/api/document",
			Method:       GET,
			CSRFRequired: false,
			Handler:      serveGet,
			Description:  "Get document by URL",
			Args: []*EndpointArg{
				&EndpointArg{
					Name:        "url",
					Type:        "string",
					Required:    true,
					Description: "URL of the document",
				},
			},
		},
		&Endpoint{
			Name:         "Rules",
			Path:         "/api/rules",
			Method:       GET,
			CSRFRequired: true,
			Handler:      serveRules,
			Description:  "Rules page",
		},
		&Endpoint{
			Name:         "Save rules",
			Path:         "/api/rules",
			Method:       POST,
			CSRFRequired: true,
			Handler:      serveRules,
			Description:  "Save rules",
		},
		&Endpoint{
			Name:         "History",
			Path:         "/api/history",
			Method:       GET,
			CSRFRequired: true,
			Handler:      serveHistory,
			Description:  "History page",
		},
		&Endpoint{
			Name:         "Add history item",
			Path:         "/api/history",
			Method:       POST,
			CSRFRequired: true,
			Handler:      serveHistory,
			Description:  "Add new history item",
		},
		&Endpoint{
			Name:         "Delete",
			Path:         "/api/delete",
			Method:       POST,
			CSRFRequired: true,
			Handler:      serveDeleteDocument,
			Description:  "Delete document endpoint",
		},
		&Endpoint{
			Name:         "Delete alias",
			Path:         "/api/delete_alias",
			Method:       POST,
			CSRFRequired: true,
			Handler:      serveDeleteAlias,
			Description:  "Delete alias",
		},
		&Endpoint{
			Name:         "Add alias",
			Path:         "/api/add_alias",
			Method:       POST,
			CSRFRequired: true,
			Handler:      serveAddAlias,
			Description:  "Add alias",
		},
		&Endpoint{
			Name:         "Readable",
			Path:         "/api/readable",
			Method:       GET,
			CSRFRequired: false,
			Handler:      serveReadable,
			Description:  "Readabilty view",
		},
		&Endpoint{
			Name:         "Stats",
			Path:         "/api/stats",
			Method:       GET,
			CSRFRequired: false,
			Handler:      serveStats,
			Description:  "Search engine statistics",
		},
		&Endpoint{
			Name:         "API",
			Path:         "/api",
			Method:       GET,
			CSRFRequired: false,
			Handler:      serveAPI,
			Description:  "API documentation",
		},
	}
}
