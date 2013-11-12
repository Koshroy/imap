package imap

import (
	"bufio"
	"crypto/tls"
	"log"
	"net"
	"strings"
	"time"
)

type Server struct {
	Addr      string
	TLSConfig *tls.Config
}

type Session struct {
	auth       bool
	closed     bool
	conn       net.Conn
	srvMux     *ServeMux
	authHandle AuthHandler
	lineBuff   *bufio.ReadWriter
	timeout    time.Duration
}

var DefaultServeMux = NewServeMux()
var DefaultAuthHander = &DummyAuthHandler{}

func (s *Session) setReadTimeout(dur time.Duration) {
	s.timeout = dur
}

func (s *Session) ReadLine() (string, error) {
	s.conn.SetReadDeadline(time.Now().Add(s.timeout))
	l, err := s.lineBuff.ReadString('\n')
	s.conn.SetReadDeadline(time.Time{})
	if err != nil {
		return "", err
	}
	nl := strings.Replace(l, "\r\n", "", -1)
	return nl, nil
}

func (s *Session) WriteLine(writeStr string) error {
	_, err := s.lineBuff.WriteString(writeStr + "\n")
	if err != nil {
		return err
	}
	s.lineBuff.Flush()
	return nil
}

func (s *Session) WriteLineTag(tag string, writeStr string) error {
	return s.WriteLine(tag + " " + writeStr)
}

func (s *Session) WriteStatus(writeStr string) error {
	return s.WriteLineTag("*", writeStr)
}

func (s *Session) AuthHandler() AuthHandler {
	return s.authHandle
}

func (s *Session) SetAuthHandler(a AuthHandler) {
	s.authHandle = a
}

func (s *Session) Authenticate(username string, password string) {
	s.auth = s.authHandle.Authenticate(username, password)
}

func (s *Session) ServeMux() *ServeMux {
	return s.srvMux
}

func (s *Session) SetServeMux(m *ServeMux) {
	s.srvMux = m
}

func (s *Session) IsAuth() bool {
	return s.auth
}

func NewSession(conn net.Conn, timeout time.Duration) *Session {
	lr := bufio.NewReader(conn)
	lw := bufio.NewWriter(conn)
	return &Session{auth: false, closed: false, conn: conn, lineBuff: bufio.NewReadWriter(lr, lw),
		timeout: timeout, srvMux: DefaultServeMux, authHandle: DefaultAuthHander}
}

func parseAddrProto(addr string) string {
	// TODO: actual protocol detection based on addr
	return "tcp4"
}

func Serve(l net.Listener) error {
	return ServeWithTimeout(l, 300)
}

func ServeWithTimeout(l net.Listener, timeout time.Duration) error {
	for {
		log.Println("waiting for connections on", l.Addr())
		conn, e := l.Accept()
		if e != nil {
			l.Close()
			return e
		}
		s := NewSession(conn, timeout*time.Second)
		go handleConn(s)
	}
}

func ListenAndServe(addr string) error {
	l, err := net.Listen(parseAddrProto(addr), addr)
	if err != nil {
		return err
	}
	Serve(l)
	return nil
}

func Handle(command string, handle Handler) {
	DefaultServeMux.Handle(command, handle)
}

func HandleFunc(command string, handler func(string, *ImapReply)) {
	DefaultServeMux.HandleFunc(command, handler)
}
