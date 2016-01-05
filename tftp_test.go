package tftp

import (
    "testing"
    "strings"
    "bytes"
)

// packet.go tests
func TestRequestBuild(t *testing.T) {
    var tests = []struct {
        in string
        filename string
        mode string
        err error
    }{
        {"02a\x00m\x00", "a", "m", nil},
        {"02\x00", "", "", UnexpectedDelimiterError(2)},
        {"02a", "", "", DelimiterNotFoundError(3)},
        {"02a\x00a", "", "", DelimiterNotFoundError(5)},
    }
    for _, test := range tests {
        r := strings.NewReader(test.in)
        b := make([]byte, len(test.in))
        _, err := r.Read(b)
        req := &Request{}
        err = req.Build(b)
        if err != test.err {
            t.Errorf("Build(%v) error %v, want error %v", test.in, err, test.err)
        }

        if err == nil {
            if req.Filename != test.filename {
                t.Fatalf("Build(%v) => filename %v, want %v", test.in, req.Filename, test.filename)
            }
            if req.Mode != test.mode {
                t.Fatalf("Build(%v) => mode %v, want %v", test.in, req.Mode, test.mode)
            }
        }
    }
}

func TestDataBuild(t *testing.T) {
    var tests = []struct {
        in []byte
        blockn uint16
        data []byte
    }{
        {[]byte{0,3,0,1,0}, uint16(1), []byte{0}},
        {[]byte{0,0,0,0}, uint16(0), []byte{}},
        {[]byte{0,3,1,1,255,255}, uint16(257), []byte{255,255}},
    }
    for _, test := range tests {
        d := &Data{}
        err := d.Build(test.in)
        if err != nil {
            t.Errorf("Error on Build(%v): %v", test.in, err)
        } else {
            if d.BlockNumber != test.blockn {
                t.Fatalf("Build(%v) => blockn %v, want %v", test.in, d.BlockNumber, test.blockn)
            }
            if !bytes.Equal(d.Data, test.data) {
                t.Fatalf("Build(%v) => data %v, want %v", test.in, d.Data, test.data)
            }
        }
    }
}

func TestAckFormat(t *testing.T) {
    var tests = []struct {
        in Ack
        out []byte
    }{
        {Ack{1}, []byte{0,4,0,1}},
        {Ack{0}, []byte{0,4,0,0}},
        {Ack{257}, []byte{0,4,1,1}},
    }
    for _, test := range tests {
        b := test.in.Format()
        if !bytes.Equal(b, test.out) {
            t.Fatalf("Format(%v) => Ack Packet %v, want %v", test.in, b, test.out)
        }
    }
}

func TestErrBuild(t *testing.T) {
    var tests = []struct {
        in []byte
        out Err
        err error
    }{
        {[]byte{0,5,0,0,0}, Err{uint16(0), ""}, nil},
        {[]byte{0,5,0,4,101,114,114,33,0}, Err{uint16(4), "err!"}, nil},
        {[]byte{0,5,0,4,97,0,98}, Err{}, UnexpectedDelimiterError(5)},
        {[]byte{0,5,0,4,97}, Err{}, DelimiterNotFoundError(5)},
    }
    for _, test := range tests {
        e := &Err{}
        err := e.Build(test.in)
        if err != test.err {
            t.Errorf("Build(%v) error %v, want error %v", test.in, err, test.err)
        }

        if err == nil {
            if e.Code != test.out.Code {
                t.Fatalf("Build(%v) => errcode %v, want %v", test.in, e.Code, test.out.Code)
            }
            if e.Msg != test.out.Msg {
                t.Fatalf("Build(%v) => errmsg %v, want %v", test.in, e.Msg, test.out.Msg)
            }
        }
    }
}