package mailutil

import "fmt"
import "time"
import "net/mail"
import "net/smtp"

func SendHtmlMessage(from, to mail.Address, subject, body string) (err error) {
	// Describe the mail headers:
	headers := make(map[string]string)
	headers["From"] = from.String()
	headers["To"] = to.String()
	headers["Subject"] = subject
	headers["Date"] = time.Now().Format(time.RFC1123Z)
	// Use 'text/plain' as an alternative.
	headers["Content-Type"] = `text/html; charset="UTF-8"`

	// Build the formatted message body:
	message := ""
	for k, v := range headers {
		message += fmt.Sprintf("%s: %s\r\n", k, v)
	}
	message += "\r\n" + body

	// Deliver email:
	err = smtp.SendMail("localhost:25", nil, from.Address, []string{to.Address}, []byte(message))
	return
}
