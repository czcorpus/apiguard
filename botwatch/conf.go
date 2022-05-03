// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package botwatch

// BotDetection defines important parameters of bot detection module which determine
// how dense and regular request activity is considered as scripted/bot-like.
type BotDetectionConf struct {

	// BotDefsPath is either a local filesystem path or http resource path
	// where a list of bots to ignore etc. is defined
	BotDefsPath string `json:"botDefsPath"`

	// WatchedTimeWindowSecs specifies a time interval during which IP activies are evaluated.
	// In other words - each new record is considered along with older records at most as old
	// as specified by this property
	WatchedTimeWindowSecs int `json:"watchedTimeWindowSecs"`

	// NumRequestsThreshold specifies how many requests must be present during
	// WatchedTimeWindowSecs to treat the series as "bot-like"
	NumRequestsThreshold int `json:"numRequestsThreshold"`

	// RSDThreshold is a relative standard deviation (aka Coefficient of variation)
	// threshold of subsequent request intervals considered as bot-like
	RSDThreshold float64 `json:"rsdThreshold"`
}
