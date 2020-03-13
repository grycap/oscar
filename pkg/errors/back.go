// Copyright (C) GRyCAP - I3M - UPV
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package errors

import (
	"errors"
	"fmt"
)

type backError struct {
	err        error
	statusCode int
}

// BackError interface to represent ServerlessBackend errors
type BackError interface {
	error
	Status() int
}

// NewBackError returns a new ServerlessBackend error
func NewBackError(statusCode int, msg string) BackError {
	return &backError{
		err:        errors.New(msg),
		statusCode: statusCode,
	}
}

// NewBackErrorf returns a new ServerlessBackend error with formatted message
func NewBackErrorf(statusCode int, msg string, args ...interface{}) BackError {
	return &backError{
		err:        fmt.Errorf(msg, args...),
		statusCode: statusCode,
	}
}

// Error implements the error interface
func (b *backError) Error() string {
	return b.err.Error()
}

// Status returns the error status code
func (b *backError) Status() int {
	return b.statusCode
}
