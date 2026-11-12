// Package mockdata holds the provider responses given in the assignment.
package mockdata

import _ "embed"

//go:embed garuda_indonesia_search_response.json
var GarudaIndonesia []byte

//go:embed lion_air_search_response.json
var LionAir []byte

//go:embed batik_air_search_response.json
var BatikAir []byte

//go:embed airasia_search_response.json
var AirAsia []byte
