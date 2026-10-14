package console

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

// Source RCON packet types. Minecraft speaks the same wire format.
const (
	typeResponse = 0
	typeExec     = 2
	typeAuthResp = 2
	typeAuth     = 3

	maxPacket = 4096 + 14
)

// RCON forwards console lines to a container-local RCON server. It connects
// lazily — the game opens RCON only once it is up — and reconnects after an
// error.
type RCON struct {
	Addr     string
	Password string
	Out      func(line string)
	Timeout  time.Duration

	mu     sync.Mutex
	conn   net.Conn
	nextID int32
}

func (r *RCON) Send(cmd string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	resp, err := r.exec(cmd)
	if err != nil {
		r.closeLocked()
		// One retry on a fresh connection: the server may have restarted.
		if resp, err = r.exec(cmd); err != nil {
			r.closeLocked()
			return err
		}
	}
	if resp != "" && r.Out != nil {
		for _, line := range bytes.Split([]byte(resp), []byte("\n")) {
			if len(bytes.TrimSpace(line)) > 0 {
				r.Out(string(bytes.TrimRight(line, "\r")))
			}
		}
	}
	return nil
}

func (r *RCON) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closeLocked()
}

func (r *RCON) closeLocked() {
	if r.conn != nil {
		_ = r.conn.Close()
		r.conn = nil
	}
}

func (r *RCON) timeout() time.Duration {
	if r.Timeout > 0 {
		return r.Timeout
	}
	return 5 * time.Second
}

func (r *RCON) exec(cmd string) (string, error) {
	if r.conn == nil {
		if err := r.connect(); err != nil {
			return "", err
		}
	}
	id := r.id()
	_ = r.conn.SetDeadline(time.Now().Add(r.timeout()))
	if err := writePacket(r.conn, id, typeExec, cmd); err != nil {
		return "", err
	}
	var out bytes.Buffer
	for first := true; ; first = false {
		if !first {
			// Long responses arrive as several packets with no terminator
			// both servers agree on; take whatever follows within a beat.
			_ = r.conn.SetReadDeadline(time.Now().Add(150 * time.Millisecond))
		}
		pid, typ, body, err := readPacket(r.conn)
		if err != nil {
			var ne net.Error
			if !first && errors.As(err, &ne) && ne.Timeout() {
				return out.String(), nil
			}
			return "", err
		}
		if pid == id && typ == typeResponse {
			out.WriteString(body)
		}
	}
}

func (r *RCON) connect() error {
	conn, err := net.DialTimeout("tcp", r.Addr, r.timeout())
	if err != nil {
		return err
	}
	_ = conn.SetDeadline(time.Now().Add(r.timeout()))
	id := r.id()
	if err := writePacket(conn, id, typeAuth, r.Password); err != nil {
		conn.Close()
		return err
	}
	// Source servers send an empty RESPONSE_VALUE before the AUTH_RESPONSE.
	for {
		pid, typ, _, err := readPacket(conn)
		if err != nil {
			conn.Close()
			return err
		}
		if typ != typeAuthResp {
			continue
		}
		if pid == -1 {
			conn.Close()
			return errors.New("rcon authentication failed")
		}
		if pid == id {
			r.conn = conn
			return nil
		}
	}
}

func (r *RCON) id() int32 {
	r.nextID++
	if r.nextID <= 0 {
		r.nextID = 1
	}
	return r.nextID
}

func writePacket(w io.Writer, id, typ int32, body string) error {
	buf := make([]byte, 0, 14+len(body))
	buf = binary.LittleEndian.AppendUint32(buf, uint32(10+len(body)))
	buf = binary.LittleEndian.AppendUint32(buf, uint32(id))
	buf = binary.LittleEndian.AppendUint32(buf, uint32(typ))
	buf = append(buf, body...)
	buf = append(buf, 0, 0)
	_, err := w.Write(buf)
	return err
}

func readPacket(r io.Reader) (id, typ int32, body string, err error) {
	var size int32
	if err = binary.Read(r, binary.LittleEndian, &size); err != nil {
		return
	}
	if size < 10 || size > maxPacket {
		err = fmt.Errorf("rcon: bad packet size %d", size)
		return
	}
	payload := make([]byte, size)
	if _, err = io.ReadFull(r, payload); err != nil {
		return
	}
	id = int32(binary.LittleEndian.Uint32(payload[0:4]))
	typ = int32(binary.LittleEndian.Uint32(payload[4:8]))
	body = string(bytes.TrimRight(payload[8:], "\x00"))
	return
}
