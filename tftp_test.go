package tftp

import (
    "testing"
    "strings"
    "bytes"
    "net"
    "time"
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

func TestErrFormat(t *testing.T) {
    var tests = []struct {
        in Err
        out []byte
    }{
        {Err{uint16(0),""}, []byte{0,5,0,0,0}},
        {Err{uint16(3),"err!"}, []byte{0,5,0,3,101,114,114,33,0}},
    }
    for _, test := range tests {
        b := test.in.Format()
        if !bytes.Equal(b, test.out) {
            t.Fatalf("Format(%v) => Err Packet %v, want %v", test.in, b, test.out)
        }
    }
}

// server.go tests
// THIS WILL FAIL IF YOU DONT CHANGE TEST_PORT TO THE SERVER PORT!!!!!!
// need to manually test by starting the server, dont have a way of signalling
// shutdown right now, need to create channel
var TEST_PORT string = "57295"
func TestDataPacketLoss(t *testing.T) {
    conn, err := net.ListenPacket("udp", "127.0.0.1:0")
    if err != nil {
        t.Fatal(err)
    }
    defer conn.Close()
    conn.SetReadDeadline(time.Now().Add(10000 * time.Millisecond))
    ra, err := net.ResolveUDPAddr("udp", "127.0.0.1:" + TEST_PORT)
    if err != nil {
        t.Fatal(err)
    }

    wrq := []byte{0,2,97,0,111,99,116,101,116,0}
    if _, err = conn.(*net.UDPConn).WriteToUDP(wrq, ra); err != nil {
        t.Fatal(err)
    }

    b := make([]byte, MAX_DATAGRAM_SIZE)
    n, _, err := conn.ReadFrom(b)
    if err != nil {
        t.Fatal(err)
    }

    p, err := Parse(b[:n])
    if err != nil {
        t.Fatal(err)
    }
    // check first ack packet
    if ack, ok := p.(*Ack); ok && ack.BlockNumber == uint16(0){
        // simulate lost packet by waiting for server timeout and getting a second ack packet
        n, raddr, err := conn.ReadFrom(b)
        if err != nil {
            t.Fatal(err)
        }
        p, err := Parse(b[:n])
        if err != nil {
            t.Fatal(err)
        }
        // check second ack packet
        if ack, ok := p.(*Ack); ok && ack.BlockNumber == uint16(0) {
            // send first and final data packet
            data := []byte{0,3,0,1,255,255}
            if _, err = conn.WriteTo(data, raddr); err != nil {
                t.Fatal(err)
            }
            n, _, err := conn.ReadFrom(b)
            if err != nil {
                t.Fatal(err)
            }
            p, err := Parse(b[:n])
            if err != nil {
                t.Fatal(err)
            }
            // check third ack packet
            if ack, ok := p.(*Ack); !ok || ack.BlockNumber != uint16(1){
                t.Fatal("Failed to receive third Ack Packet")
            }
        } else {
            t.Fatal("Failed to receive second Ack Packet")
        }
    } else {
        t.Fatal("Failed to receive Ack Packet")
    }

}
