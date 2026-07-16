package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
)

func getLinesChannel(f io.ReadCloser) <-chan string {
	out := make(chan string, 1)

	go func() {
		defer f.Close()
		defer close(out)

		buffer := make([]byte, 8)
		var line string
		for {
			n, err := f.Read(buffer)
			data := buffer[:n]

			for len(data) > 0 {
				if i := bytes.IndexByte(data, '\n'); i != -1 {
					line += string(data[:i])
					out <- line
					line = ""
					data = data[i+1:]
				} else {
					line += string(data)
					break
				}
			}

			if err == io.EOF {
				if len(line) > 0 {
					out <- line
				}
				break
			}

			if err != nil {
				if len(line) > 0 {
					out <- line // flush remaining data before dying
				}
				log.Println("error", err) // log without os.Exit
				break                     // let defers run (close channel, close file)
			}
		}
	}()

	return out
}

func main() {
	// f, err := os.Open("messages.txt")
	// if err != nil {
	// 	log.Fatal("error", err)
	// }
	// defer f.Close()

	// lines := getLinesChannel(f)
	// for line := range lines {
	// 	fmt.Printf("read: %s\n", line)
	// }

	listener, err := net.Listen("tcp", ":42069")
	if err != nil {
		log.Fatal("error", err)
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Fatal("error", err)
		}
		for line := range getLinesChannel(conn) {
			fmt.Printf("read: %s\n", line)
		}
	}

}
