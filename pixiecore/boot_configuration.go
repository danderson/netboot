package pixiecore

import (
	"bytes"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const x86HTTPClient = 0x10

// StaticBootConfiguration provides values for dhcp options that remain unchanged until restart
type StaticBootConfiguration struct {
	HTTPBootURL   []byte
	IPxeBootURL   []byte
	RecursiveDNS  []net.IP
	Preference    []byte
	UsePreference bool
}

// MakeStaticBootConfiguration creates a new StaticBootConfiguration with provided values
func MakeStaticBootConfiguration(httpBootURL, ipxeBootURL string, preference uint8, usePreference bool,
	dnsServerAddresses []net.IP) *StaticBootConfiguration {
	ret := &StaticBootConfiguration{HTTPBootURL: []byte(httpBootURL), IPxeBootURL: []byte(ipxeBootURL), UsePreference: usePreference}
	if usePreference {
		ret.Preference = make([]byte, 1)
		ret.Preference[0] = preference
	}
	ret.RecursiveDNS = dnsServerAddresses
	return ret
}

// GetBootURL returns Boot File URL, see RFC 5970
func (bc *StaticBootConfiguration) GetBootURL(id []byte, clientArchType uint16) ([]byte, error) {
	if clientArchType == x86HTTPClient {
		return bc.HTTPBootURL, nil
	}
	return bc.IPxeBootURL, nil
}

// GetPreference returns server's Preference, see RFC 3315
func (bc *StaticBootConfiguration) GetPreference() []byte {
	return bc.Preference
}

// GetRecursiveDNS returns list of addresses of recursive DNS servers, see RFC 3646
func (bc *StaticBootConfiguration) GetRecursiveDNS() []net.IP {
	return bc.RecursiveDNS
}

// APIBootConfiguration provides an interface to retrieve Boot File URL from an external server based on
// client ID and architecture type
type APIBootConfiguration struct {
	Client        *http.Client
	URLPrefix     string
	RecursiveDNS  []net.IP
	Preference    []byte
	UsePreference bool
}

// MakeAPIBootConfiguration creates a new APIBootConfiguration initialized with provided values
func MakeAPIBootConfiguration(url string, timeout time.Duration, preference uint8, usePreference bool,
	dnsServerAddresses []net.IP) *APIBootConfiguration {
	if !strings.HasSuffix(url, "/") {
		url += "/"
	}
	ret := &APIBootConfiguration{
		Client:        &http.Client{Timeout: timeout},
		URLPrefix:     url + "v1",
		UsePreference: usePreference,
	}
	if usePreference {
		ret.Preference = make([]byte, 1)
		ret.Preference[0] = preference
	}
	ret.RecursiveDNS = dnsServerAddresses

	return ret
}

// GetBootURL returns Boot File URL, see RFC 5970
func (bc *APIBootConfiguration) GetBootURL(id []byte, clientArchType uint16) ([]byte, error) {
	reqURL := fmt.Sprintf("%s/boot/%x/%d", bc.URLPrefix, id, clientArchType)
	resp, err := bc.Client.Get(reqURL)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("%s: %s", reqURL, http.StatusText(resp.StatusCode))
	}
	defer resp.Body.Close()

	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(resp.Body)
	if err != nil {
		return nil, err
	}
	url, _ := bc.makeURLAbsolute(buf.String())

	return []byte(url), nil
}

func (bc *APIBootConfiguration) makeURLAbsolute(urlStr string) (string, error) {
	u, err := url.Parse(urlStr)
	if err != nil {
		return "", fmt.Errorf("%q is not an URL", urlStr)
	}
	if !u.IsAbs() {
		base, err := url.Parse(bc.URLPrefix)
		if err != nil {
			return "", err
		}
		u = base.ResolveReference(u)
	}
	return u.String(), nil
}

// GetPreference returns server's Preference, see RFC 3315
func (bc *APIBootConfiguration) GetPreference() []byte {
	return bc.Preference
}

// GetRecursiveDNS returns list of addresses of recursive DNS servers, see RFC 3646
func (bc *APIBootConfiguration) GetRecursiveDNS() []net.IP {
	return bc.RecursiveDNS
}
