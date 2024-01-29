// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package kwords

import (
	"apiguard/alarms"
	"apiguard/cnc/guard"
	"apiguard/ctx"
	"apiguard/services"
	"apiguard/services/cnc"
	"apiguard/services/defaults"
	"errors"
	"net/http"

	"github.com/czcorpus/cnc-gokit/collections"
)

type KWordsProxy struct {
	cnc.CoreProxy
	defaults *collections.ConcurrentMap[string, defaults.Args]
	analyzer *guard.CNCUserAnalyzer
}

func (kp *KWordsProxy) CreateDefaultArgs(reqProps services.ReqProperties) defaults.Args {
	dfltArgs, ok := kp.defaults.GetWithTest(reqProps.SessionID)
	if !ok {
		dfltArgs = defaults.NewServiceDefaults("format", "corpname", "usesubcorp")
		dfltArgs.Set("format", "json")
		kp.defaults.Set(reqProps.SessionID, dfltArgs)
	}
	return dfltArgs
}

func (kp *KWordsProxy) SetDefault(req *http.Request, key, value string) error {
	sessionID := kp.analyzer.GetSessionID(req)
	if sessionID == "" {
		return errors.New("session not found")
	}
	kp.defaults.Get(sessionID).Set(key, value)
	return nil
}

func (kp *KWordsProxy) GetDefault(req *http.Request, key string) (string, error) {
	sessionID := kp.analyzer.GetSessionID(req)
	if sessionID == "" {
		return "", errors.New("session not found")
	}
	return kp.defaults.Get(sessionID).Get(key), nil
}

func (kp *KWordsProxy) GetDefaults(req *http.Request) (defaults.Args, error) {
	sessionID := kp.analyzer.GetSessionID(req)
	if sessionID == "" {
		return map[string][]string{}, errors.New("session not found")
	}
	return kp.defaults.Get(sessionID), nil
}

func NewKWordsProxy(
	globalCtx *ctx.GlobalContext,
	conf *cnc.ProxyConf,
	gConf *cnc.EnvironConf,
	analyzer *guard.CNCUserAnalyzer,
	reqCounter chan<- alarms.RequestInfo,
) *KWordsProxy {
	return &KWordsProxy{
		CoreProxy: *cnc.NewCoreProxy(globalCtx, conf, gConf, analyzer, reqCounter),
		defaults:  collections.NewConcurrentMap[string, defaults.Args](),
	}
}
