# tftp
This repository contains a simple tftp implementation that can only handle writes.
It has the foundation to support reads, but needs additional work.

Install by using:
```
go get https://github.com/YiboDuan/tftp
```

To use, just import it and call Run with a port to listen to (i suggest 0 or the usual 69 if it is available):

```
import "github.com/yiboduan/tftp"

func main() {
  tftp.Run("0")
}
```

