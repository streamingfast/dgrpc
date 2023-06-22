// Copyright 2019 dfuse Platform Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package connectweb

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestContentTypeFilter(t *testing.T) {

	tests := []struct {
		in         http.Header
		expectPass bool
	}{
		{
			http.Header{"Content-Type": []string{"application/json"}},
			false,
		},
		{
			http.Header{"Content-Type": []string{"application/grpc+proto"}},
			true,
		},
		{
			http.Header{"Content-Type": []string{"application/grpc"}},
			true,
		},
		{
			http.Header{"Content-Type": []string{"application/connect+json"}},
			false,
		},
	}
	for i, c := range tests {

		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			cti := ContentTypeInterceptor{}
			err := cti.checkContentType(c.in)
			if c.expectPass {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}

}
