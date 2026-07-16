package main

import (
	"flag"
	"log"
	"time"

	"github.com/harini0-0/Adaptive-DNS-Resolver-with-Self-Optimizing-Cache/internal/cache"
	"github.com/harini0-0/Adaptive-DNS-Resolver-with-Self-Optimizing-Cache/internal/dns"
)

func main() {
	listen := flag.String("listen", ":8053", "UDP address to listen on")
	upstream := flag.String("upstream", "1.1.1.1:53", "upstream DNS resolver")
	timeout := flag.Duration("timeout", 5*time.Second, "upstream query timeout")
	workers := flag.Int("workers", 6, "number of concurrent query handlers")
	queue := flag.Int("queue", 2, "pending-query buffer size")
	cacheSize := flag.Int("cache-size", 10000, "max number of cached questions")
	flag.Parse()

	srv := &dns.Server{
		Addr:      *listen,
		Upstream:  *upstream,
		Timeout:   *timeout,
		Workers:   *workers,
		QueueSize: *queue,
		Cache:     cache.New(*cacheSize),
	}
	log.Fatal(srv.ListenAndServe())
}
