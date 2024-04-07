package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
)

var (
	folder *string
	port   *int
)

func init() {
	folder = flag.String("f", ".", "folder to serve")
	port = flag.Int("p", 69, "port number")
}

type TftpReader bytes.Reader

func (r *TftpReader) ReadByte() (byte, error) {
	return (*bytes.Reader)(r).ReadByte()
}

func (r *TftpReader) Read(p []byte) (n int, err error) {
	return (*bytes.Reader)(r).Read(p)
}

func (r *TftpReader) ReadInt16() (int, error) {
	upper, err := r.ReadByte()
	if err != nil {
		return 0, err
	}
	lower, err := r.ReadByte()
	if err != nil {
		return 0, err
	}
	return int(upper)<<8 + int(lower), nil
}

func (r *TftpReader) ReadNullTerminatedString() (string, error) {
	s := ""
	for {
		b, err := r.ReadByte()
		if err != nil {
			return "", err
		}
		if b == 0 {
			break
		}
		s += string(b)
	}
	return s, nil
}

func NewTftpReader(b []byte) *TftpReader {
	return (*TftpReader)(bytes.NewReader(b))
}

// https://datatracker.ietf.org/doc/html/rfc1350
// https://www.geeksforgeeks.org/what-is-tftp-trivial-file-transfer-protocol/
func main() {
	flag.Parse()

	if (*folder)[len(*folder)-1] != '/' {
		*folder += "/"
	}

	fmt.Printf("Serving files from %s on port %d\n", *folder, *port)

	udpServer, err := net.ListenPacket("udp", "0.0.0.0:"+strconv.Itoa(*port))
	if err != nil {
		panic(err)
	}

	defer udpServer.Close()

	transmissionInProgress := false
	hideDataMessages := false
	filename := ""
	var file *os.File

	for {
		buf := make([]byte, 1024)
		n, addr, err := udpServer.ReadFrom(buf)
		if err != nil {
			println("Error reading")
			continue
		}

		buf = buf[:n]

		msg := NewTftpReader(buf)

		msgType, err := msg.ReadInt16()
		if err != nil {
			println("Error reading msgType")
			continue
		}
		switch msgType {
		case 1: // RRQ (Read request)
			fmt.Println("Read Request: ")

			fileName, err := msg.ReadNullTerminatedString()
			if err != nil {
				println("Error reading fileName")
				continue
			}

			fmt.Printf("Filename: %s\n", fileName)

			mode, err := msg.ReadNullTerminatedString()
			if err != nil {
				println("Error reading mode")
				continue
			}

			fmt.Printf("Mode: %s\n", mode)

			if mode != "octet" {
				fmt.Printf("Unsopported Mode: %s\n", fileName)
			}

			// the rest of the message is unsupported info

			file, err := os.Open(*folder + fileName)
			if err != nil {
				println("Error opening file")
				continue
			}

			blockNumber := 1

			for {
				send := make([]byte, 512)
				n, err := file.Read(send)
				if err != nil {
					if err == io.EOF {
						break
					}
					println("Error reading file")
					continue
				}

				send = send[:n]
				send = append([]byte{0, 3, byte(blockNumber >> 8), byte(blockNumber)}, send...)

				_, err = udpServer.WriteTo(send, addr)
				if err != nil {
					println("Error sending data")
					continue
				}

				// wait for ack
				buf := make([]byte, 1024)
				n, _, err = udpServer.ReadFrom(buf)
				if err != nil {
					println("Error reading")
					continue
				}

				buf = buf[:n]
				msg := NewTftpReader(buf)
				msgType, err := msg.ReadInt16()
				if err != nil {
					println("Error reading msgType")
					continue
				}

				if msgType != 4 {
					println("Expected ack")
					continue
				}

				ackBlockNumber, err := msg.ReadInt16()
				if err != nil {
					println("Error reading ackBlockNumber")
					continue
				}

				if ackBlockNumber != blockNumber {
					println("Expected ackBlockNumber to be", blockNumber, "but got", ackBlockNumber)
					continue
				}
				blockNumber++
			}
			println("End of transmission")

		case 2: // WRQ (Write request)
			if transmissionInProgress {
				fmt.Println("Transmission already in progress")
				continue
			}

			fmt.Println("Write Request: ")

			fileName, err := msg.ReadNullTerminatedString()
			if err != nil {
				println("Error reading fileName")
				continue
			}

			fmt.Printf("Filename: %s\n", fileName)

			mode, err := msg.ReadNullTerminatedString()
			if err != nil {
				println("Error reading mode")
				continue
			}

			fmt.Printf("Mode: %s\n", mode)

			if mode != "octet" {
				fmt.Printf("Unsopported Mode: %s\n", fileName)
			}

			// the rest of the message is unsupported info

			transmissionInProgress = true
			filename = fileName
			file, err = os.Create(*folder + filename)
			if err != nil {
				println("Error creating file")
				continue
			}

			// send ack
			_, err = udpServer.WriteTo([]byte{0, 4, 0, 0}, addr)
			if err != nil {
				println("Error sending ack")
				continue
			}
		case 3: // DATA
			if !transmissionInProgress {
				fmt.Println("No transmission in progress")
				continue
			}

			if !hideDataMessages {
				fmt.Println("Data: ")
			}

			blockNum, err := msg.ReadInt16()
			if err != nil {
				println("Error reading blockNum")
				continue
			}

			if blockNum > 1000 && !hideDataMessages { // when there are over 1000 blocks, printing takes too long lol
				println("Too many blocks, hiding data messages")
				hideDataMessages = true
			}
			if blockNum%10000 == 0 {
				fmt.Printf("Block Number: %d\n", blockNum)
			}

			if !hideDataMessages {
				fmt.Printf("Block Number: %d\n", blockNum)
			}

			blockDate := make([]byte, len(buf)-4)
			i, err := msg.Read(blockDate)
			if err != nil {
				println("Error reading blockDate")
				continue
			}

			if !hideDataMessages {
				//fmt.Printf("Data: %s\n", blockDate)
				fmt.Printf("Data Length: %d\n", i)
			}

			_, err = file.Write(blockDate)
			if err != nil {
				println("Error writing to file")
				continue
			}

			if i < 512 {
				_, err := file.Seek(0, 0)
				if err != nil {
					println("Error seeking file")
					continue
				}
				fileContent, err := io.ReadAll(file)
				if err != nil {
					println("Error reading file content")
					continue
				}
				fmt.Printf("End of transmission\nFile: %s\nData:\n%s\n", filename, fileContent)
				transmissionInProgress = false
				hideDataMessages = false
			}

			// send ack
			_, err = udpServer.WriteTo([]byte{0, 4, byte(blockNum >> 8), byte(blockNum)}, addr)
			if err != nil {
				println("Error sending ack")
				continue
			}
		default:
			fmt.Println("Unsupported msgType: ", msgType)
		}
	}
}
