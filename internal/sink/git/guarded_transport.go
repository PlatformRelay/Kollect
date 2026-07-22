// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

import (
	transportclient "github.com/go-git/go-git/v5/plumbing/transport/client"
	transporthttp "github.com/go-git/go-git/v5/plumbing/transport/http"

	"github.com/platformrelay/kollect/internal/sink/netguard"
)

func init() {
	guarded := transporthttp.NewClient(netguard.HTTPClient(0))
	transportclient.InstallProtocol("http", guarded)
	transportclient.InstallProtocol("https", guarded)
}
