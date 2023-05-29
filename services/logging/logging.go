// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package logging

import (
	"apiguard/monitoring"
	"net/http"

	"github.com/czcorpus/cnc-gokit/unireq"
	"github.com/rs/zerolog/log"
)

func LogServiceRequest(
	req *http.Request,
	bReq *monitoring.BackendRequest,
) {
	event := log.Info().
		Bool("accessLog", true).
		Str("type", "apiguard").
		Str("service", bReq.Service).
		Float64("procTime", bReq.ProcTime).
		Bool("isCached", bReq.IsCached).
		Bool("isIndirect", bReq.IndirectCall).
		Str("ipAddress", unireq.ClientIP(req).String()).
		Str("userAgent", req.UserAgent())
	if bReq.UserID.IsValid() {
		event.Int("userId", int(bReq.UserID))
	}
	event.Send()
}
