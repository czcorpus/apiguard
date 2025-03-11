// Copyright 2025 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2025 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2025 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package frodo

import (
	"apiguard/guard"
	"apiguard/guard/token"
	"apiguard/services/cnc"
	"fmt"
)

type Conf struct {
	cnc.ProxyConf
	GuardType       guard.GuardType   `json:"guardType"`
	TokenHeaderName string            `json:"tokenHeaderName"`
	Tokens          []token.TokenConf `json:"tokens"`
}

func (c *Conf) Validate(context string) error {
	if err := c.ProxyConf.Validate(context); err != nil {
		return err
	}
	if c.GuardType == guard.GuardTypeToken && len(c.Tokens) == 0 {
		return fmt.Errorf("no tokens defined for token guard - the service won't be accessible")
	}
	return nil
}
