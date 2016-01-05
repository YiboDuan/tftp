package tftp

import (
    "fmt"
    "net"
    "bufio"
    "time"
    "os"
    // "log"
)

type Transfer struct {
    conn net.PacketConn
    addr net.Addr
    Filename string
    Mode string
}

func handleRead(t *Transfer) error {
    panic("read not implemented")
    return nil
}

func handleWrite(t *Transfer) error {
    t.conn.SetReadDeadline(time.Now().Add(7000 * time.Millisecond))

    fo, err := os.Create(t.Filename)
    if err != nil {
        errPacket := Err{0, "failed to create file"}
        t.conn.WriteTo(errPacket.Format(), t.addr)
        return fmt.Errorf("failed to create file %v: %v", t.Filename, err)
    }

    defer fo.Close()
    fmt.Println("Begin transfer of file:", t.Filename)
    // prepare writer and buffer
    w := bufio.NewWriter(fo)
    b := make([]byte, MAX_DATAGRAM_SIZE)

    currentBlock := uint16(0)
    dallying := false
    filesize := 0

    for {
        // send acknowledgement
        ack := Ack{currentBlock}
        if _, err := t.conn.WriteTo(ack.Format(), t.addr); err != nil {
            return fmt.Errorf("could not write to: %v, %v", t.addr, err)
        }

        currentBlock += 1

        read:
        // read data packet
        n, _, err := t.conn.ReadFrom(b)
        if err != nil {
            if e, ok := err.(net.Error); ok && e.Timeout() {
                if dallying {
                    // no response after sending last ack packet, assume received
                    fmt.Println("dallying termination complete")
                    break
                }
                fmt.Printf("timed out while waiting for block %v, resending ACK", int(currentBlock))
                currentBlock -= 1
                continue
            } else {
                return fmt.Errorf("error while reading from packet conn: %v", err)
            }
        }

        // parse data packet
        p, err := Parse(b[:n])
        if err != nil {
            return err
        }

        // write data
        if data, ok := p.(*Data); ok {
            fmt.Printf("received block %d, %d bytes\n", data.BlockNumber, len(data.Data))
            if data.BlockNumber == currentBlock {
                if _, err := w.Write(data.Data) ; err != nil {
                    return fmt.Errorf("failed to write to file %v on block %d: %v", t.Filename, currentBlock, err)
                }
                // check for last packet
                if currentBlock > 0 && len(data.Data) < DATA_FIELD_SIZE {
                    filesize = len(data.Data)
                    fmt.Println("last data packet received, dallying for last ack packet")
                    dallying = true
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

    if err := w.Flush(); err != nil {
        return fmt.Errorf("error while flushing writer: %v", err)
    }
    filesize += (int(currentBlock)-2)*DATA_FIELD_SIZE
    fmt.Printf("writing completed after %v data packets\n for a total of %v bytes", currentBlock-1, filesize)
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
}

func Run(port string) {
    conn, err := net.ListenPacket("udp", "127.0.0.1:" + port)
    fmt.Println("simple tftp server running, listening to port:", conn.LocalAddr())
    if err != nil {
        panic(err)
    }
    defer conn.Close()

    b := make([]byte, MAX_DATAGRAM_SIZE)

    for {
        n, raddr, err := conn.ReadFrom(b)

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