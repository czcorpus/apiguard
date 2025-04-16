// Copyright 2025 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2025 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2025 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package wagstream

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/rs/zerolog/log"
)

func testDataForError(data string) (string, error) {
	ddata := make(map[string]any)
	if err := sonic.Unmarshal([]byte(data), &ddata); err != nil {
		return "", err
	}
	errMsg, ok := ddata["error"]
	if ok {
		return fmt.Sprintf("%s", errMsg), nil
	}
	return "", nil
}

// findErrorMsgInStream searches for "data:" containing a JSON with the 'error'
// entry. If found, than the entry is considered as having an error (which e.g.
// prevents us from caching the stream)
func findErrorMsgInStream(inputData []byte) (string, string, error) {
	var eventType, data string
	scanner := bufio.NewScanner(bytes.NewReader(inputData))
	for scanner.Scan() {
		line := scanner.Text()

		if line == "" {
			if data != "" {
				errMsg, err := testDataForError(strings.TrimSpace(data))
				if err != nil {
					log.Error().Err(err).Msg("failed to read error information in wagstream")
					continue
				}
				if errMsg != "" {
					log.Debug().Str("msg", errMsg).Str("event", eventType).Msg("detected an error in data stream")
					return errMsg, eventType, nil
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
	return "", "", nil
}
