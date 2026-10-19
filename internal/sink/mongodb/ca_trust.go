// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package mongodb

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"go.mongodb.org/mongo-driver/mongo/options"

	kollecterrors "github.com/platformrelay/kollect/internal/errors"
	"github.com/platformrelay/kollect/internal/sink/netguard"
	"github.com/platformrelay/kollect/internal/sink/sinktls"
)

// Lower-case URI option names read in more than one place.
const (
	optTLS       = "tls"
	optSSL       = "ssl"
	optTLSCAFile = "tlscafile"
	optSSLCAFile = "sslcertificateauthorityfile"
)

// canonicalKeys maps the lower-case URI option names this check reads to the
// spelling the errors name; the driver matches option names case-insensitively.
var canonicalKeys = map[string]string{
	optTLS:                        optTLS,
	optSSL:                        optSSL,
	optTLSCAFile:                  "tlsCAFile",
	optSSLCAFile:                  "sslCertificateAuthorityFile",
	"tlsinsecure":                 "tlsInsecure",
	"sslinsecure":                 "sslInsecure",
	"tlsallowinvalidcertificates": "tlsAllowInvalidCertificates",
	"tlsallowinvalidhostnames":    "tlsAllowInvalidHostnames",
}

// tlsOnKeys are the options besides tls/ssl that turn TLS on in the driver
// (connstring.go addOptions); a later tls=false turns it off again.
var tlsOnKeys = map[string]bool{
	optTLSCAFile: true, optSSLCAFile: true,
	"tlscertificatekeyfile": true, "sslclientcertificatekeyfile": true,
	"tlscertificatefile": true, "tlsprivatekeyfile": true,
	"tlsdisableocspendpointcheck": true,
}

// uriTLS is what a mongodb URI's own text says about TLS, read without DNS.
type uriTLS struct {
	tls          bool   // TLS on, as the driver's ConnString.SSL
	tlsKey       string // tls or ssl when one of them was given, else ""
	caFileKey    string // the CA file option present (any value), else ""
	insecure     bool   // effective tlsInsecure/sslInsecure, as ConnString.SSLInsecure
	insecureKey  string // the option that set insecure
	allowInvalid string // a tlsAllowInvalid* option with a value other than false
}

// clientOptions builds the driver options for uri. Without caPEM they are what they
// always were. With caPEM the URI must use TLS, verify certificates and name no CA
// file of its own: checked on the text first, so a mongodb+srv URI is rejected
// without a DNS lookup, and again on the driver-built config. That config, which
// keeps the URI's client certificate, then gets the system roots plus the CA.
func clientOptions(uri string, caPEM []byte) (*options.ClientOptions, error) {
	if len(caPEM) == 0 {
		return options.Client().ApplyURI(uri).SetDialer(netguard.DefaultDialer), nil
	}

	if err := checkCAURI(uri); err != nil {
		return nil, err
	}

	roots, err := sinktls.AppendedPool(caPEM)
	if err != nil {
		return nil, err
	}

	opts := options.Client().ApplyURI(uri)
	if err := opts.Validate(); err != nil {
		return nil, fmt.Errorf("mongodb connect: %w", err)
	}

	if err := checkDriverTLS(opts.TLSConfig); err != nil {
		return nil, err
	}

	tlsConfig := opts.TLSConfig.Clone()
	tlsConfig.RootCAs = roots

	return opts.SetTLSConfig(tlsConfig).SetDialer(netguard.DefaultDialer), nil
}

// checkCAURI rejects a URI on which a spec.tls CA would be ignored or overridden.
// Errors name the option, never the URI.
func checkCAURI(uri string) error {
	u, err := readURITLS(uri)
	if err != nil {
		return err
	}

	switch {
	case u.caFileKey != "":
		return kollecterrors.Terminal(fmt.Errorf(
			"%w: %s; remove it or remove spec.tls", ErrCAFileConflict, u.caFileKey))
	case !u.tls && u.tlsKey != "":
		return kollecterrors.Terminal(fmt.Errorf(
			"%w: %s=false; remove it or remove spec.tls", ErrCATLSOff, u.tlsKey))
	case !u.tls:
		return kollecterrors.Terminal(fmt.Errorf(
			"%w: a mongodb:// URI needs tls=true for spec.tls to apply", ErrCATLSOff))
	case u.insecure:
		return kollecterrors.Terminal(fmt.Errorf(
			"%w: %s=true; remove it or remove spec.tls", ErrCAInsecure, u.insecureKey))
	case u.allowInvalid != "":
		return kollecterrors.Terminal(fmt.Errorf(
			"%w: %s; remove it or remove spec.tls", ErrCAInsecure, u.allowInvalid))
	}

	return nil
}

// checkDriverTLS is the backstop after ApplyURI: a config the key check should have
// refused (no TLS, a root pool from a URI CA file, verification off) is rejected
// the same way, so a spelling the key check misses cannot pass.
func checkDriverTLS(c *tls.Config) error {
	switch {
	case c == nil:
		return kollecterrors.Terminal(fmt.Errorf(
			"%w: the driver built no TLS config; set tls=true for spec.tls to apply", ErrCATLSOff))
	case c.RootCAs != nil:
		return kollecterrors.Terminal(fmt.Errorf(
			"%w: the driver loaded a root certificate from the URI (tlsCAFile); remove it or remove spec.tls",
			ErrCAFileConflict))
	case c.InsecureSkipVerify:
		return kollecterrors.Terminal(fmt.Errorf(
			"%w: the driver turned verification off (tlsInsecure); remove it or remove spec.tls", ErrCAInsecure))
	}

	return nil
}

// readURITLS reads the TLS options of uri the way the driver's connstring parser
// does ('&' or ';' separated pairs, unescaped, names matched case-insensitively,
// last value wins) but never resolves DNS: a mongodb+srv URI has TLS on by default,
// and its TXT record may carry only authSource, replicaSet and loadBalanced (driver
// dns.go). A URI the driver refuses is refused too.
func readURITLS(uri string) (uriTLS, error) {
	var u uriTLS

	query, srv, err := uriQuery(uri)
	if err != nil {
		return u, invalidURI(err.Error())
	}

	u.tls = srv
	var tlsSSL *bool // tls and ssl, when both are given, must agree (driver rule)

	for _, pair := range strings.FieldsFunc(query, func(r rune) bool { return r == ';' || r == '&' }) {
		rawKey, rawValue, found := strings.Cut(pair, "=")
		if !found || rawKey == "" {
			return u, invalidURI("an option has no '=' or no name")
		}

		key, err := url.QueryUnescape(rawKey)
		if err != nil {
			return u, invalidURI("an option name is not URL-encoded correctly")
		}

		value, err := url.QueryUnescape(rawValue)
		if err != nil {
			return u, invalidURI("an option value is not URL-encoded correctly")
		}

		lower := strings.ToLower(key)
		name := canonicalKeys[lower]

		switch lower {
		case optTLS, optSSL:
			on, ok := parseBool(value)
			if !ok {
				return u, invalidURI(name + " is neither true nor false")
			}

			if tlsSSL != nil && *tlsSSL != on {
				return u, invalidURI("tls and ssl disagree")
			}

			tlsSSL = &on
			u.tls, u.tlsKey = on, name
		case "tlsinsecure", "sslinsecure":
			on, ok := parseBool(value)
			if !ok {
				return u, kollecterrors.Terminal(fmt.Errorf(
					"%w: %s is neither true nor false; remove it or remove spec.tls", ErrCAInsecure, name))
			}

			u.insecure, u.insecureKey = on, name
		case "tlsallowinvalidcertificates", "tlsallowinvalidhostnames":
			// The driver ignores these; any value but false still asks for less
			// verification than spec.tls gives, so it is refused.
			if value != "false" {
				u.allowInvalid = name
			}
		case optTLSCAFile, optSSLCAFile:
			u.caFileKey = name
		case "tlsdisableocspendpointcheck":
			if _, ok := parseBool(value); !ok {
				return u, invalidURI("tlsDisableOCSPEndpointCheck is neither true nor false")
			}
		}

		if tlsOnKeys[lower] {
			u.tls = true
		}
	}

	return u, nil
}

// uriQuery returns the query of a mongodb URI and whether it is mongodb+srv,
// following connstring.go parse: the user info ends at the first '@', the hosts at
// the first '/', '?' or '@', the database at the first '?'.
func uriQuery(uri string) (string, bool, error) {
	var srv bool

	switch {
	case strings.HasPrefix(uri, "mongodb+srv://"):
		srv, uri = true, uri[len("mongodb+srv://"):]
	case strings.HasPrefix(uri, "mongodb://"):
		uri = uri[len("mongodb://"):]
	default:
		return "", false, errors.New("the scheme is not mongodb:// or mongodb+srv://")
	}

	if i := strings.Index(uri, "@"); i >= 0 {
		user, password, _ := strings.Cut(uri[:i], ":")
		if strings.Contains(user, "/") || strings.ContainsAny(password, ":/") {
			// Also a URI whose query holds an '@': the driver reads all before it
			// as user info and refuses the '/'.
			return "", false, errors.New("unescaped '/' or ':' in the user info")
		}

		uri = uri[i+1:]
	}

	i := strings.IndexAny(uri, "/?@")
	if i < 0 {
		return "", srv, nil
	}

	if uri[i] != '/' {
		return "", false, errors.New("the hosts are not followed by '/'")
	}

	database, query, _ := strings.Cut(uri[i+1:], "?")
	if _, err := url.QueryUnescape(database); err != nil {
		return "", false, errors.New("the database name is not URL-encoded correctly")
	}

	return query, srv, nil
}

func parseBool(v string) (bool, bool) {
	switch v {
	case "true":
		return true, true
	case "false":
		return false, true
	}

	return false, false
}

func invalidURI(reason string) error {
	return kollecterrors.Terminal(fmt.Errorf("%w: %s", ErrCAURIInvalid, reason))
}
