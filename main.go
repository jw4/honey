package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"path"
	"strings"
	"syscall"
	"time"
)

var (
	logs = "logs"
	host = "0.0.0.0"
)

func init() {
	flag.StringVar(&logs, "logs", logs, "directory to store logs")
	flag.StringVar(&host, "host", host, "binding host")
}

func main() {
	flag.Parse()
	args := flag.Args()
	if len(args) == 0 {
		log.Fatal("no ports configured")
	}

	if _, err := os.Stat(logs); os.IsNotExist(err) {
		if err = os.Mkdir(logs, 0770); err != nil {
			log.Fatalf("error creating directory %q: %v", logs, err)
		}
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	for _, arg := range args {
		proto := "tcp"
		pair := strings.Split(arg, "/")
		val := pair[0]
		if len(pair) == 2 {
			proto = pair[1]
		}
		port, err := net.LookupPort(proto, val)
		if err != nil {
			log.Fatalf("port %q proto %q invalid: %v", val, proto, err)
		}
		go listenAndLog(port, proto)
	}

	<-signals
	fmt.Printf("Shutting down ...\n")
	fmt.Printf("Done.\n")
}

func listenAndLog(port int, proto string) {
	fmt.Printf("Listening on %d/%s\n", port, proto)
	switch proto {
	case "tcp", "tcp4", "tcp6":
		address := fmt.Sprintf("%s:%d", host, port)
		addr, err := net.ResolveTCPAddr(proto, address)
		if err != nil {
			log.Fatalf("error resolving tcp address %q: %v", address, err)
		}
		listener, err := net.ListenTCP(proto, addr)
		if err != nil {
			log.Fatalf("error listening on tcp address %q: %v", address, err)
		}
		defer listener.Close()
		for {
			conn, err := listener.Accept()
			if err != nil {
				log.Fatalf("error accepting connection on tcp address %q: %v", address, err)
			}
			go func(c net.Conn) {
				defer c.Close()
				now := time.Now()
				if err := c.SetDeadline(now.Add(10 * time.Second)); err != nil {
					log.Printf("error setting connection deadline %q: %v", address, err)
					return
				}
				name := path.Join(logs, strings.Map(pathSafe, fmt.Sprintf("%s_%s-%s", now.Format(time.RFC3339), conn.LocalAddr().String(), conn.RemoteAddr().String())))
				fd, err := os.Create(name)
				if err != nil {
					log.Printf("error creating logfile %q: %v", name, err)
				}
				defer fd.Close()
				log.Printf("starting connection, logging to: %s", name)
				if _, err := io.Copy(fd, c); err != nil {
					if ierr, ok := err.(net.Error); ok && ierr.Timeout() {
						log.Printf("closing connection %s", name)
					} else {
						log.Printf("error copying connection: %v", err)
					}
				}
			}(conn)
		}
	}
}

func pathSafe(r rune) rune {
	switch {
	case r >= ' ' && ',' >= r:
		return '_'
	case r >= '-' && '.' >= r:
	case r >= '0' && ':' >= r:
	case r >= 'A' && '~' >= r:
	default:
		return -1
	}
	return r
}
