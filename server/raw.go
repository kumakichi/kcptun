package main

import (
	"io"
	"log"
	"net"

	"github.com/xtaci/kcptun/generic"
)

func serveRaw(config *Config) {
	logln := func(v ...interface{}) {
		if !config.Quiet {
			log.Println(v...)
		}
	}

	rawListener, err := net.Listen("tcp", config.Listen)
	logln("raw tcp listen on:", config.Listen)
	if err != nil {
		log.Fatal(err)
	}

	for {
		c, err := rawListener.Accept()
		if err != nil {
			logln("raw tcp accept error:", err)
			continue
		}
		logln("raw tcp remote address:", c.RemoteAddr())

		go func(p1 net.Conn) {
			defer p1.Close()
			p2, err := net.Dial("tcp", config.Target)
			if err != nil {
				logln("raw dial direct to target fail:", err)
				return
			}
			defer p2.Close()

			streamCopy := func(dst io.Writer, src io.ReadCloser) {
				if _, err := generic.Copy(dst, src); err != nil {
					logln("generic copy err:", err)
				}
				p1.Close()
				p2.Close()
			}
			go streamCopy(p2, p1)
			streamCopy(p1, p2)
		}(c)
	}
}
