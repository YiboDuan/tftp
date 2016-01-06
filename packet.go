package tftp

import (
    "fmt"
    "encoding/binary"
    "bytes"
)

const (
    // opcodes
    RRQ_CODE = uint16(1)
    WRQ_CODE = uint16(2)
    DATA_CODE = uint16(3)
    ACK_CODE = uint16(4)
    ERR_CODE = uint16(5)

    // sizes
    DATA_FIELD_SIZE = 512
    TFTP_HEADER_SIZE = 2
    BLOCK_NUMBER_SIZE = 2
    ERROR_CODE_SIZE = 2

    META_DATA_SIZE = TFTP_HEADER_SIZE + BLOCK_NUMBER_SIZE
    MAX_DATAGRAM_SIZE = DATA_FIELD_SIZE + META_DATA_SIZE
)

type Packet interface {
    Build([]byte) error
    Format() []byte
}

// Write/Read Request packet
type Request struct {
    Opcode uint16
    Filename string
    Mode string // not used for now, assume octet
}

type UnexpectedDelimiterError int

func (u UnexpectedDelimiterError) Error() string {
    return fmt.Sprintf("unexpected delimiter at index %v", int(u))
}

type DelimiterNotFoundError int

func (d DelimiterNotFoundError) Error() string{
    return fmt.Sprintf("reached end of packet of size %v, expected delimiter", int(d))
}

func (r *Request) Build(b []byte) error {
    var modeIndex int
    for i := TFTP_HEADER_SIZE; i < len(b); i++ {
        if b[i] == 0 {         // 0 byte delimiter
            if i == TFTP_HEADER_SIZE {
                return UnexpectedDelimiterError(i)
            }

            if r.Filename == "" {
                modeIndex = i + 1
                r.Filename = string(b[TFTP_HEADER_SIZE:i])
            } else {
                r.Mode = string(b[modeIndex:i])
                return nil
            }
        }
    }
    return DelimiterNotFoundError(len(b))
}

// Not used for Store-Only server, not tested
func (r *Request) Format() []byte {
    b := new(bytes.Buffer)
    binary.Write(b, binary.BigEndian, r.Opcode)
    b.WriteString(r.Filename)
    b.WriteByte(0)
    b.WriteString(r.Mode)
    b.WriteByte(0)
    return b.Bytes()
}

type Data struct {
    BlockNumber uint16
    Data []byte
}

func (d *Data) Build(b []byte) error {
    d.BlockNumber = binary.BigEndian.Uint16(b[TFTP_HEADER_SIZE:META_DATA_SIZE])
    d.Data = b[META_DATA_SIZE:]
    return nil
}

// Not used for Store-Only server, not tested
func (d *Data) Format() []byte {
    b := new(bytes.Buffer)
    binary.Write(b, binary.BigEndian, DATA_CODE)
    binary.Write(b, binary.BigEndian, d.BlockNumber)
    b.Write(d.Data)
    return b.Bytes()
}

type Ack struct {
    BlockNumber uint16
}

// Not used for Store-Only server, incomplete
func (a *Ack) Build(b []byte) error {
    a.BlockNumber = binary.BigEndian.Uint16(b[TFTP_HEADER_SIZE:META_DATA_SIZE])
    return nil
}

func (a *Ack) Format() []byte {
    b := new(bytes.Buffer)
    binary.Write(b, binary.BigEndian, ACK_CODE)
    binary.Write(b, binary.BigEndian, a.BlockNumber)
    return b.Bytes()
}

type Err struct {
    Code uint16
    Msg string
}

func (e *Err) Build(b []byte) error {
    e.Code = binary.BigEndian.Uint16(b[TFTP_HEADER_SIZE:META_DATA_SIZE])

    // detect errors and extract ErrMsg
    for i := META_DATA_SIZE; i < len(b); i++ {
        if b[i] == 0 {
            if i != len(b) - 1 {
                return UnexpectedDelimiterError(i)
            }
            e.Msg = string(b[META_DATA_SIZE:i])
            return nil
        }
    }
    return DelimiterNotFoundError(len(b))
}

func (e *Err) Format() []byte {
    b := new(bytes.Buffer)
    binary.Write(b, binary.BigEndian, ERR_CODE)
    binary.Write(b, binary.BigEndian, e.Code)
    b.WriteString(e.Msg)
    b.WriteByte(0)
    return b.Bytes()
}

func Parse(b []byte) (Packet, error) {
    var p Packet
    opcode := binary.BigEndian.Uint16(b[:TFTP_HEADER_SIZE])

    switch opcode {
    case RRQ_CODE:
        fallthrough
    case WRQ_CODE:
        p = &Request{Opcode: opcode}
    case DATA_CODE:
        p = &Data{}
    case ACK_CODE:
        p = &Ack{}
    case ERR_CODE:
        p = &Err{}
    default:
        return nil, fmt.Errorf("invalid opcode %v found", opcode)
    }
    err := p.Build(b)
    return p, err
}