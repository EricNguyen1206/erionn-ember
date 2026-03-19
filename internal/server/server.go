package server

import (
	"fmt"
	"io"
	"log"
	"memkv/internal/config"
	"memkv/internal/core"
	"net"
)

// StartSimpleTCPServer is a simplified version of the server.
// It is fully workable and can be called from main.go.
func StartSimpleTCPServer() error {
	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)

	// 1. Listen on a TCP port
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	defer listener.Close()

	log.Printf("Starting simple (single-client) TCP server on %s", addr)

	for {
		// 2. Accept a connection (Blocks until a client connects)
		conn, err := listener.Accept()
		if err != nil {
			log.Println("Accept error:", err)
			continue
		}

		log.Println("New client connected (Exclusive session)")

		// 3. Handle the client synchronously (Blocks ALL other clients)
		handleClientSimple(conn)
	}
}

func handleClientSimple(conn net.Conn) {
	defer conn.Close()
	defer log.Println("Client disconnected")

	for {
		// Read buffer
		buf := make([]byte, 1024)
		n, err := conn.Read(buf)
		if err != nil {
			if err != io.EOF {
				log.Println("Read error:", err)
			}
			return
		}

		// Parse command from RESP protocol
		cmd, err := core.ParseCmd(buf[:n])
		if err != nil {
			log.Println("Parse error:", err)
			// Send error response back if possible
			conn.Write([]byte(fmt.Sprintf("-ERR %s\r\n", err.Error())))
			continue
		}

		// Execute and send response
		err = core.EvalAndResponse(cmd, conn)
		if err != nil {
			log.Println("Eval error:", err)
			continue
		}
	}
}
