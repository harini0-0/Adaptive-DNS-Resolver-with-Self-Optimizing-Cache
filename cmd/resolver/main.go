package main

import (
	"flag"
	"log"
	"time"

	"github.com/harini0-0/Adaptive-DNS-Resolver-with-Self-Optimizing-Cache/internal/dns"
)

func main() {
	listen := flag.String("listen", ":8053", "UDP address to listen on")
	upstream := flag.String("upstream", "1.1.1.1:53", "upstream DNS resolver")
	timeout := flag.Duration("timeout", 5*time.Second, "upstream query timeout")
	flag.Parse()

	srv := &dns.Server{Addr: *listen, Upstream: *upstream, Timeout: *timeout}
	log.Fatal(srv.ListenAndServe())
}