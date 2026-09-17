package server

import (
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/rs/zerolog/log"
)

const storedFaviconCacheControl = "public, max-age=604800, immutable"

func serveStoredFavicon(c *webContext) {
	// Favicons are untrusted, including those stored before these checks existed.
	// SVG can execute scripts when opened as a document, so isolate its origin
	// and block active content and external resources. Inline styles keep SVG
	// icons usable without allowing external stylesheets.
	c.Response.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'; style-src 'unsafe-inline'")
	c.Response.Header().Set("X-Content-Type-Options", "nosniff")
	c.Response.Header().Set("Cache-Control", "no-store")

	key := c.Request.URL.Query().Get("key")
	dataURI, err := c.Indexer.ReadFavicon(key)
	if err != nil {
		http.Error(c.Response, "favicon not found", http.StatusNotFound)
		return
	}
	contentType, data, err := decodeFaviconDataURI(string(dataURI))
	if err != nil {
		http.Error(c.Response, "invalid favicon data", http.StatusUnsupportedMediaType)
		return
	}
	c.Response.Header().Set("Content-Type", contentType)
	c.Response.Header().Set("Cache-Control", storedFaviconCacheControl)
	c.Response.Header().Set("ETag", `"`+key+`"`)
	if _, err := c.Response.Write(data); err != nil {
		log.Warn().Err(err).Str("key", key).Msg("failed to write stored favicon response")
	}
}

func decodeFaviconDataURI(dataURI string) (string, []byte, error) {
	data := []byte(dataURI)
	if strings.HasPrefix(dataURI, "data:") {
		meta, payload, ok := strings.Cut(dataURI, ",")
		if !ok {
			return "", nil, errors.New("invalid data URI")
		}
		_, params, _ := strings.Cut(meta, ";")
		base64Encoded := false
		for param := range strings.SplitSeq(params, ";") {
			if strings.EqualFold(param, "base64") {
				base64Encoded = true
				break
			}
		}
		// Percent escapes apply to both plain and base64 data URI payloads.
		payload, err := url.PathUnescape(payload)
		if err != nil {
			return "", nil, err
		}
		if base64Encoded {
			data, err = base64.StdEncoding.DecodeString(payload)
			if err != nil {
				return "", nil, err
			}
		} else {
			data = []byte(payload)
		}
	}
	contentType := faviconContentType(data)
	if contentType == "" {
		return "", nil, errors.New("unsupported favicon content")
	}
	return contentType, data, nil
}

// faviconContentType identifies supported image formats from their contents,
// never from the media type supplied by the site or client. Raster signatures
// select inert image types; SVG additionally requires the serving CSP above.
func faviconContentType(data []byte) string {
	switch contentType := http.DetectContentType(data); contentType {
	case "image/png", "image/jpeg", "image/gif", "image/webp", "image/bmp", "image/x-icon":
		return contentType
	}

	// DetectContentType does not recognize SVG. Check the XML root so HTML or
	// arbitrary XML cannot be accepted just by claiming image/svg+xml.
	decoder := xml.NewDecoder(bytes.NewReader(bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))))
	for {
		token, err := decoder.Token()
		if err != nil {
			return ""
		}
		switch token := token.(type) {
		case xml.StartElement:
			if token.Name.Local == "svg" && token.Name.Space == "http://www.w3.org/2000/svg" {
				return "image/svg+xml"
			}
			return ""
		case xml.CharData:
			if len(bytes.TrimSpace(token)) != 0 {
				return ""
			}
		}
	}
}
