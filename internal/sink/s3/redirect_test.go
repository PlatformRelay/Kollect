// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package s3

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	kollecterrors "github.com/platformrelay/kollect/internal/errors"
)

// redirectRT answers the first request with a 307 to location (or, with location
// empty, to the request's own URL, every time) and fails any later hop with hopErr.
type redirectRT struct {
	location string
	hopErr   error
	calls    int
}

func (rt *redirectRT) RoundTrip(req *http.Request) (*http.Response, error) {
	rt.calls++
	if rt.calls > 1 && rt.location != "" {
		return nil, rt.hopErr
	}

	loc := rt.location
	if loc == "" {
		loc = req.URL.String()
	}

	return &http.Response{
		StatusCode: http.StatusTemporaryRedirect,
		Header:     http.Header{"Location": []string{loc}},
		Body:       http.NoBody,
		Request:    req,
	}, nil
}

// The guarded client withholds the server-sent target of every redirect failure,
// in the unit tier (no listener): the redirect limit, also when every hop is the
// original URL; an unparsable Location; a failed hop, keeping a context error's
// identity; a downgrade to http (terminal). A failure without a redirect passes
// through unchanged.
func TestRedactedRedirectClient_withholdsTarget(t *testing.T) {
	t.Parallel()

	const (
		marker   = "sentinel-redirect"
		original = "https://endpoint.invalid/b"
	)

	cases := []struct {
		name     string
		rt       *redirectRT
		want     []error
		terminal bool
	}{
		{"redirect loop on the original URL", &redirectRT{}, []error{ErrRedirectFailed}, false},
		{"unparsable Location", &redirectRT{location: "https://h.invalid/%zz?" + marker}, []error{ErrRedirectFailed}, false},
		{"followed hop times out", &redirectRT{location: "https://other.invalid/" + marker, hopErr: context.DeadlineExceeded},
			[]error{ErrRedirectFailed, context.DeadlineExceeded}, false},
		{"downgrade to http", &redirectRT{location: "http://other.invalid/" + marker}, []error{ErrCAInsecureRedirect}, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client := redactedRedirectClient{&http.Client{Transport: tc.rt, CheckRedirect: refuseInsecureRedirect}}

			req, err := http.NewRequestWithContext(t.Context(), http.MethodHead, original, http.NoBody)
			if err != nil {
				t.Fatal(err)
			}

			resp, err := client.Do(req)
			if resp != nil {
				t.Errorf("response %v returned with the error", resp.Status)
			}

			for _, w := range tc.want {
				if !errors.Is(err, w) {
					t.Errorf("error %v does not wrap %v", err, w)
				}
			}

			if got := kollecterrors.IsTerminal(err); got != tc.terminal {
				t.Errorf("error %v: terminal %v, want %v", err, got, tc.terminal)
			}

			for _, target := range []string{marker, "other.invalid", "h.invalid", original} {
				if err != nil && strings.Contains(err.Error(), target) {
					t.Errorf("error %q names %q", err, target)
				}
			}
		})
	}

	t.Run("failure without a redirect passes through", func(t *testing.T) {
		t.Parallel()

		boom := errors.New("connection refused")
		client := redactedRedirectClient{&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, boom
		}), CheckRedirect: refuseInsecureRedirect}}

		req, err := http.NewRequestWithContext(t.Context(), http.MethodHead, original, http.NoBody)
		if err != nil {
			t.Fatal(err)
		}

		//nolint:bodyclose // the transport fails; there is no response
		if _, err := client.Do(req); !errors.Is(err, boom) || errors.Is(err, ErrRedirectFailed) {
			t.Errorf("error %v; want the transport error unchanged", err)
		}
	})
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }
