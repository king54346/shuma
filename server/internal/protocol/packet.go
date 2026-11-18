package protocol

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"math"
	"math/rand"
	"strconv"
)

const (
	initPwdAES = "tjshuma081610888"
	aesIV      = "YouMi_Technology"

	FlagOkAck    byte = 1 << 0
	FlagErrAck   byte = 1 << 1
	FlagHeartSyn byte = 1 << 2
	FlagZip      byte = 1 << 3
	FlagLongLen  byte = 1 << 4
	FlagAck      byte = 1 << 5
	FlagURL      byte = 1 << 6
	FlagMagic    byte = 1 << 7

	flagOkAck    = FlagOkAck
	flagErrAck   = FlagErrAck
	flagHeartSyn = FlagHeartSyn
	flagZip      = FlagZip
	flagLongLen  = FlagLongLen
	flagAck      = FlagAck
	flagURL      = FlagURL
	flagMagic    = FlagMagic
)

// InitialKey returns the initial WANetTask session key.
func InitialKey() []byte {
	return []byte(initPwdAES)
}

// Packet represents a decoded WANetTask frame.
type Packet struct {
	Flag   byte
	Cmd    uint16
	SynID  uint16
	URL    string
	Data   any
	RawLen int
}

func (p Packet) IsHeartbeat() bool {
	return p.Flag&flagHeartSyn != 0
}

func (p Packet) IsControlACK() bool {
	return p.Flag&(flagOkAck|flagErrAck) != 0
}

func (p Packet) IsOkAck() bool {
	return p.Flag&flagOkAck != 0
}

func (p Packet) IsErrAck() bool {
	return p.Flag&flagErrAck != 0
}

func aesCBCEncrypt(plaintext, key []byte) ([]byte, error) {
	if len(plaintext)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("plaintext length %d not multiple of 16", len(plaintext))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	out := make([]byte, len(plaintext))
	cipher.NewCBCEncrypter(block, []byte(aesIV)).CryptBlocks(out, plaintext)
	return out, nil
}

func aesCBCDecrypt(ciphertext, key []byte) ([]byte, error) {
	if len(ciphertext)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("ciphertext length %d not multiple of 16", len(ciphertext))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	out := make([]byte, len(ciphertext))
	cipher.NewCBCDecrypter(block, []byte(aesIV)).CryptBlocks(out, ciphertext)
	return out, nil
}

// lz4DecompressWithHeader decodes util.lz4 format: 4-byte LE uncompressed len + raw LZ4 block.
func lz4DecompressWithHeader(src []byte) ([]byte, error) {
	if len(src) < 4 {
		return nil, errors.New("lz4 payload too short")
	}
	uncompLen := int(binary.LittleEndian.Uint32(src[:4]))
	if uncompLen < 0 || uncompLen > 16*1024*1024 {
		return nil, fmt.Errorf("invalid lz4 uncompLen %d", uncompLen)
	}
	block := src[4:]
	dst := make([]byte, 0, uncompLen)
	i := 0
	for i < len(block) {
		token := block[i]
		i++
		litLen := int(token >> 4)
		if litLen == 15 {
			for i < len(block) {
				b := block[i]
				i++
				litLen += int(b)
				if b != 255 {
					break
				}
			}
		}
		if i+litLen > len(block) {
			return nil, errors.New("lz4 literal out of bounds")
		}
		dst = append(dst, block[i:i+litLen]...)
		i += litLen
		if i >= len(block) {
			break
		}
		if i+2 > len(block) {
			return nil, errors.New("lz4 offset truncated")
		}
		offset := int(block[i]) | (int(block[i+1]) << 8)
		i += 2
		if offset <= 0 || offset > len(dst) {
			return nil, fmt.Errorf("lz4 invalid offset %d", offset)
		}
		matchLen := int(token&0x0f) + 4
		if (token & 0x0f) == 15 {
			for i < len(block) {
				b := block[i]
				i++
				matchLen += int(b)
				if b != 255 {
					break
				}
			}
		}
		for range matchLen {
			dst = append(dst, dst[len(dst)-offset])
		}
	}
	return dst, nil
}

// lz4CompressWithHeader writes 4-byte LE uncompressed len + valid literal-only LZ4 block.
func lz4CompressWithHeader(src []byte) []byte {
	var buf bytes.Buffer
	var hdr [4]byte
	binary.LittleEndian.PutUint32(hdr[:], uint32(len(src)))
	buf.Write(hdr[:])

	if len(src) < 15 {
		buf.WriteByte(byte(len(src) << 4))
	} else {
		buf.WriteByte(0xf0)
		rem := len(src) - 15
		for rem >= 255 {
			buf.WriteByte(255)
			rem -= 255
		}
		buf.WriteByte(byte(rem))
	}
	buf.Write(src)
	return buf.Bytes()
}

func msgpackEncode(v any) ([]byte, error) {
	var buf bytes.Buffer
	if err := writeMsgpack(&buf, v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeMsgpack(buf *bytes.Buffer, v any) error {
	switch val := v.(type) {
	case nil:
		buf.WriteByte(0xc0)
	case bool:
		if val {
			buf.WriteByte(0xc3)
		} else {
			buf.WriteByte(0xc2)
		}
	case int:
		return writeMsgpackInt(buf, int64(val))
	case int64:
		return writeMsgpackInt(buf, val)
	case uint64:
		if val <= math.MaxInt64 {
			return writeMsgpackInt(buf, int64(val))
		}
		buf.WriteByte(0xcf)
		var b [8]byte
		binary.BigEndian.PutUint64(b[:], val)
		buf.Write(b[:])
	case float64:
		buf.WriteByte(0xcb)
		var b [8]byte
		binary.BigEndian.PutUint64(b[:], math.Float64bits(val))
		buf.Write(b[:])
	case string:
		n := len(val)
		switch {
		case n <= 31:
			buf.WriteByte(0xa0 | byte(n))
		case n <= 0xff:
			buf.WriteByte(0xd9)
			buf.WriteByte(byte(n))
		case n <= 0xffff:
			buf.WriteByte(0xda)
			var b [2]byte
			binary.BigEndian.PutUint16(b[:], uint16(n))
			buf.Write(b[:])
		default:
			buf.WriteByte(0xdb)
			var b [4]byte
			binary.BigEndian.PutUint32(b[:], uint32(n))
			buf.Write(b[:])
		}
		buf.WriteString(val)
	case []byte:
		n := len(val)
		switch {
		case n <= 0xff:
			buf.WriteByte(0xc4)
			buf.WriteByte(byte(n))
		case n <= 0xffff:
			buf.WriteByte(0xc5)
			var b [2]byte
			binary.BigEndian.PutUint16(b[:], uint16(n))
			buf.Write(b[:])
		default:
			buf.WriteByte(0xc6)
			var b [4]byte
			binary.BigEndian.PutUint32(b[:], uint32(n))
			buf.Write(b[:])
		}
		buf.Write(val)
	case []any:
		n := len(val)
		switch {
		case n <= 15:
			buf.WriteByte(0x90 | byte(n))
		case n <= 0xffff:
			buf.WriteByte(0xdc)
			var b [2]byte
			binary.BigEndian.PutUint16(b[:], uint16(n))
			buf.Write(b[:])
		default:
			buf.WriteByte(0xdd)
			var b [4]byte
			binary.BigEndian.PutUint32(b[:], uint32(n))
			buf.Write(b[:])
		}
		for _, item := range val {
			if err := writeMsgpack(buf, item); err != nil {
				return err
			}
		}
	case map[string]any:
		n := len(val)
		switch {
		case n <= 15:
			buf.WriteByte(0x80 | byte(n))
		case n <= 0xffff:
			buf.WriteByte(0xde)
			var b [2]byte
			binary.BigEndian.PutUint16(b[:], uint16(n))
			buf.Write(b[:])
		default:
			buf.WriteByte(0xdf)
			var b [4]byte
			binary.BigEndian.PutUint32(b[:], uint32(n))
			buf.Write(b[:])
		}
		for k, item := range val {
			if err := writeMsgpack(buf, k); err != nil {
				return err
			}
			if err := writeMsgpack(buf, item); err != nil {
				return err
			}
		}
	case map[int64]any:
		n := len(val)
		switch {
		case n <= 15:
			buf.WriteByte(0x80 | byte(n))
		case n <= 0xffff:
			buf.WriteByte(0xde)
			var b [2]byte
			binary.BigEndian.PutUint16(b[:], uint16(n))
			buf.Write(b[:])
		default:
			buf.WriteByte(0xdf)
			var b [4]byte
			binary.BigEndian.PutUint32(b[:], uint32(n))
			buf.Write(b[:])
		}
		for k, item := range val {
			if err := writeMsgpack(buf, k); err != nil {
				return err
			}
			if err := writeMsgpack(buf, item); err != nil {
				return err
			}
		}
	case map[any]any:
		n := len(val)
		switch {
		case n <= 15:
			buf.WriteByte(0x80 | byte(n))
		case n <= 0xffff:
			buf.WriteByte(0xde)
			var b [2]byte
			binary.BigEndian.PutUint16(b[:], uint16(n))
			buf.Write(b[:])
		default:
			buf.WriteByte(0xdf)
			var b [4]byte
			binary.BigEndian.PutUint32(b[:], uint32(n))
			buf.Write(b[:])
		}
		for k, item := range val {
			if err := writeMsgpack(buf, k); err != nil {
				return err
			}
			if err := writeMsgpack(buf, item); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("unsupported msgpack type %T", v)
	}
	return nil
}

func writeMsgpackInt(buf *bytes.Buffer, v int64) error {
	switch {
	case v >= 0 && v <= 127:
		buf.WriteByte(byte(v))
	case v >= -32 && v < 0:
		buf.WriteByte(byte(v))
	case v >= 0 && v <= 0xff:
		buf.WriteByte(0xcc)
		buf.WriteByte(byte(v))
	case v >= 0 && v <= 0xffff:
		buf.WriteByte(0xcd)
		var b [2]byte
		binary.BigEndian.PutUint16(b[:], uint16(v))
		buf.Write(b[:])
	case v >= 0 && v <= 0xffffffff:
		buf.WriteByte(0xce)
		var b [4]byte
		binary.BigEndian.PutUint32(b[:], uint32(v))
		buf.Write(b[:])
	case v >= -128 && v < 0:
		buf.WriteByte(0xd0)
		buf.WriteByte(byte(int8(v)))
	case v >= -32768 && v < 0:
		buf.WriteByte(0xd1)
		var b [2]byte
		binary.BigEndian.PutUint16(b[:], uint16(int16(v)))
		buf.Write(b[:])
	case v >= -2147483648 && v < 0:
		buf.WriteByte(0xd2)
		var b [4]byte
		binary.BigEndian.PutUint32(b[:], uint32(int32(v)))
		buf.Write(b[:])
	default:
		buf.WriteByte(0xd3)
		var b [8]byte
		binary.BigEndian.PutUint64(b[:], uint64(v))
		buf.Write(b[:])
	}
	return nil
}

func msgpackDecode(data []byte) (any, error) {
	r := bytes.NewReader(data)
	return readMsgpack(r)
}

func readMsgpack(r *bytes.Reader) (any, error) {
	b, err := r.ReadByte()
	if err != nil {
		return nil, err
	}
	switch {
	case b <= 0x7f:
		return int64(b), nil
	case b >= 0xe0:
		return int64(int8(b)), nil
	case b >= 0x80 && b <= 0x8f:
		return readMsgpackMap(r, int(b&0x0f))
	case b >= 0x90 && b <= 0x9f:
		return readMsgpackArray(r, int(b&0x0f))
	case b >= 0xa0 && b <= 0xbf:
		return readMsgpackString(r, int(b&0x1f))
	}
	switch b {
	case 0xc0:
		return nil, nil
	case 0xc2:
		return false, nil
	case 0xc3:
		return true, nil
	case 0xc4, 0xd9:
		n, err := r.ReadByte()
		if err != nil {
			return nil, err
		}
		return readMsgpackString(r, int(n))
	case 0xc5, 0xda:
		var l uint16
		if err := binary.Read(r, binary.BigEndian, &l); err != nil {
			return nil, err
		}
		return readMsgpackString(r, int(l))
	case 0xc6, 0xdb:
		var l uint32
		if err := binary.Read(r, binary.BigEndian, &l); err != nil {
			return nil, err
		}
		return readMsgpackString(r, int(l))
	case 0xca:
		var f float32
		if err := binary.Read(r, binary.BigEndian, &f); err != nil {
			return nil, err
		}
		return float64(f), nil
	case 0xcb:
		var f float64
		if err := binary.Read(r, binary.BigEndian, &f); err != nil {
			return nil, err
		}
		return f, nil
	case 0xcc:
		v, err := r.ReadByte()
		return int64(v), err
	case 0xcd:
		var v uint16
		err := binary.Read(r, binary.BigEndian, &v)
		return int64(v), err
	case 0xce:
		var v uint32
		err := binary.Read(r, binary.BigEndian, &v)
		return int64(v), err
	case 0xcf:
		var v uint64
		err := binary.Read(r, binary.BigEndian, &v)
		return v, err
	case 0xd0:
		v, err := r.ReadByte()
		return int64(int8(v)), err
	case 0xd1:
		var v int16
		err := binary.Read(r, binary.BigEndian, &v)
		return int64(v), err
	case 0xd2:
		var v int32
		err := binary.Read(r, binary.BigEndian, &v)
		return int64(v), err
	case 0xd3:
		var v int64
		err := binary.Read(r, binary.BigEndian, &v)
		return v, err
	case 0xdc:
		var n uint16
		if err := binary.Read(r, binary.BigEndian, &n); err != nil {
			return nil, err
		}
		return readMsgpackArray(r, int(n))
	case 0xdd:
		var n uint32
		if err := binary.Read(r, binary.BigEndian, &n); err != nil {
			return nil, err
		}
		return readMsgpackArray(r, int(n))
	case 0xde:
		var n uint16
		if err := binary.Read(r, binary.BigEndian, &n); err != nil {
			return nil, err
		}
		return readMsgpackMap(r, int(n))
	case 0xdf:
		var n uint32
		if err := binary.Read(r, binary.BigEndian, &n); err != nil {
			return nil, err
		}
		return readMsgpackMap(r, int(n))
	default:
		return nil, fmt.Errorf("unsupported msgpack tag 0x%02x", b)
	}
}

func readMsgpackString(r *bytes.Reader, n int) (string, error) {
	if n < 0 || n > 16*1024*1024 {
		return "", fmt.Errorf("invalid msgpack string len %d", n)
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", err
	}
	return string(buf), nil
}

func readMsgpackArray(r *bytes.Reader, n int) ([]any, error) {
	out := make([]any, 0, n)
	for range n {
		v, err := readMsgpack(r)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func readMsgpackMap(r *bytes.Reader, n int) (map[string]any, error) {
	out := make(map[string]any, n)
	for range n {
		k, err := readMsgpack(r)
		if err != nil {
			return nil, err
		}
		v, err := readMsgpack(r)
		if err != nil {
			return nil, err
		}
		out[fmt.Sprint(k)] = v
	}
	return out, nil
}

// ReadPacket reads one WANetTask frame from r using sessionKey.
func ReadPacket(r io.Reader, sessionKey []byte) (*Packet, error) {
	var flagBuf [1]byte
	if _, err := io.ReadFull(r, flagBuf[:]); err != nil {
		return nil, err
	}
	flag := flagBuf[0]
	if flag&flagMagic == 0 {
		return nil, fmt.Errorf("invalid magic flag 0x%02x", flag)
	}

	pkt := &Packet{Flag: flag}
	if flag&(flagOkAck|flagErrAck) != 0 {
		var synBuf [2]byte
		if _, err := io.ReadFull(r, synBuf[:]); err != nil {
			return nil, err
		}
		pkt.SynID = binary.BigEndian.Uint16(synBuf[:])
		return pkt, nil
	}
	if flag&flagHeartSyn != 0 {
		return pkt, nil
	}

	var dataLen int
	if flag&flagLongLen != 0 {
		var hdr [13]byte
		if _, err := io.ReadFull(r, hdr[:]); err != nil {
			return nil, err
		}
		dataLen = int(binary.BigEndian.Uint32(hdr[0:4]))
		pkt.Cmd = binary.BigEndian.Uint16(hdr[9:11])
		pkt.SynID = binary.BigEndian.Uint16(hdr[11:13])
		padLen := int(hdr[4])
		crcExpected := binary.BigEndian.Uint32(hdr[5:9])
		return readPacketBody(r, pkt, dataLen, padLen, crcExpected, sessionKey)
	}

	var hdr [11]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return nil, err
	}
	dataLen = int(binary.BigEndian.Uint16(hdr[0:2]))
	padLen := int(hdr[2])
	crcExpected := binary.BigEndian.Uint32(hdr[3:7])
	pkt.Cmd = binary.BigEndian.Uint16(hdr[7:9])
	pkt.SynID = binary.BigEndian.Uint16(hdr[9:11])
	return readPacketBody(r, pkt, dataLen, padLen, crcExpected, sessionKey)
}

func readPacketBody(r io.Reader, pkt *Packet, dataLen, padLen int, crcExpected uint32, sessionKey []byte) (*Packet, error) {
	if pkt.Flag&flagURL != 0 {
		var urlLenBuf [1]byte
		if _, err := io.ReadFull(r, urlLenBuf[:]); err != nil {
			return nil, err
		}
		urlLen := int(urlLenBuf[0])
		encLen := ((urlLen + 2 + 15) / 16) * 16
		encURL := make([]byte, encLen)
		if _, err := io.ReadFull(r, encURL); err != nil {
			return nil, err
		}
		decURL, err := aesCBCDecrypt(encURL, []byte(initPwdAES))
		if err != nil {
			return nil, err
		}
		pkt.URL = string(decURL[:urlLen])
	}

	if dataLen < 0 || dataLen > 16*1024*1024 {
		return nil, fmt.Errorf("invalid packet dataLen %d", dataLen)
	}
	zdata := make([]byte, dataLen)
	if _, err := io.ReadFull(r, zdata); err != nil {
		return nil, err
	}
	pkt.RawLen = dataLen

	var crcBuf bytes.Buffer
	crcBuf.Write(zdata)
	crcBuf.WriteString(strconv.Itoa(int(pkt.Cmd)))
	crcBuf.WriteString(strconv.Itoa(int(pkt.SynID)))
	if pkt.Flag&flagURL != 0 {
		crcBuf.WriteString(pkt.URL)
	}
	if crc32.ChecksumIEEE(crcBuf.Bytes()) != crcExpected {
		return nil, fmt.Errorf("crc32 mismatch on %s", pkt.URL)
	}

	dec, err := aesCBCDecrypt(zdata, sessionKey)
	if err != nil {
		return nil, err
	}
	if padLen > 0 && padLen <= len(dec) {
		dec = dec[:len(dec)-padLen]
	}
	if pkt.Flag&flagZip != 0 {
		dec, err = lz4DecompressWithHeader(dec)
		if err != nil {
			return nil, err
		}
	}
	val, err := msgpackDecode(dec)
	if err != nil {
		return nil, err
	}
	pkt.Data = val
	return pkt, nil
}

// PackAckPacket builds an encrypted WANetTask ACK frame for the client.
func PackAckPacket(url string, ackID uint16, data any, sessionKey []byte) ([]byte, error) {
	pdata, err := msgpackEncode(data)
	if err != nil {
		return nil, err
	}
	zdata := lz4CompressWithHeader(pdata)
	padLen := 0
	if rem := len(zdata) % 16; rem != 0 {
		padLen = 16 - rem
		zdata = append(zdata, make([]byte, padLen)...)
	}

	encData, err := aesCBCEncrypt(zdata, sessionKey)
	if err != nil {
		return nil, err
	}

	cmd := uint16(rand.Intn(65534) + 1)
	var crcBuf bytes.Buffer
	crcBuf.Write(encData)
	crcBuf.WriteString(strconv.Itoa(int(cmd)))
	crcBuf.WriteString(strconv.Itoa(int(ackID)))
	crcBuf.WriteString(url)
	crcVal := crc32.ChecksumIEEE(crcBuf.Bytes())

	urlPlain := make([]byte, len(url)+2)
	copy(urlPlain, url)
	urlPlain[len(url)] = byte(rand.Intn(255) + 1)
	urlPlain[len(url)+1] = byte(rand.Intn(255) + 1)
	if rem := len(urlPlain) % 16; rem != 0 {
		urlPlain = append(urlPlain, make([]byte, 16-rem)...)
	}
	encURL, err := aesCBCEncrypt(urlPlain, []byte(initPwdAES))
	if err != nil {
		return nil, err
	}

	flag := flagMagic | flagURL | flagAck | flagZip
	var out bytes.Buffer
	if len(encData) > 65535 {
		flag |= flagLongLen
		out.WriteByte(flag)
		var l [4]byte
		binary.BigEndian.PutUint32(l[:], uint32(len(encData)))
		out.Write(l[:])
	} else {
		out.WriteByte(flag)
		var l [2]byte
		binary.BigEndian.PutUint16(l[:], uint16(len(encData)))
		out.Write(l[:])
	}
	out.WriteByte(byte(padLen))
	var tail [8]byte
	binary.BigEndian.PutUint32(tail[0:4], crcVal)
	binary.BigEndian.PutUint16(tail[4:6], cmd)
	binary.BigEndian.PutUint16(tail[6:8], ackID)
	out.Write(tail[:])
	out.WriteByte(byte(len(url)))
	out.Write(encURL)
	out.Write(encData)
	return out.Bytes(), nil
}

// PackHeartbeatPacket builds a 1-byte WANetTask heart_syn control frame (0x84).
func PackHeartbeatPacket() []byte {
	return []byte{FlagMagic | FlagHeartSyn}
}

// PackOkAckPacket builds a 3-byte WANetTask ok_ack control frame.
func PackOkAckPacket(synID uint16) []byte {
	var b [3]byte
	b[0] = FlagMagic | FlagOkAck
	binary.BigEndian.PutUint16(b[1:3], synID)
	return b[:]
}

// PackErrAckPacket builds a 3-byte WANetTask err_ack control frame.
func PackErrAckPacket(synID uint16) []byte {
	var b [3]byte
	b[0] = FlagMagic | FlagErrAck
	binary.BigEndian.PutUint16(b[1:3], synID)
	return b[:]
}
