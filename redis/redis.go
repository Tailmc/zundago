package redis

import (
	"bufio"
	"errors"
	"io"
	"net"
	"strconv"
)

const (
	Nil Error = "redis:nil"
)

type Error string

type DB struct {
	URI  string
	Port int
	Pass string
	DB   int
	conn net.Conn
	buf  *bufio.Reader
}

func (err Error) Error() string {
	return string(err)
}

func (db *DB) Dial() error {
	if db.URI == "" {
		return errors.New("empty URI")
	}

	if db.Port == 0 {
		db.Port = 6379
	}

	port := strconv.Itoa(db.Port)
	req := net.JoinHostPort(db.URI, port)

	conn, err := net.Dial("tcp",req)
	if err != nil {
		return err
	}

	db.conn = conn
	db.buf = bufio.NewReader(conn)

	if db.Pass != "" {
		err := db.Send("AUTH", db.Pass)
		if err != nil {
			return err
		}
		res := db.Receive()
		if err, ok := res.(error); ok {
			return err
		}
	}

	if db.DB > 0 {
		err := db.Send("SELECT", db.DB)
		if err != nil {
			return err
		}
		res := db.Receive()
		if err, ok := res.(error); ok {
			return err
		}
	}
	
	return nil
}

func (db *DB) Send(args ...interface{}) error {
	_, err := db.conn.Write(command(args...))
	if err != nil {
		return err
	}
	return nil
}

func (db *DB) Receive() interface{} {
	byt, err := db.buf.ReadBytes('\n')
	if err != nil {
		return err
	}

	line := byt[:len(byt)-2]
	switch line[0] {
	case '-':
		return errors.New(string(line[1:]))
	case '+':
		return string(line[1:])
	case ':':
		i64, err := strconv.ParseInt(string(line[1:]), 10, 64)
		if err != nil {
			return err
		}
		return i64
	case '$':
		i32, err := strconv.Atoi(string(line[1:]))
		if err != nil {
			return err
		}
		if i32 < 0 {
			err = Nil
			return err
		}
		buf := make([]byte, i32+2)
		_, err = io.ReadFull(db.buf, buf)
		if err != nil {
			return err
		}
		return buf[:i32]
	default:
		return errors.New("unknown message received")
	}
}

func command(args ...interface{}) []byte {
	res := make([]byte, 1, 16*len(args))
	res[0] = '*'

	res = strconv.AppendInt(res, int64(len(args)), 10)
	res = newLine(res)
	for _, arg := range args {
		res = append(res, '$')
		switch arg := arg.(type) {
		case []byte:
			res = strconv.AppendInt(res, int64(len(arg)), 10)
			res = newLine(res)
			res = append(res, arg...)
		case string:
			res = strconv.AppendInt(res, int64(len(arg)), 10)
			res = newLine(res)
			res = append(res, arg...)
		case int:
			res = strconv.AppendInt(res, int64(ln(arg)), 10)
			res = newLine(res)
			res = strconv.AppendInt(res, int64(arg), 10)
		default:
			return nil
		}
		res = newLine(res)
	}
	return res
}

func newLine(byt []byte) []byte {
	return append(byt, '\r', '\n')
}

func ln(int int) int {
	pos, pos10 := 1, 10
	if int < 0 {
		int = -int
		pos++
	}
	for int >= pos10 {
		pos10 *= 10
		pos++
	}
	return pos
}
