package tftp

import (
    "fmt"
    "net"
    "io/ioutil"
    "time"
    "bytes"
)

type Transfer struct {
    conn net.PacketConn
    addr net.Addr
    Filename string
    Mode string
}

func (t *Transfer) sendError(e Err) {
    t.conn.WriteTo(e.Format(), t.addr)
}

func handleRead(t *Transfer) error {
    b := make([]byte, DATA_FIELD_SIZE)
    currentBlock := uint16(0)

    fileb, err := ioutil.ReadFile(t.Filename)
    filesize := len(fileb)
    if err != nil {
        t.sendError(Err{uint16(1), "failed to open file"})
        return err
    }
    filer := bytes.NewReader(fileb)

    for {
        currentBlock += 1
        n, err := filer.Read(b)
        if err != nil {
            t.sendError(Err{uint16(0), "failed to read data into buffer"})
            return err
        }

        data := &Data{currentBlock, b[:n]}
        datab := data.Format()

        write:  // label for resending data after lost ack packet
        if _, err := t.conn.WriteTo(datab, t.addr); err != nil {
            t.sendError(Err{uint16(0), "failed to write to connection"})
            return fmt.Errorf("could not write to: %v, %v", t.addr, err)
        }

        read:
        // wait for acknowledgement
        n, _, err = t.conn.ReadFrom(b)
        if err != nil {
            if e, ok := err.(net.Error); ok && e.Timeout() {
                fmt.Printf("timed out while waiting for block %v Ack, resending data\n", int(currentBlock))
                t.conn.SetReadDeadline(time.Now().Add(7000 * time.Millisecond))
                goto write
            } else {
                t.sendError(Err{uint16(0), err.Error()})
                return fmt.Errorf("error while reading from packet conn: %v", err)
            }
        }

        // parse ack packet
        p, err := Parse(b[:n])
        if err != nil {
            t.sendError(Err{uint16(4), err.Error()})
            return err
        }

        // check that packet is ack and is correct
        if ack, ok := p.(*Ack); ok {
            if ack.BlockNumber != currentBlock {
                // duplicate ack packet!!! read another one
                goto read
            }
        } else {
            t.sendError(Err{(4), "packet received was not the expected Ack"})
        }

        // acknowledgement found, if data field is less than 512, done
        if len(data.Data) < DATA_FIELD_SIZE {
            break
        }

    }
    fmt.Printf("reading completed after %v data packets for a total of %v bytes\n", currentBlock, filesize)
    return nil
}

func handleWrite(t *Transfer) error {
    b := make([]byte, MAX_DATAGRAM_SIZE)
    // write to buffer and then to a file later so it's not visible until completed
    fileb := new(bytes.Buffer)

    currentBlock := uint16(0)
    dallying := false
    var filesize int

    for {
        // send acknowledgement
        ack := Ack{currentBlock}
        if _, err := t.conn.WriteTo(ack.Format(), t.addr); err != nil {
            t.sendError(Err{uint16(0), "failed to send Ack"})
            return fmt.Errorf("could not write to: %v, %v", t.addr, err)
        }

        currentBlock += 1

        read:   // label to read but not send another ack packet
        // read data packet
        n, _, err := t.conn.ReadFrom(b)
        if err != nil {
            if e, ok := err.(net.Error); ok && e.Timeout() {
                if dallying {
                    // no response after sending last ack packet, assume received
                    fmt.Println("dallying termination complete")
                    break
                }
                fmt.Printf("timed out while waiting for block %v, resending ACK\n", int(currentBlock))
                t.conn.SetReadDeadline(time.Now().Add(7000 * time.Millisecond))
                currentBlock -= 1
                continue
            } else {
                t.sendError(Err{uint16(0), err.Error()})
                return fmt.Errorf("error while reading from packet conn: %v", err)
            }
        }

        // parse data packet
        p, err := Parse(b[:n])
        if err != nil {
            t.sendError(Err{uint16(4), err.Error()})
            return err
        }

        // write data
        if data, ok := p.(*Data); ok {
            fmt.Printf("received block %d, %d bytes\n", data.BlockNumber, len(data.Data))
            // confirming expected BlockNumber
            if data.BlockNumber == currentBlock {
                if _, err := fileb.Write(data.Data) ; err != nil {
                    return fmt.Errorf("failed to write to file %v on block %d: %v", t.Filename, currentBlock, err)
                }
                // check for last packet
                if currentBlock > 0 && len(data.Data) < DATA_FIELD_SIZE {
                    // last data packet received, write to file and dallying for last ack packet
                    filesize = len(data.Data)
                    dallying = true
                    err := ioutil.WriteFile(t.Filename, fileb.Bytes(), 0666)
                    if err != nil {
                        t.sendError(Err{0, "failed to write/create file"})
                        return fmt.Errorf("failed to write to file %v: %v", t.Filename, err)
                    }
                }
            } else {
                if dallying {
                    // at this point last data packet was transferred again, which means ack packet was lost
                    fmt.Println("last ack packet lost, trying again")
                    currentBlock -= 1
                    continue
                }
                // duplicate data packet, reread without sending ack packet
                goto read
            }
        } else if e, ok := p.(*Err); ok {
            return fmt.Errorf("error packet received: %v, %v", e.Code, e.Msg)
        }
    }

    filesize += (int(currentBlock)-2)*DATA_FIELD_SIZE
    fmt.Printf("writing completed after %v data packets for a total of %v bytes\n", currentBlock-1, filesize)
    return nil
}

func handleRequest(r *Request, raddr net.Addr) {
    // setup port for sending/receiving of packets
    conn, err := net.ListenPacket("udp", "127.0.0.1:0")
    fmt.Println("listening on transfer port", conn.LocalAddr())
    if err != nil {
        // send Err Packet
        panic(err)
    }
    conn.SetReadDeadline(time.Now().Add(7000 * time.Millisecond))
    t := &Transfer{conn, raddr, r.Filename, r.Mode}
    switch r.Opcode {
        case RRQ_CODE:
            fmt.Println("handling read request...")
            err = handleRead(t)
        case WRQ_CODE:
            fmt.Println("handling write request...")
            err = handleWrite(t)
        default:
            panic("only request packets should be in here!")
    }
    if err != nil {
        panic(err)
    }
}

type Server struct {
    port string
    shutdown bool
    Setupdone chan int
}

func (s *Server) Run() {
    conn, err := net.ListenPacket("udp", "127.0.0.1:" + s.port)
    s.Setupdone <- 1  // signal setup done, used as a semaphore
    fmt.Println("simple tftp server running, listening to port:", conn.LocalAddr())
    if err != nil {
        panic(err)
    }
    defer conn.Close()

    b := make([]byte, MAX_DATAGRAM_SIZE)

    for {
        conn.SetReadDeadline(time.Now().Add(7000 * time.Millisecond))
        n, raddr, err := conn.ReadFrom(b)

        if err != nil {
            // check for shutdown flag
            if s.shutdown {
                break
            }
            continue
        }

        fmt.Println("received", n, "packets from", raddr)
        p, err := Parse(b[:n])
        if err != nil {
            panic(err)
        }

        if req, ok := p.(*Request); ok {
            go handleRequest(req, raddr)
        } else {
            fmt.Printf("received a non-request packet from %v\n", raddr)
        }
    }
}

func (s *Server) Stop() {
    s.shutdown = true
}

func NewServer(port string) *Server{
    s := &Server{
        port,
        false,
        make(chan int, 1),
    }
    return s
}