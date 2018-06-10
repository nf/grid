package main

import (
	"errors"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/scgolang/osc"
)

func main() {
	g, err := NewGrid(func(x, y int, down bool) {
		log.Printf("x: %d y: %d: down: %v", x, y, down)
	})
	if err != nil {
		log.Fatal(err)
	}
	for step := 0; ; step++ {
		for y := 0; y < 8; y++ {
			for i := 0; i < 16; i++ {
				c := (i + y + step) % 32
				if c > 15 {
					c = (31 - c)
				}
				g.Set(i, y, c)
			}
		}
		g.Update()
		time.Sleep(50 * time.Millisecond)
	}
}

type Grid struct {
	w, h   int
	field  []byte // indexed as [y*w+x]
	client *osc.UDPConn
}

type OnKeyFunc func(x, y int, down bool)

func (g *Grid) All(level int) {
	level %= 16
	for i := range g.field {
		g.field[i] = byte(level)
	}
}

func (g *Grid) Set(x, y, level int) {
	x %= g.w
	y %= g.h
	level %= 16
	g.field[x+y*g.w] = byte(level)
}

func (g *Grid) Update() {
	s1 := []osc.Argument{osc.Int(0), osc.Int(0)}
	s2 := []osc.Argument{osc.Int(8), osc.Int(0)}
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			i := x + y*g.w
			s1 = append(s1, osc.Int(g.field[i]))
			s2 = append(s2, osc.Int(g.field[i+8]))
		}
	}

	m := osc.Message{
		Address:   prefix + "/grid/led/level/map",
		Arguments: s1,
	}
	g.client.Send(m)
	m = osc.Message{
		Address:   prefix + "/grid/led/level/map",
		Arguments: s2,
	}
	g.client.Send(m)
}

const (
	localPort = 13002
	prefix    = "/monome"
)

func NewGrid(onKey OnKeyFunc) (*Grid, error) {
	port, err := findMonomePort()
	if err != nil {
		return nil, err
	}
	client, err := dial(fmt.Sprintf("localhost:%d", port))
	if err != nil {
		return nil, err
	}

	go func() {
		err := client.Serve(1, osc.Dispatcher{
			"/sys/id": osc.Method(func(msg osc.Message) error {
				//log.Printf("id: %v", msg.Arguments)
				return nil
			}),
			"/sys/prefix": osc.Method(func(msg osc.Message) error {
				//log.Printf("prefix: %v", msg.Arguments)
				return nil
			}),
			prefix + "/grid/key": osc.Method(func(msg osc.Message) error {
				//log.Printf("key: %v", msg.Arguments)
				x, err := msg.Arguments[0].ReadInt32()
				if err != nil {
					return err
				}
				y, err := msg.Arguments[1].ReadInt32()
				if err != nil {
					return err
				}
				s, err := msg.Arguments[2].ReadInt32()
				if err != nil {
					return err
				}
				onKey(int(x), int(y), s > 0)
				return nil
			}),
		})
		log.Printf("Serve: %v", err)
	}()

	msgs := []osc.Message{
		{
			Address: "/sys/port",
			Arguments: []osc.Argument{
				osc.Int(localPort),
			},
		},
		{
			Address: "/sys/host",
			Arguments: []osc.Argument{
				osc.String("localhost"),
			},
		},
		{
			Address: "/sys/prefix",
			Arguments: []osc.Argument{
				osc.String("/monome"),
			},
		},
	}
	for _, m := range msgs {
		if err := client.Send(m); err != nil {
			return nil, err
		}
	}

	return &Grid{
		w:     16,
		h:     8,
		field: make([]byte, 16*8),

		client: client,
	}, nil
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
	case <-time.After(1 * time.Second):
		return 0, errors.New("no monome grid detected")
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
