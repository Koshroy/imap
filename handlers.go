package imap

import (
	"bytes"
	"io"
	"log"
	"strings"
)

type ImapReply struct {
	Status io.Writer
	Response io.Writer
	Auth bool
}

type Handler interface {
	ImapCommand(arg string, reply *ImapReply)
}

type funcHandler struct {
	handleFunc func(string, *ImapReply)
}

func (f *funcHandler) ImapCommand(arg string, reply *ImapReply) {
	f.handleFunc(arg, reply)
}

func newFuncHandler(hFunc func(string, *ImapReply)) (f *funcHandler) {
	return &funcHandler{handleFunc: hFunc}
}

type CommandNotFoundHandler struct{}

func (cf *CommandNotFoundHandler) ImapCommand(arg string, reply *ImapReply) {
	reply.Response.Write([]byte("BAD error in IMAP command received by server"))
}

type AuthHandler interface {
	Authenticate(username string, password string) bool
}

type DummyAuthHandler struct{}

func (a *DummyAuthHandler) Authenticate(username string, password string) bool {
	return false
}

type ServeMux struct {
	//Handle(command string, handler Handler)
	//HandleFunc(command string, handler func(string, io.Writer))
	//Handler(command string) (h Handler, reqString string) -- needed?
	//ServeImap(inputCommand string, w io.Writer)
	handlerTable map[string]Handler
}

func (s *ServeMux) Handle(command string, handler Handler) {
	s.handlerTable[command] = handler
}

func (s *ServeMux) HandleFunc(command string, handler func(string, *ImapReply)) {
	s.handlerTable[command] = newFuncHandler(handler)
}

func (s *ServeMux) ImapCommand(inputCommand string, reply *ImapReply) {
	inputCommandSplit := strings.Split(inputCommand, " ")
	h, ok := s.handlerTable[strings.ToLower(inputCommandSplit[0])]
	if !ok {
		c := &CommandNotFoundHandler{}
		c.ImapCommand(inputCommand, reply)
	} else {
		h.ImapCommand(inputCommand, reply)
	}
}

func NewServeMux() *ServeMux {
	return &ServeMux{handlerTable: make(map[string]Handler)}
}

func validTag(tag string) bool {
	for _, b := range []byte(tag) {
		// check if bytes in the tag are alphanumeric
		if !((b >= '0' && b <= '9') || (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z')) {
			return false
		}
	}
	return true
}

func handleConn(session *Session) {
	for {
		if session.closed {
			return
		}
		l, err := session.ReadLine()
		if err != nil {
			if !session.closed {
				log.Println(err)
			}
			return
		}
		log.Println("received line", "["+l+"]")

		// inserting ctrl+c into session causes session to break
		var statusBuf, respBuf bytes.Buffer
		recvTagEnd := strings.Index(l, " ") - 1
		if recvTagEnd < 0 {
			log.Println("no tag in imap command found")
			c := &CommandNotFoundHandler{}
			reply := &ImapReply{Status: &statusBuf, Response: &respBuf, Auth: session.IsAuth()}
			c.ImapCommand(l, reply)
			if err != nil {
				log.Println(err)
			}
		} else {
			splitL := strings.Split(l, " ")
			// IMAP commmands are case insensitive
			if len(splitL) >= 2 {
				splitL[1] = strings.ToLower(splitL[1])
			}
			tagStr := splitL[0]
			reply := &ImapReply{Status: &statusBuf, Response: &respBuf, Auth: session.IsAuth()}
			if !validTag(tagStr) {
				log.Println("imap tag is not valid")
				c := &CommandNotFoundHandler{}
				c.ImapCommand(l, reply)
			} else {
				if splitL[1] == "auth" {
					session.Authenticate(splitL[1], splitL[2])
				} else {
					session.srvMux.ImapCommand(strings.Join(splitL[1:], " "), reply)
				}
			}
			if statusBuf.Len() > 0 {
				err = session.WriteStatus(statusBuf.String())
				if err != nil {
					log.Println(err)
				}
			}

			if respBuf.Len() > 0 {
				err = session.WriteLineTag(tagStr, respBuf.String())
				if err != nil {
					log.Println(err)
				}
			}
			
			
		}
	}
}
