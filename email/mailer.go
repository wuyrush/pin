package email

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
)

// Mailer vends email dispatch logic via SSL/TLS only.
type Mailer struct {
	skipTLSVerify bool // NOTE only set to true during testing
}

// Mail encapsulates email details necessary for dispatch
// When provide sender's authentication, NOTE some mail service(like netease) will use dedicated authorization
// code as secret instead of email account login password
type Mail struct {
	Addr                       string // address of smtp server in host:port format
	From                       mail.Address
	To                         []mail.Address
	Subj, ContentType, Content string    // TODO: make content as an io.Reader to handle input of larger size
	Auth                       smtp.Auth // usually smtp server will publicize the auth extension they support, so let client decide this
}

// Send sends the given email via SSL/TLS.
func (ml *Mailer) Send(m *Mail) error {
	c, err := ml.newTLSClient(m.Addr)
	if err != nil {
		return fmt.Errorf("error creating TLS smtp client: %w", err)
	}
	defer c.Close()
	// TODO: understand the effect of QUIT command - for now we are not sure whether we can always execute it
	// or not
	defer c.Quit()
	// auth
	if m.Auth != nil {
		if ok, exts := c.Extension("AUTH"); ok {
			if err := c.Auth(m.Auth); err != nil {
				return fmt.Errorf("error authenticating client: %w. smtp server at %s supports AUTH extensions [%s]", err, m.Addr, exts)
			}
		} else {
			// never disclose client's auth details to untrusted network
			return fmt.Errorf("smtp server at %s has no auth support", m.Addr)
		}
	}
	// prepare email transaction
	if err := c.Mail(m.From.Address); err != nil {
		return fmt.Errorf("error executing MAIL command with smtp server at %s: %w", m.Addr, err)
	}
	for _, t := range m.To {
		if err := c.Rcpt(t.Address); err != nil {
			return fmt.Errorf("error executing RCPT command with smtp server at %s: %w", m.Addr, err)
		}
	}
	// write actual email content to server
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("error executing DATA command with smtp server at %s: %w", m.Addr, err)
	}
	// close the writer before we can call any other c's methods
	defer w.Close()
	msg := compose(m)
	// DEBUG: log.Println("composed message:", msg)
	if _, err := w.Write([]byte(msg)); err != nil {
		return fmt.Errorf("error writing email data to smtp server at %s: %w", m.Addr, err)
	}
	return nil
}

func (ml *Mailer) newTLSClient(addr string) (*smtp.Client, error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("smtp server address %s is invalid: %w", addr, err)
	}
	tlsCfg := &tls.Config{
		InsecureSkipVerify: ml.skipTLSVerify,
		ServerName:         host,
	}
	conn, err := tls.Dial("tcp", addr, tlsCfg)
	if err != nil {
		return nil, fmt.Errorf("error dialing %s via TLS: %w", addr, err)
	}
	return smtp.NewClient(conn, host)
}

func compose(m *Mail) string {
	headers := map[string]string{
		"From":         m.From.String(),
		"Subject":      m.Subj,
		"Content-Type": m.ContentType,
	}
	var b strings.Builder
	for i, t := range m.To {
		// NOTE msut format the mail address explictly by calling String() otherwise it uses default format
		fmt.Fprint(&b, t.String())
		if i < len(m.To)-1 {
			fmt.Fprint(&b, ",")
		}
	}
	headers["To"] = b.String()
	b.Reset()
	for k, v := range headers {
		fmt.Fprintf(&b, "%s: %s\r\n", k, v)
	}
	fmt.Fprint(&b, "\r\n")
	fmt.Fprint(&b, m.Content)
	return b.String()
}
