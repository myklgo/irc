package main

import (
	//"bufio"
	"bytes"
	"io"
	"io/ioutil"
	"log"
	"net"
	"strconv"
	"sync"
	"time"
)

var (
	wg    sync.WaitGroup
	debug bool
)

func init() {
	debug = true
}

func main() {
	var conn net.Conn
	var ok bool
	if conn, ok = connect(); !ok {
		log.Println("Connection failed. Terminating.")
		return
	}
	defer conn.Close()
	wg.Add(1)
	toDB := intoDB()
	wg.Add(1)
	toServer := intoServer(conn)
	wg.Add(1)
	go fromServer(conn, toDB)
	//wg.Add(1)
	//logic(src,swc,urc,uwc)
	toServer <- []byte("01234567db\n")
	wg.Wait()
}

func connect() (net.Conn, bool) {
	var err error
	conn, err := net.Dial("tcp", "127.0.0.1:20000")
	if err != nil {
		log.Println(err)
		return nil, false
	}
	return conn, true
}

//Change to record and respond to history requests
func intoServer(c net.Conn) chan []byte {
	ch := make(chan []byte, 3)
	go func(c net.Conn, ch <-chan []byte) {
		defer wg.Done()
		var b []byte
		var ok bool
		var err error
		for {
			if b, ok = <-ch; !ok {
				break
			}
			if _, err = c.Write(b); err != nil {
				break
			}
		}
		log.Println("ts")
	}(c, ch)
	return ch
}

func fromServer(c net.Conn, toDB chan<- []byte) {
	defer wg.Done()
	b := make([]byte, 576)
	var n int
	var err error
	for {
	start:
		if n, err = c.Read(b); err != nil && err != io.EOF {
			log.Println("Error: fromServer Read ", err.Error())
			break
		}
		if err == io.EOF {
			goto start
		}
		bb := make([]byte, n, n)
		copy(bb, b[:n])
		toDB <- bb
	}
	log.Println("fs")
}

type tmsg struct {
	when int64
	raw  []byte
}

func intoDB() chan []byte {
	ch := make(chan []byte, 3)
	go func(toDB <-chan []byte) {
		defer wg.Done()
		var tmsgs []*tmsg
		ti := time.Now()
		yd := (ti.Year() * 1000) + ti.YearDay()
		var m []byte
		var ok bool
		var yd2 int
		for {
			if m, ok = <-toDB; !ok {
				log.Panicln("Error: DB fromIrc, ", m)
				return
			}
			ti = time.Now()
			yd2 = (ti.Year() * 1000) + ti.YearDay()
			if yd2 != yd {
				yd = yd2
				saveTmsgs(yd, tmsgs)
				tmsgs = make([]*tmsg, 0)
			}
			t := &tmsg{when: ti.Unix(), raw: m}
			tmsgs = append(tmsgs, t)
			log.Println(t.when, " ", string(t.raw))
		}
	}(ch)
	return ch
}

func saveTmsgs(yd int, msgs []*tmsg) {
	if len(msgs) == 0 {
		return
	}
	go func(yd int, msgs []*tmsg) {
		bmsgs := make([][]byte, len(msgs), len(msgs))
		ii := 0
		for i := 0; i < len(msgs); i++ {
			bmsgs[ii] = []byte(strconv.FormatInt(msgs[i].when, 10))
			ii++
			bmsgs[ii] = msgs[i].raw
			ii++
		}
		ioutil.WriteFile("msgs"+strconv.Itoa(yd)+".dbw", bytes.Join(bmsgs, []byte{'\x02'}), 0644)
	}(yd, msgs)
}
