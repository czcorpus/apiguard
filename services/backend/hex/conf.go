// Copyright 2024 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2024 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2024 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package hex

import (
	"apiguard/proxy"
	"apiguard/services/cnc"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

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
func Interceptor(pr *proxy.ProxiedResponse) {
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
