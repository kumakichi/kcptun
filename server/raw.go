package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"

	"github.com/xtaci/kcptun/generic"
)

const ( // cmds
	// protocol version 1:
	cmdSYN byte = iota // stream open
	cmdFIN             // stream close, a.k.a EOF mark
	cmdPSH             // data push
	cmdNOP             // no operation

	// protocol version 2 extra commands
	// notify bytes consumed by remote peer-end
	cmdUPD
)

const (
	// data size of cmdUPD, format:
	// |4B data consumed(ACK)| 4B window size(WINDOW) |
	szCmdUPD = 8
)

const (
	// initial peer window guess, a slow-start
	initialPeerWindow = 262144
)

const (
	sizeOfVer    = 1
	sizeOfCmd    = 1
	sizeOfLength = 2
	sizeOfSid    = 4
	headerSize   = sizeOfVer + sizeOfCmd + sizeOfSid + sizeOfLength
)

// Frame defines a packet from or to be multiplexed into a single connection
type Frame struct {
	ver  byte
	cmd  byte
	sid  uint32
	data []byte
}

func newFrame(version byte, cmd byte, sid uint32) Frame {
	return Frame{ver: version, cmd: cmd, sid: sid}
}

type rawHeader [headerSize]byte

func (h rawHeader) Version() byte {
	return h[0]
}

func (h rawHeader) Cmd() byte {
	return h[1]
}

func (h rawHeader) Length() uint16 {
	return binary.LittleEndian.Uint16(h[2:])
}

func (h rawHeader) StreamID() uint32 {
	return binary.LittleEndian.Uint32(h[4:])
}

func (h rawHeader) String() string {
	return fmt.Sprintf("Version:%d Cmd:%d StreamID:%d Length:%d",
		h.Version(), h.Cmd(), h.StreamID(), h.Length())
}

type updHeader [szCmdUPD]byte

func (h updHeader) Consumed() uint32 {
	return binary.LittleEndian.Uint32(h[:])
}
func (h updHeader) Window() uint32 {
	return binary.LittleEndian.Uint32(h[4:])
}

func checkKCP(conn net.Conn, version ...byte) (isKCP bool, hdr [2]byte, n int, err error) {
	if n, err = io.ReadFull(conn, hdr[:]); err != nil {
		return
	}

	if hdr[0] != 1 && hdr[0] != 2 {
		return
	}

	if len(version) > 0 && hdr[0] != version[0] {
		return
	}

	if hdr[1] < cmdNOP || hdr[1] > cmdUPD {
		return
	}

	n = 2
	isKCP = true
	return
}

func serveRaw(config *Config) {
	logln := func(v ...interface{}) {
		if !config.Quiet {
			log.Println(v...)
		}
	}

	rawListener, err := net.Listen("tcp", config.ListenTCP)
	logln("raw tcp listen on:", config.ListenTCP)
	if err != nil {
		log.Fatal(err)
	}

	for {
		c, err := rawListener.Accept()
		if err != nil {
			logln()
			continue
		}
		logln("raw tcp remote address:", c.RemoteAddr())

		go func(p1 net.Conn) {
			defer p1.Close()

			var p2 net.Conn
			isKCP, hdr, n, err := checkKCP(p1)
			logln("isKCP, hdr, n, err:", isKCP, hdr, n, err)
			if isKCP {
				p2, err = net.Dial("tcp", config.Listen)
				if err != nil {
					logln("raw dial kcp fail:", err)
					return
				}
			} else {
				p2, err = net.Dial("tcp", config.Target)
				if err != nil {
					logln("raw dial direct to target fail:", err)
					return
				}
			}

			_, err = p2.Write(hdr[:n])
			if err != nil {
				logln("raw write header back failed:", err)
				return
			}

			streamCopy := func(dst io.WriteCloser, src io.ReadCloser) {
				if _, err := generic.Copy(dst, src); err != nil {
					logln("generic copy err:", err)
				}
				dst.Close()
				src.Close()
			}
			go streamCopy(p2, p1)
			streamCopy(p1, p2)
		}(c)
	}
}
