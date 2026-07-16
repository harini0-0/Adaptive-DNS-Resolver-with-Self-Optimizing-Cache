package dns

import (
	"context"
	"log"
	"net"
	"time"
)

// Server is a bare forwarding DNS resolver (no cache, no worker pool yet).
type Server struct {
	Addr     string        // e.g. ":8053"
	Upstream string        // e.g. "1.1.1.1:53"
	Timeout  time.Duration // per-query upstream timeout
}

func (s *Server) ListenAndServe() error {
	addr, err := net.ResolveUDPAddr("udp", s.Addr)
	if err != nil {
		return err
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()
	log.Printf("DNS listening on %s, upstream %s", s.Addr, s.Upstream)

	buf := make([]byte, 512) // 512 = classic UDP DNS max; TCP/EDNS deferred
	for {
		n, client, err := conn.ReadFromUDP(buf)
		if err != nil {
			log.Printf("read error: %v", err)
			continue
		}
		// buf is reused on the next iteration, so copy this packet out.
		packet := make([]byte, n)
		copy(packet, buf[:n])
		s.handle(conn, client, packet)
	}
}

func (s *Server) handle(conn *net.UDPConn, client *net.UDPAddr, packet []byte) {
	query, err := ParseQuery(packet)
	if err != nil {
		log.Printf("malformed query from %s: %v", client, err)
		return // reject bad packets, keep serving
	}
	if len(query.Questions) > 0 {
		q := query.Questions[0]
		log.Printf("query id=%d %s %s from %s",
			query.Header.ID, q.Name, TypeName(q.Type), client)
	}

	resp, err := s.forward(packet)
	if err != nil {
		log.Printf("upstream error: %v", err)
		return
	}
	if _, err := conn.WriteToUDP(resp, client); err != nil {
		log.Printf("write to client error: %v", err)
	}
}

// forward sends the raw query to the upstream and returns the raw response.
func (s *Server) forward(query []byte) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.Timeout)
	defer cancel()

	var d net.Dialer
	conn, err := d.DialContext(ctx, "udp", s.Upstream)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	if _, err := conn.Write(query); err != nil {
		return nil, err
	}
	resp := make([]byte, 512)
	n, err := conn.Read(resp)
	if err != nil {
		return nil, err
	}
	return resp[:n], nil
}
