// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package kontext

import (
	"apiguard/alarms"
	"apiguard/ctx"
	"apiguard/guard"
	"apiguard/services"
	"apiguard/services/cnc"
	"apiguard/services/defaults"
	"errors"
	"net/http"

	"github.com/czcorpus/cnc-gokit/collections"
)

type KonTextProxy struct {
	cnc.CoreProxy
	defaults *collections.ConcurrentMap[string, defaults.Args]
	analyzer *guard.CNCUserAnalyzer
}

func (kp *KonTextProxy) CreateDefaultArgs(reqProps services.ReqProperties) defaults.Args {
	dfltArgs, ok := kp.defaults.GetWithTest(reqProps.SessionID)
	if !ok {
		dfltArgs = defaults.NewServiceDefaults("format", "corpname", "usesubcorp")
		dfltArgs.Set("format", "json")
		kp.defaults.Set(reqProps.SessionID, dfltArgs)
	}
	return dfltArgs
}

func (kp *KonTextProxy) SetDefault(req *http.Request, key, value string) error {
	sessionID := kp.analyzer.GetSessionID(req)
	if sessionID == "" {
		return errors.New("session not found")
	}
	kp.defaults.Get(sessionID).Set(key, value)
	return nil
}

func (kp *KonTextProxy) GetDefault(req *http.Request, key string) (string, error) {
	sessionID := kp.analyzer.GetSessionID(req)
	if sessionID == "" {
		return "", errors.New("session not found")
	}
	return kp.defaults.Get(sessionID).Get(key), nil
}

func (kp *KonTextProxy) GetDefaults(req *http.Request) (defaults.Args, error) {
	sessionID := kp.analyzer.GetSessionID(req)
	if sessionID == "" {
		return map[string][]string{}, errors.New("session not found")
	}
	return kp.defaults.Get(sessionID), nil
}

func NewKontextProxy(
	globalCtx *ctx.GlobalContext,
	conf *cnc.ProxyConf,
	gConf *cnc.EnvironConf,
	analyzer *guard.CNCUserAnalyzer,
	reqCounter chan<- alarms.RequestInfo,
) *KonTextProxy {
	return &KonTextProxy{
		CoreProxy: *cnc.NewCoreProxy(globalCtx, conf, gConf, analyzer, reqCounter),
		defaults:  collections.NewConcurrentMap[string, defaults.Args](),
	}
}
