package ogg

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"io"
)

const (
	headers = 27
	seg     = 255
)

var (
	oggs = []byte{'O', 'g', 'g', 'S'}
)

type Ogg struct {
	page    *page
	idx     int
	pos     int
	packet  bytes.Buffer
	decoder decoder
	data    []byte
	buf     []byte
}
type decoder struct {
	reader io.Reader
	buf    [headers + seg + seg*255]byte
}

type page struct {
	Oggs    [4]byte
	Version byte
	Type    byte
	Granule int64
	Serial  uint32
	Page    uint32
	Crc     uint32
	Nsegs   byte
}

func New(red io.Reader) *Ogg {
	return &Ogg{
		decoder: decoder{
			reader: bufio.NewReader(red),
		},
	}
}

func (dec *decoder) Decode() ([]byte, []byte, *page, error) {
	header := dec.buf[:headers]

	var pos int
	for {
		_, err := io.ReadFull(dec.reader, header[pos:])
		if err != nil {
			return nil, nil, nil, err
		}

		var idx int
		if idx = bytes.Index(header, oggs); idx == 0 {
			break
		} else if idx == -1 {
			idx = bytes.IndexByte(header, oggs[0])
		}

		if idx > 0 {
			pos = copy(header, header[idx:])
		}
	}

	var page page
	byt := bytes.NewReader(header)
	binary.Read(byt, binary.LittleEndian, &page)
	if page.Nsegs < 1 {
		return nil, nil, nil, errors.New("bad nsegs")
	}

	nsegs := int(page.Nsegs)
	buf := dec.buf[headers : headers+nsegs]
	_, err := io.ReadFull(dec.reader, buf)
	if err != nil {
		return nil, nil, nil, err
	}

	var ln int
	for idx := range buf {
		ln += int(buf[idx])
	}
	pkt := dec.buf[headers+nsegs : headers+nsegs+ln]
	_, err = io.ReadFull(dec.reader, pkt)
	if err != nil {
		return nil, nil, nil, err
	}

	c32 := dec.buf[:headers+nsegs+ln]
	c32[22] = 0
	c32[23] = 0
	c32[24] = 0
	c32[25] = 0
	if Crc(c32) != page.Crc {
		return nil, nil, nil, errors.New("bad crc")
	}

	return buf, pkt, &page, nil
}

func (ogg *Ogg) Decode() ([]byte, error) {
	
	var err error
	if ogg.page == nil {
		ogg.buf, ogg.data, ogg.page, err = ogg.decoder.Decode()
		if err != nil {
			return nil, err
		}
	}

	for {
		for ogg.idx < len(ogg.buf) {
			size := ogg.buf[ogg.idx]

			if size != 0 {
				ogg.packet.Write(ogg.data[ogg.pos : ogg.pos+int(size)])
			}

			ogg.pos += int(size)
			ogg.idx++

			if size < 0xff {
				packet := make([]byte, ogg.packet.Len())
				_, err := ogg.packet.Read(packet)
				if err != nil {
					return nil, err
				}
				ogg.packet.Reset()

				return packet, nil
			}
		}

		ogg.buf, ogg.data, ogg.page, err = ogg.decoder.Decode()
		if err != nil {
			return nil, err
		}
		ogg.pos = 0
		ogg.idx = 0
	}
}
