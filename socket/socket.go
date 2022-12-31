package socket

import (
	"bufio"
	"compress/zlib"
	"crypto/rand"
	"crypto/sha1"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"zundago/dns"
)

type Conn struct {
	reader *bufio.Reader
	writer *bufio.Writer
	conn   net.Conn
	read   *sync.Mutex
	write  *sync.Mutex
}

type writer struct {
	buf *bufio.Writer
	typ int
}

const (
	version = 13
	guid    = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"
	max     = 32 << 20
)

const (
	Text    = 1
	Binary  = 2
	Close   = 8
	Ping    = 9
	Pong    = 10
	Unknown = 255
)

func dial(uri *url.URL) (net.Conn, error) {
	if uri.Scheme == "ws" {
		return dns.Dialer.Dial("tcp", net.JoinHostPort(uri.Hostname(), "80"))
	}
	if uri.Scheme == "wss" {
		return tls.DialWithDialer(dns.Dialer, "tcp", net.JoinHostPort(uri.Hostname(), "443"), nil)
	}

	return nil, errors.New("bad uri scheme")
}

func Dial(raw string) (*Conn, error) {
	conn := new(Conn)

	uri, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}

	conn.conn, err = dial(uri)
	if err != nil {
		return nil, err
	}

	conn.reader = bufio.NewReader(conn.conn)
	conn.writer = bufio.NewWriter(conn.conn)

	conn.read = new(sync.Mutex)
	conn.write = new(sync.Mutex)

	key := make([]byte, 16)
	_, err = io.ReadFull(rand.Reader, key)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, 24)
	base64.StdEncoding.Encode(nonce, key)

	str := []string{
		"GET " + uri.RequestURI() + " HTTP/1.1\r\n",
		"Host: " + uri.Host + "\r\n",
		"Upgrade: websocket\r\n",
		"Connection: Upgrade\r\n",
		"Sec-WebSocket-Key: " + string(nonce) + "\r\n",
		"Sec-WebSocket-Version: " + strconv.Itoa(version) + "\r\n",
	}
	for idx := range str {
		_, err := conn.writer.WriteString(str[idx])
		if err != nil {
			return nil, err
		}
	}

	_, err = conn.writer.WriteString("\r\n")
	if err != nil {
		return nil, err
	}

	err = conn.writer.Flush()
	if err != nil {
		return nil, err
	}

	req := &http.Request{Method: http.MethodGet}
	res, err := http.ReadResponse(conn.reader, req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusSwitchingProtocols {
		return nil, fmt.Errorf("bad status code:%d", res.StatusCode)
	}
	if strings.ToLower(res.Header.Get("Upgrade")) != "websocket" {
		return nil, errors.New("bad upgrade header")
	}
	if strings.ToLower(res.Header.Get("Connection")) != "upgrade" {
		return nil, errors.New("bad connection header")
	}
	defer res.Body.Close()

	sha := sha1.New()
	_, err = sha.Write(nonce)
	if err != nil {
		return nil, err
	}

	_, err = sha.Write([]byte(guid))
	if err != nil {
		return nil, err
	}

	exp := make([]byte, 28)
	base64.StdEncoding.Encode(exp, sha.Sum(nil))

	if res.Header.Get("Sec-WebSocket-Accept") != string(exp) {
		return nil, errors.New("mismatched challenge response")
	}

	return conn, nil
}

func (conn *Conn) WriteJSON(val interface{}) error {

	conn.write.Lock()
	defer conn.write.Unlock()

	jsn, err := json.Marshal(val)
	if err != nil {
		return err
	}

	_, err = (&writer{conn.writer, Text}).Write(jsn)
	if err != nil {
		return err
	}

	return nil
}

func (conn *Conn) Close() error {
	return conn.conn.Close()
}

func (conn *Conn) WriteClose(code int) error {

	conn.write.Lock()
	defer conn.write.Unlock()

	stt := make([]byte, 2)
	binary.BigEndian.PutUint16(stt, uint16(code))

	_, err := (&writer{conn.writer, Close}).Write(stt)
	if err != nil {
		return err
	}

	return nil
}

func (writer *writer) Write(val []byte) (int, error) {
	key := make([]byte, 4)

	_, err := io.ReadFull(rand.Reader, key)
	if err != nil {
		return 0, err
	}

	var header []byte
	var byt byte

	byt = 0x80
	byt |= byte(writer.typ)
	header = append(header, byt)

	byt = 0x80
	var fields int
	switch {
	case len(val) <= 125:
		byt |= byte(len(val))
	case len(val) < 65536:
		byt |= 126
		fields = 2
	default:
		byt |= 127
		fields = 8
	}
	header = append(header, byt)

	for idx := 0; idx < fields; idx++ {
		byt = byte((len(val) >> uint((fields-idx-1)*8)) & 0xff)
		header = append(header, byt)
	}
	header = append(header, key...)

	writer.buf.Write(header)
	data := make([]byte, len(val))
	for idx := range data {
		data[idx] = val[idx] ^ key[idx%4]
	}
	writer.buf.Write(data)

	return len(val) + len(header), writer.buf.Flush()
}

func (conn *Conn) ReadJSON(val interface{}) error {

	conn.read.Lock()
	defer conn.read.Unlock()

	var header []byte
	var byt byte

	byt, err := conn.reader.ReadByte()
	if err != nil {
		return err
	}
	header = append(header, byt)

	byt, err = conn.reader.ReadByte()
	if err != nil {
		return err
	}
	header = append(header, byt)
	byt &= 0x7f

	var ln int64
	var fields int
	switch {
	case byt <= 125:
		ln = int64(byt)
	case byt == 126:
		fields = 2
	case byt == 127:
		fields = 8
	}

	for idx := 0; idx < fields; idx++ {
		byt, err = conn.reader.ReadByte()
		if err != nil {
			return err
		}
		if fields == 8 && idx == 0 {
			byt &= 0x7f
		}
		header = append(header, byt)
		ln = ln*256 + int64(byt)
	}

	if ln > max {
		return errors.New("frame length too large")
	}

	reader := io.LimitReader(conn.reader, ln)

	op := header[0] & 0x0f
	switch op {
	case Text:
	case Binary:
		reader, err = zlib.NewReader(reader)
		if err != nil {
			return err
		}
	case Close:
		return io.EOF
	default:
		return errors.New("unknown op code:" + strconv.Itoa(int(op)))
	}

	if closer, ok := reader.(io.ReadCloser); ok {
		defer closer.Close()
	}

	return json.NewDecoder(reader).Decode(val)
}
