// Copyright 2025 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2025 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2025 Department of Linguistics,
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

package wagstream

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/rs/zerolog/log"
)

// testDataForError tries to find json-encoded data from a data stream chunk
// It assumes that error is always encoded in an object under the 'error' key.
// A possible slice of maps (map[string]any) is reported as "no error".
func testDataForError(data string) (string, error) {
	ddata := make(map[string]any)
	var altData []map[string]any
	err := sonic.Unmarshal([]byte(data), &ddata)
	if err != nil {
		err = sonic.Unmarshal([]byte(data), &altData)
		if err != nil {
			return "", fmt.Errorf("testDataForError failed - JSON response type not recognized: %w", err)
		}
		return "", nil // returned list of object is considered "no error" situation
	}
	errMsg, ok := ddata["error"]
	if ok {
		return fmt.Sprintf("%s", errMsg), nil
	}
	return "", nil
}

// findErrorMsgInStream searches for "data:" containing a JSON with the 'error'
// entry. If found, than the entry is considered as having an error (which e.g.
// prevents us from caching the stream).
// Please note that even failing to detect an error is considered an error just
// like the ones returned by backends themselves.
func findErrorMsgInStream(inputData []byte) (string, string) {
	var eventType, data string
	scanner := bufio.NewScanner(bytes.NewReader(inputData))
	for scanner.Scan() {
		line := scanner.Text()

		if line == "" {
			if data != "" {
				errMsg, err := testDataForError(strings.TrimSpace(data))
				if err != nil {
					return fmt.Sprintf("unknown error: %s", strings.TrimSpace(data)), eventType
				}
				if errMsg != "" {
					log.Debug().Str("msg", errMsg).Str("event", eventType).Msg("detected an error in data stream")
					return errMsg, eventType
				}
				data = ""
				eventType = ""
			}
			continue
		}

		if strings.HasPrefix(line, "event:") {
			eventType = strings.TrimSpace(line[6:])

		} else if strings.HasPrefix(line, "data:") {
			if data != "" {
				data += "\n"
			}
			data += strings.TrimSpace(line[5:])

		}
	}
	return "", ""
}
