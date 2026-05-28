package main

import (
	"log"
	"net"
	"net/rpc"
)

type Args struct {
	Num1 int
	Num2 int
}

type Reply struct {
	Resultado int
}

type Servicio struct{}

func (s *Servicio) Suma(args *Args, reply *Reply) error {
	reply.Resultado = args.Num1 + args.Num2
	return nil
}

func main() {
	if err := rpc.Register(&Servicio{}); err != nil {
		log.Fatal(err)
	}
	listener, err := net.Listen("tcp", ":1234")
	if err != nil {
		log.Fatal(err)
	}
	rpc.Accept(listener)
}
