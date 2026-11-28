package domain

import (
	"fmt"
	"net/netip"
	"strings"
)

// Device is what can be told about a client from its user agent.
type Device struct {
	Browser string
	OS      string
}

type uaRule struct {
	token string
	name  string
}

// Rules are checked in order: the first token found wins, so the specific products come before the engines
// they embed (Edge and Opera carry "Chrome", Chrome carries "Safari").
var browserRules = []uaRule{
	{"Edg/", "Edge"}, {"EdgA/", "Edge"}, {"EdgiOS/", "Edge"}, {"OPR/", "Opera"}, {"Opera", "Opera"},
	{"SamsungBrowser", "Samsung Internet"}, {"Firefox/", "Firefox"}, {"FxiOS/", "Firefox"},
	{"Chromium/", "Chromium"}, {"CriOS/", "Chrome"}, {"Chrome/", "Chrome"},
	{"MSIE ", "Internet Explorer"}, {"Trident/", "Internet Explorer"},
	{"Safari/", "Safari"}, {"curl/", "curl"}, {"PostmanRuntime", "Postman"},
}

var osRules = []uaRule{
	{"Windows", "Windows"}, {"Android", "Android"}, {"iPhone", "iOS"}, {"iPad", "iOS"}, {"iPod", "iOS"},
	{"CrOS", "ChromeOS"}, {"Mac OS X", "macOS"}, {"Macintosh", "macOS"}, {"Linux", "Linux"}, {"X11", "Linux"},
}

// ParseUserAgent extracts the browser and operating system with a small table. Unknown parts stay empty.
func ParseUserAgent(ua string) Device {
	var d Device
	for _, r := range browserRules {
		if strings.Contains(ua, r.token) {
			d.Browser = r.name
			break
		}
	}
	for _, r := range osRules {
		if strings.Contains(ua, r.token) {
			d.OS = r.name
			break
		}
	}
	return d
}

// Label is the Spanish description of the device ("Chrome en Windows").
func (d Device) Label() string {
	switch {
	case d.Browser != "" && d.OS != "":
		return d.Browser + " en " + d.OS
	case d.Browser != "":
		return d.Browser
	case d.OS != "":
		return "un dispositivo con " + d.OS
	default:
		return "un dispositivo desconocido"
	}
}

// MaskIP hides the host part of an address so it can be shown and stored without identifying one machine:
// IPv4 loses the last octet ("203.0.113.x"), IPv6 keeps its /64 prefix ("2001:db8:1:2::/64"). An address that
// cannot be parsed gives an empty string.
func MaskIP(raw string) string {
	addr, err := netip.ParseAddr(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	addr = addr.Unmap().WithZone("")
	if addr.Is4() {
		b := addr.As4()
		return fmt.Sprintf("%d.%d.%d.x", b[0], b[1], b[2])
	}
	prefix, err := addr.Prefix(64)
	if err != nil {
		return ""
	}
	return prefix.String()
}
