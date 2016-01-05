package tftp

import (
    "testing"
    "strings"
)

func TestRequestBuild(t *testing.T) {
    var tests = []struct {
        in string
        filename string
        mode string
        err error
    }{
        {"00afile\x00octet\x00", "afile", "octet", nil},
        {"00a\x00m\x00", "a", "m", nil},
        {"00\x00", "", "", UnexpectedDelimiterError(2)},
        {"00a", "", "", DelimiterNotFoundError(3)},
        {"00a\x00a", "", "", DelimiterNotFoundError(5)},
    }
    for _, test := range tests {
        r := strings.NewReader(test.in)
        b := make([]byte, len(test.in))
        _, err := r.Read(b)
        req := &Request{}
        if err = req.Build(b); err != test.err {
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