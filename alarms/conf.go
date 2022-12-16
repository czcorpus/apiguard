// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package alarms

type Limit struct {
	ReqPerTimeThreshold     int `json:"reqPerTimeThreshold"`
	ReqCheckingIntervalSecs int `json:"reqCheckingIntervalSecs"`
}

type AlarmConf struct {
	Recipients []string `json:"recipients"`
}

type MailConf struct {
	Sender              string `json:"sender"`
	SMTPServer          string `json:"smtpServer"`
	SmtpUsername        string `json:"smtpUsername"`
	SmtpPassword        string `json:"smtpPassword"`
	ConfirmationBaseURL string `json:"confirmationBaseURL"`
}
