// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package mail

import (
	"bytes"
	"fmt"
	"net/smtp"
	"strings"
)

// SendNotification sends a general e-mail notification based on
// a respective monitoring configuration. The 'alarmToken' argument
// can be nil - in such case the 'turn of the alarm' text won't be
// part of the message.
func SendNotification(
	client *smtp.Client,
	sender string,
	receivers []string,
	subject string,
	msgParagraphs ...string,
) error {
	client.Mail(sender)
	for _, rcpt := range receivers {
		client.Rcpt(rcpt)
	}

	wc, err := client.Data()
	if err != nil {
		return err
	}
	defer wc.Close()

	headers := make(map[string]string)
	headers["From"] = sender
	headers["To"] = strings.Join(receivers, ",")
	headers["Subject"] = subject
	headers["MIME-Version"] = "1.0"
	headers["Content-Type"] = "text/html; charset=UTF-8"

	body := ""
	for k, v := range headers {
		body += fmt.Sprintf("%s: %s\r\n", k, v)
	}
	for _, par := range msgParagraphs {
		body += "<p>" + par + "</p>\r\n\r\n"
	}

	buf := bytes.NewBufferString(body)
	_, err = buf.WriteTo(wc)
	return err
}
