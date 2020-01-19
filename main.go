package main

import (
	"bytes"
	"flag"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"time"
)

func forward(frontconn net.Conn, addr string) {
	t := time.Now()
	now := t.Format("20060102_150405.000")

	backconn, err := net.Dial("tcp", addr)
	defer frontconn.Close()
	if err != nil {
		log.Print(err)
		return
	}
	defer backconn.Close()

	reqbuf := &bytes.Buffer{}
	respbuf := &bytes.Buffer{}
	reqreader := io.TeeReader(frontconn, reqbuf)
	respreader := io.TeeReader(backconn, respbuf)

	inch := make(chan bool)
	outch := make(chan bool)
	go func() {
		n, err := io.Copy(backconn, reqreader)
		log.Printf("<-: request %d bytes, err: %v\n", n, err)
		close(inch)
	}()
	go func() {
		n, err := io.Copy(frontconn, respreader)
		log.Printf(":-> response %d bytes, err: %v\n", n, err)
		close(outch)
	}()
	<-inch
	<-outch

	if err = ioutil.WriteFile("teedump/"+now+"_in.txt", reqbuf.Bytes(), 0644); err != nil {
		log.Printf("Write file error: %v\n", err)
	}
	if err = ioutil.WriteFile("teedump/"+now+"_out.txt", respbuf.Bytes(), 0644); err != nil {
		log.Printf("Write file error: %v\n", err)
	}
}

func main() {
	// Parse flag
	lp := flag.String("listen", "8233", "Listen port for serve")
	fp := flag.String("filesrv", "8244", "File server port for watch teedump file")
	be := flag.String("backend", "", "Backend server address to forward")
	flag.Parse()
	if *be == "" {
		flag.Usage()
		return
	}

	// Run file server
	go func() {
		log.Printf("File server listen at %s\n", *fp)
		err := http.ListenAndServe(":"+*fp, http.FileServer(http.Dir("teedump")))
		panic("file server is panic: " + err.Error())
	}()

	// Run forward server
	l, err := net.Listen("tcp", ":"+*lp)
	if err != nil {
		panic(err)
	}
	log.Printf("Listen at %s\n", *lp)
	for {
		conn, err := l.Accept()
		if err != nil {
			log.Println(err)
			continue
		}
		log.Printf("Accept connection from %s, forward to %s\n", conn.RemoteAddr().String(), *be)
		go forward(conn, *be)
	}
}
