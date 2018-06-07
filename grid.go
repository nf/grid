package main

import (
	"fmt"
	"log"
	"net"
	"time"

	"github.com/scgolang/osc"
)

const localPort = 13002

func main() {
	port, err := findMonomePort()
	if err != nil {
		log.Fatal(err)
	}
	log.Println("port:", port)

	client, err := dial(fmt.Sprintf("localhost:%d", port))
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	const prefix = "/monome"

	errc := make(chan error)
	go func() {
		errc <- client.Serve(1, osc.Dispatcher{
			"/sys/id": osc.Method(func(msg osc.Message) error {
				log.Printf("id: %v", msg.Arguments)
				return nil
			}),
			"/sys/prefix": osc.Method(func(msg osc.Message) error {
				log.Printf("prefix: %v", msg.Arguments)
				return nil
			}),
			prefix + "/grid/key": osc.Method(func(msg osc.Message) error {
				log.Printf("key: %v", msg.Arguments)
				return nil
			}),
		})
	}()

	m := osc.Message{
		Address: "/sys/port",
		Arguments: []osc.Argument{
			osc.Int(localPort),
		},
	}
	if err := client.Send(m); err != nil {
		log.Fatal(err)
	}
	m = osc.Message{
		Address: "/sys/host",
		Arguments: []osc.Argument{
			osc.String("localhost"),
		},
	}
	if err := client.Send(m); err != nil {
		log.Fatal(err)
	}
	m = osc.Message{
		Address: "/sys/prefix",
		Arguments: []osc.Argument{
			osc.String("/monome"),
		},
	}
	if err := client.Send(m); err != nil {
		log.Fatal(err)
	}
	for i := 15; i >= 0; i-- {
		m = osc.Message{
			Address: "/monome/grid/led/level/all",
			Arguments: []osc.Argument{
				osc.Int(i),
			},
		}
		if err := client.Send(m); err != nil {
			log.Fatal(err)
		}
		time.Sleep(50 * time.Millisecond)
	}

	log.Fatal(<-errc)
}

func findMonomePort() (int, error) {
	client, err := dial("localhost:12002")
	if err != nil {
		return 0, err
	}
	defer client.Close()

	port := make(chan int)
	errc := make(chan error)
	go func() {
		errc <- client.Serve(1, osc.Dispatcher{
			"/serialosc/device": osc.Method(func(msg osc.Message) error {
				log.Printf("device: %v", msg.Arguments)
				p, err := msg.Arguments[2].ReadInt32()
				if err != nil {
					return err
				}
				port <- int(p)
				return nil
			}),
		})
	}()

	list := osc.Message{
		Address: "/serialosc/list",
		Arguments: []osc.Argument{
			osc.String("localhost"),
			osc.Int(localPort),
		},
	}
	if err := client.Send(list); err != nil {
		return 0, err
	}

	select {
	case p := <-port:
		return p, nil
	case err := <-errc:
		return 0, err
	}
}

func dial(addr string) (*osc.UDPConn, error) {
	raddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}
	laddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("localhost:%d", localPort))
	if err != nil {
		return nil, err
	}
	return osc.DialUDP("udp", laddr, raddr)
}
