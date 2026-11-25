/*
Copyright (C) GRyCAP - I3M - UPV

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package backends

import (
	"fmt"
	"log"

	"github.com/google/go-containerregistry/pkg/name"
)

// parseAllowedRepositories parses an image repository allowlist. Entries that
// are not valid fully qualified registry/repository references are logged and
// skipped so that a malformed entry does not disable the restriction.
func parseAllowedRepositories(allowed []string, policy string) []name.Repository {
	repositories := make([]name.Repository, 0, len(allowed))
	for _, entry := range allowed {
		// Require a fully qualified registry/repository, not an implicit Docker Hub name.
		repo, err := name.NewRepository(entry, name.StrictValidation)
		if err != nil {
			log.Printf("WARNING: skipping malformed allowed image repository %q in %s: %v", entry, policy, err)
			continue
		}
		repositories = append(repositories, repo)
	}
	return repositories
}

// ValidateAllowedImageRepositories checks that every entry is a fully
// qualified image repository reference using strict validation. It returns one
// error message per malformed entry; an empty result means the list is valid.
func ValidateAllowedImageRepositories(allowed []string) []string {
	invalid := make([]string, 0)
	for _, entry := range allowed {
		if _, err := name.NewRepository(entry, name.StrictValidation); err != nil {
			invalid = append(invalid, fmt.Sprintf("invalid allowed image repository %q: %v", entry, err))
		}
	}
	return invalid
}
