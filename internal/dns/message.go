package dns

import (
	"encoding/binary"
	"errors"
	"strconv"
	"strings"
	"time"
)

type Header struct {
	ID      uint16
	Flags   uint16
	QDCount uint16
	ANCount uint16
	NSCount uint16
	ARCount uint16
}

// Question is a single entry in the question section.
type Question struct {
	Name  string
	Type  uint16
	Class uint16
}

// Query is a parsed DNS query (header + questions).
type Query struct {
	Header    Header
	Questions []Question
}

func parseHeader(msg []byte) (Header, error){
	if(len(msg) < 12) {
		return Header{}, errors.New("message too short for header")
	}
	return Header{
		ID: 	binary.BigEndian.Uint16(msg[0:2]),
		Flags: 	binary.BigEndian.Uint16(msg[2:4]),
		QDCount: binary.BigEndian.Uint16(msg[4:6]),
		ANCount: binary.BigEndian.Uint16(msg[6:8]),
		NSCount: binary.BigEndian.Uint16(msg[8:10]),
		ARCount: binary.BigEndian.Uint16(msg[10:12]),
	}, nil
}


// parseName decodes a domain name starting at offset, following compression
// pointers. It returns the name, the offset to continue reading from in the
// original stream, and any error.
func parseName(msg []byte, offset int) (name string, next int, err error) {
	var labels []string
	pos := offset
	next = -1
	jumps := 0
	for {
		if pos >= len(msg) {
			return "", 0, errors.New("name out of bounds")
		}
		b := msg[pos]
		switch {
		case b == 0: // end of name
			pos++
			if next == -1 {
				next = pos
			}
			return strings.Join(labels, "."), next, nil
		case b&0xC0 == 0xC0: // compression pointer
			if pos+1 >= len(msg) {
				return "", 0, errors.New("truncated pointer")
			}
			if next == -1 {
				next = pos + 2 // resume after the pointer in the original stream
			}
			pos = int(b&0x3F)<<8 | int(msg[pos+1])
			jumps++
			if jumps > 255 {
				return "", 0, errors.New("too many compression jumps")
			}
		default: // a label
			l := int(b)
			pos++
			if pos+l > len(msg) {
				return "", 0, errors.New("label out of bounds")
			}
			labels = append(labels, string(msg[pos:pos+l]))
			pos += l
		}
	}
}

func parseQuestion(msg []byte, offset int) (Question, int, error) {
	name, next, err := parseName(msg, offset)
	if err != nil {
		return Question{}, 0, err
	}
	if next+4 > len(msg) {
		return Question{}, 0, errors.New("truncated question")
	}
	return Question{
		Name:  name,
		Type:  binary.BigEndian.Uint16(msg[next : next+2]),
		Class: binary.BigEndian.Uint16(msg[next+2 : next+4]),
	}, next + 4, nil
}

// ParseQuery parses the header and question section of a DNS message.
func ParseQuery(msg []byte) (*Query, error) {
	h, err := parseHeader(msg)
	if err != nil {
		return nil, err
	}
	offset := 12
	qs := make([]Question, 0, h.QDCount)
	for i := 0; i < int(h.QDCount); i++ {
		q, next, err := parseQuestion(msg, offset)
		if err != nil {
			return nil, err
		}
		qs = append(qs, q)
		offset = next
	}
	return &Query{Header: h, Questions: qs}, nil
}

// walkAnswers steps through the answer section of a raw DNS message (after
// skipping the header and question section) and invokes fn once per resource
// record with the byte offset of that record's 4-byte TTL field and its
// rdlength. It stops and returns an error on any malformed record.
func walkAnswers(msg []byte, fn func(ttlOffset, rdlength int) error) error {
	h, err := parseHeader(msg)
	if err != nil {
		return err
	}
	offset := 12
	for i := 0; i < int(h.QDCount); i++ {
		_, next, err := parseQuestion(msg, offset)
		if err != nil {
			return err
		}
		offset = next
	}
	for i := 0; i < int(h.ANCount); i++ {
		_, next, err := parseName(msg, offset)
		if err != nil {
			return err
		}
		offset = next
		// fixed fields after the name: type(2) class(2) ttl(4) rdlength(2)
		if offset+10 > len(msg) {
			return errors.New("truncated resource record")
		}
		rdlength := int(binary.BigEndian.Uint16(msg[offset+8 : offset+10]))
		if err := fn(offset+4, rdlength); err != nil {
			return err
		}
		offset += 10 + rdlength
		if offset > len(msg) {
			return errors.New("truncated resource record data")
		}
	}
	return nil
}

// ExtractTTL returns the minimum TTL among a response's answer records. ok is
// false if there are no answer records (e.g. NXDOMAIN) or the message can't
// be walked.
func ExtractTTL(resp []byte) (ttl time.Duration, ok bool) {
	var minSecs uint32
	err := walkAnswers(resp, func(ttlOffset, _ int) error {
		secs := binary.BigEndian.Uint32(resp[ttlOffset : ttlOffset+4])
		if !ok || secs < minSecs {
			minSecs = secs
			ok = true
		}
		return nil
	})
	if err != nil || !ok {
		return 0, false
	}
	return time.Duration(minSecs) * time.Second, true
}

// PatchTTLs returns a copy of resp with every answer record's TTL field
// rewritten to newTTL. The original slice is left untouched, since cached
// responses are shared across concurrent readers.
func PatchTTLs(resp []byte, newTTL time.Duration) []byte {
	out := make([]byte, len(resp))
	copy(out, resp)

	secs := uint32(newTTL / time.Second)
	_ = walkAnswers(out, func(ttlOffset, _ int) error {
		binary.BigEndian.PutUint32(out[ttlOffset:ttlOffset+4], secs)
		return nil
	})
	return out
}

// TypeName maps common record type numbers to names for logging.
func TypeName(t uint16) string {
	switch t {
	case 1:
		return "A"
	case 28:
		return "AAAA"
	case 5:
		return "CNAME"
	case 15:
		return "MX"
	case 16:
		return "TXT"
	case 2:
		return "NS"
	default:
		return "TYPE" + strings.TrimSpace(strconv.Itoa(int(t)))
	}
}
