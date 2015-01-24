package main

import (
	//"bufio"
	"bytes"
	"io"
	"log"
	"net"
	"os"
	"sync"
)

var (
	wg sync.WaitGroup
)

func init() {
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
	uwc := toUser()
	wg.Add(1)
	swc := toServer(conn)
	wg.Add(1)
	go fromServer(conn, uwc)
	wg.Add(1)
	go fromUser(swc)
	//wg.Add(1)
	//logic(src,swc,urc,uwc)
	swc <- []byte("0123456789\n")
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

func toServer(c net.Conn) chan []byte {
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

func fromServer(c net.Conn, uwc chan []byte) {
	defer wg.Done()
	b := make([]byte, 576)
	var n int
	var err error
	for {
	start:
		if n, err = c.Read(b); err != nil && err != io.EOF {
			log.Println("Error: fromServer Read")
			break
		}
		if err == io.EOF {
			goto start
		}
		bb := make([]byte, n, n)
		copy(bb, b[:n])
		uwc <- bb
	}
	log.Println("fs")
}

func fromUser(swc chan []byte) {
	defer wg.Done()
	b := make([]byte, 1, 1)
	bs := make([]byte, 640)
	var err error
	i := 0
stop:
	for {
	start:
		i = 0
		for {
			if _, err = os.Stdin.Read(b); err != nil {
				break stop
			}
			if err == io.EOF {
				log.Println("from User chan eof")
				goto start
			}
			bs[i] = b[0]
			if b[0] == '\n' {
				break
			}
			i++
		}
		bb := make([]byte, i, i)
		copy(bb, bs[:i])
		//ch <- bb//parse
		log.Print("client from user ", string(bb))
		swc <- bb
	}
	log.Println("fu")
}

func toUser() chan []byte {
	ch := make(chan []byte, 3)
	go func(ch <-chan []byte) {
		defer wg.Done()
		egray := []byte("\033[37m")
		eblue := []byte("\033[34m")
		eltgreen := []byte("\033[92m")
		ereset := []byte("\033[0m")
		//timefmt := "15:04:05.000 "
		lf := []byte{'\n'}
		sp := []byte{' '}
		em := []byte{'!'}
		cln := []byte{':'}
		//qt := []byte("QUIT")
		//pt := []byte("PART")
		//jn := []byte("JOIN")
		aQ := make([][]byte, 8)
		aQ[0] = egray //1 = server
		aQ[2] = sp    //3 = who
		aQ[4] = sp    //5 = what
		aQ[6] = ereset
		aQ[7] = lf
		aP := make([][]byte, 8)
		aP[0] = egray
		aP[2] = eltgreen
		aP[4] = sp
		aP[5] = ereset
		aP[7] = lf
		aN := make([][]byte, 10)
		aN[0] = egray
		aN[2] = sp
		aN[3] = eblue
		aN[4] = []byte("NICK ")
		aN[6] = []byte(" -> ")
		aN[8] = ereset
		aN[9] = lf
		aA := make([][]byte, 7)
		aA[0] = egray
		aA[3] = sp
		aA[5] = ereset
		aA[6] = lf
		//var i, i1, ia, im int
		var bb []byte
		var ok bool
		for {
		start:
			if bb, ok = <-ch; !ok {
				log.Println("toUser chan not ok")
				return
			}
			
			//log.Println("\nclient toUser ", string(bb))
			ssb := bytes.SplitN(bb, sp, 4)
			/*   			for _, v := range ssb {
			   				log.Println(string(v))
			   			}
			   			log.Println("ssb len ",len(ssb))
			*/
			if bytes.Equal([]byte("PING"), ssb[0]) {
				//handle PING
				goto start
			} else if bytes.Equal([]byte("QUIT"), ssb[0]) {
				//handle QUIT
				goto start
			} else if bytes.Equal([]byte("TIME"), ssb[0]) {
				//handle TIME
				goto start
			} else if bytes.Equal([]byte("PING"), ssb[1]) {
				//handle PING
				goto start
			} else if len(ssb) < 3 || ssb[1][0] != ':' {
				log.Println("Error: toUser malformed message. ", string(bb))
				goto start
			} else if bytes.Equal([]byte("QUIT"), ssb[2]) || bytes.Equal([]byte("JOIN"), ssb[2]) ||
				bytes.Equal([]byte("PART"), ssb[2]) {
				aQ[1] = ssb[0]
				aQ[3] = ssb[1][1:bytes.Index(ssb[1], em)]
				aQ[5] = ssb[2]
				os.Stdout.Write(bytes.Join(aQ, nil))
				goto start
			} else if bytes.Equal([]byte("NICK"), ssb[2]) {
				aN[1] = ssb[0]
				aN[4] = ssb[1][1:bytes.Index(ssb[1], em)]
				aN[6] = ssb[3][1:]
				os.Stdout.Write(bytes.Join(aN, nil))
				goto start
			} else if bytes.Equal([]byte("NOTICE"), ssb[1]) {
				//handle NOTICE
				goto start
			} else if bytes.Equal([]byte("ACTION"), ssb[1]) {
				//handle ACTION
				goto start
			} else if bytes.Equal([]byte("PRIVMSG"), ssb[2]) {
				aP[1] = ssb[0]
				aP[3] = ssb[1][1:bytes.Index(ssb[1], em)]
				aP[6] = ssb[3][bytes.LastIndex(ssb[2], cln)+1:]
				os.Stdout.Write(bytes.Join(aP, nil))
				goto start
			} else {
				log.Println("fell through")
			}
		}
		log.Println("ts")
	}(ch)
	return ch
}

/*func logic() {
	defer wg.Done()
	for {
		select {
		case m := <-src:
			log.Print(m) //should be parsed
		case m := <-urc:
			log.Print(m)
			switch {
			case bytes.Equal(m, []byte("qq\n")):
				return
			default:
				//swc <- m
			}

		}
	}
	log.Println("lL")
	return
}*/
