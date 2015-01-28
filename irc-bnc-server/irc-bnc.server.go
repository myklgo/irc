package main

import (
	"bytes"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"private/gates"

	"private/go-ircevent"
)

var (
	debug bool
)

func init() {
	debug = true
}

type tircc struct {
	enable bool //activate or not
	active gates.Gates
	c,
	n,
	u string
	from,
	to chan []byte
	irc   *irc.Connection
	sDate time.Time
}

//ircc.activate() inits and connects an irc/tircc
//do not call until after init()s
func (t *tircc) activate() {
	if !t.enable || t.active.IsOpen() {
		return
	}
	t.active.Open()
	ic := irc.IRC(t.n, t.u)
	ic.VerboseCallbackHandler = true
	ic.Debug = true
	err := ic.Connect(irccs.ircServer)
	if err != nil {
		ic.Quit()
		t.active.Close()
		log.Println(err.Error())
	}
	ic.AddCallback("001",
		func(e *irc.Event) { ic.Join(t.c) })
	t.from = make(chan []byte, 5)
	t.to = make(chan []byte, 5)

	//from Irc
	sp := []byte{' '}
	go func(from <-chan []byte, toAll chan<- []byte, cb, sp []byte) {
		var b []byte
		var ok bool
		for {
			if b, ok = <-from; !ok {
				break
			}
			toAll <- bytes.Join([][]byte{cb, b}, sp)
		}
	}(t.from, clients.to, bytes.TrimPrefix([]byte(t.c), sp), sp)

	//to Irc
	go func(to <-chan []byte, cb []byte) {
		var b []byte
		var ok bool
		for {
			if b, ok = <-to; !ok {
				break
			}
			if b[0] == 'P' {
				t.irc.Privmsg(string(cb), string(b[1:]))
			} else if b[0] == 'N' {
				t.irc.Nick(string(b[1:]))
			} else if b[0] == 'W' {
				t.irc.Whois(string(b[1:]))
			} else if b[0] == 'A' {
				t.irc.Action(string(b), string(b[1:]))
			} else if b[0] == 'o' { //N'o'tice
				t.irc.Notice(string(cb), string(b[1:]))
			} else if b[0] == 'h' { //W'h'o
				t.irc.Who(string(b[1:]))
			}
		}
	}(t.to, []byte(t.c))

	ic.AddCallback("*",
		func(e *irc.Event) {
			t.from <- []byte(e.Raw)
		})
	go ic.Loop()
}
func (t *tircc) deActivate() {
	if t.active.Is0() {
		return
	}
	t.active.Close()
	t.irc.Quit()
	close(t.from)
	close(t.to)
}

type tirccs struct {
	mu   sync.RWMutex
	ircs []*tircc
	ircServer,
	serverIP string
}

func (t *tirccs) addIrcc(v *tircc) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.ircs = append(t.ircs, v)
}

//irccs.activate connection iteration for ircs channel specific
func (t *tirccs) activate(id []byte) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	for i := 0; i < len(t.ircs); i++ {
		if bytes.Equal(id, []byte(t.ircs[i].c)) {
			t.ircs[i].activate()
			break
		}
	}
}

//irccs.activateAll init/ connection iteration for ircs
func (t *tirccs) activateAll() {
	t.mu.RLock()
	defer t.mu.RUnlock()

	for i := 0; i < len(t.ircs); i++ {
		t.ircs[i].activate()
	}
}
func (t *tirccs) deActivate(id []byte) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	for i := 0; i < len(t.ircs); i++ {
		if bytes.Equal(id, []byte(t.ircs[i].c)) {
			t.ircs[i].deActivate()
			break
		}
	}
}
func (t *tirccs) deActivateAll() {
	for i := 0; i < len(t.ircs); i++ {
		t.ircs[i].deActivate()
	}
}
func (t *tirccs) to(id, msg []byte) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	for i := 0; i < len(t.ircs); i++ {
		if bytes.Equal(id, []byte(t.ircs[i].c)) && t.ircs[i].active.IsOpen() {
			t.ircs[i].to <- msg
			break
		}
	}
}
func (t *tirccs) toAll(msg []byte) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	for i := 0; i < len(t.ircs); i++ {
		if t.ircs[i].active.IsOpen() {
			t.ircs[i].to <- msg
		}
	}
}

var irccs tirccs

func init() {
	irccs.ircServer = "irc.freenode.net:6667"
	irccs.serverIP = "127.0.0.1:20000"
	irccs.addIrcc(&tircc{
		enable: false,
		c:      "#go-nuts",
		n:      "go-bnc",
		u:      "go-bncTest",
	})
	irccs.addIrcc(&tircc{
		enable: false,
		c:      "#rust",
		n:      "rust-bnc",
		u:      "rust-bncTest",
	})
	irccs.addIrcc(&tircc{
		enable: false,
		c:      "#freenode",
		n:      "irc-bnc",
		u:      "irc-bncTest",
	})
	irccs.addIrcc(&tircc{
		enable: true,
		c:      "#test",
		n:      "test-bnc",
		u:      "test-bncTst",
	})
}

type client struct {
	active gates.Gates
	lastMsg,
	lastOn time.Time
	id []byte
	to,
	from chan []byte
	cs []int
	silent,
	msgs,
	errs,
	cmds,
	logs,
	admin bool
	conn net.Conn
}

func (t *client) activate(c net.Conn) {
	if t.active.IsOpen() {
		log.Println("Error: client active, ", string(t.id))
		return
	}
	t.to = make(chan []byte, 5)
	t.from = make(chan []byte, 5)
	t.conn = c
	t.ioFrom()
	t.ioTo()
	t.active.Open()
}
func (t *client) deActivate() {
	if t.active.Is0() {
		log.Println("Error: client not active, ", string(t.id))
		return
	}
	t.active.Close()
	close(t.to)
	close(t.from)
	t.conn.Close()
}
func (t *client) ioFrom() {
	go func() {
		b := make([]byte, 576)
		var n int
		//var il int
		var err error
		//var yes bool
		sp := []byte{' '}
		for {
		start:
			if n, err = t.conn.Read(b); err != nil {
				log.Println("Error: client conn.Read")
				break
			}
			if n < 3 {
				log.Println(string(b))
				break
			}
			bb := make([]byte, n)
			copy(bb, b[:n])

			//irc commands
			if bytes.HasPrefix(bb, []byte("/quit")) {
				t.deActivate()
				return
			} else {
				i := 0
				for i = 0; i < len(irccs.ircs); i++ {
					if bytes.HasPrefix(bb, []byte(irccs.ircs[i].c)) {
						break
					}
				}
				if i < len(irccs.ircs) && irccs.ircs[i].active.IsOpen() {
					irccs.ircs[i].irc.Privmsg(irccs.ircs[i].n, string(bb[bytes.Index(bb, sp)+1:]))
				} else {
					log.Println("Error !channel")
					goto start
				}

			}
			log.Print(string(bb))

		}
	}()
}
func (t *client) ioTo() {
	go func(to <-chan []byte) {
		var b []byte
		var ok bool
		for {
			if b, ok = <-to; !ok {
				log.Print("Error: chan to client not ok")
				break
			}
			if _, err := t.conn.Write(b); err != nil {
				log.Println("ioTo", string(b))
				break
			}
			t.lastMsg = time.Now()
		}
	}(t.to)
}

type tclients struct {
	cs        []*client
	to        chan []byte
	activateq chan net.Conn
}

func (t *tclients) add(v *client) {
	t.cs = append(t.cs, v)
}
func (t *tclients) toAll() {
	go func(to <-chan []byte) {
		var b []byte
		var ok bool
		for {
			if b, ok = <-to; !ok {
				log.Print("clients.cast error")
				break
			}
			for i := 0; i < len(clients.cs); i++ {
				if t.cs[i].active.IsOpen() && t.cs[i].msgs { //&& check if wants the channel
					t.cs[i].to <- b
				}
			}
		}
	}(t.to)
}
func (t *tclients) activate() {
	t.activateq = make(chan net.Conn, 1)
	go func(q <-chan net.Conn) {
		b := make([]byte, 512)
		var c net.Conn
		var ok bool
		var err error
		var n int
		for {
			if c, ok = <-q; !ok {
				log.Println("Error: Client activate read q not ok")
				break
			}
			/*if err := c.SetReadDeadline(time.Now().Add(time.Second * 3)); err != nil {
				log.Println("Error: client.activate read deadline")
				c.Close()
				break
			}*/
			//log.Println("c <-q")
			if n, err = c.Read(b); err != nil && err != io.EOF {
				log.Println("Error: client.activate read")
				c.Close()
				break
			}
			//c.SetReadDeadline(time.Now().Add(time.Hour))
			bb := make([]byte, n)
			copy(bb, b)
			for i := 0; i < len(t.cs); i++ {
				if bytes.Equal(bb, t.cs[i].id) {
					t.cs[i].activate(c)
					break
				}
			}
		}
	}(t.activateq)
}
func (t *tclients) deActivate(id []byte) {
	for i := 0; i < len(t.cs); i++ {
		if bytes.Equal(id, t.cs[i].id) {
			t.cs[i].deActivate()
			break
		}
	}
}
func (t *tclients) deActivateAll() {
	for i := 0; i < len(t.cs); i++ {
		t.cs[i].deActivate()
	}
}

var clients tclients

func init() {
	clients.add(&client{
		lastMsg: time.Now(),
		id:      []byte("0123456789\n"),
		to:      make(chan []byte, 5),
		from:    make(chan []byte, 5),
		msgs:    true,
		cs:      []int{0, 1, 2},
	})
	clients.add(&client{
		lastMsg: time.Now(),
		id:      []byte("01234567db\n"),
		to:      make(chan []byte, 5),
		from:    make(chan []byte, 5),
		msgs:    true,
		cs:      []int{0, 1, 2},
	})
	clients.to = make(chan []byte, 5)
	clients.toAll()
	clients.activate()
}

var hangUp chan struct{}

func init() {
	hangUp = make(chan struct{})
}
func main() {
	irccs.activateAll()
	listen()
	<-hangUp //and clean up
	close(hangUp)
	clients.deActivateAll()
	irccs.deActivateAll()
}

func listen() {
	// Listen on TCP port 2000 on all interfaces.
	var l net.Listener
	var err error
	if l, err = net.Listen("tcp", irccs.serverIP); err != nil {
		log.Fatal(err)
	}
	go func(l net.Listener) {
		defer l.Close()
		for {
			// Wait for a connection.
			conn, err := l.Accept()
			if err != nil {
				log.Println(err)
				return
			}
			println("accepted")
			//Handle the connection.
			clients.activateq <- conn
		}
		println("SERVER DEBUG: l close")
	}(l)
}
