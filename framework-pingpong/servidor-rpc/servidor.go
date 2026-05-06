package main 

type Args struct {
	Num1 int
	Num2 int

}

type Reply struct {
	Resultado int 
}

type Servicio struct {
}
func (s *Servicio) Suma(args *Args, reply *Reply) error {
	reply.Resultado = args.Num1 + args.Num2
	return nil
} 

func main(){
	rcp.Register(&Servicio{})
	listener, _ := net.Listen("tcp", ":1234")
	rpc.Accept(listener)

}