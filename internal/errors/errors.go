// SPDX-License-Identifier: MIT
// Copyright (c) 2026 Konrad Heimel

// Package errors provides typed reconcile error classes (ADR-0602).
package errors

import (
	"errors"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// Reconcile error classes — align with metrics label values in internal/metrics.
const (
	ClassTransient = "transient"
	ClassTerminal  = "terminal"
	ClassForbidden = "forbidden"
)

// Sentinel values for errors.Is classification.
var (
	ErrTransient = errors.New("transient")
	ErrTerminal  = errors.New("terminal")
	ErrForbidden = errors.New("forbidden")
)

// ClassError wraps an underlying error with a reconcile class.
type ClassError struct {
	Class error
	Err   error
}

func (e *ClassError) Error() string {
	if e == nil || e.Err == nil {
		return ""
	}

	return e.Err.Error()
}

func (e *ClassError) Unwrap() error {
	if e == nil {
		return nil
	}

	return e.Err
}

func (e *ClassError) Is(target error) bool {
	if e == nil {
		return false
	}

	return errors.Is(e.Class, target)
}

// Transient marks err as retryable (network, throttling, transient sink failure).
func Transient(err error) error {
	if err == nil {
		return nil
	}

	return &ClassError{Class: ErrTransient, Err: err}
}

// Terminal marks err as non-retryable until spec changes (bad config, missing refs).
func Terminal(err error) error {
	if err == nil {
		return nil
	}

	return &ClassError{Class: ErrTerminal, Err: err}
}

// Forbidden marks RBAC/SAR denial (partial degradation).
func Forbidden(err error) error {
	if err == nil {
		return nil
	}

	return &ClassError{Class: ErrForbidden, Err: err}
}

// ClassOf returns the metrics error_class for err (defaults to transient).
func ClassOf(err error) string {
	var ce *ClassError
	if errors.As(err, &ce) && ce.Class != nil {
		switch {
		case errors.Is(ce.Class, ErrTerminal):
			return ClassTerminal
		case errors.Is(ce.Class, ErrForbidden):
			return ClassForbidden
		default:
			return ClassTransient
		}
	}

	if apierrors.IsNotFound(err) || apierrors.IsInvalid(err) || apierrors.IsBadRequest(err) {
		return ClassTerminal
	}

	if apierrors.IsForbidden(err) {
		return ClassForbidden
	}

	return ClassTransient
}

// IsTransient reports whether err should be retried with backoff.
func IsTransient(err error) bool {
	return ClassOf(err) == ClassTransient
}

// IsTerminal reports whether err should stop requeue until spec changes.
func IsTerminal(err error) bool {
	return ClassOf(err) == ClassTerminal
}

// IsForbidden reports whether err is an RBAC/SAR denial.
func IsForbidden(err error) bool {
	return ClassOf(err) == ClassForbidden
}

// ClassifyAPI maps common Kubernetes API errors to typed classes.
func ClassifyAPI(err error) error {
	if err == nil {
		return nil
	}

	switch {
	case apierrors.IsNotFound(err), apierrors.IsInvalid(err), apierrors.IsBadRequest(err):
		return Terminal(err)
	case apierrors.IsForbidden(err):
		return Forbidden(err)
	default:
		return Transient(err)
	}
}

// Format wraps a message around an already-classified error.
func Format(class error, format string, args ...any) error {
	msg := fmt.Errorf(format, args...)
	if class == nil {
		return msg
	}

	return &ClassError{Class: class, Err: msg}
}
