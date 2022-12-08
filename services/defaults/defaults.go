// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package defaults

import (
	"fmt"
	"net/url"
)

type Args map[string][]string

func (sd Args) ApplyTo(args url.Values) {
	for k, v := range sd {
		if len(v) > 0 {
			args[k] = v
		}
	}
}

func (sd Args) Set(key, value string) error {
	if _, ok := sd[key]; !ok {
		return fmt.Errorf("key %s not found in service defaults", key)
	}
	sd[key] = []string{value}
	return nil
}

func (sd Args) Get(key string) string {
	v, ok := sd[key]
	if !ok || len(v) == 0 {
		return ""
	}
	return v[0]
}

func NewServiceDefaults(keys ...string) Args {
	ans := make(Args)
	for _, key := range keys {
		ans[key] = make([]string, 0, 1)
	}
	return ans
}
