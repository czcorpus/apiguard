//go:build !closed

package tlmtr

import (
	"apiguard/botwatch"
	"apiguard/globctx"
	"apiguard/guard"
	"apiguard/guard/null"
	"apiguard/telemetry"
)

func New(
	globalCtx *globctx.Context,
	conf *botwatch.Conf,
	telemetryConf *telemetry.Conf,
) (guard.ServiceGuard, error) {
	return &null.Guard{}, nil
}
