// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

package git

const (
	TypeName    = "git"
	schemeHTTP  = "http"
	schemeHTTPS = "https"
	schemeSSH   = "ssh"
	schemeFile  = "file"

	defaultCommitMessage = "kollect: export inventory"
	defaultAuthorName    = "kollect"
	defaultAuthorEmail   = "kollect@kollect.dev"
	defaultCloneDepth    = 1
)

type PushPolicy string

const (
	PushPolicyCommit    PushPolicy = "Commit"
	PushPolicyForcePush PushPolicy = "ForcePush"
)

type AuthType string

const (
	AuthTypeToken AuthType = "token"
	AuthTypeSSH   AuthType = "ssh"
)
