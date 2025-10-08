// Copyright 2024 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2024 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2024 Department of Linguistics,
//                Faculty of Arts, Charles University
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

package hex

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/czcorpus/apiguard/proxy"
	"github.com/czcorpus/apiguard/services/cnc"

	"github.com/rs/zerolog/log"
)

type Conf = cnc.ProxyConf

// Interceptor function is used to handle specific Hex behavior
// in case a provided word is not found. In such case, a simple
// http client may end up in an infinite loop (APIGuard by default
// would end up with status 429 (too many requests)).
// The function searches for specific Location in headers where the
// URL contains a message that the word was not found. In such case
// we rewrite the response to provide an empty result.
func Interceptor(pr *proxy.BackendProxiedResponse) {
	// we must find and handle the following redirect:
	// index.php?msg=Nenalezeny žádné výsledky pro dotaz 'rouhat'
	// ... to prevent infinite redirect loop and make sure something
	// valid is returned.
	location := pr.Headers.Get("location")
	if location != "" {
		locUrl, err := url.Parse(location)
		if err != nil {
			log.Error().
				Err(err).
				Msg("failed to parse header Location from Hex backend, ignoring")
			return
		}
		msg := locUrl.Query().Get("msg")
		if strings.HasPrefix(strings.ToLower(msg), "nenalezeny") {
			fakePage := []byte("<html><body><script>\n" +
				"var hex = " +
				"{\"size\": {}, \"count\": 0, \"countY\": {}, \"table\": [], \"sorting\": {}};\n" +
				"</script></body></html>\n")
			pr.StatusCode = http.StatusOK
			pr.Headers.Del("location")
			pr.Headers.Del("content-encoding")
			pr.Headers.Set("content-length", fmt.Sprintf("%d", len(fakePage)))
			pr.BodyReader = io.NopCloser(bytes.NewReader(fakePage))
		}

	} else {
		defer pr.BodyReader.Close()
		body, err := io.ReadAll(pr.BodyReader)
		if err != nil {
			log.Error().
				Err(err).
				Msg("failed to read response body")
			return
		}
		srch := bytes.Index(body, []byte{'v', 'a', 'r', ' ', 'h', 'e', 'x'})
		if srch > 0 {
			pr.BodyReader = io.NopCloser(bytes.NewReader(body[srch-5:]))
		}
	}
}
